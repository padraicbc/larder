package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/padraicbc/larder/internal/models"
)

func TestShopScan(t *testing.T) {
	var hits int
	off := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`{"status":1,"product":{"product_name":"Cola","ingredients_text":"Carbonated water, sugar, colour (E150d), phosphoric acid","nutriments":{"energy-kcal_100g":42,"proteins_100g":0,"carbohydrates_100g":10.6,"fat_100g":0}}}`))
	}))
	defer off.Close()
	c0, s := newTestServer(t)
	s.OFFBase, s.HTTP = off.URL, off.Client()
	tick := s.Now()
	s.Now = func() time.Time { tick = tick.Add(time.Minute); return tick }
	c := signUp(t, c0.srv, "shop@example.com")

	// Unknown barcode with online lookup off: a helpful message, no request out.
	if code := c.do("GET", "/api/scan/barcode/5449000000996", nil, nil); code != 404 || hits != 0 {
		t.Fatalf("disabled = %d hits %d", code, hits)
	}
	if code := c.do("GET", "/api/scan/barcode/12345", nil, nil); code != 400 {
		t.Errorf("bad barcode = %d", code)
	}

	// One of your own products is found offline and rated.
	c.do("POST", "/api/products", models.Product{Name: "Oats", Barcode: "96385074", Ingredients: "Wholegrain oats", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true,
		Macros: models.Macros{Calories: 370, Protein: 13, Carbs: 60, Fat: 7, Fiber: 10}}, nil)
	var own models.Scan
	if code := c.do("GET", "/api/scan/barcode/96385074", nil, &own); code != 200 || own.Source != "saved" || own.ProductID == "" || own.Rating == nil || own.Rating.Score < 85 {
		t.Fatalf("saved product = %d %+v", code, own)
	}

	// Online lookup on: Open Food Facts result is rated, and not stored.
	llm := map[string]string{"provider": "mock"}
	c.do("PUT", "/api/settings", map[string]any{"llm": llm, "lookup": models.LookupSettings{OpenFoodFacts: true}}, nil)
	var sc models.Scan
	if code := c.do("GET", "/api/scan/barcode/5449000000996", nil, &sc); code != 200 || sc.Source != "openfoodfacts" || sc.Rating == nil || sc.Rating.Score >= 90 || sc.ID != "" {
		t.Fatalf("off = %d %+v", code, sc)
	}
	if sc.Warning == "" || !sc.Rating.Provisional { // the fake only has the default-language field
		t.Errorf("non-English ingredients should be flagged: %+v", sc)
	}
	// Adding the ingredients from a photo and re-rating clears the doubt.
	fixed := sc
	fixed.Warning, fixed.Ingredients = "", "Carbonated water, sugar, colour (E150d), phosphoric acid"
	var firmed models.Scan
	if code := c.do("POST", "/api/scan/rate", fixed, &firmed); code != 200 || firmed.Rating == nil || firmed.Rating.Provisional {
		t.Fatalf("rate = %d %+v", code, firmed)
	}
	if c.do("POST", "/api/scan/rate", models.Scan{Name: "x"}, nil) != 400 {
		t.Error("rating nothing should fail")
	}
	var saved []models.Scan
	c.do("GET", "/api/scans", nil, &saved)
	if len(saved) != 0 {
		t.Fatal("looking something up must not save it")
	}

	// Saving a COMPLETE scan (ingredients and nutrition) makes it a real product, not a draft.
	var kept models.Scan
	if code := c.do("POST", "/api/scans", sc, &kept); code != 201 || kept.ProductID == "" || kept.ID != "" || kept.Rating == nil {
		t.Fatalf("save = %d %+v", code, kept)
	}
	var products []models.Product
	c.do("GET", "/api/products?q=cola", nil, &products)
	if len(products) != 1 || products[0].Barcode != "5449000000996" || products[0].Source != "openfoodfacts" || products[0].Rating == nil {
		t.Fatalf("a complete scan should appear in Products: %+v", products)
	}
	c.do("GET", "/api/scans", nil, &saved)
	if len(saved) != 0 {
		t.Fatalf("complete scans are not left behind as drafts: %+v", saved)
	}
	// Saving again by barcode updates the product rather than duplicating it.
	sc.Name = "Cola zero"
	c.do("POST", "/api/scans", sc, nil)
	c.do("GET", "/api/products", nil, &products)
	if len(products) != 2 { // the oats from earlier in this test, and the cola
		t.Fatalf("barcode upsert should not duplicate: %+v", products)
	}
	// Found by barcode with online lookup off, so you can re-check it in the shop.
	c.do("PUT", "/api/settings", map[string]any{"llm": llm, "lookup": models.LookupSettings{}}, nil)
	hits = 0
	var recheck models.Scan
	if code := c.do("GET", "/api/scan/barcode/5449000000996", nil, &recheck); code != 200 || recheck.ProductID != kept.ProductID || recheck.Name != "Cola zero" || hits != 0 {
		t.Fatalf("saved lookup = %d %+v hits %d", code, recheck, hits)
	}

	// No barcode: ingredients from a photo (demo model). Alone they are not rated and stay a draft.
	var photo models.Scan
	if code := c.uploadTo("/api/scan/ingredients", map[string][]byte{"ing.png": pngBytes(t)}, &photo); code != 200 {
		t.Fatalf("ingredients photo = %d", code)
	}
	if photo.Source != "photo" || photo.HasNutrition || photo.Rating != nil || photo.Ingredients == "" {
		t.Fatalf("ingredients alone must not be rated: %+v", photo)
	}
	photo.Barcode = "4006381333931"
	var draft models.Scan
	if code := c.do("POST", "/api/scans", photo, &draft); code != 201 || draft.ID == "" || draft.ProductID != "" || draft.Rating != nil {
		t.Fatalf("an incomplete scan stays a draft: %d %+v", code, draft)
	}
	c.do("GET", "/api/scans", nil, &saved)
	if len(saved) != 1 {
		t.Fatalf("one draft expected: %+v", saved)
	}
	// Add the nutrition: it becomes a product and the draft is gone.
	draft.HasNutrition, draft.BasisQuantity, draft.BasisUnit = true, 100, "g"
	draft.Macros = models.Macros{Calories: 380, Protein: 8, Carbs: 70, Fat: 8, Fiber: 4}
	var both models.Scan
	if code := c.do("POST", "/api/scan/rate", draft, &both); code != 200 || both.Rating == nil || both.Rating.Score > 60 || len(both.Rating.Factors) == 0 {
		t.Fatalf("with nutrition added = %d %+v", code, both)
	}
	var promoted models.Scan
	if code := c.do("POST", "/api/scans", draft, &promoted); code != 201 || promoted.ProductID == "" || promoted.Rating == nil {
		t.Fatalf("completed draft = %d %+v", code, promoted)
	}
	c.do("GET", "/api/scans", nil, &saved)
	if len(saved) != 0 {
		t.Fatalf("the finished draft should be removed: %+v", saved)
	}

	// Nothing to save; negative nutrition; isolation.
	if code := c.do("POST", "/api/scans", models.Scan{Name: "Empty"}, nil); code != 400 {
		t.Errorf("empty scan = %d", code)
	}
	bad := draft
	bad.Macros.Fat = -1
	if c.do("POST", "/api/scans", bad, nil) != 400 {
		t.Error("negative nutrition accepted")
	}
	other := signUp(t, c0.srv, "shop2@example.com")
	var theirs []models.Scan
	other.do("GET", "/api/scan/search?q=cola", nil, &theirs)
	if len(theirs) != 0 {
		t.Error("search leaked across accounts")
	}
}

