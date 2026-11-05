package nutrition

import "strings"

var (
	produceWords = []string{"apple", "banana", "berry", "berries", "strawberry", "blueberry", "raspberry", "lemon", "lime", "orange",
		"grape", "pear", "peach", "plum", "mango", "pineapple", "melon", "kiwi", "avocado", "tomato", "cucumber", "lettuce",
		"spinach", "kale", "rocket", "arugula", "cabbage", "broccoli", "cauliflower", "carrot", "celery", "onion", "shallot",
		"leek", "garlic", "ginger", "potato", "sweet potato", "pepper", "chili", "chilli", "courgette", "zucchini",
		"aubergine", "eggplant", "mushroom", "pea", "bean sprout", "corn", "squash", "pumpkin", "beetroot", "radish",
		"asparagus", "green bean", "herb", "basil", "parsley", "coriander", "cilantro", "mint", "dill", "spring onion", "scallion"}
	proteinWords = append(append(append([]string{}, meat...), seafood...),
		"egg", "tofu", "tempeh", "seitan", "chicken", "lentil", "chickpea", "black bean", "kidney bean", "edamame")
	grainWords = []string{"rice", "pasta", "spaghetti", "noodle", "bread", "sourdough", "tortilla", "wrap", "oat", "oats",
		"quinoa", "couscous", "bulgur", "flour", "barley", "cereal", "bagel", "pita", "cracker"}
	drinkWords = []string{"juice", "coffee", "tea", "wine", "beer", "soda", "kombucha", "sparkling water"}
)

// GuessCategory files an ingredient into a shopping aisle from its name.
// It's a best guess for typed items; users can still change it.
func GuessCategory(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(n, "frozen ") {
		return "frozen"
	}
	n = stripLookAlikes(n) // "oat milk" is not dairy
	for _, c := range []struct {
		cat   string
		words []string
	}{
		{"drinks", drinkWords}, {"dairy", dairy}, {"protein", proteinWords}, {"grains", grainWords}, {"produce", produceWords},
	} {
		if _, ok := findTerm(n, c.words); ok {
			return c.cat
		}
	}
	return "pantry"
}
