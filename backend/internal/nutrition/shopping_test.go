package nutrition

import (
	"testing"

	"github.com/padraicbc/larder/internal/models"
)

func findItem(items []models.ShoppingItem, name string) (models.ShoppingItem, bool) {
	for _, it := range items {
		if it.Name == name {
			return it, true
		}
	}
	return models.ShoppingItem{}, false
}

func TestBuildShoppingList(t *testing.T) {
	recipes := map[string]models.Recipe{
		"oats": {ID: "oats", Servings: 1, Ingredients: []models.Ingredient{
			{Name: "rolled oats", Quantity: 60, Unit: "g", Category: "grains"},
			{Name: "Bananas", Quantity: 1, Category: "produce"},
			{Name: "oat milk", Quantity: 1, Unit: "cup", Category: "dairy"},
		}},
		"curry": {ID: "curry", Servings: 2, Ingredients: []models.Ingredient{
			{Name: "oat milk", Quantity: 60, Unit: "ml", Category: "dairy"},
			{Name: "banana", Quantity: 2, Category: "produce"},
			{Name: "chickpeas", Quantity: 1, Unit: "kg", Category: "pantry"},
		}},
	}
	plan := models.MealPlan{Days: []models.PlanDay{
		{Day: 1, RecipeIDs: []string{"oats", "curry"}},
		{Day: 2, RecipeIDs: []string{"oats", "missing-recipe"}},
	}}

	items := BuildShoppingList(plan, recipes, 2) // household of 2

	// oats: 2 days × 60 g × 2 people = 240 g
	if it, _ := findItem(items, "rolled oat"); it.Quantity != 240 || it.Unit != "g" {
		t.Errorf("oats = %+v", it)
	}
	// bananas merge across plural/case: 2×1×2 + 2×(2/2) = 6
	if it, _ := findItem(items, "banana"); it.Quantity != 6 {
		t.Errorf("banana = %+v", it)
	}
	// oat milk: 2 × 240 ml × 2 + 60 ml × (2/2) = 1020 ml -> 1.02 l
	if it, _ := findItem(items, "oat milk"); it.Quantity != 1.02 || it.Unit != "l" {
		t.Errorf("oat milk = %+v", it)
	}
	// chickpeas: 1 kg × (2/2) = 1 kg
	if it, _ := findItem(items, "chickpea"); it.Quantity != 1 || it.Unit != "kg" {
		t.Errorf("chickpeas = %+v", it)
	}
	// produce comes before pantry
	if items[0].Category != "produce" {
		t.Errorf("first item should be produce, got %+v", items[0])
	}
}

func TestMergeChecked(t *testing.T) {
	old := []models.ShoppingItem{
		{Key: "rice|g", Name: "rice", Checked: true},
		{Key: "manual|1", Name: "coffee", Manual: true, Category: "other"},
	}
	fresh := []models.ShoppingItem{{Key: "rice|g", Name: "rice"}, {Key: "lime|", Name: "lime"}}
	got := MergeChecked(old, fresh)
	if len(got) != 3 {
		t.Fatalf("want 3 items, got %+v", got)
	}
	if it, _ := findItem(got, "rice"); !it.Checked {
		t.Error("rice should stay checked")
	}
	if _, ok := findItem(got, "coffee"); !ok {
		t.Error("manual item should survive a rebuild")
	}
}
