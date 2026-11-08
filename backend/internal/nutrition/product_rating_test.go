package nutrition

import (
	"strings"
	"testing"
	"time"

	"github.com/padraicbc/larder/internal/models"
)

func prod(ingredients string, m models.Macros) models.Product {
	return models.Product{Ingredients: ingredients, BasisQuantity: 100, BasisUnit: "g", Macros: m}
}

func TestRateProduct(t *testing.T) {
	oats := RateProduct(prod("Wholegrain oats", models.Macros{Calories: 370, Protein: 13, Carbs: 60, Fat: 7, Fiber: 10}))
	junk := RateProduct(prod("Sugar, glucose-fructose syrup, wheat flour, palm oil, flavourings, colour (E102, E110, E129), emulsifier (E471), preservative (E211), maltodextrin, dextrose, salt, modified starch",
		models.Macros{Calories: 480, Protein: 2, Carbs: 70, Fat: 20}))
	if oats.Score < 85 || oats.Label != "excellent" {
		t.Errorf("oats should rate excellent, got %+v", oats)
	}
	if junk.Score > 30 || junk.Label == "good" {
		t.Errorf("junk should rate poor, got %+v", junk)
	}
	var texts []string
	for _, f := range junk.Factors {
		texts = append(texts, f.Text)
	}
	all := strings.Join(texts, "|")
	for _, want := range []string{"E102", "E211", "high-fructose", "first three ingredients"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing factor %q in %s", want, all)
		}
	}
}

func TestRateProductEdges(t *testing.T) {
	macros := models.Macros{Calories: 370, Protein: 13, Fiber: 10}
	// A rating needs BOTH halves.
	if RateProduct(models.Product{BasisUnit: "serving"}) != nil {
		t.Error("nothing known should be unrated")
	}
	if r := RateProduct(models.Product{BasisUnit: "g", BasisQuantity: 50, Macros: macros}); r != nil {
		t.Errorf("nutrition without ingredients must not be rated: %+v", r)
	}
	if r := RateProduct(models.Product{Ingredients: "Oats, sugar"}); r != nil {
		t.Errorf("ingredients without nutrition must not be rated: %+v", r)
	}
	if r := RateProduct(models.Product{Ingredients: "Oats", BasisUnit: "g", BasisQuantity: 100}); r != nil {
		t.Errorf("an all-zero nutrition entry counts as missing: %+v", r)
	}
	if r := RateProduct(models.Product{Ingredients: "Oats", BasisUnit: "serving", BasisQuantity: 1, Macros: macros}); r == nil {
		t.Error("per-serving nutrition is nutrition")
	}
	if full := RateProduct(prod("Oats", macros)); full == nil || full.Provisional || full.Score < 85 {
		t.Errorf("both known: %+v", full)
	}
	// Both known but the ingredients are flagged (another language): rated, marked provisional.
	if w := RateScan(models.Scan{Ingredients: "Oats", Warning: "other language", HasNutrition: true, BasisUnit: "g", BasisQuantity: 100, Macros: macros}); w == nil || !w.Provisional {
		t.Error("a warning should make the rating provisional")
	}
	if RateScan(models.Scan{Ingredients: "Oats", Macros: macros}) != nil {
		t.Error("a scan without nutrition (HasNutrition false) must not be rated even if macros are set")
	}
	// brackets keep sub-ingredients together
	if got := splitIngredients("chocolate (sugar, cocoa), flour. "); len(got) != 2 {
		t.Errorf("split: %q", got)
	}
}

func TestRatingExplainsEveryFactor(t *testing.T) {
	r := RateProduct(prod("Sugar, wheat flour, colour (E102), sweetener (aspartame), unknownthing E999, flavourings",
		models.Macros{Calories: 480, Protein: 2, Fat: 35, Fiber: 0}))
	if r.Base != 90 {
		t.Errorf("base = %d", r.Base)
	}
	sum := r.Base
	for _, f := range r.Factors {
		sum += f.Points
		if f.Points != 0 && f.Why == "" {
			t.Errorf("factor %q has no explanation", f.Text)
		}
	}
	if want := min(max(sum, 0), 100); want != r.Score {
		t.Errorf("base + factors = %d but score = %d", want, r.Score)
	}
}

