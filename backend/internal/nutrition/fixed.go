package nutrition

import (
	"strconv"
	"strings"

	"github.com/padraicbc/larder/internal/models"
)

var mealOrder = map[string]int{"breakfast": 0, "lunch": 1, "dinner": 2, "snack": 3}

// FixedMeals returns the request's usable fixed meals (those with a title),
// one per slot, with meal types normalised.
func FixedMeals(req PlanRequest) []models.FixedMeal {
	var out []models.FixedMeal
	seen := map[string]bool{}
	for _, f := range req.FixedMeals {
		f.Title = strings.TrimSpace(f.Title)
		if f.Title == "" {
			continue
		}
		f.MealType = strings.ToLower(strings.TrimSpace(f.MealType))
		if _, ok := mealOrder[f.MealType]; !ok {
			f.MealType = "breakfast"
		}
		if seen[f.MealType] {
			continue
		}
		seen[f.MealType] = true
		out = append(out, f)
	}
	return out
}

// fixedRecipe turns a fixed meal into a one-serving recipe for a plan.
func fixedRecipe(f models.FixedMeal) models.Recipe {
	ings := make([]models.Ingredient, len(f.Ingredients))
	copy(ings, f.Ingredients)
	for i := range ings {
		ings[i].Category = strings.ToLower(orDefault(ings[i].Category, "other"))
	}
	return models.Recipe{
		Title: f.Title, MealType: f.MealType, Servings: 1,
		Ingredients: ings, Macros: f.Macros, Tags: []string{"usual"},
	}
}

// addFixedMeals puts the fixed meals into every day, breakfast first, and
// checks them against the profile's allergies and exclusions like any recipe.
func addFixedMeals(plan *GeneratedPlan, p models.Profile, fixed []models.FixedMeal, days int) {
	if len(fixed) == 0 {
		return
	}
	for len(plan.Days) < days {
		plan.Days = append(plan.Days, GeneratedDay{Day: len(plan.Days) + 1})
	}
	for di := range plan.Days {
		var meals []models.Recipe
		for _, f := range fixed {
			r := fixedRecipe(f)
			r.Warnings = CheckRecipe(r, p)
			meals = append(meals, r)
		}
		meals = append(meals, plan.Days[di].Meals...)
		// stable order: breakfast, lunch, dinner, snack
		for i := 1; i < len(meals); i++ {
			for j := i; j > 0 && mealRank(meals[j]) < mealRank(meals[j-1]); j-- {
				meals[j], meals[j-1] = meals[j-1], meals[j]
			}
		}
		plan.Days[di].Meals = meals
	}
}

func mealRank(r models.Recipe) int {
	if n, ok := mealOrder[strings.ToLower(r.MealType)]; ok {
		return n
	}
	return 4
}

var knownUnits = map[string]string{"g": "g", "gram": "g", "grams": "g", "kg": "kg", "ml": "ml", "l": "l", "oz": "oz", "lb": "lb",
	"tbsp": "tbsp", "tsp": "tsp", "cup": "cup", "cups": "cup"}

// ParseQuantity reads "75 g", "2", "2 biscuits" or "1.5 cups". Anything it can't
// read counts as 1 item, and unit words that aren't real units ("biscuits",
// "scoop") become a plain count so the shopping list stays tidy.
func ParseQuantity(s string) (float64, string) {
	f := strings.Fields(strings.ToLower(strings.ReplaceAll(s, ",", ".")))
	if len(f) == 0 {
		return 1, ""
	}
	num, rest := f[0], f[1:]
	unit := ""
	for _, u := range []string{"kg", "ml", "oz", "lb", "g", "l"} { // "75g"
		if strings.HasSuffix(num, u) && len(num) > len(u) {
			num, unit = strings.TrimSuffix(num, u), u
			break
		}
	}
	q, err := strconv.ParseFloat(num, 64)
	if err != nil || q <= 0 {
		return 1, ""
	}
	if unit == "" && len(rest) > 0 {
		unit = knownUnits[rest[0]]
	}
	return q, unit
}

// FixedFromSaved turns a saved meal into a plan slot. Its items become the
// ingredients, so they show up on the shopping list.
func FixedFromSaved(m models.SavedMeal, slot string) models.FixedMeal {
	ings := make([]models.Ingredient, 0, len(m.Items))
	for _, it := range m.Items {
		q, unit := ParseQuantity(it.Quantity)
		ings = append(ings, models.Ingredient{Name: it.Name, Quantity: q, Unit: unit, Category: GuessCategory(it.Name)})
	}
	return models.FixedMeal{MealType: slot, Title: m.Name, Ingredients: ings, Macros: m.Macros}
}
