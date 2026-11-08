package api

import (
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/nutrition"
)

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	products := s.Store.Products(userID(r))
	cfg := s.Store.RatingSettings(userID(r))
	for i := range products {
		products[i].Rating = nutrition.RateProductWith(products[i], cfg)
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query != "" {
		filtered := make([]models.Product, 0)
		for _, p := range products {
			if strings.Contains(strings.ToLower(p.Name), query) || (p.Barcode != "" && strings.Contains(p.Barcode, query)) {
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
	if p.Generic && strings.TrimSpace(p.Ingredients) == "" {
		p.Ingredients = p.Name // whole food: the food is its own ingredient
	}
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
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) || !p.Macros.ValidSaturatedFat() {
			writeErr(w, 400, "nutrition values must be zero or greater")
			return
		}
	}
	if p.Barcode = strings.TrimSpace(p.Barcode); p.Barcode != "" {
		code, ok := nutrition.NormalizeBarcode(p.Barcode)
		if !ok {
			writeErr(w, 400, "that barcode isn't valid; check the digits (EAN-8, UPC-A or EAN-13)")
			return
		}
		p.Barcode = code
	}
	p.Portions = nutrition.CleanPortions(p.Portions)
	if p.Source != "" && p.Source != "openfoodfacts" && p.Source != "usda" && p.Source != "ai" {
		p.Source = ""
	}
	if !p.FromNutritionLabel {
		p.Confidence = "low"
	}
	p.Rating = nil // computed, never stored
	saved, err := s.Store.PutProduct(userID(r), p, s.Now())
	if err != nil {
		storeErr(w, err)
		return
	}
	saved.Rating = nutrition.RateProductWith(saved, s.Store.RatingSettings(userID(r)))
	writeJSON(w, 200, saved)
}

func (s *Server) deleteProduct(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteProduct(userID(r), r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// lookupBarcode finds a barcode in the user's saved products first, then (only
// if an admin enabled it in Settings) in Open Food Facts. Nothing is saved here.
func (s *Server) lookupBarcode(w http.ResponseWriter, r *http.Request) {
	code, ok := nutrition.NormalizeBarcode(r.PathValue("barcode"))
	if !ok {
		writeErr(w, 400, "that barcode isn't valid; check the digits (EAN-8, UPC-A or EAN-13)")
		return
	}
	uid := userID(r)
	if saved, err := s.Store.ProductByBarcode(uid, code); err == nil {
		s.fillLookupSaturatedFat(r, code, saved.BasisQuantity, saved.BasisUnit, &saved.Macros)
		saved.Rating = nutrition.RateProductWith(saved, s.Store.RatingSettings(uid))
		writeJSON(w, 200, map[string]any{"source": "saved", "product": saved})
		return
	}
	if !s.Store.Lookup().OpenFoodFacts {
		writeErr(w, 404, "not in your saved products. Turn on online barcode lookup in Settings to search Open Food Facts, or photograph the label")
		return
	}
	found, err := nutrition.LookupOpenFoodFacts(r.Context(), s.HTTP, s.OFFBase, code)
	if errors.Is(err, nutrition.ErrBarcodeUnknown) {
		writeErr(w, 404, err.Error()+"; photograph the label instead")
		return
	}
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"source": "openfoodfacts", "product": found})
}

// readBarcodePhoto reads the digits from a photo of a barcode. It only returns
// the code; the client then calls the lookup endpoint like any typed barcode.
func (s *Server) readBarcodePhoto(w http.ResponseWriter, r *http.Request) {
	images, ok := readPhotos(w, r)
	if !ok {
		return
	}
	if len(images) != 1 {
		writeErr(w, 400, "add one clear photo of the barcode")
		return
	}
	c, err := s.visionClient()
	if err != nil {
		writeErr(w, 400, "model not configured for photos: "+err.Error())
		return
	}
	code, err := nutrition.ReadBarcodePhoto(r.Context(), c, images)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"barcode": code})
}

// setProductIngredients adds the ingredients to a saved product that didn't have them
// (e.g. photographed in the shop), so its rating stops being provisional.
func (s *Server) setProductIngredients(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ingredients string `json:"ingredients"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Ingredients = strings.TrimSpace(body.Ingredients)
	if body.Ingredients == "" || len(body.Ingredients) > 4000 {
		writeErr(w, 400, "enter the ingredients list (up to 4000 characters)")
		return
	}
	p, err := s.Store.SetProductIngredients(userID(r), r.PathValue("id"), body.Ingredients, s.Now())
	if err != nil {
		storeErr(w, err)
		return
	}
	p.Rating = nutrition.RateProductWith(p, s.Store.RatingSettings(userID(r)))
	writeJSON(w, 200, p)
}

// fillLookupSaturatedFat fills a missing nutrient for this response only. A saved
// product from before saturated fat was captured must not hide online label data.
// Keep the saved basis and all user-entered values; never persist a lookup or score.
func (s *Server) fillLookupSaturatedFat(r *http.Request, code string, quantity float64, unit string, m *models.Macros) {
	if m.SaturatedFat != nil || m.Fat <= 0 || quantity <= 0 || (unit != "g" && unit != "ml") || !s.Store.Lookup().OpenFoodFacts {
		return
	}
	found, err := nutrition.LookupOpenFoodFacts(r.Context(), s.HTTP, s.OFFBase, code)
	if err != nil || found.BasisUnit != unit || found.Macros.SaturatedFat == nil {
		return
	}
	value := *found.Macros.SaturatedFat * quantity / found.BasisQuantity
	candidate := *m
	candidate.SaturatedFat = &value
	if candidate.ValidSaturatedFat() {
		m.SaturatedFat = &value
	}
}

func (s *Server) setProductFavourite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Favourite *bool `json:"favourite"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Favourite == nil {
		writeErr(w, 400, "choose whether the product is a favourite")
		return
	}
	p, err := s.Store.SetProductFavourite(userID(r), r.PathValue("id"), *body.Favourite, s.Now())
	if err != nil {
		storeErr(w, err)
		return
	}
	p.Rating = nutrition.RateProductWith(p, s.Store.RatingSettings(userID(r)))
	writeJSON(w, 200, p)
}
