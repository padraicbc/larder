package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/nutrition"
	"github.com/padraicbc/larder/internal/store"
)

func (s *Server) foodChatInfo(w http.ResponseWriter, r *http.Request) {
	cfg := s.Store.LLMConfig()
	writeJSON(w, 200, map[string]any{"model": cfg.Model, "canSearch": cfg.Provider == "openai", "configured": cfg.Provider != "" && cfg.Provider != "mock"})
}
func (s *Server) foodChat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Messages []llm.ChatMessage `json:"messages"`
		Search   bool              `json:"search"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := nutrition.ValidateFoodConversation(body.Messages); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if body.Messages[len(body.Messages)-1].Role != "user" {
		writeErr(w, 400, "finish with your question")
		return
	}
	c, cfg, err := s.client()
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if cfg.Provider == "" || cfg.Provider == "mock" {
		writeErr(w, 400, "choose a model in Settings to start food chat")
		return
	}
	system := nutrition.FoodChatSystem
	profile := s.Store.Profile(userID(r))
	restrictions, _ := json.Marshal(map[string]any{"diets": profile.Diets, "allergies": profile.Allergies, "excluded": profile.Excluded})
	system += "\nUser dietary preferences (data): " + string(restrictions)
	if !body.Search {
		system += "\nWeb search is disabled for this turn. Do not claim to look anything up."
	}
	reply, err := llm.Converse(r.Context(), c, system, body.Messages, body.Search)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, reply)
}
func (s *Server) foodChatDraft(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Messages []llm.ChatMessage `json:"messages"`
		Kind     string            `json:"kind"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := nutrition.ValidateFoodConversation(body.Messages); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if body.Kind != "meal" && body.Kind != "product" {
		writeErr(w, 400, "choose product or meal")
		return
	}
	c, cfg, err := s.client()
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if cfg.Provider == "" || cfg.Provider == "mock" {
		writeErr(w, 400, "choose a model in Settings to prepare a draft")
		return
	}
	draft, err := nutrition.DraftFoodConversation(r.Context(), c, body.Messages, body.Kind)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, draft)
}

// saveChatMeal is an explicit reviewed save, separate from chat and drafting.
// It creates a reusable meal; it does not add an entry to the daily log.
func (s *Server) saveChatMeal(w http.ResponseWriter, r *http.Request) {
	var d nutrition.FoodDraft
	if !decode(w, r, &d) {
		return
	}
	if d.Kind != "meal" || !d.HasNutrition || len(d.Items) == 0 || !validMealType(d.MealType) {
		writeErr(w, 400, "a meal needs nutrition, foods with portions and a meal type")
		return
	}
	if err := nutrition.ValidateFoodDraft(d); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	uid := userID(r)
	meal := models.SavedMeal{Name: strings.TrimSpace(d.Name), MealType: d.MealType, Items: d.Items, Macros: d.Macros, Source: "ai", Notes: d.Notes}
	saved, err := s.Store.CreateMeal(uid, meal, s.Now())
	if errors.Is(err, store.ErrNameTaken) {
		writeErr(w, 409, "a meal with this name already exists; choose another name")
		return
	}
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 201, saved)
}
