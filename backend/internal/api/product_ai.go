package api

import (
	"net/http"
	"strings"

	"github.com/padraicbc/larder/internal/nutrition"
)

func (s *Server) suggestProduct(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Description string `json:"description"`
	}
	if !decode(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Description) == "" || len([]rune(body.Description)) > 1000 {
		writeErr(w, 400, "describe the food in 1 to 1000 characters, including brand and variety if known")
		return
	}
	c, cfg, err := s.client()
	if err != nil {
		writeErr(w, 400, "model not configured: "+err.Error())
		return
	}
	if cfg.Provider == "" || cfg.Provider == "mock" {
		writeErr(w, 400, "choose a model in Settings to ask AI about a food")
		return
	}
	sc, err := nutrition.SuggestProduct(r.Context(), c, body.Description)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, s.rated(userID(r), sc)) // preview only; no store writes
}
