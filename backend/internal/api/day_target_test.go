package api

import (
	"github.com/you/nutrition-app/internal/models"
	"testing"
)

func TestDayCalorieAdjustments(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "active@example.com")
	var day dayLog
	if code := c.do("PUT", "/api/log/target?date=2026-03-10", models.DayAdjustment{Calories: 350, Note: "Gym session"}, &day); code != 200 {
		t.Fatalf("adjust = %d", code)
	}
	if day.Targets.Calories != 2350 || day.BaseTargets.Calories != 2000 || day.Targets.Protein != 120 || day.Adjustment.Note != "Gym session" {
		t.Fatalf("adjusted day = %+v", day)
	}
	c.do("GET", "/api/log?date=2026-03-11", nil, &day)
	if day.Targets.Calories != 2000 || day.Adjustment.Calories != 0 {
		t.Fatal("adjustment affected another day")
	}
	var history struct {
		Days []struct {
			Date    string        `json:"date"`
			Targets models.Macros `json:"targets"`
		} `json:"days"`
	}
	c.do("GET", "/api/log/history?days=2&end=2026-03-11", nil, &history)
	if len(history.Days) != 2 || history.Days[0].Targets.Calories != 2350 || history.Days[1].Targets.Calories != 2000 {
		t.Fatalf("history = %+v", history)
	}
	stranger := signUp(t, c0.srv, "other-active@example.com")
	stranger.do("GET", "/api/log?date=2026-03-10", nil, &day)
	if day.Targets.Calories != 2000 {
		t.Fatal("another user's adjustment visible")
	}
	if code := c.do("PUT", "/api/log/target?date=2026-03-10", models.DayAdjustment{Calories: -200}, &day); code != 200 || day.Targets.Calories != 1800 {
		t.Fatalf("lower target: %d %+v", code, day)
	}
	if code := c.do("PUT", "/api/log/target?date=2026-03-10", models.DayAdjustment{Calories: -2000}, nil); code != 400 {
		t.Errorf("nonpositive target = %d", code)
	}
	if code := c.do("PUT", "/api/log/target?date=bad", models.DayAdjustment{}, nil); code != 400 {
		t.Errorf("bad date = %d", code)
	}
	c.do("GET", "/api/log?date=2026-03-10", nil, &day)
	if day.Targets.Calories != 1800 {
		t.Fatal("invalid update changed the target")
	}
	c.do("PUT", "/api/log/target?date=2026-03-10", models.DayAdjustment{}, &day)
	if day.Targets.Calories != 2000 || day.Adjustment.Note != "" {
		t.Fatal("reset failed")
	}
}
