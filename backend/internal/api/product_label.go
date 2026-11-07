package api

import (
	"net/http"

	"github.com/padraicbc/larder/internal/nutrition"
)

func (s *Server) readProductLabel(w http.ResponseWriter, r *http.Request) {
	images, ok := readPhotos(w, r)
	if !ok {
		return
	}
	if len(images) != 1 {
		writeErr(w, 400, "add one clear photo of the product label")
		return
	}
	c, err := s.visionClient()
	if err != nil {
		writeErr(w, 400, "model not configured for photos: "+err.Error())
		return
	}
	result, err := nutrition.ReadProductLabel(r.Context(), c, images)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	// Keep printed precision; round only when the chosen portion is logged.
	writeJSON(w, 200, result)
}
