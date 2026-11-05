package nutrition

import (
	"strings"
	"testing"

	"github.com/you/nutrition-app/internal/models"
)

func recipeWith(ings ...string) models.Recipe {
	r := models.Recipe{Title: "Test"}
	for _, n := range ings {
		r.Ingredients = append(r.Ingredients, models.Ingredient{Name: n})
	}
	return r
}

func TestCheckRecipe(t *testing.T) {
	tests := []struct {
		name    string
		profile models.Profile
		ing     string
		flagged bool
	}{
		{"vegetarian blocks chicken", models.Profile{Diets: []string{"vegetarian"}}, "chicken thighs", true},
		{"vegetarian blocks fish sauce", models.Profile{Diets: []string{"vegetarian"}}, "fish sauce", true},
		{"vegetarian allows tofu", models.Profile{Diets: []string{"vegetarian"}}, "firm tofu", false},
		{"vegetarian allows peanut butter", models.Profile{Diets: []string{"vegetarian"}}, "peanut butter", false},
		{"pescatarian allows salmon", models.Profile{Diets: []string{"pescatarian"}}, "salmon fillet", false},
		{"pescatarian blocks bacon", models.Profile{Diets: []string{"pescatarian"}}, "smoked bacon", true},
		{"vegan blocks milk", models.Profile{Diets: []string{"vegan"}}, "whole milk", true},
		{"vegan allows oat milk", models.Profile{Diets: []string{"vegan"}}, "oat milk", false},
		{"vegan blocks buttermilk", models.Profile{Diets: []string{"vegan"}}, "buttermilk", true},
		{"vegan allows butternut squash", models.Profile{Diets: []string{"vegan"}}, "butternut squash", false},
		{"vegan blocks eggs", models.Profile{Diets: []string{"vegan"}}, "large eggs", true},
		{"vegan allows eggplant", models.Profile{Diets: []string{"vegan"}}, "eggplant", false},
		{"vegan blocks honey", models.Profile{Diets: []string{"vegan"}}, "honey", true},
		{"gluten-free blocks soy sauce", models.Profile{Diets: []string{"gluten-free"}}, "soy sauce", true},
		{"gluten-free allows tamari", models.Profile{Diets: []string{"gluten-free"}}, "tamari", false},
		{"gluten-free allows corn tortillas", models.Profile{Diets: []string{"gluten-free"}}, "corn tortillas", false},
		{"diet names are case-insensitive", models.Profile{Diets: []string{"Vegetarian"}}, "beef", true},
		{"excluded plural matches singular", models.Profile{Excluded: []string{"mushrooms"}}, "button mushroom", true},
		{"excluded does not over-match", models.Profile{Excluded: []string{"egg"}}, "eggplant", false},
		{"excluding olives keeps olive oil", models.Profile{Excluded: []string{"olives"}}, "extra virgin olive oil", false},
		{"excluding olives still catches olives", models.Profile{Excluded: []string{"olives"}}, "kalamata olives", true},
		{"nut allergy catches peanut butter", models.Profile{Allergies: []string{"nuts"}}, "peanut butter", true},
		{"nut allergy catches almonds", models.Profile{Allergies: []string{"nuts"}}, "toasted almonds", true},
		{"sesame allergy catches tahini", models.Profile{Allergies: []string{"sesame"}}, "tahini", true},
		{"dairy allergy allows oat milk", models.Profile{Allergies: []string{"dairy"}}, "oat milk", false},
		{"nut allergy still catches almond milk", models.Profile{Allergies: []string{"nuts"}}, "almond milk", true},
		{"vegan allows vegan cheese", models.Profile{Diets: []string{"vegan"}}, "vegan cheese", false},
		{"soy allergy catches tamari", models.Profile{Allergies: []string{"soy"}}, "tamari", true},
		{"unknown allergy matches literally", models.Profile{Allergies: []string{"kiwi"}}, "kiwi slices", true},
		{"no constraints, no problems", models.Profile{}, "chicken", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckRecipe(recipeWith(tt.ing), tt.profile)
			if (len(got) > 0) != tt.flagged {
				t.Errorf("CheckRecipe(%q) = %v, flagged want %v", tt.ing, got, tt.flagged)
			}
		})
	}
}

func TestCheckRecipeMarksAllergies(t *testing.T) {
	got := CheckRecipe(recipeWith("shrimp"), models.Profile{Allergies: []string{"shellfish"}})
	if len(got) != 1 || !strings.HasPrefix(got[0], "ALLERGY") {
		t.Errorf("want one ALLERGY warning, got %v", got)
	}
}

func TestSingular(t *testing.T) {
	for in, want := range map[string]string{
		"Mushrooms": "mushroom", "tomatoes": "tomato", "berries": "berry",
		"hummus": "hummus", "glass": "glass", "egg": "egg", "peas": "pea",
	} {
		if got := Singular(in); got != want {
			t.Errorf("Singular(%q) = %q, want %q", in, got, want)
		}
	}
}
