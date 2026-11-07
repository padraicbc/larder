package api

import (
	"net/http"

	"github.com/padraicbc/larder/internal/nutrition"
)

func (s *Server) estimatePhoto(w http.ResponseWriter, r *http.Request) {
	images, ok := readPhotos(w, r)
	if !ok {
		return
	}
	if len(images) != 1 {
		writeErr(w, 400, "add one photo of your meal")
		return
	}
	c, err := s.visionClient()
	if err != nil {
		writeErr(w, 400, "model not configured for photos: "+err.Error())
		return
	}
	est, err := nutrition.EstimateMealPhoto(r.Context(), c, images)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	est.Macros = roundMacros(est.Macros)
	writeJSON(w, 200, est)
}
