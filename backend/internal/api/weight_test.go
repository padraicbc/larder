package api

import (
	"testing"

	"github.com/padraicbc/larder/internal/models"
)

func TestWeightEntries(t *testing.T) {
	c0, _ := newTestServer(t) // test clock: 2026-03-10
	c := signUp(t, c0.srv, "weight@example.com")
	for _, w := range []map[string]any{{"date": "2026-03-01", "kg": 82.46}, {"date": "2026-01-05", "kg": 85}, {"date": "2026-03-10", "kg": 81.9}} {
		if code := c.do("PUT", "/api/weight", w, nil); code != 200 {
			t.Fatalf("%v = %d", w, code)
		}
	}
	// Same day replaces; omitted date means today.
	var got models.WeightEntry
	c.do("PUT", "/api/weight", map[string]any{"kg": 81.5}, &got)
	if got.Date != "2026-03-10" || got.Kg != 81.5 {
		t.Errorf("today = %+v", got)
	}
	var list []models.WeightEntry
	c.do("GET", "/api/weight", nil, &list)
	if len(list) != 3 || list[0].Date != "2026-01-05" || list[1].Kg != 82.5 || list[2].Kg != 81.5 {
		t.Fatalf("list should be oldest first, one per day, rounded to 0.1: %+v", list)
	}
	for _, bad := range []map[string]any{{"kg": 0}, {"kg": 5}, {"kg": 900}, {"date": "2026-03-11", "kg": 80}, {"date": "nope", "kg": 80}} {
		if code := c.do("PUT", "/api/weight", bad, nil); code != 400 {
			t.Errorf("%v = %d", bad, code)
		}
	}
	other := signUp(t, c0.srv, "weight2@example.com")
	other.do("GET", "/api/weight", nil, &list)
	if len(list) != 0 || other.do("DELETE", "/api/weight/2026-03-01", nil, nil) != 404 {
		t.Error("weights leaked across accounts")
	}
	if code := c.do("DELETE", "/api/weight/2026-03-01", nil, nil); code != 204 {
		t.Errorf("delete = %d", code)
	}
	c.do("GET", "/api/weight", nil, &list)
	if len(list) != 2 {
		t.Errorf("after delete: %+v", list)
	}
}
