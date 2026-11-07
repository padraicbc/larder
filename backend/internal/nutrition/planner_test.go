package nutrition

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
)

// scripted is a fake model that returns replies in order and records prompts.
type scripted struct {
	replies []string
	prompts []string
}

func (s *scripted) Complete(_ context.Context, system, user string) (string, error) {
	s.prompts = append(s.prompts, user)
	if len(s.replies) == 0 {
		return "", errors.New("no more replies")
	}
	r := s.replies[0]
	s.replies = s.replies[1:]
	return r, nil
}

const chickenPlan = `{"title":"t","days":[{"day":1,"meals":[{"title":"Chicken bowl","servings":1,"ingredients":[{"name":"chicken breast","quantity":200,"unit":"g"}],"macros":{"calories":500}}]}]}`
const tofuPlan = `{"title":"t","days":[{"day":1,"meals":[{"title":"Tofu bowl","mealType":"Dinner","ingredients":[{"name":"tofu","quantity":200,"unit":"g"}],"macros":{"calories":450}}]}]}`

var veg = models.Profile{Diets: []string{"vegetarian"}, MealsPerDay: 3}

func TestGeneratePlanRetriesOnViolation(t *testing.T) {
	m := &scripted{replies: []string{chickenPlan, tofuPlan}}
	plan, err := GeneratePlan(context.Background(), m, veg, Feedback{}, nil, PlanRequest{Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.prompts) != 2 {
		t.Fatalf("want a retry (2 calls), got %d", len(m.prompts))
	}
	if !strings.Contains(m.prompts[1], "BROKE HARD CONSTRAINTS") || !strings.Contains(m.prompts[1], "chicken") {
		t.Errorf("retry prompt should explain the violation:\n%s", m.prompts[1])
	}
	meal := plan.Days[0].Meals[0]
	if meal.Title != "Tofu bowl" || len(meal.Warnings) != 0 {
		t.Errorf("want clean tofu bowl, got %+v", meal)
	}
	if meal.Servings != 1 || meal.MealType != "dinner" || meal.Ingredients[0].Category != "other" {
		t.Errorf("normalize didn't fill defaults: %+v", meal)
	}
}

func TestGeneratePlanKeepsWarningsWhenModelWontComply(t *testing.T) {
	m := &scripted{replies: []string{chickenPlan, chickenPlan}}
	plan, err := GeneratePlan(context.Background(), m, veg, Feedback{}, nil, PlanRequest{Days: 1})
	if err != nil {
		t.Fatal(err)
	}
	if w := plan.Days[0].Meals[0].Warnings; len(w) == 0 {
		t.Error("expected warnings on the stubborn recipe")
	}
}

func TestGeneratePlanErrors(t *testing.T) {
	ctx := context.Background()
	if _, err := GeneratePlan(ctx, &scripted{}, veg, Feedback{}, nil, PlanRequest{Days: 8}); err == nil {
		t.Error("8 days should be rejected")
	}
	m := &scripted{replies: []string{"sorry, I can't", "still no"}}
	if _, err := GeneratePlan(ctx, m, veg, Feedback{}, nil, PlanRequest{Days: 1}); err == nil {
		t.Error("non-JSON reply should error")
	}
	m = &scripted{replies: []string{`{"days":[]}`, `{"days":[]}`}}
	if _, err := GeneratePlan(ctx, m, veg, Feedback{}, nil, PlanRequest{Days: 1}); err == nil {
		t.Error("empty plan should error after retry")
	}
}

func TestPlanPromptIncludesEverything(t *testing.T) {
	p := models.Profile{
		Diets: []string{"vegan"}, Excluded: []string{"olives"}, Allergies: []string{"sesame"},
		Cuisines: []string{"Thai"}, MealsPerDay: 4, MaxPrepMins: 30,
		Targets: models.Macros{Calories: 2100, Protein: 140},
	}
	fb := Feedback{RatingCount: 1, Loved: []RatedRecipe{{Title: "Pad thai", Avg: 5}}}
	_, user := BuildPlanPrompt(p, fb, []string{"Old soup"}, PlanRequest{Days: 5, Notes: "cheap lunches"})
	for _, want := range []string{"TASK: meal_plan", "DAYS: 5", "MEALS_PER_DAY: 4", "2100 kcal", "140g protein",
		"vegan", "olives", "sesame", "Thai", "30 minutes", "Pad thai", "Old soup", "cheap lunches", "snacks"} {
		if !strings.Contains(user, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

// The offline mock should produce a plan that matches the requested shape.
func TestGeneratePlanWithMock(t *testing.T) {
	p := models.Profile{MealsPerDay: 4}
	plan, err := GeneratePlan(context.Background(), llm.Mock{}, p, Feedback{}, nil, PlanRequest{Days: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Days) != 2 || len(plan.Days[1].Meals) != 4 {
		t.Errorf("mock plan shape: %d days, %d meals", len(plan.Days), len(plan.Days[1].Meals))
	}
}

func TestEstimateMacros(t *testing.T) {
	m := &scripted{replies: []string{"```json\n{\"name\":\"Toast\",\"macros\":{\"calories\":180,\"protein\":6}}\n```"}}
	e, err := EstimateMacros(context.Background(), m, "two slices of toast")
	if err != nil || e.Name != "Toast" || e.Macros.Calories != 180 {
		t.Errorf("got %+v, %v", e, err)
	}
	if _, err := EstimateMacros(context.Background(), m, "   "); err == nil {
		t.Error("blank text should error")
	}
}

var oats = models.FixedMeal{
	MealType: "Breakfast", Title: "Porridge with berries",
	Ingredients: []models.Ingredient{{Name: "oats", Quantity: 60, Unit: "g"}},
	Macros:      models.Macros{Calories: 400, Protein: 15},
}

func TestFixedMealsAddedToEveryDay(t *testing.T) {
	p := models.Profile{MealsPerDay: 3, Targets: models.Macros{Calories: 2000}}
	plan, err := GeneratePlan(context.Background(), llm.Mock{}, p, Feedback{}, nil, PlanRequest{Days: 2, FixedMeals: []models.FixedMeal{oats}})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range plan.Days {
		if len(d.Meals) != 3 {
			t.Fatalf("day %d: want 3 meals (1 fixed + 2 planned), got %d", d.Day, len(d.Meals))
		}
		if d.Meals[0].Title != "Porridge with berries" || d.Meals[0].MealType != "breakfast" || d.Meals[0].Servings != 1 {
			t.Errorf("day %d: breakfast not first/normalised: %+v", d.Day, d.Meals[0])
		}
	}
}

func TestFixedMealsPromptAndSafety(t *testing.T) {
	p := models.Profile{MealsPerDay: 3, Targets: models.Macros{Calories: 2000}, Allergies: []string{"oats"}}
	_, user := BuildPlanPrompt(p, Feedback{}, nil, PlanRequest{Days: 1, FixedMeals: []models.FixedMeal{oats}})
	for _, want := range []string{"MEALS_PER_DAY: 2", "Porridge with berries", "1600 kcal"} {
		if !strings.Contains(user, want) {
			t.Errorf("prompt missing %q:\n%s", want, user)
		}
	}
	p = models.Profile{MealsPerDay: 1, Allergies: []string{"oats"}}
	plan, _ := GeneratePlan(context.Background(), llm.Mock{}, p, Feedback{}, nil, PlanRequest{Days: 1, FixedMeals: []models.FixedMeal{oats}})
	if len(plan.Days) != 1 || len(plan.Days[0].Meals) != 1 || len(plan.Days[0].Meals[0].Warnings) == 0 {
		t.Errorf("all-fixed day should skip the model and still flag allergens: %+v", plan)
	}
}

func TestParseQuantityAndFixedFromSaved(t *testing.T) {
	for in, want := range map[string][2]any{"75 g": {75.0, "g"}, "75g": {75.0, "g"}, "2": {2.0, ""}, "2 biscuits": {2.0, ""}, "1 scoop": {1.0, ""}, "1.5 cups": {1.5, "cup"}, "": {1.0, ""}, "a bit": {1.0, ""}} {
		if q, u := ParseQuantity(in); q != want[0] || u != want[1] {
			t.Errorf("%q = %v %q, want %v", in, q, u, want)
		}
	}
	f := FixedFromSaved(models.SavedMeal{Name: "Usual breakfast", Items: []models.MealItem{{Name: "Weetabix", Quantity: "2 biscuits"}, {Name: "Porridge oats", Quantity: "75 g"}}, Macros: models.Macros{Calories: 600}}, "breakfast")
	if f.Title != "Usual breakfast" || len(f.Ingredients) != 2 || f.Ingredients[1].Quantity != 75 || f.Ingredients[1].Unit != "g" || f.Macros.Calories != 600 {
		t.Errorf("%+v", f)
	}
}
