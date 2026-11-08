package nutrition

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"

	"github.com/padraicbc/larder/internal/models"
)

// FoodsSource says where the built-in everyday foods come from.
const FoodsSource = "USDA FoodData Central (SR Legacy), public domain"

// GenericFood is a loose everyday food with its nutrition per 100 g (raw, edible portion).
// The numbers are copied from USDA FoodData Central records by scripts, not typed by hand;
// FDCID and USDA name the exact record so any value can be checked.
type GenericFood struct {
	Name     string           `json:"name"`
	Aliases  []string         `json:"aliases"`
	Category string           `json:"category"`
	Calories float64          `json:"calories"`
	Protein  float64          `json:"protein"`
	Carbs    float64          `json:"carbs"`
	Fat      float64          `json:"fat"`
	Fiber    float64          `json:"fiber"`
	Portions []models.Portion `json:"portions"`
	FDCID    int              `json:"fdcId"`
	USDA     string           `json:"usda"`
}

//go:embed foods.json
var foodsJSON []byte

var everydayFoods = func() []GenericFood {
	var foods []GenericFood
	if err := json.Unmarshal(foodsJSON, &foods); err != nil {
		panic("foods.json: " + err.Error())
	}
	return foods
}()

// CleanPortions keeps only sensible portions: a short label and a weight up to 5 kg, at
// most 8 of them, none repeated.
func CleanPortions(in []models.Portion) []models.Portion {
	var out []models.Portion
	seen := map[string]bool{}
	for _, p := range in {
		label := strings.Join(strings.Fields(p.Label), " ")
		key := strings.ToLower(label)
		if label == "" || len([]rune(label)) > 40 || !(p.Grams > 0) || p.Grams > 5000 || seen[key] || len(out) >= 8 {
			continue
		}
		seen[key] = true
		out = append(out, models.Portion{Label: label, Grams: p.Grams})
	}
	return out
}

// Foods returns every built-in everyday food.
func Foods() []GenericFood { return append([]GenericFood(nil), everydayFoods...) }

// SearchFoods finds foods whose name or an alias contains q (any case). Names that start
// with q come first. An empty q returns everything, in the list's own order.
func SearchFoods(q string, limit int) []GenericFood {
	q = strings.ToLower(strings.TrimSpace(q))
	var out []GenericFood
	if q == "" {
		out = Foods()
	} else {
		type hit struct {
			f    GenericFood
			rank int
		}
		var hits []hit
		for _, f := range everydayFoods {
			rank := -1
			if strings.HasPrefix(strings.ToLower(f.Name), q) {
				rank = 0
			} else if strings.Contains(strings.ToLower(f.Name), q) {
				rank = 1
			}
			for _, a := range f.Aliases {
				al := strings.ToLower(a)
				switch {
				case strings.HasPrefix(al, q) && (rank < 0 || rank > 2):
					rank = 2
				case strings.Contains(al, q) && rank < 0:
					rank = 3
				}
			}
			if rank >= 0 {
				hits = append(hits, hit{f, rank})
			}
		}
		sort.SliceStable(hits, func(i, j int) bool { return hits[i].rank < hits[j].rank })
		for _, h := range hits {
			out = append(out, h.f)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
