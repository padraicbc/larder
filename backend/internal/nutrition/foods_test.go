package nutrition

import (
	"strings"
	"testing"

	"github.com/padraicbc/larder/internal/models"
)

// Plausible ceilings per 100 g for each category (bacon ~470, nuts ~650, butter ~720, oil ~884).
var kcalLimit = map[string]float64{
	"Fruit": 250, "Vegetables": 350, "Eggs": 650, "Meat & poultry": 650, "Fish & seafood": 400, "Dairy": 800,
	"Bread, rice & pasta": 450, "Beans & pulses": 450, "Nuts & seeds": 750, "Fats & oils": 900, "Sweet & spreads": 650, "Drinks": 100,
}

// The list was generated from USDA records. This catches a bad value or a broken file:
// energy has to roughly agree with the macros (4 kcal/g protein and carbs, 9 kcal/g fat).
func TestEverydayFoodsAreSane(t *testing.T) {
	foods := Foods()
	if len(foods) < 200 {
		t.Fatalf("expected a decent list, got %d", len(foods))
	}
	seen := map[string]bool{}
	for _, f := range foods {
		if f.Name == "" || f.FDCID == 0 || f.USDA == "" || f.Category == "" {
			t.Errorf("%+v is missing its name, category or source record", f)
		}
		if seen[strings.ToLower(f.Name)] {
			t.Errorf("duplicate %q", f.Name)
		}
		seen[strings.ToLower(f.Name)] = true
		limit, ok := kcalLimit[f.Category]
		if !ok {
			t.Errorf("%s: unknown category %q", f.Name, f.Category)
		}
		if f.Calories <= 0 || f.Calories > limit {
			t.Errorf("%s: %v kcal per 100 g looks wrong", f.Name, f.Calories)
		}
		if len(f.Portions) > 4 {
			t.Errorf("%s has too many portions", f.Name)
		}
		for _, q := range f.Portions {
			if !strings.HasPrefix(q.Label, "1 ") || q.Grams <= 0 || q.Grams > 3000 {
				t.Errorf("%s: odd portion %+v", f.Name, q)
			}
		}
		for _, v := range []float64{f.Protein, f.Carbs, f.Fat, f.Fiber} {
			if v < 0 || v > 100 {
				t.Errorf("%s: a macro of %v per 100 g looks wrong", f.Name, v)
			}
		}
		atwater := 4*f.Protein + 4*f.Carbs + 9*f.Fat
		if diff := f.Calories - atwater; diff > 25+0.3*f.Calories || diff < -(25+0.3*f.Calories) {
			t.Errorf("%s: %v kcal doesn't match its macros (%.0f)", f.Name, f.Calories, atwater)
		}
	}
	// A few values anyone can sanity-check from a food label.
	byName := map[string]GenericFood{}
	for _, f := range foods {
		byName[f.Name] = f
	}
	for name, kcal := range map[string]float64{"Carrots": 41, "Bananas": 89, "Tomatoes": 18, "Potatoes (white)": 69, "Grapes": 69, "Eggs (boiled)": 155, "Chicken breast (cooked)": 165, "Milk (whole)": 61, "Salmon (baked)": 206, "Bread (white)": 266, "Rice (white, cooked)": 130, "Olive oil": 884, "Butter (salted)": 717, "Sugar": 387} {
		if got := byName[name].Calories; got != kcal {
			t.Errorf("%s = %v kcal, want %v", name, got, kcal)
		}
	}
}

func TestSearchFoods(t *testing.T) {
	names := func(fs []GenericFood) []string {
		var out []string
		for _, f := range fs {
			out = append(out, f.Name)
		}
		return out
	}
	if got := names(SearchFoods("potato", 0)); len(got) < 4 || !strings.HasPrefix(got[0], "Potatoes") {
		t.Errorf("potato: %v", got)
	}
	if got := names(SearchFoods("zucchini", 0)); len(got) != 1 || got[0] != "Courgette" {
		t.Errorf("aliases work: %v", got)
	}
	if got := names(SearchFoods("EGGPLANT", 0)); len(got) != 1 || got[0] != "Aubergine" {
		t.Errorf("case-insensitive: %v", got)
	}
	// names that start with the text come before names that merely contain it
	if got := names(SearchFoods("pea", 0)); len(got) < 2 || got[0] != "Peas (green)" {
		t.Errorf("prefix first: %v", got)
	}
	if len(SearchFoods("", 0)) != len(Foods()) || len(SearchFoods("", 5)) != 5 {
		t.Error("empty search lists everything, limit applies")
	}
	if len(SearchFoods("zzzz", 0)) != 0 {
		t.Error("no match")
	}
	// Returned slices are copies: callers can't change the built-in data.
	a := Foods()
	a[0].Name = "changed"
	if Foods()[0].Name == "changed" {
		t.Error("Foods() must return a copy")
	}
}

