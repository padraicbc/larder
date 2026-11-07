package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
)

type replacementModel struct{ reply string }

func (m replacementModel) Complete(context.Context, string, string) (string, error) {
	return m.reply, nil
}

func TestReplaceOnlySelectedPlanMeal(t *testing.T) {
	c0, s := newTestServer(t)
	c := signUp(t, c0.srv, "replace@example.com")
	user, _ := s.Store.UserByEmail("replace@example.com")
	rs := []models.Recipe{
		{ID: "breakfast", Title: "Porridge", MealType: "breakfast", Macros: models.Macros{Calories: 600, Protein: 40, Carbs: 80, Fat: 20, Fiber: 10}},
		{ID: "lunch", Title: "Soup", MealType: "lunch", Macros: models.Macros{Calories: 800, Protein: 40, Carbs: 80, Fat: 30, Fiber: 10}},
		{ID: "dinner", Title: "Old dinner", MealType: "dinner", Servings: 1, Ingredients: []models.Ingredient{{Name: "old food", Quantity: 1}}, Macros: models.Macros{Calories: 600, Protein: 40, Carbs: 60, Fat: 20, Fiber: 10}},
	}
	s.Store.PutRecipes(user.ID, rs...)
	plan := models.MealPlan{ID: "plan", Days: []models.PlanDay{{Day: 1, RecipeIDs: []string{"breakfast", "lunch", "dinner"}}, {Day: 2, RecipeIDs: []string{"dinner"}}}}
	s.Store.PutPlan(user.ID, plan, models.ShoppingList{PlanID: "plan", Items: []models.ShoppingItem{{Key: "lentil|g", Name: "lentil", Checked: true}, {Key: "manual", Name: "Coffee", Manual: true}}})
	next := models.Recipe{Title: "Lentil stew", MealType: "dinner", Servings: 1, Ingredients: []models.Ingredient{{Name: "lentils", Quantity: 200, Unit: "g"}}, Steps: []string{"Cook lentils"}, Macros: rs[2].Macros}
	reply, _ := json.Marshal(map[string]any{"recipe": next})
	s.NewLLM = func(llm.Config) (llm.Client, error) { return replacementModel{string(reply)}, nil }
	body := map[string]any{"day": 1, "index": 2, "recipeId": "dinner"}
	var out struct {
		Plan    models.MealPlan          `json:"plan"`
		Recipes map[string]models.Recipe `json:"recipes"`
	}
	if code := c.do("POST", "/api/plans/plan/replace-meal", body, &out); code != 200 {
		t.Fatalf("replace = %d", code)
	}
	ids := out.Plan.Days[0].RecipeIDs
	if ids[0] != "breakfast" || ids[1] != "lunch" || ids[2] == "dinner" || out.Plan.Days[1].RecipeIDs[0] != "dinner" || out.Recipes[ids[2]].Title != "Lentil stew" {
		t.Fatalf("wrong meals changed: %+v", out)
	}
	list, _ := s.Store.Shopping(user.ID, "plan")
	var manual, checked, old bool
	for _, it := range list.Items {
		manual = manual || it.Manual
		checked = checked || (it.Name == "lentil" && it.Checked)
		old = old || it.Name == "old food"
	}
	if !manual || !checked || !old {
		t.Fatalf("shopping state lost: %+v", list)
	}
	if _, err := s.Store.Recipe(user.ID, "dinner"); err != nil {
		t.Fatal("original recipe removed")
	}
	if code := c.do("POST", "/api/plans/plan/replace-meal", body, nil); code != 409 {
		t.Errorf("stale replacement = %d", code)
	}
	stranger := signUp(t, c0.srv, "stranger@example.com")
	if code := stranger.do("POST", "/api/plans/plan/replace-meal", body, nil); code != 404 {
		t.Errorf("cross-user replacement = %d", code)
	}
	// Rejected nutrition must leave the plan unchanged.
	next.Macros.Calories = 3000
	reply, _ = json.Marshal(map[string]any{"recipe": next})
	body["recipeId"] = ids[2]
	if code := c.do("POST", "/api/plans/plan/replace-meal", body, nil); code != 502 {
		t.Errorf("bad macros = %d", code)
	}
	current, _ := s.Store.Plan(user.ID, "plan")
	if current.Days[0].RecipeIDs[2] != ids[2] {
		t.Fatal("failed generation changed the plan")
	}
}
