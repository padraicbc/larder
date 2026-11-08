package nutrition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/padraicbc/larder/internal/models"
)

// DefaultOFFBase is the public Open Food Facts API.
const DefaultOFFBase = "https://world.openfoodfacts.org"

var ErrBarcodeUnknown = errors.New("that barcode isn't in Open Food Facts")

// LookupOpenFoodFacts fetches a product by barcode. Only the barcode leaves
// the device. Nutrition is taken per 100 g (or per 100 ml for drinks).
func LookupOpenFoodFacts(ctx context.Context, hc *http.Client, base, barcode string) (LabelEstimate, error) {
	var out LabelEstimate
	if base == "" {
		base = DefaultOFFBase
	}
	u := strings.TrimRight(base, "/") + "/api/v2/product/" + url.PathEscape(barcode) +
		".json?lang=en&fields=product_name,generic_name,brands,quantity,ingredients_text,ingredients_text_en,ingredients,nutriments"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("User-Agent", "Larder/1.0 (self-hosted nutrition tracker)")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return out, fmt.Errorf("couldn't reach Open Food Facts: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return out, ErrBarcodeUnknown
	}
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("Open Food Facts returned HTTP %d", resp.StatusCode)
	}
	var body struct {
		Status  any `json:"status"`
		Product *struct {
			Name          string `json:"product_name"`
			Generic       string `json:"generic_name"`
			Brands        string `json:"brands"`
			Quantity      string `json:"quantity"`
			Ingredients   string `json:"ingredients_text"`
			IngredientsEN string `json:"ingredients_text_en"`
			Parts         []struct {
				Text string `json:"text"`
			} `json:"ingredients"`
			Nutriments map[string]any `json:"nutriments"`
		} `json:"product"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil {
		return out, errors.New("Open Food Facts sent an unreadable reply")
	}
	if body.Product == nil || fmt.Sprint(body.Status) == "0" {
		return out, ErrBarcodeUnknown
	}
	p := body.Product
	n := p.Nutriments
	kcal, hasKcal := offNum(n, "energy-kcal_100g")
	if !hasKcal {
		if kj, ok := offNum(n, "energy_100g"); ok {
			kcal, hasKcal = kj/4.184, true
		}
	}
	protein, hasP := offNum(n, "proteins_100g")
	carbs, hasC := offNum(n, "carbohydrates_100g")
	fat, hasF := offNum(n, "fat_100g")
	if !hasKcal || !hasP || !hasC || !hasF {
		return out, errors.New("Open Food Facts has this product but not its full nutrition; photograph the label instead")
	}
	fiber, hasFib := offNum(n, "fiber_100g")
	var saturated *float64
	if v, ok := offNum(n, "saturated-fat_100g"); ok && v <= fat {
		saturated = &v
	}

	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = strings.TrimSpace(p.Generic)
	}
	if name == "" {
		name = "Product " + barcode
	}
	if b := strings.TrimSpace(strings.SplitN(p.Brands, ",", 2)[0]); b != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(b)) {
		name = b + " " + name
	}
	unit := "g"
	if q := strings.ToLower(strings.ReplaceAll(p.Quantity, " ", "")); strings.HasSuffix(q, "ml") || strings.HasSuffix(q, "cl") || (strings.HasSuffix(q, "l") && !strings.HasSuffix(q, "gl")) {
		unit = "ml"
	}
	notes := "From Open Food Facts (community-edited): check these values against the pack."
	if !hasFib {
		notes += " Fibre isn't listed, so it's shown as 0."
	}
	// English first: our rules read English, and the default field is in whatever
	// language the first contributor used (Coca-Cola's came back in Spanish).
	ingredients, warning := strings.TrimSpace(p.IngredientsEN), ""
	if ingredients == "" {
		if ingredients = strings.TrimSpace(p.Ingredients); ingredients == "" {
			var parts []string
			for _, part := range p.Parts {
				if t := strings.TrimSpace(part.Text); t != "" {
					parts = append(parts, t)
				}
			}
			ingredients = strings.Join(parts, ", ")
		}
		if ingredients == "" {
			warning = "Open Food Facts has no ingredients for this product, so additives and sugars can't be checked. Photograph the ingredients to rate it properly."
		} else {
			warning = "Open Food Facts only has these ingredients in another language, so some additives or sugars may be missed. Photograph the ingredients to be sure."
		}
	}
	out = LabelEstimate{
		Estimate: Estimate{
			Name:       name,
			Macros:     models.Macros{Calories: kcal, Protein: protein, Carbs: carbs, Fat: fat, Fiber: fiber, SaturatedFat: saturated},
			Confidence: "medium",
			Notes:      notes,
		},
		Ingredients:        ingredients,
		IngredientsWarning: warning,
		BasisQuantity:      100,
		BasisUnit:          unit,
		FromNutritionLabel: true,
		Barcode:            barcode,
		Source:             "openfoodfacts",
	}
	return out, nil
}

func offNum(m map[string]any, key string) (float64, bool) {
	var f float64
	switch v := m[key].(type) {
	case float64:
		f = v
	case string:
		x, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, false
		}
		f = x
	default:
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return 0, false
	}
	return f, true
}
