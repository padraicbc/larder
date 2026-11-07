package nutrition

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
)

// Replacement targets account for meals the user is keeping. A replacement
// may not worsen an already off-target day beyond the normal 10% tolerance.
func GenerateReplacement(ctx context.Context, c llm.Client, p models.Profile, fb Feedback, old models.Recipe, kept []models.Recipe, notes string) (models.Recipe, error) {
	_, constraints := BuildPlanPrompt(p, fb, []string{old.Title}, PlanRequest{Days: 1, Notes: notes})
	constraints = constraints[strings.Index(constraints, "Daily targets"):strings.Index(constraints, "Return JSON exactly")]
	original, _ := json.Marshal(old)
	existing, _ := json.Marshal(kept)
	target, _ := json.Marshal(p.Targets)
	user := "TASK: replace_meal\nORIGINAL_MEAL: " + string(original) + "\nReplace only the " + old.MealType + " meal named " + old.Title + ". Return a clearly different recipe.\n" +
		"Meals being kept unchanged (one serving each): " + string(existing) + "\nDaily macro targets: " + string(target) + "\n" +
		"Choose portions so the kept meals PLUS one serving of your replacement meet the daily targets. If a target is already exceeded, do not make that deviation worse.\n" +
		"The following profile constraints and preferences apply:\n" + constraints +
		"\nOVERRIDE OUTPUT SHAPE: return ONLY {\"recipe\": {\"title\":\"...\",\"description\":\"...\",\"mealType\":\"" + old.MealType + "\",\"servings\":1,\"prepMinutes\":20,\"ingredients\":[{\"name\":\"...\",\"quantity\":100,\"unit\":\"g\",\"category\":\"produce\"}],\"steps\":[\"...\"],\"macros\":{\"calories\":0,\"protein\":0,\"carbs\":0,\"fat\":0,\"fiber\":0},\"tags\":[]}}. Macros are per serving, ingredients cover the stated servings."
	var problems []string
	for attempt := 0; attempt < MaxAttempts; attempt++ {
		var out struct {
			Recipe models.Recipe `json:"recipe"`
		}
		if err := llm.CompleteJSON(ctx, c, planSystem, user, &out); err != nil {
			return models.Recipe{}, err
		}
		r := out.Recipe
		problems = CheckRecipe(r, p)
		if strings.TrimSpace(r.Title) == "" || len(r.Ingredients) == 0 || len(r.Steps) == 0 {
			problems = append(problems, "recipe needs a title, ingredients and cooking steps")
		}
		if strings.EqualFold(strings.TrimSpace(r.Title), strings.TrimSpace(old.Title)) {
			problems = append(problems, "provide a different dish from "+old.Title)
		}
		if !strings.EqualFold(r.MealType, old.MealType) {
			problems = append(problems, "keep mealType "+old.MealType)
		}
		if r.PrepMinutes < 0 || (p.MaxPrepMins > 0 && r.PrepMinutes > p.MaxPrepMins) {
			problems = append(problems, "respect the maximum preparation time")
		}
		if r.Servings <= 0 {
			problems = append(problems, "servings must be positive")
		}
		for _, ing := range r.Ingredients {
			if strings.TrimSpace(ing.Name) == "" || ing.Quantity <= 0 || math.IsNaN(ing.Quantity) || math.IsInf(ing.Quantity, 0) {
				problems = append(problems, "each ingredient needs a name and a positive quantity")
			}
		}
		problems = append(problems, replacementMacroProblems(p.Targets, old.Macros, r.Macros, kept)...)
		if len(problems) == 0 {
			r.MealType = old.MealType
			r.Warnings = nil
			return r, nil
		}
		user += "\nCorrect these problems in the replacement recipe only: " + strings.Join(problems, "; ") + ". Return the same recipe JSON shape."
	}
	return models.Recipe{}, fmt.Errorf("couldn't find a suitable replacement: %s", strings.Join(problems, "; "))
}

func replacementMacroProblems(target, old, next models.Macros, kept []models.Recipe) []string {
	var total models.Macros
	for _, r := range kept {
		total = total.Add(r.Macros)
	}
	values := func(m models.Macros) []float64 { return []float64{m.Calories, m.Protein, m.Carbs, m.Fat, m.Fiber} }
	names := []string{"calories", "protein", "carbs", "fat", "fiber"}
	t, o, n, k := values(target), values(old), values(next), values(total)
	var problems []string
	for i, v := range n {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) || (i == 0 && v == 0) {
			problems = append(problems, "invalid "+names[i])
			continue
		}
		if t[i] <= 0 {
			continue
		}
		tolerance := math.Max(t[i]*0.10, math.Abs(k[i]+o[i]-t[i]))
		if math.Abs(k[i]+v-t[i]) > tolerance+0.01 {
			problems = append(problems, fmt.Sprintf("%s day total %.1f misses target %.1f; aim for %.1f in the replacement", names[i], k[i]+v, t[i], math.Max(0, t[i]-k[i])))
		}
	}
	return problems
}
