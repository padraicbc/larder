package api

import (
	"github.com/you/nutrition-app/internal/models"
	"github.com/you/nutrition-app/internal/nutrition"
	"testing"
)

func TestProductLabelEndpoint(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "label@example.com")
	var label nutrition.LabelEstimate
	if code := c.uploadTo("/api/log/label", map[string][]byte{"oats.png": pngBytes(t)}, &label); code != 200 {
		t.Fatalf("label scan = %d", code)
	}
	if label.Name == "" || label.BasisQuantity != 100 || label.BasisUnit != "g" || label.Confidence != "low" || !label.FromNutritionLabel {
		t.Fatalf("mock label = %+v", label)
	}
	var day dayLog
	c.do("GET", "/api/log", nil, &day)
	if len(day.Entries) != 0 {
		t.Fatal("scan saved a meal without review")
	}
	var entry models.LogEntry
	if code := c.do("POST", "/api/log", map[string]any{"name": label.Name, "mealType": "breakfast", "servings": 1, "macros": label.Macros.Scale(0.5), "items": []models.MealItem{{Name: label.Name, Quantity: "50 g"}}}, &entry); code != 201 {
		t.Fatalf("log = %d", code)
	}
	if entry.Macros.Calories != 190 || len(entry.Items) != 1 || entry.Items[0].Quantity != "50 g" {
		t.Fatalf("logged portion: %+v", entry)
	}
	for _, files := range []map[string][]byte{{}, {"bad.png": []byte("not an image")}, {"a.png": pngBytes(t), "b.png": pngBytes(t)}} {
		if code := c.uploadTo("/api/log/label", files, nil); code != 400 {
			t.Errorf("invalid upload = %d", code)
		}
	}
	if code := c0.uploadTo("/api/log/label", map[string][]byte{"label.png": pngBytes(t)}, nil); code != 401 {
		t.Errorf("unauthenticated scan = %d", code)
	}
}
