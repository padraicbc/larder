package api

import (
	"net/http"

	"github.com/padraicbc/larder/internal/nutrition"
)

// listFoods returns the built-in everyday foods (loose fruit, veg and potatoes), optionally
// narrowed by ?q=. The numbers are per 100 g, raw, from USDA FoodData Central.
func (s *Server) listFoods(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"source": nutrition.FoodsSource,
		"foods":  nutrition.SearchFoods(r.URL.Query().Get("q"), 0),
	})
}
