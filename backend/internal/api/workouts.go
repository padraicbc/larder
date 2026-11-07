package api

import (
	"net/http"
	"strings"

	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/store"
)

func (s *Server) listWorkouts(w http.ResponseWriter, r *http.Request) {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if from == "" {
		from = "0000-01-01"
	}
	if to == "" {
		to = "9999-12-31"
	}
	if !validDate(from) || !validDate(to) {
		writeErr(w, 400, "from and to must be YYYY-MM-DD")
		return
	}
	writeJSON(w, 200, s.Store.Workouts(userID(r), from, to))
}

func (s *Server) addWorkout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date     string `json:"date"`
		Kind     string `json:"kind"`
		Minutes  int    `json:"minutes"`
		Calories int    `json:"calories"`
		Note     string `json:"note"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Date == "" {
		body.Date = s.today()
	}
	if !validDate(body.Date) {
		writeErr(w, 400, "date must be YYYY-MM-DD")
		return
	}
	body.Kind = strings.Join(strings.Fields(body.Kind), " ")
	if body.Kind == "" || len([]rune(body.Kind)) > 40 {
		writeErr(w, 400, "choose a kind of workout (up to 40 characters)")
		return
	}
	if body.Minutes < 1 || body.Minutes > 1440 {
		writeErr(w, 400, "enter a duration between 1 and 1440 minutes")
		return
	}
	if body.Calories < 0 || body.Calories > 20000 {
		writeErr(w, 400, "calories burned should be between 0 and 20000, or left empty")
		return
	}
	note := strings.TrimSpace(body.Note)
	if len([]rune(note)) > 200 {
		writeErr(w, 400, "keep the note under 200 characters")
		return
	}
	saved, err := s.Store.AddWorkout(userID(r), models.Workout{
		ID: store.NewID(), Date: body.Date, Kind: body.Kind, Minutes: body.Minutes, Calories: body.Calories, Note: note, CreatedAt: s.Now(),
	})
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 201, saved)
}

func (s *Server) deleteWorkout(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteWorkout(userID(r), r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listWorkoutKinds(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"kinds": s.Store.WorkoutKinds(userID(r)), "builtIn": models.DefaultWorkoutKinds})
}

func (s *Server) deleteWorkoutKind(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteWorkoutKind(userID(r), r.PathValue("name")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
