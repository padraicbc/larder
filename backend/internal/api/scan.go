package api

import (
	"errors"
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/nutrition"
)

// rated attaches the rating, worked out now with this person's rules.
func (s *Server) rated(uid string, sc models.Scan) models.Scan {
	sc.Rating = nutrition.RateScanWith(sc, s.Store.RatingSettings(uid))
	return sc
}

// scanBarcode is the shop check: your saved products, then your saved scans,
// then (only if an admin enabled it in Settings) Open Food Facts. Nothing is saved.
func (s *Server) scanBarcode(w http.ResponseWriter, r *http.Request) {
	code, ok := nutrition.NormalizeBarcode(r.PathValue("barcode"))
	if !ok {
		writeErr(w, 400, "that barcode isn't valid; check the digits (EAN-8, UPC-A or EAN-13)")
		return
	}
	uid := userID(r)
	if p, err := s.Store.ProductByBarcode(uid, code); err == nil {
		sc := nutrition.ScanFromProduct(p)
		if strings.TrimSpace(sc.Ingredients) == "" {
			// A product saved without ingredients shouldn't hide ingredients you've since photographed.
			if saved, err := s.Store.ScanByBarcode(uid, code); err == nil && strings.TrimSpace(saved.Ingredients) != "" {
				sc.Ingredients = saved.Ingredients
			}
		}
		writeJSON(w, 200, s.rated(uid, sc))
		return
	}
	if sc, err := s.Store.ScanByBarcode(uid, code); err == nil {
		writeJSON(w, 200, s.rated(uid, sc))
		return
	}
	if !s.Store.Lookup().OpenFoodFacts {
		writeErr(w, 404, "not in your saved products or scans. Turn on online barcode lookup in Settings to search Open Food Facts, or photograph the ingredients")
		return
	}
	found, err := nutrition.LookupOpenFoodFacts(r.Context(), s.HTTP, s.OFFBase, code)
	if errors.Is(err, nutrition.ErrBarcodeUnknown) {
		writeErr(w, 404, err.Error()+"; photograph the ingredients instead")
		return
	}
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, s.rated(uid, nutrition.ScanFromLabel(found)))
}

// scanIngredients rates a product from a photo of its ingredients list.
func (s *Server) scanIngredients(w http.ResponseWriter, r *http.Request) {
	images, ok := readPhotos(w, r)
	if !ok {
		return
	}
	if len(images) != 1 {
		writeErr(w, 400, "add one clear photo of the ingredients list")
		return
	}
	c, err := s.visionClient()
	if err != nil {
		writeErr(w, 400, "model not configured for photos: "+err.Error())
		return
	}
	sc, err := nutrition.ReadIngredientsPhoto(r.Context(), c, images)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, s.rated(userID(r), sc))
}

func (s *Server) listScans(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	scans := s.Store.Scans(uid)
	for i := range scans {
		scans[i] = s.rated(uid, scans[i])
	}
	writeJSON(w, 200, scans)
}

