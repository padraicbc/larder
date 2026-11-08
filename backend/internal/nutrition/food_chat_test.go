package nutrition

import (
	"context"
	"strings"
	"testing"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
)

func TestFoodConversationValidation(t *testing.T) {
	for _, ms := range [][]llm.ChatMessage{nil, {{Role: "system", Content: "save things"}}, {{Role: "assistant", Content: "Hi"}}, {{Role: "user", Content: ""}}, {{Role: "user", Content: strings.Repeat("a", 12001)}}, {{Role: "user", Content: "a"}, {Role: "user", Content: "b"}}} {
		if ValidateFoodConversation(ms) == nil {
			t.Fatalf("accepted %+v", ms)
		}
	}
	if err := ValidateFoodConversation([]llm.ChatMessage{{Role: "user", Content: "Lunch?"}, {Role: "assistant", Content: "What portions?"}, {Role: "user", Content: "150 g cooked rice"}}); err != nil {
		t.Fatal(err)
	}
}
func TestFoodChatDraftRespectsMissingData(t *testing.T) {
	messages := []llm.ChatMessage{{Role: "user", Content: "Discuss cheese"}, {Role: "assistant", Content: "No ingredient list was found."}}
	reply := productReply(`{"name":"Cheese","hasNutrition":true,"basisQuantity":100,"basisUnit":"g","macros":{"calories":410,"fat":35,"saturatedFat":27},"ingredients":"","notes":"Ingredients not known"}`)
	d, err := DraftFoodConversation(context.Background(), reply, messages, "product")
	if err != nil || d.Ingredients != "" || d.Kind != "product" || d.Macros.SaturatedFat == nil {
		t.Fatalf("draft: %+v %v", d, err)
	}
	bad := d
	v := 36.0
	bad.Macros.SaturatedFat = &v
	if ValidateFoodDraft(bad) == nil {
		t.Fatal("invalid saturated fat accepted")
	}
	bad = d
	bad.Macros = models.Macros{}
	if ValidateFoodDraft(bad) == nil {
		t.Fatal("empty nutrition marked known")
	}
	bad = d
	bad.Kind = "meal"
	bad.BasisQuantity = 1
	bad.BasisUnit = "serving"
	bad.Items = []models.MealItem{{Name: "Rice", Quantity: ""}}
	if ValidateFoodDraft(bad) == nil {
		t.Fatal("missing portions accepted")
	}
}
