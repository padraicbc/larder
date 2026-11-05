package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/you/nutrition-app/internal/llm"
	"github.com/you/nutrition-app/internal/models"
	"github.com/you/nutrition-app/internal/nutrition"
	"github.com/you/nutrition-app/internal/store"
)

const (
	maxPhotoBytes = 8 << 20  // per photo (the web app shrinks them first)
	maxScanBody   = 34 << 20 // 4 photos + form overhead
	maxPantryAdd  = 100
)

var photoTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true}

func (s *Server) getPantry(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.Store.Pantry(userID(r)))
}

func (s *Server) addPantry(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	var body struct {
		Items []models.PantryItem `json:"items"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.Items) == 0 || len(body.Items) > maxPantryAdd {
		writeErr(w, 400, "add between 1 and 100 items")
		return
	}
	var added int
	err := s.Store.UpdatePantry(uid, func(p map[string]models.PantryItem) error {
		added = nutrition.MergePantry(p, body.Items, store.NewID, s.Now())
		return nil
	})
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": s.Store.Pantry(uid), "added": added})
}

func (s *Server) updatePantryItem(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	var body struct {
		Name     *string `json:"name"`
		Quantity *string `json:"quantity"`
		Category *string `json:"category"`
	}
	if !decode(w, r, &body) {
		return
	}
	id := r.PathValue("id")
	err := s.Store.UpdatePantry(uid, func(p map[string]models.PantryItem) error {
		it, ok := p[id]
		if !ok {
			return store.ErrNotFound
		}
		if body.Name != nil {
			if n := strings.TrimSpace(*body.Name); n != "" {
				it.Name = strings.ToLower(n)
			}
		}
		if body.Quantity != nil {
			it.Quantity = strings.TrimSpace(*body.Quantity)
		}
		if body.Category != nil && *body.Category != "" {
			it.Category = strings.ToLower(*body.Category)
		}
		p[id] = it
		return nil
	})
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, s.Store.Pantry(uid))
}

func (s *Server) deletePantryItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.Store.UpdatePantry(userID(r), func(p map[string]models.PantryItem) error {
		if _, ok := p[id]; !ok {
			return store.ErrNotFound
		}
		delete(p, id)
		return nil
	})
	if err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) clearPantry(w http.ResponseWriter, r *http.Request) {
	err := s.Store.UpdatePantry(userID(r), func(p map[string]models.PantryItem) error {
		clear(p)
		return nil
	})
	if err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// scanPantry reads up to 4 photos (multipart field "photos") and returns what
// the model sees. Nothing is saved; the user reviews and then POSTs /api/pantry.
func (s *Server) scanPantry(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	images, ok := readPhotos(w, r)
	if !ok {
		return
	}
	vc, err := s.visionClient()
	if err != nil {
		writeErr(w, 400, "model not configured for photos: "+err.Error())
		return
	}
	items, err := nutrition.ScanPantry(r.Context(), vc, images, s.Store.Pantry(uid))
	if err != nil {
		writeErr(w, 502, "couldn't read the photos: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) cookFromPantry(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	var req nutrition.CookRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Count == 0 {
		req.Count = 2
	}
	if req.Count < 1 || req.Count > 3 {
		writeErr(w, 400, "ask for 1–3 ideas")
		return
	}
	pantry := s.Store.Pantry(uid)
	if len(pantry) == 0 {
		writeErr(w, 400, "add some ingredients to your pantry first")
		return
	}
	c, cfg, err := s.client()
	if err != nil {
		writeErr(w, 400, "model not configured: "+err.Error())
		return
	}
	fb := nutrition.Summarize(s.Store.Recipes(uid), s.Store.Ratings(uid))
	recipes, err := nutrition.CookFromPantry(r.Context(), c, s.Store.Profile(uid), fb, pantry, req)
	if err != nil {
		writeErr(w, 502, "couldn't come up with recipes: "+err.Error())
		return
	}
	now := s.Now()
	for i := range recipes {
		recipes[i].Demo = cfg.Provider == "" || cfg.Provider == "mock"
		recipes[i].ID = store.NewID()
		recipes[i].CreatedAt = now
	}
	if err := s.Store.PutRecipes(uid, recipes...); err != nil {
		storeErr(w, err)
		return
	}
	out := make([]recipeSummary, len(recipes))
	for i, rec := range recipes {
		out[i] = recipeSummary{Recipe: rec}
	}
	writeJSON(w, 201, map[string]any{"recipes": out})
}

func readPhotos(w http.ResponseWriter, r *http.Request) ([]llm.Image, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxScanBody)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeErr(w, 400, "photos are too large or the upload was malformed")
		return nil, false
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["photos"]
	if len(files) == 0 {
		writeErr(w, 400, "add at least one photo")
		return nil, false
	}
	if len(files) > nutrition.MaxScanImages {
		writeErr(w, 400, "up to 4 photos at a time")
		return nil, false
	}
	var images []llm.Image
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			writeErr(w, 400, "could not read "+fh.Filename)
			return nil, false
		}
		data, err := io.ReadAll(io.LimitReader(f, maxPhotoBytes+1))
		f.Close()
		if err != nil || len(data) > maxPhotoBytes {
			writeErr(w, 400, fh.Filename+" is over 8 MB")
			return nil, false
		}
		// Trust the bytes, not the filename or the client's Content-Type.
		mt := http.DetectContentType(data)
		if !photoTypes[mt] {
			writeErr(w, 400, fh.Filename+" isn't a JPEG, PNG, WebP or GIF")
			return nil, false
		}
		images = append(images, llm.Image{MediaType: mt, Data: data})
	}
	return images, true
}
