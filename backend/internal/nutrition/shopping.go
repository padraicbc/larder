package nutrition

import (
	"math"
	"sort"
	"strings"

	"github.com/padraicbc/larder/internal/models"
)

// unit -> (base unit, multiplier)
var unitTable = map[string]struct {
	base string
	mul  float64
}{
	"g": {"g", 1}, "gram": {"g", 1}, "grams": {"g", 1}, "kg": {"g", 1000},
	"oz": {"g", 28.35}, "lb": {"g", 453.6}, "lbs": {"g", 453.6},
	"ml": {"ml", 1}, "l": {"ml", 1000}, "litre": {"ml", 1000}, "liter": {"ml", 1000},
	"tsp": {"ml", 5}, "teaspoon": {"ml", 5}, "tbsp": {"ml", 15}, "tablespoon": {"ml", 15},
	"cup": {"ml", 240}, "cups": {"ml", 240},
}

// normalizeUnit converts to g/ml where possible; other units (slice, clove, "")
// stay as-is and are treated as counts.
func normalizeUnit(qty float64, unit string) (float64, string) {
	u := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(unit, ".")))
	if t, ok := unitTable[u]; ok {
		return qty * t.mul, t.base
	}
	return qty, Singular(u)
}

// prettyUnit turns 1500 g into 1.5 kg and rounds sensibly.
func prettyUnit(qty float64, unit string) (float64, string) {
	switch {
	case unit == "g" && qty >= 1000:
		return round(qty/1000, 2), "kg"
	case unit == "ml" && qty >= 1000:
		return round(qty/1000, 2), "l"
	case unit == "g" || unit == "ml":
		return math.Ceil(qty), unit
	}
	return round(qty, 2), unit
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

// BuildShoppingList aggregates every meal occurrence in a plan, scaled to the
// household: each occurrence = one serving per person.
func BuildShoppingList(plan models.MealPlan, recipes map[string]models.Recipe, householdSize int) []models.ShoppingItem {
	if householdSize < 1 {
		householdSize = 1
	}
	type acc struct {
		name, unit, category string
		qty                  float64
	}
	items := map[string]*acc{}
	for _, d := range plan.Days {
		for _, id := range d.RecipeIDs {
			r, ok := recipes[id]
			if !ok {
				continue
			}
			servings := r.Servings
			if servings < 1 {
				servings = 1
			}
			scale := float64(householdSize) / float64(servings)
			for _, ing := range r.Ingredients {
				name := Singular(ing.Name)
				if name == "" {
					continue
				}
				q, u := normalizeUnit(ing.Quantity*scale, ing.Unit)
				key := name + "|" + u
				a := items[key]
				if a == nil {
					a = &acc{name: name, unit: u, category: orDefault(ing.Category, "other")}
					items[key] = a
				}
				a.qty += q
			}
		}
	}
	out := make([]models.ShoppingItem, 0, len(items))
	for key, a := range items {
		q, u := prettyUnit(a.qty, a.unit)
		out = append(out, models.ShoppingItem{Key: key, Name: a.name, Quantity: q, Unit: u, Category: a.category})
	}
	SortShopping(out)
	return out
}

var categoryOrder = map[string]int{"produce": 0, "protein": 1, "dairy": 2, "grains": 3, "pantry": 4, "frozen": 5, "other": 6}

// SortShopping orders by store aisle then name.
func SortShopping(items []models.ShoppingItem) {
	rank := func(c string) int {
		if r, ok := categoryOrder[c]; ok {
			return r
		}
		return len(categoryOrder)
	}
	sort.Slice(items, func(i, j int) bool {
		ri, rj := rank(items[i].Category), rank(items[j].Category)
		if ri != rj {
			return ri < rj
		}
		return items[i].Name < items[j].Name
	})
}

// MergeChecked keeps ticked boxes and manual items when a list is rebuilt.
func MergeChecked(old, fresh []models.ShoppingItem) []models.ShoppingItem {
	checked := map[string]bool{}
	for _, it := range old {
		if it.Checked {
			checked[it.Key] = true
		}
	}
	for i := range fresh {
		fresh[i].Checked = checked[fresh[i].Key]
	}
	for _, it := range old {
		if it.Manual {
			fresh = append(fresh, it)
		}
	}
	SortShopping(fresh)
	return fresh
}
