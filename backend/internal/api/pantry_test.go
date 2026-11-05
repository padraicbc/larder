package api

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/you/nutrition-app/internal/models"
	"github.com/you/nutrition-app/internal/nutrition"
)

func TestPantryCRUD(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "cook@example.com")

	var res struct {
		Items []models.PantryItem `json:"items"`
		Added int                 `json:"added"`
	}
	c.do("POST", "/api/pantry", map[string]any{"items": []map[string]string{
		{"name": "Eggs", "quantity": "6", "category": "protein"}, {"name": "spinach"}, {"name": "feta"},
	}}, &res)
	if res.Added != 3 || len(res.Items) != 3 {
		t.Fatalf("add = %+v", res)
	}
	c.do("POST", "/api/pantry", map[string]any{"items": []map[string]string{{"name": "egg", "quantity": "12"}}}, &res)
	if res.Added != 0 || len(res.Items) != 3 {
		t.Errorf("re-adding eggs should update, not duplicate: %+v", res)
	}

	id := res.Items[0].ID
	var items []models.PantryItem
	if code := c.do("PATCH", "/api/pantry/"+id, map[string]string{"quantity": "a few"}, &items); code != 200 {
		t.Errorf("patch = %d", code)
	}
	if code := c.do("DELETE", "/api/pantry/"+id, nil, nil); code != 204 {
		t.Errorf("delete = %d", code)
	}
	if code := c.do("DELETE", "/api/pantry/"+id, nil, nil); code != 404 {
		t.Errorf("delete twice = %d", code)
	}
	c.do("GET", "/api/pantry", nil, &items)
	if len(items) != 2 {
		t.Errorf("want 2 left, got %d", len(items))
	}
	c.do("DELETE", "/api/pantry", nil, nil)
	c.do("GET", "/api/pantry", nil, &items)
	if len(items) != 0 {
		t.Error("clear didn't empty the pantry")
	}

	if code := c.do("POST", "/api/pantry", map[string]any{"items": []any{}}, nil); code != 400 {
		t.Errorf("empty add = %d", code)
	}
}

func TestCookFromPantryEndpoint(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "cook@example.com")

	if code := c.do("POST", "/api/pantry/cook", map[string]any{}, nil); code != 400 {
		t.Errorf("cook with empty pantry = %d", code)
	}
	c.do("POST", "/api/pantry", map[string]any{"items": []map[string]string{{"name": "eggs"}, {"name": "spinach"}, {"name": "feta"}}}, nil)

	var res struct {
		Recipes []recipeSummary `json:"recipes"`
	}
	if code := c.do("POST", "/api/pantry/cook", map[string]any{"count": 2, "mealType": "breakfast"}, &res); code != 201 {
		t.Fatalf("cook = %d", code)
	}
	if len(res.Recipes) != 2 || res.Recipes[0].Source != "pantry" || len(res.Recipes[0].UsesPantry) == 0 {
		t.Errorf("recipes = %+v", res.Recipes)
	}
	// They're saved like any recipe, so they can be rated and logged.
	if code := c.do("POST", "/api/recipes/"+res.Recipes[0].ID+"/ratings", map[string]int{"stars": 4}, nil); code != 201 {
		t.Errorf("rate pantry recipe = %d", code)
	}
	if code := c.do("POST", "/api/pantry/cook", map[string]any{"count": 9}, nil); code != 400 {
		t.Errorf("count 9 = %d", code)
	}
}

func pngBytes(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{200, 50, 50, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// upload posts files under the "photos" field.
func (c *client) upload(files map[string][]byte, out any) int {
	return c.uploadTo("/api/pantry/scan", files, out)
}

func (c *client) uploadTo(path string, files map[string][]byte, out any) int {
	c.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, data := range files {
		fw, _ := mw.CreateFormFile("photos", name)
		fw.Write(data)
	}
	mw.Close()
	req, _ := http.NewRequest("POST", c.srv.URL+path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return c.send(req, out)
}

func TestScanPantryEndpoint(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "cook@example.com")
	c.do("POST", "/api/pantry", map[string]any{"items": []map[string]string{{"name": "eggs"}}}, nil)

	var res struct {
		Items []nutrition.DetectedItem `json:"items"`
	}
	if code := c.upload(map[string][]byte{"fridge.png": pngBytes(t)}, &res); code != 200 {
		t.Fatalf("scan = %d", code)
	}
	if len(res.Items) == 0 || res.Items[0].Name != "eggs" || !res.Items[0].Have {
		t.Errorf("want mock items with eggs marked as already owned, got %+v", res.Items)
	}
	var items []models.PantryItem
	c.do("GET", "/api/pantry", nil, &items)
	if len(items) != 1 {
		t.Error("scanning must not save anything by itself")
	}

	var e apiError
	if code := c.upload(map[string][]byte{"evil.png": []byte("#!/bin/sh\nrm -rf /")}, &e); code != 400 {
		t.Errorf("non-image = %d (%s)", code, e.Error)
	}
	five := map[string][]byte{}
	for _, n := range []string{"a.png", "b.png", "c.png", "d.png", "e.png"} {
		five[n] = pngBytes(t)
	}
	if code := c.upload(five, nil); code != 400 {
		t.Errorf("5 photos = %d", code)
	}
	if code := c.upload(map[string][]byte{}, nil); code != 400 {
		t.Errorf("no photos = %d", code)
	}
}
