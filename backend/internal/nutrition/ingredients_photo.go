package nutrition

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
)

// ReadIngredientsPhoto reads a product's ingredients list (and the nutrition
// panel or barcode, if they happen to be in the photo). It is the fallback for
// shop checks when a barcode can't be found. Only the ingredients are required.
func ReadIngredientsPhoto(ctx context.Context, c llm.VisionClient, images []llm.Image) (models.Scan, error) {
	if len(images) != 1 {
		return models.Scan{}, errors.New("add one clear photo of the ingredients list")
	}
	var out struct {
		Name          string        `json:"name"`
		Barcode       string        `json:"barcode"`
		Ingredients   string        `json:"ingredients"`
		HasNutrition  bool          `json:"hasNutrition"`
		BasisQuantity float64       `json:"basisQuantity"`
		BasisUnit     string        `json:"basisUnit"`
		Macros        models.Macros `json:"macros"`
		Notes         string        `json:"notes"`
	}
	err := llm.CompleteJSONWithImages(ctx, c, "Read food product labels accurately. Respond with ONLY JSON.", `TASK: ingredients_photo
Read this food product's ingredients list from the photo.
- Copy the ingredients exactly as printed, in order, comma-separated, including E-numbers and anything in brackets. Never invent, complete or translate ingredients you can't read. If the list is cut off or unreadable, return an empty string.
- name: the product name if visible, else "".
- barcode: the digits printed under a barcode if visible, else "".
- If a nutrition panel is also readable, copy its printed values (prefer the per-100 g/ml column), set hasNutrition true and keep every macro on the same basis. Otherwise hasNutrition false and all macros 0. Never estimate nutrition.
Return {"name":"","barcode":"","ingredients":"","hasNutrition":false,"basisQuantity":100,"basisUnit":"g|ml|serving","macros":{"calories":0,"protein":0,"carbs":0,"fat":0,"fiber":0},"notes":"anything unclear"}.`, images, &out)
	if err != nil {
		return models.Scan{}, err
	}
	sc := models.Scan{
		Name:        strings.TrimSpace(out.Name),
		Ingredients: strings.TrimSpace(out.Ingredients),
		Source:      "photo",
		Notes:       strings.TrimSpace(out.Notes),
	}
	if sc.Ingredients == "" {
		return sc, errors.New("couldn't read an ingredients list in that photo; get closer, keep it flat and in focus, and make sure the whole list is in the picture")
	}
	if sc.Name == "" {
		sc.Name = "Scanned product"
	}
	sc.Barcode, _ = NormalizeBarcode(out.Barcode)

	unit := strings.ToLower(strings.TrimSpace(out.BasisUnit))
	m := out.Macros
	ok := out.HasNutrition && out.BasisQuantity > 0 && !math.IsNaN(out.BasisQuantity) && !math.IsInf(out.BasisQuantity, 0) &&
		(unit == "g" || unit == "ml" || unit == "serving") && m.Calories > 0
	for _, v := range []float64{m.Calories, m.Protein, m.Carbs, m.Fat, m.Fiber} {
		ok = ok && v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
	}
	if ok { // an unusable panel is ignored; the ingredients still rate
		sc.HasNutrition, sc.BasisQuantity, sc.BasisUnit, sc.Macros = true, out.BasisQuantity, unit, m
	}
	return sc, nil
}
