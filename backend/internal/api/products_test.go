package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/padraicbc/larder/internal/models"
)

func TestSavedProducts(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "products@example.com")
	p := models.Product{Name: "My porridge oats", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true, Macros: models.Macros{Calories: 379.5, Protein: 13.2, Carbs: 60, Fat: 7, Fiber: 10}}
	var saved models.Product
	if code := c.do("POST", "/api/products", p, &saved); code != 200 {
		t.Fatal(code)
	}
	if saved.ID == "" || saved.Macros.Calories != 379.5 || saved.BasisQuantity != 100 {
		t.Fatalf("saved = %+v", saved)
	}
	var products []models.Product
	c.do("GET", "/api/products?q=PORRIDGE", nil, &products)
	if len(products) != 1 || products[0].ID != saved.ID {
		t.Fatalf("lookup = %+v", products)
	}
	var day dayLog
	c.do("GET", "/api/log", nil, &day)
	if len(day.Entries) != 0 {
		t.Fatal("saving a product logged a meal")
	}
	// Rescanning the same product name updates its values without duplicating it.
	p.Name = "my porridge oats"
	p.Macros.Calories = 380
	var updated models.Product
	c.do("POST", "/api/products", p, &updated)
	if updated.ID != saved.ID || updated.Macros.Calories != 380 {
		t.Fatalf("upsert = %+v", updated)
	}
	p.ID = saved.ID
	p.Name = "Brand-name oats"
	c.do("POST", "/api/products", p, &updated)
	c.do("GET", "/api/products", nil, &products)
	if len(products) != 1 || products[0].Name != "Brand-name oats" {
		t.Fatalf("rename = %+v", products)
	}
	stranger := signUp(t, c0.srv, "other-products@example.com")
	stranger.do("GET", "/api/products", nil, &products)
	if len(products) != 0 {
		t.Fatal("products visible across accounts")
	}
	if code := stranger.do("POST", "/api/products", p, nil); code != 404 {
		t.Errorf("cross-user update = %d", code)
	}
	if code := stranger.do("DELETE", "/api/products/"+saved.ID, nil, nil); code != 404 {
		t.Errorf("cross-user delete = %d", code)
	}
	for _, invalid := range []models.Product{{Name: "", BasisQuantity: 100, BasisUnit: "g"}, {Name: "Bad", BasisQuantity: 0, BasisUnit: "g"}, {Name: "Bad", BasisQuantity: 100, BasisUnit: "unknown"}, {Name: "Bad", BasisQuantity: 100, BasisUnit: "g", Macros: models.Macros{Calories: -1}}} {
		if code := c.do("POST", "/api/products", invalid, nil); code != 400 {
			t.Errorf("invalid product = %d", code)
		}
	}
	if code := c.do("DELETE", "/api/products/"+saved.ID, nil, nil); code != 204 {
		t.Errorf("delete = %d", code)
	}
}

