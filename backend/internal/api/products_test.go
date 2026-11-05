package api

import (
	"github.com/you/nutrition-app/internal/models"
	"testing"
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
