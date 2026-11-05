package api

import (
	"math"
	"net/http"
	"strings"

	"github.com/you/nutrition-app/internal/models"
)

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	products := s.Store.Products(userID(r))
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query != "" {
		filtered := make([]models.Product, 0)
		for _, p := range products {
			if strings.Contains(strings.ToLower(p.Name), query) {
				filtered = append(filtered, p)
			}
		}
		products = filtered
	}
	writeJSON(w, 200, products)
}

func (s *Server) saveProduct(w http.ResponseWriter, r *http.Request) {
	var p models.Product
	if !decode(w, r, &p) {
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	p.BasisUnit = strings.ToLower(strings.TrimSpace(p.BasisUnit))
	if p.Name == "" {
		writeErr(w, 400, "give the product a name")
		return
	}
	if p.BasisQuantity <= 0 || math.IsNaN(p.BasisQuantity) || math.IsInf(p.BasisQuantity, 0) {
		writeErr(w, 400, "enter a positive nutrition basis, such as 100 g")
		return
	}
	if p.BasisUnit != "g" && p.BasisUnit != "ml" && p.BasisUnit != "serving" {
		writeErr(w, 400, "choose grams, millilitres or servings")
		return
	}
	for _, v := range []float64{p.Macros.Calories, p.Macros.Protein, p.Macros.Carbs, p.Macros.Fat, p.Macros.Fiber} {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			writeErr(w, 400, "nutrition values must be zero or greater")
			return
		}
	}
	if !p.FromNutritionLabel {
		p.Confidence = "low"
	}
	saved, err := s.Store.PutProduct(userID(r), p, s.Now())
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, saved)
}

func (s *Server) deleteProduct(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteProduct(userID(r), r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
