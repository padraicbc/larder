package api

import (
	"math"
	"net/http"

	"github.com/padraicbc/larder/internal/models"
)

func (s *Server) listWeights(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.Store.Weights(userID(r)))
}

// putWeight records (or corrects) the reading for one day.
func (s *Server) putWeight(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date string  `json:"date"`
		Kg   float64 `json:"kg"`
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
	if body.Date > s.today() {
		writeErr(w, 400, "you can't log a weight for a future day")
		return
	}
	if math.IsNaN(body.Kg) || body.Kg < 20 || body.Kg > 500 {
		writeErr(w, 400, "enter a weight between 20 and 500 kg")
		return
	}
	entry := models.WeightEntry{Date: body.Date, Kg: math.Round(body.Kg*10) / 10, CreatedAt: s.Now()}
	if err := s.Store.SetWeight(userID(r), entry); err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, entry)
}

func (s *Server) deleteWeight(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteWeight(userID(r), r.PathValue("date")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
