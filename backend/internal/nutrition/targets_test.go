package nutrition

import (
	"testing"

	"github.com/you/nutrition-app/internal/models"
)

func TestBMR(t *testing.T) {
	if got := BMR("male", 80, 180, 30); got != 1780 {
		t.Errorf("male BMR = %v, want 1780", got)
	}
	if got := BMR("female", 60, 165, 30); got != 1320.25 {
		t.Errorf("female BMR = %v, want 1320.25", got)
	}
}

func TestCalculateTargets(t *testing.T) {
	base := models.Profile{Sex: "male", Age: 30, HeightCM: 180, WeightKG: 80, Activity: "moderate", Goal: "maintain"}

	got, ok := CalculateTargets(base)
	if !ok {
		t.Fatal("expected ok")
	}
	want := models.Macros{Calories: 2760, Protein: 128, Carbs: 375, Fat: 83, Fiber: 39}
	if got != want {
		t.Errorf("maintain targets = %+v, want %+v", got, want)
	}

	lose := base
	lose.Goal = "lose"
	l, _ := CalculateTargets(lose)
	if l.Calories >= got.Calories || l.Protein <= got.Protein {
		t.Errorf("lose should cut calories and raise protein: %+v vs %+v", l, got)
	}

	keto := base
	keto.Diets = []string{"keto"}
	k, _ := CalculateTargets(keto)
	if k.Carbs != 30 {
		t.Errorf("keto carbs = %v, want 30", k.Carbs)
	}
	// Macros should still add up to roughly the calorie target.
	if kcal := k.Protein*4 + k.Carbs*4 + k.Fat*9; kcal < k.Calories-10 || kcal > k.Calories+10 {
		t.Errorf("keto macros sum to %v kcal, target %v", kcal, k.Calories)
	}

	if _, ok := CalculateTargets(models.Profile{Sex: "male"}); ok {
		t.Error("missing stats should not be ok")
	}
}

func TestSumEntries(t *testing.T) {
	got := SumEntries([]models.LogEntry{
		{Macros: models.Macros{Calories: 100, Protein: 10}},
		{Macros: models.Macros{Calories: 250, Protein: 5, Fat: 3}},
	})
	if got.Calories != 350 || got.Protein != 15 || got.Fat != 3 {
		t.Errorf("SumEntries = %+v", got)
	}
}
