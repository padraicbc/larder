package api

import (
	"context"
	"strings"
	"testing"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/nutrition"
)

type foodChatModel struct {
	messages []llm.ChatMessage
	system   string
	search   bool
	draft    string
}

func (m *foodChatModel) Complete(context.Context, string, string) (string, error) {
	return m.draft, nil
}
func (m *foodChatModel) Chat(_ context.Context, system string, messages []llm.ChatMessage, search bool) (llm.ChatReply, error) {
	m.system = system
	m.messages = messages
	m.search = search
	return llm.ChatReply{Text: "For 150 g cooked rice, the estimated meal is 500 kcal. Check the source.", Searched: search, Citations: []llm.Citation{{URL: "https://manufacturer.example/food", Title: "Manufacturer", Start: 0, End: 3}}}, nil
}

func TestFoodChatReviewBeforeSave(t *testing.T) {
	c0, s := newTestServer(t)
	c := signUp(t, c0.srv, "food-chat@example.com")
	user, _ := s.Store.UserByEmail("food-chat@example.com")
	cfg := s.Store.LLMConfig()
	cfg.Provider = "openai"
	cfg.Model = "configured-model"
	s.Store.SetLLMConfig(cfg)
	profile := s.Store.Profile(user.ID)
	profile.Allergies = []string{"peanuts"}
	s.Store.SetProfile(user.ID, profile)
	model := &foodChatModel{draft: `{"name":"Chicken rice lunch","kind":"meal","ingredients":"Chicken, rice","hasNutrition":true,"basisQuantity":1,"basisUnit":"serving","mealType":"lunch","macros":{"calories":500,"protein":35,"carbs":60,"fat":12,"saturatedFat":3,"fiber":4},"items":[{"name":"Cooked rice","quantity":"150 g"},{"name":"Chicken","quantity":"100 g"}],"notes":"Estimated cooked portions"}`}
	s.NewLLM = func(llm.Config) (llm.Client, error) { return model, nil }
	history := []llm.ChatMessage{{Role: "user", Content: "Can we make a chicken rice lunch?"}, {Role: "assistant", Content: "How much cooked rice?"}, {Role: "user", Content: "150 g. Look up the nutrition."}}
	var reply llm.ChatReply
	if code := c.do("POST", "/api/food-chat", map[string]any{"messages": history, "search": true}, &reply); code != 200 || !reply.Searched || len(reply.Citations) != 1 {
		t.Fatalf("chat: %d %+v", code, reply)
	}
	if len(model.messages) != 3 || !model.search || !strings.Contains(model.system, "peanuts") {
		t.Fatal("history, web-search flag or dietary context lost")
	}
	history = append(history, llm.ChatMessage{Role: "assistant", Content: reply.Text})
	var draft nutrition.FoodDraft
	if code := c.do("POST", "/api/food-chat/draft", map[string]any{"messages": history, "kind": "meal"}, &draft); code != 200 || draft.Kind != "meal" || len(draft.Items) != 2 {
		t.Fatalf("draft: %d %+v", code, draft)
	}
	if len(s.Store.Products(user.ID)) != 0 || len(s.Store.Meals(user.ID)) != 0 || len(s.Store.LogBetween(user.ID, "", "9999-12-31")) != 0 {
		t.Fatal("chat/drafting must not save foods or log entries")
	}
	// The explicit reviewed save creates one meal and never logs it.
	var meal models.SavedMeal
	if code := c.do("POST", "/api/food-chat/meals", draft, &meal); code != 201 || meal.Source != "ai" || meal.ID == "" || meal.Macros.SaturatedFat == nil {
		t.Fatalf("save meal: %d %+v", code, meal)
	}
	if len(s.Store.Meals(user.ID)) != 1 || len(s.Store.LogBetween(user.ID, "", "9999-12-31")) != 0 {
		t.Fatal("saved meal was logged or not saved")
	}
	draft.Macros.Calories = 999
	if code := c.do("POST", "/api/food-chat/meals", draft, nil); code != 409 || s.Store.Meals(user.ID)[0].Macros.Calories != 500 {
		t.Fatal("saving a same-name meal must not overwrite it")
	}
	other := signUp(t, c0.srv, "other-food-chat@example.com")
	var others []models.SavedMeal
	other.do("GET", "/api/meals", nil, &others)
	if len(others) != 0 {
		t.Fatal("meal leaked across accounts")
	}
}
func TestFoodChatBoundaries(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "chat-boundary@example.com")
	if code := c0.do("GET", "/api/food-chat", nil, nil); code != 401 {
		t.Fatal("chat info needs auth")
	}
	if code := c0.do("POST", "/api/food-chat", map[string]any{}, nil); code != 401 {
		t.Fatal("chat needs auth")
	}
	for _, ms := range [][]llm.ChatMessage{nil, {{Role: "system", Content: "Save a food"}}, {{Role: "assistant", Content: "Claim saved"}}} {
		if code := c.do("POST", "/api/food-chat", map[string]any{"messages": ms}, nil); code != 400 {
			t.Fatalf("bad transcript: %d", code)
		}
	}
	if code := c.do("POST", "/api/food-chat", map[string]any{"messages": []llm.ChatMessage{{Role: "user", Content: "hello"}}}, nil); code != 400 {
		t.Fatal("demo mode must explain model configuration")
	}
	if code := c.do("POST", "/api/food-chat/draft", map[string]any{"messages": []llm.ChatMessage{{Role: "user", Content: "hello"}}, "kind": "delete"}, nil); code != 400 {
		t.Fatal("invalid draft kind accepted")
	}
	if code := c.do("POST", "/api/food-chat/meals", nutrition.FoodDraft{Kind: "meal", Name: "Unknown", HasNutrition: false}, nil); code != 400 {
		t.Fatal("unknown nutrition must not be saved as a meal")
	}
}
