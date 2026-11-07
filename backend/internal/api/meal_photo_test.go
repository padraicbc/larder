package api

import (
	"testing"

	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/nutrition"
)

func TestMealPhotoReviewAndLog(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "meal@example.com")
	var est nutrition.PhotoEstimate
	if code := c.uploadTo("/api/log/photo", map[string][]byte{"meal.png": pngBytes(t)}, &est); code != 200 {
		t.Fatalf("photo = %d", code)
	}
	if len(est.Items) != 2 || est.Items[0].Name != "eggs" || est.Macros.Calories <= 0 {
		t.Fatalf("estimate = %+v", est)
	}
	var day struct {
		Entries []models.LogEntry `json:"entries"`
	}
	c.do("GET", "/api/log?date=2026-11-05", nil, &day)
	if len(day.Entries) != 0 {
		t.Fatal("photo analysis saved without review")
	}
	est.Items[0].Quantity = "3 large"
	est.Items = append(est.Items, models.MealItem{Name: "butter", Quantity: "10 g"})
	var entry models.LogEntry
	if code := c.do("POST", "/api/log", map[string]any{
		"date": "2026-11-05", "mealType": "breakfast", "name": est.Name,
		"servings": 1, "macros": est.Macros, "items": est.Items,
	}, &entry); code != 201 {
		t.Fatalf("log = %d", code)
	}
	c.do("GET", "/api/log?date=2026-11-05", nil, &day)
	if len(day.Entries) != 1 || len(day.Entries[0].Items) != 3 || day.Entries[0].Items[0].Quantity != "3 large" {
		t.Fatalf("reviewed items not saved: %+v", day)
	}
	for _, files := range []map[string][]byte{
		{}, {"bad.jpg": []byte("not an image")}, {"a.png": pngBytes(t), "b.png": pngBytes(t)},
	} {
		if code := c.uploadTo("/api/log/photo", files, nil); code != 400 {
			t.Errorf("invalid upload = %d", code)
		}
	}
}
