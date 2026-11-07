package nutrition

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/padraicbc/larder/internal/llm"
)

func TestNormalizeBarcode(t *testing.T) {
	for in, want := range map[string]string{
		"5449000000996":    "5449000000996", // EAN-13 (Coca-Cola)
		"5449 0000 0099 6": "5449000000996", // spaces ok
		"036000291452":     "0036000291452", // UPC-A -> EAN-13
		"96385074":         "96385074",      // EAN-8
	} {
		if got, ok := NormalizeBarcode(in); !ok || got != want {
			t.Errorf("%q = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "5449000000997", "123", "abcdefghijklm", "5449000000996x"} {
		if got, ok := NormalizeBarcode(bad); ok {
			t.Errorf("%q should be invalid, got %q", bad, got)
		}
	}
}

func offServer(t *testing.T, status int, body string) (*httptest.Server, *string) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if strings.Contains(r.UserAgent(), "@") {
			t.Error("user agent must not carry an email")
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &path
}

func TestLookupOpenFoodFacts(t *testing.T) {
	srv, path := offServer(t, 200, `{"status":1,"product":{"product_name":"Porridge oats","brands":"Flahavan's,Other","quantity":"1 kg",
		"ingredients_text":"Wholegrain oats","nutriments":{"energy-kcal_100g":372,"proteins_100g":"11.5","carbohydrates_100g":60,"fat_100g":8.2,"fiber_100g":9}}}`)
	got, err := LookupOpenFoodFacts(context.Background(), srv.Client(), srv.URL, "5449000000996")
	if err != nil {
		t.Fatal(err)
	}
	if *path != "/api/v2/product/5449000000996.json" {
		t.Errorf("path = %s", *path)
	}
	if got.Name != "Flahavan's Porridge oats" || got.BasisQuantity != 100 || got.BasisUnit != "g" || got.Macros.Protein != 11.5 ||
		got.Macros.Calories != 372 || got.Barcode != "5449000000996" || got.Source != "openfoodfacts" || !got.FromNutritionLabel {
		t.Errorf("got %+v", got)
	}
}

func TestLookupOpenFoodFactsEdges(t *testing.T) {
	ctx := context.Background()
	srv, _ := offServer(t, 200, `{"status":0,"status_verbose":"product not found"}`)
	if _, err := LookupOpenFoodFacts(ctx, srv.Client(), srv.URL, "96385074"); err != ErrBarcodeUnknown {
		t.Errorf("unknown product: %v", err)
	}
	srv, _ = offServer(t, 200, `{"status":1,"product":{"product_name":"Half","nutriments":{"energy-kcal_100g":100}}}`)
	if _, err := LookupOpenFoodFacts(ctx, srv.Client(), srv.URL, "96385074"); err == nil || !strings.Contains(err.Error(), "full nutrition") {
		t.Errorf("incomplete nutrition: %v", err)
	}
	srv, _ = offServer(t, 200, `{"status":1,"product":{"product_name":"Cola","quantity":"330 ml","nutriments":{"energy_100g":180,"proteins_100g":0,"carbohydrates_100g":10.6,"fat_100g":0}}}`)
	got, err := LookupOpenFoodFacts(ctx, srv.Client(), srv.URL, "96385074")
	if err != nil || got.BasisUnit != "ml" || got.Macros.Calories < 43 || got.Macros.Calories > 43.1 {
		t.Errorf("kJ fallback / ml: %+v %v", got, err)
	}
	srv, _ = offServer(t, 503, `down`)
	if _, err := LookupOpenFoodFacts(ctx, srv.Client(), srv.URL, "96385074"); err == nil {
		t.Error("server error should fail")
	}
}

func TestReadBarcodePhoto(t *testing.T) {
	images := []llm.Image{{MediaType: "image/png", Data: []byte{1}}}
	got, err := ReadBarcodePhoto(context.Background(), mealVisionReply(`{"barcode":"5449 0000 0099 6"}`), images)
	if err != nil || got != "5449000000996" {
		t.Fatalf("got %q %v", got, err)
	}
	for _, reply := range []string{`{"barcode":""}`, `{"barcode":"5449000000997"}`, `{"barcode":"12345"}`, `nope`} {
		if _, err := ReadBarcodePhoto(context.Background(), mealVisionReply(reply), images); err == nil {
			t.Errorf("accepted %s", reply)
		}
	}
	if _, err := ReadBarcodePhoto(context.Background(), llm.Mock{}, images); err == nil {
		t.Error("demo mode should say it can't read barcodes")
	}
	if _, err := ReadBarcodePhoto(context.Background(), llm.Mock{}, nil); err == nil {
		t.Error("accepted no photo")
	}
}

func TestReadIngredientsPhoto(t *testing.T) {
	images := []llm.Image{{MediaType: "image/png", Data: []byte{1}}}
	got, err := ReadIngredientsPhoto(context.Background(), mealVisionReply(`{"name":"","barcode":"5449 0000 0099 6","ingredients":" Sugar, oats, colour (E102) ","hasNutrition":false,"macros":{}}`), images)
	if err != nil || got.Name != "Scanned product" || got.Barcode != "5449000000996" || got.Ingredients != "Sugar, oats, colour (E102)" || got.HasNutrition || got.Source != "photo" {
		t.Fatalf("ingredients only: %+v %v", got, err)
	}
	got, err = ReadIngredientsPhoto(context.Background(), mealVisionReply(`{"name":"Oats","ingredients":"Oats","hasNutrition":true,"basisQuantity":100,"basisUnit":"G","macros":{"calories":370,"protein":13}}`), images)
	if err != nil || !got.HasNutrition || got.BasisUnit != "g" || got.Macros.Calories != 370 {
		t.Fatalf("with nutrition: %+v %v", got, err)
	}
	// An unusable panel is ignored, not fatal: the ingredients still rate.
	got, err = ReadIngredientsPhoto(context.Background(), mealVisionReply(`{"ingredients":"Oats","hasNutrition":true,"basisQuantity":0,"basisUnit":"g","macros":{"calories":370}}`), images)
	if err != nil || got.HasNutrition {
		t.Fatalf("bad panel: %+v %v", got, err)
	}
	for _, reply := range []string{`{"ingredients":""}`, `{"ingredients":"   "}`, `nope`} {
		if _, err := ReadIngredientsPhoto(context.Background(), mealVisionReply(reply), images); err == nil {
			t.Errorf("accepted %s", reply)
		}
	}
	if _, err := ReadIngredientsPhoto(context.Background(), llm.Mock{}, nil); err == nil {
		t.Error("accepted no photo")
	}
}

func TestOpenFoodFactsIngredientLanguage(t *testing.T) {
	ctx := context.Background()
	nutriments := `"nutriments":{"energy-kcal_100g":42,"proteins_100g":0,"carbohydrates_100g":10.6,"fat_100g":0}`
	// English wins over the default (here Spanish) text, with no warning.
	srv, path := offServer(t, 200, `{"status":1,"product":{"product_name":"Cola","ingredients_text":"Agua carbonatada, az\u00facar","ingredients_text_en":"carbonated water, sugar, colour (caramel e150d)",`+nutriments+`}}`)
	got, err := LookupOpenFoodFacts(ctx, srv.Client(), srv.URL, "5449000000996")
	if err != nil || got.Ingredients != "carbonated water, sugar, colour (caramel e150d)" || got.IngredientsWarning != "" {
		t.Fatalf("english: %+v %v", got, err)
	}
	if !strings.Contains(*path, "5449000000996") {
		t.Errorf("path %s", *path)
	}
	// Only another language: keep it, but warn that additives may be missed.
	srv, _ = offServer(t, 200, `{"status":1,"product":{"product_name":"Cola","ingredients_text":"Agua carbonatada, az\u00facar",`+nutriments+`}}`)
	got, _ = LookupOpenFoodFacts(ctx, srv.Client(), srv.URL, "5449000000996")
	if got.Ingredients == "" || !strings.Contains(got.IngredientsWarning, "another language") {
		t.Errorf("other language: %+v", got)
	}
	// Only the parsed ingredient list.
	srv, _ = offServer(t, 200, `{"status":1,"product":{"product_name":"Cola","ingredients":[{"text":"water"},{"text":"sugar"}],`+nutriments+`}}`)
	got, _ = LookupOpenFoodFacts(ctx, srv.Client(), srv.URL, "5449000000996")
	if got.Ingredients != "water, sugar" || got.IngredientsWarning == "" {
		t.Errorf("array fallback: %+v", got)
	}
	// None at all: say so, and ask for a photo.
	srv, _ = offServer(t, 200, `{"status":1,"product":{"product_name":"Cola",`+nutriments+`}}`)
	got, _ = LookupOpenFoodFacts(ctx, srv.Client(), srv.URL, "5449000000996")
	if got.Ingredients != "" || !strings.Contains(got.IngredientsWarning, "Photograph the ingredients") {
		t.Errorf("none: %+v", got)
	}
	if sc := ScanFromLabel(got); sc.Warning == "" || RateScan(sc) != nil {
		t.Errorf("a scan with no ingredients has a warning and no rating: %+v", sc)
	}
}