func TestBarcodeProducts(t *testing.T) {
	var hits int
	off := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`{"status":1,"product":{"product_name":"Cola","nutriments":{"energy-kcal_100g":42,"proteins_100g":0,"carbohydrates_100g":10.6,"fat_100g":0}}}`))
	}))
	defer off.Close()
	c0, s := newTestServer(t)
	s.OFFBase, s.HTTP = off.URL, off.Client()
	c := signUp(t, c0.srv, "admin@example.com")
	type resp struct {
		Source  string         `json:"source"`
		Product models.Product `json:"product"`
	}
	var r resp

	// Off by default: nothing leaves the server.
	if code := c.do("GET", "/api/products/lookup/5449000000996", nil, nil); code != 404 || hits != 0 {
		t.Fatalf("disabled lookup = %d, hits %d", code, hits)
	}
	if code := c.do("GET", "/api/products/lookup/5449000000997", nil, nil); code != 400 {
		t.Errorf("bad checksum = %d", code)
	}
	var settings struct {
		Lookup models.LookupSettings `json:"lookup"`
	}
	c.do("GET", "/api/settings", nil, &settings)
	if settings.Lookup.OpenFoodFacts {
		t.Fatal("lookup should default to off")
	}
	llmCfg := map[string]string{"provider": "mock"}
	if code := c.do("PUT", "/api/settings", map[string]any{"llm": llmCfg, "lookup": models.LookupSettings{OpenFoodFacts: true}}, &settings); code != 200 || !settings.Lookup.OpenFoodFacts {
		t.Fatalf("enable = %d %+v", code, settings)
	}

	if code := c.do("GET", "/api/products/lookup/5449000000996", nil, &r); code != 200 || r.Source != "openfoodfacts" || r.Product.Name != "Cola" || hits != 1 {
		t.Fatalf("external lookup = %d %+v hits %d", code, r, hits)
	}
	// Save it (UPC-A style input with spaces is normalised), then it is found locally with no more requests.
	r.Product.Barcode = "5449 0000 0099 6"
	var saved models.Product
	if code := c.do("POST", "/api/products", r.Product, &saved); code != 200 || saved.Barcode != "5449000000996" || saved.Source != "openfoodfacts" {
		t.Fatalf("save = %d %+v", code, saved)
	}
	if code := c.do("GET", "/api/products/lookup/5449000000996", nil, &r); code != 200 || r.Source != "saved" || r.Product.ID != saved.ID || r.Product.Rating != nil || hits != 1 { // the OFF record had no ingredients, so no rating
		t.Fatalf("saved lookup = %d %+v hits %d", code, r, hits)
	}
	// Same barcode under a new name updates, not duplicates.
	again := saved
	again.ID, again.Name = "", "Coca-Cola"
	c.do("POST", "/api/products", again, nil)
	var all []models.Product
	c.do("GET", "/api/products?q=544900", nil, &all)
	if len(all) != 1 || all[0].Name != "Coca-Cola" {
		t.Errorf("barcode upsert/search = %+v", all)
	}
	bad := saved
	bad.ID, bad.Barcode = "", "123"
	if code := c.do("POST", "/api/products", bad, nil); code != 400 {
		t.Errorf("bad barcode save = %d", code)
	}
	// Another account has its own products; only the admin can flip the setting.
	other := signUp(t, c0.srv, "other@example.com")
	if code := other.do("PUT", "/api/settings", map[string]any{"llm": llmCfg, "lookup": models.LookupSettings{}}, nil); code != 403 {
		t.Errorf("non-admin settings = %d", code)
	}
	if code := other.do("GET", "/api/products/lookup/5449000000996", nil, &r); code != 200 || r.Source != "openfoodfacts" {
		t.Errorf("other user should get external, not %+v (%d)", r, code)
	}
}

func TestSetProductIngredients(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "ingredients@example.com")
	var p models.Product
	c.do("POST", "/api/products", models.Product{Name: "Cola", BasisQuantity: 100, BasisUnit: "ml", FromNutritionLabel: true, Notes: "keep me", Macros: models.Macros{Calories: 42, Carbs: 10.6}}, &p)
	var list []models.Product
	c.do("GET", "/api/products", nil, &list)
	if list[0].Rating != nil {
		t.Fatalf("a product without ingredients gets no rating at all: %+v", list[0].Rating)
	}
	var got models.Product
	if code := c.do("PUT", "/api/products/"+p.ID+"/ingredients", map[string]string{"ingredients": " Carbonated water, sugar, colour (E150d) "}, &got); code != 200 {
		t.Fatal(code)
	}
	if got.Ingredients != "Carbonated water, sugar, colour (E150d)" || got.Notes != "keep me" || got.Macros.Calories != 42 || got.Rating == nil || got.Rating.Provisional { // now it has both ingredients and nutrition
		t.Errorf("ingredients should be added without touching anything else: %+v", got)
	}
	if c.do("PUT", "/api/products/"+p.ID+"/ingredients", map[string]string{"ingredients": "  "}, nil) != 400 {
		t.Error("empty ingredients accepted")
	}
	other := signUp(t, c0.srv, "ingredients2@example.com")
	if other.do("PUT", "/api/products/"+p.ID+"/ingredients", map[string]string{"ingredients": "x"}, nil) != 404 {
		t.Error("another account could edit this product")
	}
}
