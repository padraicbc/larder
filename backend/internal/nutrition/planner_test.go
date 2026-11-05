package nutrition

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/you/nutrition-app/internal/llm"
	"github.com/you/nutrition-app/internal/models"
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