func TestAdditiveLoad(t *testing.T) {
	// A typical frozen pizza: every additive is mild alone, but there are seven of them.
	pizza := RateProduct(prod("Wheat Flour (Wheat Flour, Calcium Carbonate, Iron, Niacin, Thiamin), Tomato (17%), Mozzarella Cheese (Milk) (14%), Water, Rapeseed Oil, Sugar, Yeast, Salt, Cornflour, Modified Maize Starch, Dextrose, Stabilisers (Guar Gum, Xanthan Gum), Emulsifier (E471), Acidity Regulator (Citric Acid), Basil, Garlic, Flour Treatment Agent (Ascorbic Acid), Black Pepper, Natural Flavouring",
		models.Macros{Calories: 245, Protein: 10, Carbs: 31, Fat: 8, Fiber: 2}))
	if pizza.Score > 65 || pizza.Label == "good" || pizza.Label == "excellent" {
		t.Errorf("a heavily processed pizza must not rate well: %+v", pizza)
	}
	found := false
	for _, f := range pizza.Factors {
		if strings.Contains(f.Text, "additives and processing markers") && f.Points <= -13 && f.Why != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("the additive count should be an explained factor: %+v", pizza.Factors)
	}

	count := func(s string) int { return len(countAdditives(s)) }
	if n := count("Water, flour, xanthan gum (E415), E415, Xanthan Gum"); n != 1 {
		t.Errorf("the same additive written three ways counts once, got %d", n)
	}
	if n := count("Wheat flour (calcium carbonate, iron, niacin, thiamin), water, yeast, salt"); n != 0 {
		t.Errorf("flour fortification is not an additive, got %d", n)
	}
	if n := count("Stabilisers (guar gum, xanthan gum), emulsifier (lecithin)"); n != 3 {
		t.Errorf("a class label shouldn't count on top of the additives it introduces, got %d", n)
	}
	if n := count("Emulsifier"); n != 1 {
		t.Errorf("an unnamed emulsifier still counts, got %d", n)
	}

	// Ordinary foods with one or two mild additives are barely touched.
	bread := RateProduct(prod("Wholemeal flour, water, yeast, salt, ascorbic acid", models.Macros{Calories: 240, Protein: 9, Fiber: 6}))
	if bread.Score < 85 {
		t.Errorf("bread with one mild additive should still rate well: %+v", bread)
	}
	jam := RateProduct(prod("Strawberries, sugar, pectin, citric acid", models.Macros{Calories: 250, Fiber: 1}))
	for _, f := range jam.Factors {
		if strings.Contains(f.Text, "additives and processing markers") && f.Points < -4 {
			t.Errorf("two mild additives cost at most 4: %+v", f)
		}
	}
}

func TestProcessingMarkersAndPerRunRating(t *testing.T) {
	// A long pizza list whose "additives" are processing markers, not E-numbers: the kind that scored 82.
	long := prod("Wheat Flour (Wheat Flour, Calcium Carbonate, Iron, Niacin, Thiamin), Water, Mozzarella Cheese (Milk) (16%), Tomato Purée, Cheddar Cheese (Milk), Sundried Tomato, Rapeseed Oil, Dextrose, Yeast, Sugar, Salt, Garlic, Spices, Yeast Extract, Cheese Powder (Milk), Natural Flavouring, Acidity Regulator (Citric Acid), Chicory Root Fibre, Basil, Black Pepper",
		models.Macros{Calories: 238, Protein: 11, Carbs: 27, Fat: 9.1, Fiber: 2.1})
	r := RateProduct(long)
	if r.Score > 72 || r.Label == "excellent" || r.Label == "good" && r.Score >= 80 {
		t.Errorf("processing markers should count against it: %+v", r)
	}
	var found bool
	for _, f := range r.Factors {
		if strings.Contains(f.Text, "additives and processing markers") && strings.Contains(f.Text, "yeast extract") && strings.Contains(f.Text, "dextrose") {
			found = true
		}
	}
	if !found {
		t.Errorf("the markers should be listed: %+v", r.Factors)
	}

	// The clean UK pack has one marker (chicory fibre) and is not touched.
	clean := RateProduct(prod("WHEAT Flour, Cheese Blend (31%) (Mozzarella Cheese (MILK), Cheddar Cheese (MILK) in varying proportions), Water, Tomato Purée, Yeast, Rapeseed Oil, Chicory Root Fibre, Cornflour, Salt, Garlic Purée, Basil, Oregano, Black Pepper.",
		models.Macros{Calories: 256, Protein: 12.2, Carbs: 27.4, Fat: 10.4, Fiber: 2.1}))
	if clean.Score < 85 {
		t.Errorf("a clean ingredient list must still rate well: %+v", clean)
	}

	// Ratings are calculated on each call: the rules version and the time are on every one.
	a := RateProduct(long)
	time.Sleep(2 * time.Millisecond)
	b := RateProduct(long)
	if a.Rules != RatingRules || a.At.IsZero() || !b.At.After(a.At) {
		t.Errorf("each rating should carry the rules version and its own calculation time: %v %v", a.At, b.At)
	}
}

