package nutrition

import (
	"context"
	"github.com/you/nutrition-app/internal/llm"
	"strings"
	"testing"
)

func TestReadProductLabelSourcesAndPrecision(t *testing.T) {
	images := []llm.Image{{MediaType: "image/png", Data: []byte{1}}}
	printed := `{"name":"Oats","ingredients":"Wholegrain oats","basisQuantity":100,"basisUnit":"g","fromNutritionLabel":true,"macros":{"calories":379.5,"protein":13.2,"carbs":60,"fat":7,"fiber":10},"confidence":"high"}`
	got, err := ReadProductLabel(context.Background(), mealVisionReply(printed), images)
	if err != nil || !got.FromNutritionLabel || got.Macros.Calories != 379.5 || got.BasisQuantity != 100 || got.BasisUnit != "g" {
		t.Fatalf("printed label: %+v %v", got, err)
	}
	estimated := strings.Replace(printed, `"fromNutritionLabel":true`, `"fromNutritionLabel":false`, 1)
	got, err = ReadProductLabel(context.Background(), mealVisionReply(estimated), images)
	if err == nil {
		t.Fatalf("ingredients-only nutrition must not be accepted as label values: %+v", got)
	}
	serving := strings.Replace(strings.Replace(printed, `"basisQuantity":100`, `"basisQuantity":1`, 1), `"basisUnit":"g"`, `"basisUnit":"serving"`, 1)
	got, err = ReadProductLabel(context.Background(), mealVisionReply(serving), images)
	if err != nil || got.BasisUnit != "serving" || got.BasisQuantity != 1 {
		t.Fatalf("serving label: %+v %v", got, err)
	}
}

func TestReadProductLabelRejectsUnusableLabels(t *testing.T) {
	images := []llm.Image{{MediaType: "image/png", Data: []byte{1}}}
	for _, reply := range []string{
		`not JSON`, `{"name":""}`,
		`{"name":"Oats","basisQuantity":0,"basisUnit":"g"}`,
		`{"name":"Oats","basisQuantity":100,"basisUnit":"unknown"}`,
		`{"name":"Oats","basisQuantity":100,"basisUnit":"g","macros":{"calories":-10}}`,
	} {
		if _, err := ReadProductLabel(context.Background(), mealVisionReply(reply), images); err == nil {
			t.Errorf("accepted unusable label: %s", reply)
		}
	}
	if _, err := ReadProductLabel(context.Background(), llm.Mock{}, nil); err == nil {
		t.Fatal("accepted no photo")
	}
}
