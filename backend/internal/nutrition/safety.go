package nutrition

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/you/nutrition-app/internal/models"
)

// LLMs sometimes ignore constraints, so every generated recipe is checked
// here before it reaches the user. Keyword lists are deliberately simple —
// extend them as you find gaps.

var (
	meat = []string{"beef", "chicken", "pork", "lamb", "bacon", "ham", "turkey", "sausage", "prosciutto", "salami",
		"pepperoni", "veal", "duck", "steak", "chorizo", "mince", "gelatin", "gelatine", "lard", "venison", "goat"}
	seafood = []string{"fish", "salmon", "tuna", "cod", "shrimp", "prawn", "anchovy", "anchovies", "crab", "lobster",
		"mussel", "clam", "scallop", "sardine", "mackerel", "tilapia", "halibut", "oyster", "squid", "octopus", "trout"}
	dairy = []string{"milk", "cheese", "butter", "cream", "yogurt", "yoghurt", "whey", "ghee", "parmesan", "mozzarella",
		"feta", "ricotta", "buttermilk", "paneer", "cheddar", "halloumi", "creme fraiche", "kefir", "quark", "mascarpone"}
	eggs   = []string{"egg", "mayonnaise", "mayo", "meringue"}
	gluten = []string{"wheat", "flour", "bread", "pasta", "spaghetti", "couscous", "barley", "rye", "seitan", "bulgur",
		"semolina", "soy sauce", "breadcrumbs", "panko", "flour tortilla", "noodle", "farro", "spelt", "pita", "bagel", "croutons"}
	shellfish = []string{"shrimp", "prawn", "crab", "lobster", "mussel", "clam", "scallop", "oyster"}
	treeNuts  = []string{"almond", "cashew", "walnut", "pecan", "hazelnut", "pistachio", "macadamia", "brazil nut", "pine nut"}
	soy       = []string{"soy", "soya", "tofu", "tempeh", "edamame", "miso", "tamari"}
	sesame    = []string{"sesame", "tahini"}
)

// Plant-based look-alikes. Each phrase is rewritten to what's left that still
// matters, so "oat milk" isn't dairy but "almond milk" still trips a nut allergy.
var lookAlikes = []struct{ phrase, keep string }{
	{"peanut butter", "peanut"}, {"almond butter", "almond"}, {"cashew butter", "cashew"},
	{"nut butter", "nut"}, {"seed butter", "seed"}, {"cocoa butter", "cocoa"},
	{"oat milk", "oat"}, {"almond milk", "almond"}, {"soy milk", "soy"}, {"coconut milk", "coconut"},
	{"rice milk", "rice"}, {"cashew milk", "cashew"}, {"coconut cream", "coconut"},
	{"coconut yogurt", "coconut"}, {"cream of tartar", ""}, {"olive oil", "oil"},
}

// "vegan cheese", "dairy-free yogurt", "gluten-free pasta" are substitutes.
var substituteRe = regexp.MustCompile(`\b(vegan|plant-based|dairy-free|egg-free|gluten-free|meatless)\s+[a-z]+`)

var dietRules = map[string][][]string{
	"vegetarian":  {meat, seafood},
	"vegan":       {meat, seafood, dairy, eggs, {"honey"}},
	"pescatarian": {meat},
	"dairy-free":  {dairy},
	"gluten-free": {gluten},
	"halal":       {{"pork", "bacon", "ham", "lard", "prosciutto", "salami", "pepperoni", "chorizo", "gelatin", "wine", "beer", "rum", "brandy"}},
	"egg-free":    {eggs},
}

// What common allergy words expand into.
var allergyExpansions = map[string][]string{
	"nut":       append(append([]string{"peanut"}, treeNuts...), "nut"),
	"tree nut":  treeNuts,
	"peanut":    {"peanut"},
	"dairy":     dairy,
	"lactose":   dairy,
	"milk":      dairy,
	"gluten":    gluten,
	"wheat":     gluten,
	"shellfish": shellfish,
	"fish":      seafood,
	"egg":       eggs,
	"soy":       soy,
	"sesame":    sesame,
}

var (
	reCache   = map[string]*regexp.Regexp{}
	reCacheMu sync.Mutex
)

// wordRe matches term as a whole word with an optional plural, so "egg"
// matches "eggs" but not "eggplant".
func wordRe(term string) *regexp.Regexp {
	reCacheMu.Lock()
	defer reCacheMu.Unlock()
	if re, ok := reCache[term]; ok {
		return re
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(term) + `(s|es)?\b`)
	reCache[term] = re
	return re
}

// Singular is a crude singulariser good enough for food words.
func Singular(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.HasSuffix(s, "ies") && len(s) > 4:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "oes") && len(s) > 4:
		return s[:len(s)-2]
	case strings.HasSuffix(s, "ss"), strings.HasSuffix(s, "us"):
		return s
	case strings.HasSuffix(s, "s") && len(s) > 3:
		return s[:len(s)-1]
	}
	return s
}

func findTerm(text string, terms []string) (string, bool) {
	for _, t := range terms {
		if wordRe(t).MatchString(text) {
			return t, true
		}
	}
	return "", false
}

// stripLookAlikes rewrites substitute ingredients so keyword rules don't misfire.
func stripLookAlikes(s string) string {
	s = substituteRe.ReplaceAllString(s, " ")
	for _, l := range lookAlikes {
		s = strings.ReplaceAll(s, l.phrase, " "+l.keep+" ")
	}
	return s
}

// CheckRecipe returns human-readable problems; empty means the recipe is fine.
func CheckRecipe(r models.Recipe, p models.Profile) []string {
	var problems []string
	seen := map[string]bool{}
	add := func(msg string) {
		if !seen[msg] {
			seen[msg] = true
			problems = append(problems, msg)
		}
	}
	for _, ing := range r.Ingredients {
		name := stripLookAlikes(strings.ToLower(ing.Name))

		for _, diet := range p.Diets {
			for _, list := range dietRules[strings.ToLower(diet)] {
				if t, ok := findTerm(name, list); ok {
					add(fmt.Sprintf("%q contains %s, which isn't %s", ing.Name, t, diet))
				}
			}
		}
		for _, ex := range p.Excluded {
			term := Singular(ex)
			if term != "" && wordRe(term).MatchString(name) {
				add(fmt.Sprintf("%q is on your excluded list (%s)", ing.Name, ex))
			}
		}
		for _, al := range p.Allergies {
			term := Singular(al)
			terms := allergyExpansions[term]
			if terms == nil {
				terms = []string{term}
			}
			if t, ok := findTerm(name, terms); ok {
				add(fmt.Sprintf("ALLERGY: %q contains %s (%s)", ing.Name, t, al))
			}
		}
	}
	return problems
}