func TestShopSearch(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "search@example.com")
	c.do("POST", "/api/products", models.Product{Name: "Good Oats", Barcode: "96385074", Ingredients: "Wholegrain oats", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true, Macros: models.Macros{Calories: 370, Protein: 13, Fiber: 10}}, nil)
	c.do("POST", "/api/products", models.Product{Name: "Porridge pot", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true, Macros: models.Macros{Calories: 100}}, nil)
	c.do("POST", "/api/scans", models.Scan{Name: "Oat bar draft", Barcode: "5449000000996", Ingredients: "Oats, sugar", Source: "photo"}, nil) // no nutrition: a draft

	search := func(q string) []models.Scan {
		var out []models.Scan
		if code := c.do("GET", "/api/scan/search?q="+q, nil, &out); code != 200 {
			t.Fatalf("search %q = %d", q, code)
		}
		return out
	}
	if got := search("o"); len(got) != 0 {
		t.Errorf("one character is too short: %+v", got)
	}
	got := search("oat")
	if len(got) != 2 || got[0].Name != "Oat bar draft" || got[1].Name != "Good Oats" { // prefix match first
		t.Fatalf("name search: %+v", got)
	}
	if got[1].Rating == nil || got[1].ProductID == "" || got[0].Rating != nil || got[0].ID == "" {
		t.Errorf("products come rated, drafts without a rating: %+v", got)
	}
	if got := search("96385"); len(got) != 1 || got[0].Name != "Good Oats" {
		t.Errorf("barcode search: %+v", got)
	}
	if got := search("PORRIDGE"); len(got) != 1 {
		t.Errorf("search ignores case: %+v", got)
	}
	if got := search("zzz"); len(got) != 0 {
		t.Errorf("no match: %+v", got)
	}
}

