package nutrition

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/you/nutrition-app/internal/llm"
	"github.com/you/nutrition-app/internal/models"
)

type PhotoEstimate struct {
	Estimate
	Items []models.MealItem `json:"items"`
}

func EstimateMealPhoto(ctx context.Context, c llm.VisionClient, images []llm.Image) (PhotoEstimate, error) {
	var out PhotoEstimate
	if len(images) != 1 {
		return out, errors.New("add one photo of your meal")
	}
	err := llm.CompleteJSONWithImages(ctx, c, "Identify a meal and estimate nutrition. Respond with ONLY JSON.", `TASK: meal_photo
Identify the foods on this plate and estimate the quantity of each in grams or everyday measures.
Do not invent foods you cannot see. Include visible sauces and drinks. If no meal is identifiable, return an empty items array.
Macros must be totals for ALL the food shown, treated as ONE serving. Be honest about uncertain portions.
Return {"name":"meal name","items":[{"name":"food","quantity":"150 g"}],"macros":{"calories":0,"protein":0,"carbs":0,"fat":0,"fiber":0},"confidence":"high|medium|low","notes":"portion assumptions and uncertainty"}.`, images, &out)
	if err != nil {
		return out, err
	}
	items := []models.MealItem{}
	for _, item := range out.Items {
		item.Name = strings.TrimSpace(item.Name)
		item.Quantity = strings.TrimSpace(item.Quantity)
		if item.Name != "" {
			items = append(items, item)
		}
	}
	out.Items = items
	if len(items) == 0 {
		return out, errors.New("no meal identified; try a clearer photo or describe your meal")
	}
	for _, n := range []float64{out.Macros.Calories, out.Macros.Protein, out.Macros.Carbs, out.Macros.Fat, out.Macros.Fiber} {
		if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
			return out, errors.New("model returned invalid nutrition; try again")
		}
	}
	if strings.TrimSpace(out.Name) == "" {
		out.Name = items[0].Name
	}
	return out, nil
}
