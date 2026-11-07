// Package nutrition contains the pure logic: targets, safety checks,
// feedback summarising, prompts, planning and shopping lists.
package nutrition

import (
	"math"
	"slices"

	"github.com/padraicbc/larder/internal/models"
)

var activityFactor = map[string]float64{
	"sedentary":   1.2,
	"light":       1.375,
	"moderate":    1.55,
	"active":      1.725,
	"very_active": 1.9,
}

// BMR uses Mifflin-St Jeor.
func BMR(sex string, weightKG, heightCM float64, age int) float64 {
	base := 10*weightKG + 6.25*heightCM - 5*float64(age)
	if sex == "female" {
		return base - 161
	}
	return base + 5
}

// CalculateTargets turns body stats + goal into daily macro targets.
// ok is false when the stats needed are missing.
func CalculateTargets(p models.Profile) (t models.Macros, ok bool) {
	if p.WeightKG <= 0 || p.HeightCM <= 0 || p.Age <= 0 {
		return t, false
	}
	f, found := activityFactor[p.Activity]
	if !found {
		f = activityFactor["moderate"]
	}
	tdee := BMR(p.Sex, p.WeightKG, p.HeightCM, p.Age) * f

	proteinPerKg := 1.6
	switch p.Goal {
	case "lose":
		tdee *= 0.8
		proteinPerKg = 2.0
	case "gain":
		tdee *= 1.1
		proteinPerKg = 1.8
	}
	cal := math.Round(tdee/10) * 10
	protein := math.Round(p.WeightKG * proteinPerKg)

	var fat, carbs float64
	if slices.Contains(p.Diets, "keto") {
		carbs = 30
		fat = math.Round((cal - protein*4 - carbs*4) / 9)
	} else {
		fat = math.Round(cal * 0.27 / 9)
		carbs = math.Round((cal - protein*4 - fat*9) / 4)
	}
	if carbs < 0 {
		carbs = 0
	}
	return models.Macros{
		Calories: cal,
		Protein:  protein,
		Carbs:    carbs,
		Fat:      fat,
		Fiber:    math.Round(cal / 1000 * 14),
	}, true
}

func SumEntries(entries []models.LogEntry) models.Macros {
	var m models.Macros
	for _, e := range entries {
		m = m.Add(e.Macros)
	}
	return m
}