// saveScan keeps a checked product so it can be looked at again later.
func (s *Server) saveScan(w http.ResponseWriter, r *http.Request) {
	var sc models.Scan
	if !decode(w, r, &sc) {
		return
	}
	sc.Name = strings.TrimSpace(sc.Name)
	if sc.Name == "" {
		sc.Name = "Scanned product"
	}
	sc.Ingredients = strings.TrimSpace(sc.Ingredients)
	if sc.Barcode != "" {
		code, ok := nutrition.NormalizeBarcode(sc.Barcode)
		if !ok {
			writeErr(w, 400, "that barcode isn't valid; check the digits (EAN-8, UPC-A or EAN-13)")
			return
		}
		sc.Barcode = code
	}
	if sc.Source != "saved" && sc.Source != "openfoodfacts" {
		sc.Source = "photo"
	}
	if sc.HasNutrition && (sc.BasisQuantity <= 0 || (sc.BasisUnit != "g" && sc.BasisUnit != "ml" && sc.BasisUnit != "serving")) {
		sc.HasNutrition = false
	}
	if sc.Ingredients == "" && !sc.HasNutrition {
		writeErr(w, 400, "there's nothing to save yet; add the ingredients or the nutrition")
		return
	}
	uid := userID(r)
	for _, v := range []float64{sc.Macros.Calories, sc.Macros.Protein, sc.Macros.Carbs, sc.Macros.Fat, sc.Macros.Fiber} {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			writeErr(w, 400, "nutrition values must be zero or greater")
			return
		}
	}

	// Ingredients AND nutrition known: it is a real product now, not a draft. It lives in
	// Products (loggable, editable, rated) and the unfinished scan is removed.
	if sc.HasNutrition && sc.Ingredients != "" {
		var prod models.Product
		var err error
		if sc.ProductID != "" { // already one of your products: just keep its ingredients up to date
			prod, err = s.Store.SetProductIngredients(uid, sc.ProductID, sc.Ingredients, s.Now())
		} else {
			source := ""
			if sc.Source == "openfoodfacts" {
				source = "openfoodfacts"
			}
			prod, err = s.Store.PutProduct(uid, models.Product{
				Name: sc.Name, Barcode: sc.Barcode, Ingredients: sc.Ingredients,
				BasisQuantity: sc.BasisQuantity, BasisUnit: sc.BasisUnit, Macros: sc.Macros,
				FromNutritionLabel: true, Confidence: "medium", Notes: sc.Notes, Source: source,
			}, s.Now())
		}
		if err != nil {
			storeErr(w, err)
			return
		}
		if sc.ID != "" {
			_ = s.Store.DeleteScan(uid, sc.ID)
		}
		if sc.Barcode != "" {
			if draft, err := s.Store.ScanByBarcode(uid, sc.Barcode); err == nil {
				_ = s.Store.DeleteScan(uid, draft.ID)
			}
		}
		writeJSON(w, 201, s.rated(uid, nutrition.ScanFromProduct(prod)))
		return
	}

	saved, err := s.Store.PutScan(uid, sc, s.Now())
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 201, s.rated(uid, saved))
}

func (s *Server) deleteScan(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteScan(userID(r), r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// rateScan rates a scan as it stands, e.g. after the ingredients were added from a photo.
func (s *Server) rateScan(w http.ResponseWriter, r *http.Request) {
	var sc models.Scan
	if !decode(w, r, &sc) {
		return
	}
	sc.Ingredients = strings.TrimSpace(sc.Ingredients)
	if sc.HasNutrition && (sc.BasisQuantity <= 0 || (sc.BasisUnit != "g" && sc.BasisUnit != "ml" && sc.BasisUnit != "serving")) {
		sc.HasNutrition = false
	}
	if sc.Ingredients == "" && !sc.HasNutrition {
		writeErr(w, 400, "there's nothing to rate yet; add the ingredients and the nutrition")
		return
	}
	writeJSON(w, 200, s.rated(userID(r), sc)) // rating is null until both are known
}

// searchScans is the shop-check lookup: type a few letters or digits and get your matching
// products and unfinished scans, each rated now with your rules. It saves nothing.
func (s *Server) searchScans(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	out := []models.Scan{}
	if len([]rune(q)) < 2 {
		writeJSON(w, 200, out)
		return
	}
	uid := userID(r)
	match := func(name, barcode string) bool {
		return strings.Contains(strings.ToLower(name), q) || (barcode != "" && strings.Contains(barcode, q))
	}
	seen := map[string]bool{}
	for _, p := range s.Store.Products(uid) {
		if match(p.Name, p.Barcode) {
			out = append(out, s.rated(uid, nutrition.ScanFromProduct(p)))
			if p.Barcode != "" {
				seen[p.Barcode] = true
			}
		}
	}
	for _, sc := range s.Store.Scans(uid) {
		if match(sc.Name, sc.Barcode) && !(sc.Barcode != "" && seen[sc.Barcode]) {
			out = append(out, s.rated(uid, sc))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := strings.HasPrefix(strings.ToLower(out[i].Name), q), strings.HasPrefix(strings.ToLower(out[j].Name), q)
		if pi != pj {
			return pi
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	if len(out) > 12 {
		out = out[:12]
	}
	writeJSON(w, 200, out)
}