func factorPoints(r *models.ProductRating, key string) (int, bool) {
	for _, f := range r.Factors {
		if f.Key == key {
			return f.Points, true
		}
	}
	return 0, false
}

func TestRatingSettingsTweakTheDefaults(t *testing.T) {
	p := prod("Wholegrain wheat, sugar, palm oil, cocoa powder, salt", models.Macros{Calories: 380, Protein: 8, Fiber: 4})
	def := RateProduct(p)
	if pts, ok := factorPoints(def, "palm oil"); !ok || pts != -3 {
		t.Fatalf("default for palm oil should be -3: %+v", def.Factors)
	}
	// No settings at all is exactly the defaults.
	if same := RateProductWith(p, models.RatingSettings{}); same.Score != def.Score {
		t.Errorf("empty settings changed the score: %d vs %d", same.Score, def.Score)
	}

	// Palm oil matters more to this person.
	harsh := RateProductWith(p, models.RatingSettings{Points: map[string]int{"palm oil": -15}})
	if pts, _ := factorPoints(harsh, "palm oil"); pts != -15 || harsh.Score != def.Score-12 {
		t.Errorf("palm oil override: %+v (default score %d)", harsh, def.Score)
	}
	// 0 switches a rule off, and a mild additive can be made to cost something.
	off := RateProductWith(p, models.RatingSettings{Points: map[string]int{"palm oil": 0}})
	if _, ok := factorPoints(off, "palm oil"); ok || off.Score != def.Score+3 {
		t.Errorf("a rule set to 0 should disappear: %+v", off.Factors)
	}
	gums := prod("Water, flour, carrageenan, salt", models.Macros{Calories: 100, Protein: 1})
	if pts, _ := factorPoints(RateProductWith(gums, models.RatingSettings{Points: map[string]int{"E407": -20}}), "E407"); pts != 0 {
		t.Log("E407 only fires when written as an E-number, as before")
	}
	e407 := prod("Water, flour, thickener (E407), salt", models.Macros{Calories: 100, Protein: 1})
	if pts, ok := factorPoints(RateProductWith(e407, models.RatingSettings{Points: map[string]int{"E407": -20}}), "E407"); !ok || pts != -20 {
		t.Errorf("E-number override: %v %v", pts, ok)
	}
	// A mild additive (0 by default) can be made to count.
	e300 := prod("Water, flour, E300", models.Macros{Calories: 100, Protein: 1})
	if pts, ok := factorPoints(RateProductWith(e300, models.RatingSettings{Points: map[string]int{"E300": -6}}), "E300"); !ok || pts != -6 {
		t.Errorf("a normally free additive can be made to cost points: %v %v", pts, ok)
	}

	// Structural rules and the starting score.
	if r := RateProductWith(p, models.RatingSettings{Points: map[string]int{"base": 70}}); r.Base != 70 || r.Score != def.Score-20 {
		t.Errorf("base override: %+v", r)
	}
	if r := RateProductWith(p, models.RatingSettings{Points: map[string]int{"sugar-early": -30}}); r.Score >= def.Score {
		t.Errorf("sugar-early override: %+v", r)
	}

	// The person's own words.
	custom := RateProductWith(p, models.RatingSettings{Custom: []models.CustomRule{{Match: "cocoa", Points: -7}, {Match: "oat", Points: 5}}})
	if pts, ok := factorPoints(custom, "custom:cocoa"); !ok || pts != -7 || custom.Score != def.Score-7 {
		t.Errorf("custom word rule: %+v", custom.Factors)
	}
	if _, ok := factorPoints(custom, "custom:oat"); ok {
		t.Error("a custom rule matches whole words, so 'oat' must not match 'wholegrain wheat'")
	}
	phrase := RateProductWith(prod("Flour, Palm Oil, salt", models.Macros{Calories: 100, Protein: 1}), models.RatingSettings{Custom: []models.CustomRule{{Match: "palm oil", Points: -2}}})
	if _, ok := factorPoints(phrase, "custom:palm oil"); !ok {
		t.Errorf("phrases match regardless of case: %+v", phrase.Factors)
	}
}

