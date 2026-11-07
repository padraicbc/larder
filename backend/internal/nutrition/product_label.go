package nutrition

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/padraicbc/larder/internal/llm"
)

// Macros describe BasisQuantity in BasisUnit, not the amount the user ate.
type LabelEstimate struct {
	Estimate
	Ingredients        string  `json:"ingredients"`
	BasisQuantity      float64 `json:"basisQuantity"`
	BasisUnit          string  `json:"basisUnit"` // g | ml | serving
	FromNutritionLabel bool    `json:"fromNutritionLabel"`
	Barcode            string  `json:"barcode,omitempty"`
	Source             string  `json:"source,omitempty"`
	IngredientsWarning string  `json:"ingredientsWarning,omitempty"`
}

func ReadProductLabel(ctx context.Context, c llm.VisionClient, images []llm.Image) (LabelEstimate, error) {
	var out LabelEstimate
	if len(images) != 1 {
		return out, errors.New("add one clear photo of the product label")
	}
	err := llm.CompleteJSONWithImages(ctx, c, "Read food product labels accurately. Respond with ONLY JSON.", `TASK: product_label
Read the product name, ingredients list and nutrition panel visible in this photo.
- If a nutrition panel is readable, copy its printed values instead of estimating. Prefer the per-100-g or per-100-ml column over per-serving columns. Keep every macro on the SAME basis. Do not confuse sugars with total carbs, salt with fat, or kJ with kcal. Convert kJ to kcal only if kcal is not printed (divide kJ by 4.184).
- If only a per-serving panel is available, use basisQuantity:1 and basisUnit:"serving". Mention any printed serving weight in notes. Do not invent a weight.
- A readable nutrition panel is required. If there is only an ingredients list, or calories, protein, total carbs or fat cannot be read, return an empty name and fromNutritionLabel:false. Do not estimate missing values from ingredients or generic products.
- Ingredients should be transcribed as text, without inventing missing text. Use zero for unreported fibre but explain this in notes; don't imply the product is fibre-free.
- If a barcode is printed with its digits visible (EAN-13, EAN-8 or UPC-A), copy the digits exactly into "barcode"; otherwise use "". Never guess digits.
Return {"name":"product name or a descriptive name","barcode":"digits or empty","ingredients":"visible ingredients","basisQuantity":100,"basisUnit":"g|ml|serving","fromNutritionLabel":true,"macros":{"calories":0,"protein":0,"carbs":0,"fat":0,"fiber":0},"confidence":"high|medium|low","notes":"source, serving size, missing values or uncertainty"}.`, images, &out)
	if err != nil {
		return out, err
	}
	out.Barcode, _ = NormalizeBarcode(out.Barcode) // a misread barcode is dropped, not guessed
	out.Name = strings.TrimSpace(out.Name)
	out.BasisUnit = strings.ToLower(strings.TrimSpace(out.BasisUnit))
	if out.Name == "" {
		return out, errors.New("no readable nutrition panel found; try a closer photo showing calories, protein, carbs and fat")
	}
	if out.BasisQuantity <= 0 || math.IsNaN(out.BasisQuantity) || math.IsInf(out.BasisQuantity, 0) {
		return out, errors.New("couldn't read the label's portion basis; try a clearer photo")
	}
	if out.BasisUnit != "g" && out.BasisUnit != "ml" && out.BasisUnit != "serving" {
		return out, errors.New("couldn't read the label's units; try a clearer photo")
	}
	for _, v := range []float64{out.Macros.Calories, out.Macros.Protein, out.Macros.Carbs, out.Macros.Fat, out.Macros.Fiber} {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return out, errors.New("label returned invalid nutrition; try again")
		}
	}
	if !out.FromNutritionLabel {
		return out, errors.New("a readable nutrition panel is needed; ingredients alone cannot give the product's nutrition values")
	}
	return out, nil
}