func TestEggsAndMeatAndPortions(t *testing.T) {
	byName := map[string]GenericFood{}
	cats := map[string]int{}
	for _, f := range Foods() {
		byName[f.Name] = f
		cats[f.Category]++
	}
	if cats["Eggs"] < 4 || cats["Meat & poultry"] < 20 {
		t.Fatalf("eggs and meat should be well covered: %v", cats)
	}
	found := false
	for _, q := range byName["Eggs (boiled)"].Portions {
		found = found || (q.Label == "1 large" && q.Grams == 50) // USDA: one large egg is 50 g
	}
	if !found {
		t.Errorf("a boiled egg should list '1 large' = 50 g: %+v", byName["Eggs (boiled)"].Portions)
	}
	if b := byName["Bananas"]; len(b.Portions) == 0 || b.Portions[0].Label != "1 medium" || b.Portions[0].Grams != 118 {
		t.Errorf("a medium banana is 118 g: %+v", b.Portions)
	}
	// Raw and cooked are separate foods, and cooked chicken is denser (it loses water).
	if byName["Chicken breast (cooked)"].Protein <= byName["Chicken breast (raw)"].Protein {
		t.Error("cooked chicken has more protein per 100 g than raw")
	}
	names := func(q string) []string {
		var out []string
		for _, f := range SearchFoods(q, 0) {
			out = append(out, f.Name)
		}
		return out
	}
	if got := names("egg"); len(got) < 5 || !strings.HasPrefix(got[0], "Eggs") {
		t.Errorf("egg: %v", got)
	}
	if got := names("chicken"); len(got) < 6 {
		t.Errorf("chicken: %v", got)
	}
	if got := names("mince"); len(got) < 8 {
		t.Errorf("mince finds beef, pork, lamb, chicken and turkey: %v", got)
	}
	if got := names("bangers"); len(got) != 2 {
		t.Errorf("an alias finds sausages: %v", got)
	}
}

func TestCleanPortions(t *testing.T) {
	got := CleanPortions([]models.Portion{{Label: " 1  large ", Grams: 50}, {Label: "1 LARGE", Grams: 60}, {Label: "", Grams: 5}, {Label: "x", Grams: 0}, {Label: "huge", Grams: 99999}, {Label: strings.Repeat("a", 41), Grams: 5}, {Label: "1 small", Grams: 38}})
	if len(got) != 2 || got[0].Label != "1 large" || got[0].Grams != 50 || got[1].Label != "1 small" {
		t.Errorf("%+v", got)
	}
}

func TestWholeEverydayRange(t *testing.T) {
	cats := map[string]int{}
	byName := map[string]GenericFood{}
	for _, f := range Foods() {
		cats[f.Category]++
		byName[f.Name] = f
	}
	for cat, min := range map[string]int{"Fruit": 20, "Vegetables": 50, "Eggs": 4, "Meat & poultry": 25, "Fish & seafood": 20, "Dairy": 20, "Bread, rice & pasta": 18, "Beans & pulses": 8, "Nuts & seeds": 10, "Fats & oils": 4, "Sweet & spreads": 4, "Drinks": 4} {
		if cats[cat] < min {
			t.Errorf("%s has %d foods, want at least %d", cat, cats[cat], min)
		}
	}
	// Plenty of what people actually log is there, under the names they'd type.
	names := func(q string) int { return len(SearchFoods(q, 0)) }
	for _, q := range []string{"milk", "yoghurt", "yogurt", "cheese", "bread", "toast", "rice", "pasta", "spaghetti", "salmon", "tuna", "prawns", "baked beans", "porridge", "oats", "butter", "olive oil", "peanut butter", "chips", "banana", "coffee", "tea", "honey"} {
		if names(q) == 0 {
			t.Errorf("searching %q finds nothing", q)
		}
	}
	// Spot checks anyone can verify on a food label.
	if p := byName["Peanut butter (smooth)"].Portions; len(p) == 0 || p[0].Label != "1 tbsp" || p[0].Grams != 16 {
		t.Errorf("a tablespoon of peanut butter is 16 g: %+v", p)
	}
	if p := byName["Bread (white)"].Portions; len(p) == 0 || p[0].Label != "1 slice" || p[0].Grams != 29 {
		t.Errorf("a slice of white bread is 29 g: %+v", p)
	}
	for _, f := range byName {
		for _, q := range f.Portions {
			if strings.Contains(q.Label, "crust") || strings.Contains(q.Label, "packed") {
				t.Errorf("%s: unhelpful portion label %q", f.Name, q.Label)
			}
		}
	}
}

func TestCoffeePortionsAndMilkVariants(t *testing.T) {
	byName := map[string]GenericFood{}
	for _, f := range Foods() {
		byName[f.Name] = f
	}
	for _, name := range []string{"Cappuccino (2% milk)", "Cappuccino (skimmed milk)", "Latte (2% milk)", "Latte (skimmed milk)", "Iced latte (2% milk)", "Mocha (2% milk)"} {
		f, ok := byName[name]
		if !ok || f.Dataset != "FNDDS" || f.SaturatedFat == nil || f.Notes == "" {
			t.Fatalf("missing sourced coffee details: %s", name)
		}
		if len(f.Portions) < 3 {
			t.Fatalf("missing sizes: %s", name)
		}
		for i, size := range []string{"1 small", "1 medium", "1 large"} {
			if f.Portions[i].Label != size || (i > 0 && f.Portions[i].Grams <= f.Portions[i-1].Grams) {
				t.Fatalf("bad size ordering: %s %+v", name, f.Portions)
			}
		}
	}
	latte := byName["Latte (2% milk)"]
	medium := models.Macros{Calories: latte.Calories, Protein: latte.Protein, Fat: latte.Fat, SaturatedFat: latte.SaturatedFat}.Scale(latte.Portions[1].Grams / 100)
	if medium.Calories != 206.4 || medium.SaturatedFat == nil || *medium.SaturatedFat <= 4 {
		t.Fatalf("medium latte must scale nutrition including saturates: %+v", medium)
	}
	if byName["Latte (skimmed milk)"].Calories >= latte.Calories {
		t.Fatal("skimmed milk latte must differ from 2% milk")
	}
	if got := SearchFoods("capuchino", 0); len(got) != 2 {
		t.Fatalf("cappuccino spelling alias: %+v", got)
	}
	if len(SearchFoods("coffee", 0)) < 9 {
		t.Fatal("coffee search must find the drink types")
	}
}
