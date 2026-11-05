package nutrition

import (
	"fmt"
	"strings"

	"github.com/you/nutrition-app/internal/models"
)

const planSystem = `You are a practical meal planner with dietitian-level knowledge of nutrition.
You write realistic home-cooking recipes with accurate per-serving macros.
Hard constraints (diets, exclusions, allergies) are NEVER broken.
Respond with ONLY a JSON object. No prose, no markdown.`

const planSchema = `{
  "title": "short plan name",
  "days": [
    {
      "day": 1,
      "meals": [
        {
          "title": "string",
          "description": "one sentence",
          "mealType": "breakfast|lunch|dinner|snack",
          "servings": 2,
          "prepMinutes": 20,
          "ingredients": [{"name": "firm tofu", "quantity": 300, "unit": "g", "category": "produce|protein|dairy|grains|pantry|frozen|other"}],
          "steps": ["string"],
          "macros": {"calories": 0, "protein": 0, "carbs": 0, "fat": 0, "fiber": 0},
          "tags": ["quick", "high-protein"]
        }
      ]
    }
  ]
}`

type PlanRequest struct {
	Days  int    `json:"days"`
	Notes string `json:"notes"`
}

func orNone(s []string) string {
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, ", ")
}

// BuildPlanPrompt returns (system, user).
func BuildPlanPrompt(p models.Profile, fb Feedback, recentTitles []string, req PlanRequest) (string, string) {
	meals := p.MealsPerDay
	if meals <= 0 {
		meals = 3
	}
	var b strings.Builder
	fmt.Fprintf(&b, "TASK: meal_plan\nDAYS: %d\nMEALS_PER_DAY: %d\n\n", req.Days, meals)

	t := p.Targets
	fmt.Fprintf(&b, "Daily targets (per person): %.0f kcal, %.0fg protein, %.0fg carbs, %.0fg fat, %.0fg fiber.\n",
		t.Calories, t.Protein, t.Carbs, t.Fat, t.Fiber)
	b.WriteString("Each day's meals (one serving each) should land within ~10% of these targets.\n")
	if meals >= 4 {
		b.WriteString("Meals 4+ are snacks.\n")
	}
	fmt.Fprintf(&b, "Goal: %s weight.\n\n", orDefault(p.Goal, "maintain"))

	b.WriteString("HARD CONSTRAINTS:\n")
	fmt.Fprintf(&b, "- Diets: %s\n", orNone(p.Diets))
	fmt.Fprintf(&b, "- Never use: %s\n", orNone(p.Excluded))
	fmt.Fprintf(&b, "- Allergies (zero tolerance, including traces in sauces): %s\n", orNone(p.Allergies))
	if p.MaxPrepMins > 0 {
		fmt.Fprintf(&b, "- Max prep time per recipe: %d minutes\n", p.MaxPrepMins)
	}

	b.WriteString("\nPREFERENCES:\n")
	fmt.Fprintf(&b, "- Cuisines they enjoy: %s\n", orNone(p.Cuisines))
	b.WriteString("- Reuse ingredients across days to keep the shopping list short and reduce waste.\n")
	b.WriteString("- Leftovers are fine: a dinner with servings: 2 can feed the next day's lunch if you repeat it.\n")
	if fbText := fb.PromptText(); fbText != "" {
		b.WriteString("\nWHAT WE'VE LEARNED FROM THEIR RATINGS:\n" + fbText)
	}
	if len(recentTitles) > 0 {
		b.WriteString("\nRecently served (avoid repeating unless loved): " + strings.Join(recentTitles, "; ") + "\n")
	}
	if n := strings.TrimSpace(req.Notes); n != "" {
		b.WriteString("\nREQUEST FOR THIS PLAN: " + n + "\n")
	}
	b.WriteString("\nUse metric units (g, ml) or counts (unit \"\"). Macros are PER SERVING.\n")
	b.WriteString("Return JSON exactly in this shape:\n" + planSchema)
	return planSystem, b.String()
}

// BuildFixPrompt is appended on retry when the safety checker finds problems.
func BuildFixPrompt(problems []string) string {
	return "\n\nYOUR PREVIOUS ANSWER BROKE HARD CONSTRAINTS:\n- " + strings.Join(problems, "\n- ") +
		"\nRegenerate the whole plan, replacing those ingredients. Return only JSON."
}

const estimateSystem = `You estimate nutrition from a free-text meal description.
Assume typical portion sizes when unspecified. Respond with ONLY JSON.`

func BuildEstimatePrompt(text string) (string, string) {
	return estimateSystem, "TASK: estimate_macros\nMeal: " + text + `

Return: {"name": "short name for the log", "macros": {"calories": 0, "protein": 0, "carbs": 0, "fat": 0, "fiber": 0}, "confidence": "low|medium|high", "notes": "one sentence on assumptions"}`
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
