package api

import (
	"errors"
	"math"
	"net/http"

	"github.com/you/nutrition-app/internal/models"
	"github.com/you/nutrition-app/internal/store"
)

func adjustedTargets(base models.Macros, adjustment models.DayAdjustment) models.Macros {
	// If the normal target is later lowered below an old adjustment, keep the
	// day usable while preserving the saved adjustment for review.
	base.Calories = math.Max(1, base.Calories+adjustment.Calories)
	return base
}

func (s *Server) putDayTarget(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = s.today()
	}
	if !validDate(date) {
		writeErr(w, 400, "date must be YYYY-MM-DD")
		return
	}
	var adjustment models.DayAdjustment
	if !decode(w, r, &adjustment) {
		return
	}
	err := s.Store.SetDayAdjustment(userID(r), date, adjustment)
	if errors.Is(err, store.ErrInvalidTarget) {
		writeErr(w, 400, err.Error())
		return
	}
	if err != nil {
		storeErr(w, err)
		return
	}
	// Return the same daily summary used by the log page.
	query := r.URL.Query()
	query.Set("date", date)
	r.URL.RawQuery = query.Encode()
	s.getLog(w, r)
}
