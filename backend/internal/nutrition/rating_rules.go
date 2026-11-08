package nutrition

import (
	"fmt"
	"sort"
	"strings"
)

// RuleInfo describes one tweakable rating rule and its default points.
type RuleInfo struct {
	Key     string `json:"key"`
	Group   string `json:"group"`
	Label   string `json:"label"`
	Hint    string `json:"hint,omitempty"`
	Default int    `json:"default"`
}

const (
	GroupBase      = "Starting score"
	GroupList      = "Ingredient list"
	GroupSugar     = "Sugar"
	GroupLoad      = "Number of additives"
	GroupNutrition = "Nutrition (per 100 g or ml)"
	GroupAdditives = "Specific additives"
)

// RuleCatalogue lists every rule that can be tweaked, with the built-in defaults.
// The defaults are what the rating uses until you change a rule.
func RuleCatalogue() []RuleInfo {
	out := []RuleInfo{
		{"base", GroupBase, "Every product starts at", "Higher makes every rating kinder, lower stricter.", ratingBase},
		{"long-list-big", GroupList, "More than 15 ingredients", "A very long list usually means ultra-processed.", -8},
		{"long-list", GroupList, "11 to 15 ingredients", "", -4},
		{"short-list", GroupList, "5 or fewer ingredients", "Short lists usually mean simple, whole food.", 5},
		{"sugar-early", GroupSugar, "Sugar in the first three ingredients", "Listed by weight, so near the top means mostly sugar.", -12},
		{"sugar-kinds", GroupSugar, "Three or more kinds of sugar or syrup", "", -6},
		{"load-2", GroupLoad, "2 additives or processing markers", "Counts E-numbers, gums, acids, emulsifiers, flavourings, yeast extract, dairy powders, dextrose, added fibre…", -4},
		{"load-3", GroupLoad, "3 additives or processing markers", "", -8},
		{"load-4", GroupLoad, "4 to 5 additives or processing markers", "", -13},
		{"load-6", GroupLoad, "6 or more additives or processing markers", "", -18},
		{"unknown-e", GroupLoad, "Each E-number with no notes", "Per additive we have no information on.", -1},
		{"energy-high", GroupNutrition, "Over 450 kcal", "Very energy-dense.", -8},
		{"energy-mid", GroupNutrition, "Over 350 kcal", "Energy-dense.", -4},
		{"saturated-very-high", GroupNutrition, "Over 20 g saturated fat", "", -30},
		{"saturated-high", GroupNutrition, "Over 10 to 20 g saturated fat", "", -20},
		{"saturated-mid", GroupNutrition, "Over 5 to 10 g saturated fat", "", -12},
		{"fat-high", GroupNutrition, "Over 30 g fat", "", -4},
		{"fibre-high", GroupNutrition, "6 g fibre or more", "", 6},
		{"fibre-mid", GroupNutrition, "3 g fibre or more", "", 3},
		{"protein-high", GroupNutrition, "15 g protein or more", "", 5},
		{"protein-mid", GroupNutrition, "8 g protein or more", "", 2},
	}
	var add []RuleInfo
	for code, a := range eNumbers {
		key := "E" + strings.ToUpper(code)
		add = append(add, RuleInfo{Key: key, Group: GroupAdditives, Label: fmt.Sprintf("%s %s", key, a.name), Default: a.points})
	}
	for _, n := range namedAdditives {
		add = append(add, RuleInfo{Key: n.name, Group: GroupAdditives, Label: n.name, Default: n.points})
	}
	sort.Slice(add, func(i, j int) bool {
		if add[i].Default != add[j].Default {
			return add[i].Default < add[j].Default // harshest first
		}
		return add[i].Label < add[j].Label
	})
	return append(out, add...)
}

var ruleKeys = func() map[string]bool {
	m := map[string]bool{}
	for _, r := range RuleCatalogue() {
		m[r.Key] = true
	}
	return m
}()

// RuleKnown reports whether key names a tweakable rule.
func RuleKnown(key string) bool { return ruleKeys[key] }
