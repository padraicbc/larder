package api

import (
	"testing"

	"github.com/padraicbc/larder/internal/models"
)

func TestGroupAndReuseMeal(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "meals@example.com")
	var ids []string
	for _, f := range []struct {
		name, qty string
		kcal      float64
	}{{"Weetabix", "2 biscuits", 270}, {"Porridge oats", "75 g", 280}, {"Protein powder", "1 scoop", 120}} {
		var e models.LogEntry
		c.do("POST", "/api/log", map[string]any{"date": "2026-03-10", "mealType": "breakfast", "name": f.name, "macros": models.Macros{Calories: f.kcal, Protein: 10},
			"items": []models.MealItem{{Name: f.name, Quantity: f.qty}}}, &e)
		ids = append(ids, e.ID)
	}

	var res struct {
		Meal  models.SavedMeal `json:"meal"`
		Entry *models.LogEntry `json:"entry"`
	}
	for _, bad := range []map[string]any{{"name": "", "entryIds": ids}, {"name": "x", "entryIds": []string{}}, {"name": "x", "entryIds": []string{"nope"}}, {"name": "x", "mealType": "brunch", "entryIds": ids}} {
		if code := c.do("POST", "/api/meals/group", bad, nil); code < 400 {
			t.Errorf("%v should fail, got %d", bad, code)
		}
	}
	if code := c.do("POST", "/api/meals/group", map[string]any{"name": "Usual breakfast", "entryIds": ids, "merge": true}, &res); code != 201 {
		t.Fatal(code)
	}
	if res.Meal.Macros.Calories != 670 || res.Meal.MealType != "breakfast" || len(res.Meal.Items) != 3 || res.Entry == nil {
		t.Fatalf("meal = %+v entry %+v", res.Meal, res.Entry)
	}
	var day dayLog
	c.do("GET", "/api/log?date=2026-03-10", nil, &day)
	if len(day.Entries) != 1 || day.Entries[0].Name != "Usual breakfast" || day.Entries[0].Macros.Calories != 670 {
		t.Fatalf("merge should leave one combined entry: %+v", day.Entries)
	}

	// Reuse on another day/slot, with 2 servings.
	var e models.LogEntry
	if code := c.do("POST", "/api/meals/"+res.Meal.ID+"/log", map[string]any{"date": "2026-03-11", "mealType": "snack", "servings": 2}, &e); code != 201 {
		t.Fatal(code)
	}
	if e.Macros.Calories != 1340 || e.MealType != "snack" || e.Name != "Usual breakfast" || len(e.Items) != 3 {
		t.Errorf("logged = %+v", e)
	}

	// Rename, duplicate-name conflict, isolation, delete.
	var m models.SavedMeal
	if code := c.do("PUT", "/api/meals/"+res.Meal.ID, map[string]any{"name": "Gym breakfast", "mealType": "breakfast"}, &m); code != 200 || m.Name != "Gym breakfast" {
		t.Errorf("rename = %d %+v", code, m)
	}
	var other models.SavedMeal
	c.do("POST", "/api/log", map[string]any{"date": "2026-03-10", "mealType": "lunch", "name": "Soup", "macros": models.Macros{Calories: 200}}, &e)
	var res2 struct {
		Meal models.SavedMeal `json:"meal"`
	}
	c.do("POST", "/api/meals/group", map[string]any{"name": "Soup lunch", "entryIds": []string{e.ID}}, &res2)
	other = res2.Meal
	if code := c.do("PUT", "/api/meals/"+other.ID, map[string]any{"name": "gym breakfast", "mealType": "lunch"}, nil); code != 409 {
		t.Errorf("duplicate name = %d", code)
	}
	stranger := signUp(t, c0.srv, "stranger@example.com")
	var list []models.SavedMeal
	stranger.do("GET", "/api/meals", nil, &list)
	if len(list) != 0 || stranger.do("DELETE", "/api/meals/"+m.ID, nil, nil) != 404 || stranger.do("POST", "/api/meals/"+m.ID+"/log", map[string]any{}, nil) != 404 {
		t.Error("meals leaked across accounts")
	}
	c.do("GET", "/api/meals", nil, &list)
	if len(list) != 2 || list[0].Name != "Gym breakfast" {
		t.Errorf("list = %+v", list)
	}
	if code := c.do("DELETE", "/api/meals/"+m.ID, nil, nil); code != 204 {
		t.Errorf("delete = %d", code)
	}
}

func TestPlanWithFixedMeal(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "fixed@example.com")
	var e models.LogEntry
	c.do("POST", "/api/log", map[string]any{"date": "2026-03-10", "mealType": "breakfast", "name": "Oats", "macros": models.Macros{Calories: 400},
		"items": []models.MealItem{{Name: "Porridge oats", Quantity: "75 g"}}}, &e)
	var g struct {
		Meal models.SavedMeal `json:"meal"`
	}
	c.do("POST", "/api/meals/group", map[string]any{"name": "Usual breakfast", "entryIds": []string{e.ID}}, &g)

	if code := c.do("POST", "/api/plans", map[string]any{"days": 2, "fixed": map[string]string{"breakfast": "missing"}}, nil); code != 400 {
		t.Errorf("unknown meal = %d", code)
	}
	if code := c.do("POST", "/api/plans", map[string]any{"days": 2, "fixed": map[string]string{"brunch": g.Meal.ID}}, nil); code != 400 {
		t.Errorf("bad slot = %d", code)
	}
	var plan struct {
		Plan    models.MealPlan          `json:"plan"`
		Recipes map[string]models.Recipe `json:"recipes"`
	}
	if code := c.do("POST", "/api/plans", map[string]any{"days": 2, "fixed": map[string]string{"breakfast": g.Meal.ID}}, &plan); code != 201 {
		t.Fatal(code)
	}
	var first string
	for _, d := range plan.Plan.Days {
		r := plan.Recipes[d.RecipeIDs[0]]
		if r.Title != "Usual breakfast" || r.MealType != "breakfast" {
			t.Errorf("day %d starts with %+v", d.Day, r)
		}
		if first == "" {
			first = r.ID
		} else if r.ID != first {
			t.Error("the fixed meal should be one shared recipe across days")
		}
	}
	var list models.ShoppingList
	c.do("GET", "/api/plans/"+plan.Plan.ID+"/shopping", nil, &list)
	found := false
	for _, it := range list.Items {
		found = found || (it.Name == "porridge oat" && it.Unit == "g" && it.Quantity >= 150)
	}
	if !found {
		t.Errorf("shopping list should include 2 days of oats: %+v", list.Items)
	}
}
