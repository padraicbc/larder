package nutrition

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/you/nutrition-app/internal/llm"
	"github.com/you/nutrition-app/internal/models"
)

type GeneratedDay struct {
	Day   int             `json:"day"`
	Meals []models.Recipe `json:"meals"`
}

type GeneratedPlan struct {
	Title string         `json:"title"`
	Days  []GeneratedDay `json:"days"`
}

// MaxAttempts: first try + one corrective retry if constraints are broken.
const MaxAttempts = 2

// GeneratePlan asks the model for a plan, validates it, retries once on
// constraint violations, and attaches any remaining problems as warnings.
func GeneratePlan(ctx context.Context, c llm.Client, p models.Profile, fb Feedback, recent []string, req PlanRequest) (GeneratedPlan, error) {
	if req.Days < 1 || req.Days > 7 {
		return GeneratedPlan{}, errors.New("days must be between 1 and 7")
	}
	system, user := BuildPlanPrompt(p, fb, recent, req)

	var plan GeneratedPlan
	var problems []string
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		plan = GeneratedPlan{}
		if err := llm.CompleteJSON(ctx, c, system, user, &plan); err != nil {
			return plan, err
		}
		if err := validatePlan(plan); err != nil {
			if attempt == MaxAttempts {
				return plan, err
			}
			user += "\n\nYour previous answer was invalid: " + err.Error() + ". Return only JSON in the requested shape."
			continue
		}
		problems = applySafety(&plan, p)
		if len(problems) == 0 {
			break
		}
		user += BuildFixPrompt(problems)
	}
	normalize(&plan)
	return plan, nil
}

func validatePlan(p GeneratedPlan) error {
	if len(p.Days) == 0 {
		return errors.New("plan has no days")
	}
	for _, d := range p.Days {
		if len(d.Meals) == 0 {
			return fmt.Errorf("day %d has no meals", d.Day)
		}
		for _, m := range d.Meals {
			if strings.TrimSpace(m.Title) == "" || len(m.Ingredients) == 0 {
				return fmt.Errorf("day %d has a meal without a title or ingredients", d.Day)
			}
		}
	}
	return nil
}

// applySafety sets Warnings on each recipe and returns all problems found.
func applySafety(plan *GeneratedPlan, p models.Profile) []string {
	var all []string
	for di := range plan.Days {
		for mi := range plan.Days[di].Meals {
			r := &plan.Days[di].Meals[mi]
			r.Warnings = CheckRecipe(*r, p)
			for _, w := range r.Warnings {
				all = append(all, r.Title+": "+w)
			}
		}
	}
	return all
}

func normalize(plan *GeneratedPlan) {
	for di := range plan.Days {
		if plan.Days[di].Day == 0 {
			plan.Days[di].Day = di + 1
		}
		for mi := range plan.Days[di].Meals {
			r := &plan.Days[di].Meals[mi]
			if r.Servings <= 0 {
				r.Servings = 1
			}
			r.MealType = strings.ToLower(r.MealType)
			for ii := range r.Ingredients {
				ing := &r.Ingredients[ii]
				ing.Category = strings.ToLower(orDefault(ing.Category, "other"))
			}
		}
	}
	if plan.Title == "" {
		plan.Title = fmt.Sprintf("%d-day plan", len(plan.Days))
	}
}

type Estimate struct {
	Name       string        `json:"name"`
	Macros     models.Macros `json:"macros"`
	Confidence string        `json:"confidence"`
	Notes      string        `json:"notes"`
}

func EstimateMacros(ctx context.Context, c llm.Client, text string) (Estimate, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Estimate{}, errors.New("describe what you ate")
	}
	var e Estimate
	system, user := BuildEstimatePrompt(text)
	err := llm.CompleteJSON(ctx, c, system, user, &e)
	if e.Name == "" {
		e.Name = text
	}
	return e, err
}