func TestProductWithoutIngredientsBorrowsFromScan(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "borrow@example.com")
	// A product saved without ingredients mustn't hide ingredients photographed later for the same barcode.
	c.do("POST", "/api/products", models.Product{Name: "Bare product", Barcode: "4006381333931", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true, Macros: models.Macros{Calories: 100}}, nil)
	var bare models.Scan
	c.do("GET", "/api/scan/barcode/4006381333931", nil, &bare)
	if bare.Ingredients != "" || bare.Rating != nil {
		t.Fatalf("product without ingredients should have no rating: %+v", bare)
	}
	c.do("POST", "/api/scans", models.Scan{Name: "Bare product", Barcode: "4006381333931", Ingredients: "Oats, sugar", Source: "photo"}, nil)
	var firm models.Scan
	c.do("GET", "/api/scan/barcode/4006381333931", nil, &firm)
	if firm.Ingredients != "Oats, sugar" || firm.ProductID == "" || firm.Rating == nil || firm.Rating.Provisional {
		t.Fatalf("saved scan's ingredients should show up on the product: %+v", firm)
	}
}

func TestRatingRulesAreYours(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "rules@example.com")
	other := signUp(t, c0.srv, "rules2@example.com")
	p := models.Product{Name: "Biscuits", Ingredients: "Flour, sugar, palm oil, salt, cocoa", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true, Macros: models.Macros{Calories: 400, Protein: 6, Fiber: 3}}
	c.do("POST", "/api/products", p, nil)
	other.do("POST", "/api/products", p, nil)
	score := func(cl *client) int {
		var list []models.Product
		cl.do("GET", "/api/products", nil, &list)
		return list[0].Rating.Score
	}
	before := score(c)

	type view struct {
		Settings  models.RatingSettings `json:"settings"`
		Catalogue []struct {
			Key     string `json:"key"`
			Default int    `json:"default"`
		} `json:"catalogue"`
		Rules int `json:"rules"`
	}
	var v view
	if c.do("GET", "/api/rating-settings", nil, &v) != 200 || len(v.Settings.Points) != 0 || len(v.Catalogue) < 30 || v.Rules == 0 {
		t.Fatalf("defaults view = %+v", v)
	}

	// Palm oil matters more; "cocoa" is penalised; a value equal to its default is not remembered.
	body := models.RatingSettings{Points: map[string]int{"palm oil": -15, "sugar-early": -12}, Custom: []models.CustomRule{{Match: " Cocoa ", Points: -5}, {Match: "cocoa", Points: -9}}}
	if code := c.do("PUT", "/api/rating-settings", body, &v); code != 200 {
		t.Fatalf("put = %d", code)
	}
	if _, kept := v.Settings.Points["sugar-early"]; kept || v.Settings.Points["palm oil"] != -15 || len(v.Settings.Custom) != 1 || v.Settings.Custom[0].Match != "Cocoa" {
		t.Errorf("only real changes are kept, duplicates dropped: %+v", v.Settings)
	}
	if after := score(c); after != before-12-5 {
		t.Errorf("score should follow the new rules straight away: before %d after %d", before, after)
	}
	if score(other) != before {
		t.Error("another person's ratings must not change")
	}

	for _, bad := range []models.RatingSettings{
		{Points: map[string]int{"made-up-rule": -3}},
		{Points: map[string]int{"palm oil": -99}},
		{Points: map[string]int{"base": 120}},
		{Custom: []models.CustomRule{{Match: "x", Points: -3}}},
		{Custom: []models.CustomRule{{Match: "cocoa", Points: 0}}},
	} {
		if code := c.do("PUT", "/api/rating-settings", bad, nil); code != 400 {
			t.Errorf("%+v = %d", bad, code)
		}
	}
	if score(c) != before-12-5 {
		t.Error("a rejected change must leave the rules as they were")
	}

	// Reset to the defaults.
	var reset view // a fresh value: decoding into the old one would keep its old map
	c.do("PUT", "/api/rating-settings", models.RatingSettings{}, &reset)
	if score(c) != before || len(reset.Settings.Points) != 0 || len(reset.Settings.Custom) != 0 {
		t.Errorf("reset: %d vs %d, %+v", score(c), before, reset.Settings)
	}
}
