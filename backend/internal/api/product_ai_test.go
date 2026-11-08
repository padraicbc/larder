package api

import (
	"testing"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
)

func TestAIProductPreviewAndSave(t *testing.T) {
	c0, s := newTestServer(t)
	c := signUp(t, c0.srv, "ai-food@example.com")
	body := map[string]string{"description": "Vintage cheddar, Ireland"}
	if code := c0.do("POST", "/api/products/suggest", body, nil); code != 401 {
		t.Fatalf("unauthenticated: %d", code)
	}
	if code := c.do("POST", "/api/products/suggest", body, nil); code != 400 {
		t.Fatalf("mock must not invent food: %d", code)
	}
	cfg := s.Store.LLMConfig()
	cfg.Provider = "openai"
	cfg.Model = "test"
	s.Store.SetLLMConfig(cfg)
	s.NewLLM = func(llm.Config) (llm.Client, error) {
		return replacementModel{`{"name":"Vintage cheddar","ingredients":"Milk, salt, cultures, rennet","hasNutrition":true,"basisUnit":"g","macros":{"calories":410,"protein":25,"carbs":0,"fat":35,"saturatedFat":27,"fiber":0},"notes":"Typical cheese"}`}, nil
	}
	var sc models.Scan
	if code := c.do("POST", "/api/products/suggest", body, &sc); code != 200 || sc.Rating == nil || sc.Rating.Score != 62 || !sc.Rating.Provisional {
		t.Fatalf("preview: %d %+v", code, sc)
	}
	user, _ := s.Store.UserByEmail("ai-food@example.com")
	if len(s.Store.Products(user.ID)) != 0 {
		t.Fatal("preview saved a product")
	}
	if code := c.do("POST", "/api/scans", sc, &sc); code != 201 || sc.ProductID == "" || sc.Source != "ai" || !sc.Rating.Provisional {
		t.Fatalf("save: %d %+v", code, sc)
	}
	stored := s.Store.Products(user.ID)
	if len(stored) != 1 || stored[0].Source != "ai" || stored[0].FromNutritionLabel || stored[0].Rating != nil {
		t.Fatalf("AI provenance or calculated-only score lost: %+v", stored)
	}
	// Review can fill a saved product's missing saturated fat without replacing it.
	sat := 20.0
	sc.Macros.SaturatedFat = &sat
	if code := c.do("POST", "/api/scans", sc, &sc); code != 201 || sc.Rating.Score != 72 {
		t.Fatalf("update: %d %+v", code, sc)
	}
	stored = s.Store.Products(user.ID)
	if len(stored) != 1 || *stored[0].Macros.SaturatedFat != 20 || stored[0].ID != sc.ProductID {
		t.Fatal("saved-product AI update lost")
	}
}
