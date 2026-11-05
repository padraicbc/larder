package api

import (
	"github.com/you/nutrition-app/internal/models"
	"testing"
	"time"
)

func TestRepeatLogCopiesSavedTotals(t *testing.T) {
	c0, s := newTestServer(t)
	c := signUp(t, c0.srv, "porridge@example.com")
	var original models.LogEntry
	if code := c.do("POST", "/api/log", map[string]any{
		"date": "2026-03-09", "mealType": "breakfast", "name": "Porridge", "servings": 2,
		"macros": models.Macros{Calories: 250, Protein: 10},
		"items":  []models.MealItem{{Name: "oats", Quantity: "60 g"}},
	}, &original); code != 201 {
		t.Fatal(code)
	}
	var copied models.LogEntry
	if code := c.do("POST", "/api/log/"+original.ID+"/repeat", map[string]string{"date": "2026-03-10", "mealType": "lunch"}, &copied); code != 201 {
		t.Fatal(code)
	}
	if copied.ID == original.ID || copied.Date != "2026-03-10" || copied.MealType != "lunch" || copied.Servings != 2 || copied.Macros.Calories != 500 || len(copied.Items) != 1 || copied.Items[0].Quantity != "60 g" {
		t.Fatalf("wrong copy: %+v", copied)
	}
	var day dayLog
	c.do("GET", "/api/log?date=2026-03-09", nil, &day)
	if len(day.Entries) != 1 || day.Entries[0].MealType != "breakfast" {
		t.Fatal("original changed")
	}
	stranger := signUp(t, c0.srv, "other@example.com")
	if code := stranger.do("POST", "/api/log/"+original.ID+"/repeat", map[string]string{}, nil); code != 404 {
		t.Errorf("cross-user repeat = %d", code)
	}
	for _, body := range []map[string]string{{"date": "bad"}, {"mealType": "bad"}} {
		if code := c.do("POST", "/api/log/"+original.ID+"/repeat", body, nil); code != 400 {
			t.Errorf("invalid input = %d", code)
		}
	}
	// A missing date defaults to today, and each repeat gets a fresh identity.
	s.Now = func() time.Time { return time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC) }
	var next models.LogEntry
	c.do("POST", "/api/log/"+original.ID+"/repeat", map[string]string{}, &next)
	if next.Date != "2026-03-11" || next.ID == copied.ID {
		t.Fatalf("default repeat: %+v", next)
	}
}

func TestRecentMealsPreferFrequentAndLatest(t *testing.T) {
	c0, s := newTestServer(t)
	c := signUp(t, c0.srv, "regular@example.com")
	for i, name := range []string{"Porridge", " porridge ", "Soup"} {
		s.Now = func() time.Time { return time.Date(2026, 3, 10, 12, i, 0, 0, time.UTC) }
		c.do("POST", "/api/log", map[string]any{"name": name, "mealType": "breakfast", "servings": 1, "macros": models.Macros{Calories: float64(300 + i)}}, nil)
	}
	var recent []struct {
		models.LogEntry
		TimesLogged int `json:"timesLogged"`
	}
	if code := c.do("GET", "/api/log/recent", nil, &recent); code != 200 {
		t.Fatal(code)
	}
	if len(recent) != 2 || recent[0].TimesLogged != 2 || recent[0].Macros.Calories != 301 {
		t.Fatalf("recent = %+v", recent)
	}
	stranger := signUp(t, c0.srv, "empty@example.com")
	stranger.do("GET", "/api/log/recent", nil, &recent)
	if len(recent) != 0 {
		t.Fatal("other user's meals visible")
	}
}
