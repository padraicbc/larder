package api

import (
	"testing"

	"github.com/padraicbc/larder/internal/models"
)

func TestWorkouts(t *testing.T) {
	c0, _ := newTestServer(t) // clock: 2026-03-10
	c := signUp(t, c0.srv, "workout@example.com")
	var kinds struct {
		Kinds   []string `json:"kinds"`
		BuiltIn []string `json:"builtIn"`
	}
	c.do("GET", "/api/workout-kinds", nil, &kinds)
	if len(kinds.Kinds) < 2 || kinds.Kinds[0] != "Running" || kinds.Kinds[1] != "Weights" {
		t.Fatalf("defaults should start with running and weights: %v", kinds.Kinds)
	}

	add := func(date, kind string, mins int) models.Workout {
		var w models.Workout
		if code := c.do("POST", "/api/workouts", map[string]any{"date": date, "kind": kind, "minutes": mins}, &w); code != 201 {
			t.Fatalf("%s %s = %d", date, kind, code)
		}
		return w
	}
	run := add("2026-03-02", "running", 30) // case-insensitive match keeps the saved spelling
	if run.Kind != "Running" {
		t.Errorf("kind = %q", run.Kind)
	}
	add("2026-03-02", "Weights", 45) // two on one day
	var withCal models.Workout
	c.do("POST", "/api/workouts", map[string]any{"date": "2026-03-03", "kind": "Running", "minutes": 40, "calories": 412}, &withCal)
	if withCal.Calories != 412 {
		t.Errorf("calories = %+v", withCal)
	}
	for _, bad := range []int{-1, 20001} {
		if c.do("POST", "/api/workouts", map[string]any{"kind": "Running", "minutes": 30, "calories": bad}, nil) != 400 {
			t.Errorf("calories %d accepted", bad)
		}
	}
	add("2026-02-20", "Padel", 60) // new kind is remembered
	add("2026-03-05", "  padel  ", 90)
	c.do("GET", "/api/workout-kinds", nil, &kinds)
	if len(kinds.Kinds) != len(kinds.BuiltIn)+1 || kinds.Kinds[len(kinds.Kinds)-1] != "Padel" {
		t.Errorf("custom kind not saved once: %v", kinds.Kinds)
	}

	var march []models.Workout
	c.do("GET", "/api/workouts?from=2026-03-01&to=2026-03-31", nil, &march)
	if len(march) != 4 || march[0].Date != "2026-03-02" || march[3].Kind != "Padel" || march[3].Minutes != 90 || march[2].Calories != 412 {
		t.Fatalf("march = %+v", march)
	}
	for _, bad := range []map[string]any{{"kind": "", "minutes": 30}, {"kind": "Run", "minutes": 0}, {"kind": "Run", "minutes": 5000}, {"kind": "Run", "minutes": 30, "date": "x"}} {
		if code := c.do("POST", "/api/workouts", bad, nil); code != 400 {
			t.Errorf("%v = %d", bad, code)
		}
	}
	if c.do("GET", "/api/workouts?from=bad", nil, nil) != 400 {
		t.Error("bad range accepted")
	}

	other := signUp(t, c0.srv, "workout2@example.com")
	var theirs []models.Workout
	other.do("GET", "/api/workouts", nil, &theirs)
	if len(theirs) != 0 || other.do("DELETE", "/api/workouts/"+run.ID, nil, nil) != 404 {
		t.Error("workouts leaked across accounts")
	}
	if c.do("DELETE", "/api/workouts/"+run.ID, nil, nil) != 204 {
		t.Error("delete failed")
	}
	if c.do("DELETE", "/api/workout-kinds/Running", nil, nil) != 404 {
		t.Error("built-in kinds can't be removed")
	}
	if c.do("DELETE", "/api/workout-kinds/padel", nil, nil) != 204 {
		t.Error("custom kind delete failed")
	}
	c.do("GET", "/api/workouts?from=2026-03-01&to=2026-03-31", nil, &march)
	if len(march) != 3 || march[2].Kind != "Padel" {
		t.Errorf("logged sessions keep their kind after the kind is removed: %+v", march)
	}
}