func TestRuleCatalogueMatchesTheRules(t *testing.T) {
	cat := RuleCatalogue()
	seen := map[string]bool{}
	for _, r := range cat {
		if seen[r.Key] {
			t.Errorf("duplicate rule key %q", r.Key)
		}
		seen[r.Key] = true
		if r.Label == "" || r.Group == "" {
			t.Errorf("rule %q needs a label and group", r.Key)
		}
	}
	// Every default in the catalogue really is what the rating uses.
	for key, want := range map[string]int{"base": 90, "long-list-big": -8, "sugar-early": -12, "load-6": -18, "fibre-high": 6, "palm oil": -3, "E102": -8} {
		found := false
		for _, r := range cat {
			if r.Key == key {
				found = true
				if r.Default != want {
					t.Errorf("%s default = %d, want %d", key, r.Default, want)
				}
			}
		}
		if !found {
			t.Errorf("rule %q missing from the catalogue", key)
		}
	}
	if !RuleKnown("E150D") || !RuleKnown("high-fructose / glucose-fructose syrup") || RuleKnown("nonsense") {
		t.Error("RuleKnown")
	}
}

func TestGenericWholeFoods(t *testing.T) {
	carrots := models.Product{Name: "Carrots", Generic: true, BasisUnit: "g", BasisQuantity: 100, Macros: models.Macros{Calories: 41, Protein: 0.9, Carbs: 10, Fat: 0.2, Fiber: 2.8}}
	r := RateProduct(carrots)
	if r == nil || r.Score < 90 || r.Label != "excellent" {
		t.Fatalf("a plain whole food with no ingredients label should still be rated, and well: %+v", r)
	}
	// The same thing as a packaged product (not generic) has no ingredients, so no rating.
	carrots.Generic = false
	if RateProduct(carrots) != nil {
		t.Error("a packaged product without ingredients must not be rated")
	}
	// A generic food still needs its nutrition.
	if RateProduct(models.Product{Name: "Potatoes", Generic: true}) != nil {
		t.Error("a generic food without nutrition must not be rated")
	}
	// ...and a scan view of a generic product rates the same way.
	sc := ScanFromProduct(models.Product{Name: "Potatoes", Generic: true, BasisUnit: "g", BasisQuantity: 100, Macros: models.Macros{Calories: 77, Protein: 2, Carbs: 17, Fiber: 2.2}})
	if !sc.Generic || RateScan(sc) == nil {
		t.Errorf("scan of a generic product: %+v", sc)
	}
}

func TestSaturatedFatRating(t *testing.T) {
	for _, tc := range []struct {
		saturated float64
		key       string
		penalty   int
	}{
		{0, "", 0}, {5, "", 0}, {5.1, "saturated-mid", -12}, {10, "saturated-mid", -12}, {10.1, "saturated-high", -20}, {20, "saturated-high", -20}, {27, "saturated-very-high", -30},
	} {
		m := models.Macros{Calories: 410, Fat: 35, Protein: 25, SaturatedFat: &tc.saturated}
		p := prod("Milk, salt, cultures, rennet", m)
		r := RateProduct(p)
		if r.Provisional {
			t.Fatal("known saturated fat must complete the rating")
		}
		if tc.key != "" {
			if v, ok := factorPoints(r, tc.key); !ok || v != tc.penalty {
				t.Fatalf("%v: %+v", tc, r)
			}
		}
		if tc.saturated == 27 && r.Score != 62 {
			t.Fatalf("vintage cheese score = %d, want 62", r.Score)
		}
		p.BasisQuantity = 25
		p.Macros = m.Scale(0.25)
		if scaled := RateProduct(p); scaled.Score != r.Score {
			t.Fatal("portion basis changed score")
		}
		if tc.key != "" {
			off := RateProductWith(prod("Milk, salt, cultures, rennet", m), models.RatingSettings{Points: map[string]int{tc.key: 0}})
			if off.Score != r.Score-tc.penalty {
				t.Fatal("override ignored")
			}
		}
	}
	unknown := RateProduct(prod("Milk", models.Macros{Fat: 35}))
	if !unknown.Provisional || !strings.Contains(unknown.ProvisionalReason, "Saturated fat") {
		t.Fatal("missing saturated fat must be explained")
	}
}
