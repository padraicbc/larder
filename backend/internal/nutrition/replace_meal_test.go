package nutrition

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/you/nutrition-app/internal/models"
)

func replacementFixture(name string, macros models.Macros) models.Recipe {
	return models.Recipe{Title: name, MealType: "dinner", Servings: 1, Ingredients: []models.Ingredient{{Name: "lentils", Quantity: 200, Unit: "g"}}, Steps: []string{"Cook and serve"}, Macros: macros}
}
func replacementReply(r models.Recipe) string {
	b, _ := json.Marshal(map[string]any{"recipe": r})
	return string(b)
}

func TestReplacementChecksWholeDayAndRetries(t *testing.T) {
	p := models.DefaultProfile()
	macros := models.Macros{Calories: 600, Protein: 40, Carbs: 60, Fat: 20, Fiber: 10}
	old := replacementFixture("Old dinner", macros)
	kept := []models.Recipe{{Title: "Breakfast and lunch", Macros: models.Macros{Calories: 1400, Protein: 80, Carbs: 160, Fat: 50, Fiber: 20}}}
	bad := replacementFixture("Too much", models.Macros{Calories: 1200, Protein: 80, Carbs: 200, Fat: 60, Fiber: 25})
	good := replacementFixture("Lentil stew", macros)
	m := &scripted{replies: []string{replacementReply(bad), replacementReply(good)}}
	got, err := GenerateReplacement(context.Background(), m, p, Feedback{}, old, kept, "quicker")
	if err != nil || got.Title != good.Title {
		t.Fatalf("replacement: %+v %v", got, err)
	}
	if len(m.prompts) != 2 || !strings.Contains(m.prompts[1], "day total") || !strings.Contains(m.prompts[0], "Breakfast and lunch") || !strings.Contains(m.prompts[0], "quicker") {
		t.Fatal("missing kept-meal context or macro correction")
	}
}

func TestReplacementRejectsUnsafeSameAndUnbalancedMeals(t *testing.T) {
	p := models.DefaultProfile()
	p.Allergies = []string{"peanut"}
	old := replacementFixture("Old dinner", models.Macros{Calories: 600, Protein: 40, Carbs: 60, Fat: 20, Fiber: 10})
	kept := []models.Recipe{{Macros: models.Macros{Calories: 1400, Protein: 80, Carbs: 160, Fat: 50, Fiber: 20}}}
	unsafe := replacementFixture("Peanut bowl", old.Macros)
	unsafe.Ingredients[0].Name = "peanuts"
	highProtein := replacementFixture("Too much protein", old.Macros)
	highProtein.Macros.Protein = 150
	for _, r := range []models.Recipe{old, unsafe, highProtein, replacementFixture("No calories", models.Macros{})} {
		m := &scripted{replies: []string{replacementReply(r), replacementReply(r)}}
		if _, err := GenerateReplacement(context.Background(), m, p, Feedback{}, old, kept, ""); err == nil {
			t.Errorf("accepted invalid replacement %s", r.Title)
		}
	}
}
