package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/store"
)

// Recent meals use the latest saved portions and nutrition for each meal name.
// Frequent meals come first so daily staples stay within easy reach.
func (s *Server) recentLogs(w http.ResponseWriter, r *http.Request) {
	type recent struct {
		models.LogEntry
		TimesLogged int `json:"timesLogged"`
	}
	meals := map[string]recent{}
	for _, e := range s.Store.LogBetween(userID(r), "", "9999-12-31") {
		key := strings.ToLower(strings.TrimSpace(e.Name)) + "\x00" + e.MealType
		previous := meals[key]
		latest := previous.LogEntry
		if previous.TimesLogged == 0 || e.CreatedAt.After(latest.CreatedAt) {
			latest = e
		}
		meals[key] = recent{latest, previous.TimesLogged + 1}
	}
	out := make([]recent, 0, len(meals))
	for _, e := range meals {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TimesLogged != out[j].TimesLogged {
			return out[i].TimesLogged > out[j].TimesLogged
		}
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > 30 {
		out = out[:30]
	}
	writeJSON(w, 200, out)
}

func (s *Server) repeatLog(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date     string `json:"date"`
		MealType string `json:"mealType"`
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
	if body.MealType != "" && body.MealType != "breakfast" && body.MealType != "lunch" && body.MealType != "dinner" && body.MealType != "snack" {
		writeErr(w, 400, "choose breakfast, lunch, dinner or snack")
		return
	}
	uid := userID(r)
	// Read through the same visibility rules as the daily log, including demo filtering.
	for _, e := range s.Store.LogBetween(uid, "", "9999-12-31") {
		if e.ID != r.PathValue("id") {
			continue
		}
		e.ID = store.NewID()
		e.Date = body.Date
		e.CreatedAt = s.Now()
		if body.MealType != "" {
			e.MealType = body.MealType
		}
		// Macros are already totals. Copy them directly instead of multiplying servings again.
		if err := s.Store.AddLog(uid, e); err != nil {
			storeErr(w, err)
			return
		}
		writeJSON(w, 201, e)
		return
	}
	storeErr(w, store.ErrNotFound)
}
