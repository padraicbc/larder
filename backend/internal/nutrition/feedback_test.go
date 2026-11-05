package nutrition

import (
	"slices"
	"strings"
	"testing"

	"github.com/you/nutrition-app/internal/models"
)

func feedbackFixture() (map[string]models.Recipe, []models.Rating) {
	recipes := map[string]models.Recipe{
		"a": {ID: "a", Title: "Miso salmon", Tags: []string{"quick"}, Ingredients: []models.Ingredient{{Name: "salmon"}, {Name: "miso"}, {Name: "salt"}}},
		"b": {ID: "b", Title: "Mushroom risotto", Tags: []string{"slow"}, Ingredients: []models.Ingredient{{Name: "mushrooms"}, {Name: "arborio rice"}}},
		"c": {ID: "c", Title: "Plain rice", Ingredients: []models.Ingredient{{Name: "rice"}}},
	}
	ratings := []models.Rating{
		{RecipeID: "a", Stars: 5, Comment: "so good"},
		{RecipeID: "a", Stars: 5},
		{RecipeID: "b", Stars: 1, Comment: "too mushy"},
		{RecipeID: "c", Stars: 3},
		{RecipeID: "deleted", Stars: 5}, // unknown recipes are ignored
	}
	return recipes, ratings
}

func TestSummarize(t *testing.T) {
	fb := Summarize(feedbackFixture())

	if fb.RatingCount != 5 {
		t.Errorf("RatingCount = %d", fb.RatingCount)
	}
	if len(fb.Loved) != 1 || fb.Loved[0].Title != "Miso salmon" || fb.Loved[0].Avg != 5 {
		t.Errorf("Loved = %+v", fb.Loved)
	}
	if len(fb.Disliked) != 1 || fb.Disliked[0].Comments[0] != "too mushy" {
		t.Errorf("Disliked = %+v", fb.Disliked)
	}
	if !slices.Contains(fb.LikedIngredients, "salmon") || slices.Contains(fb.LikedIngredients, "salt") {
		t.Errorf("LikedIngredients = %v (want salmon, not the staple salt)", fb.LikedIngredients)
	}
	if !slices.Contains(fb.DislikedIngredients, "mushroom") {
		t.Errorf("DislikedIngredients = %v", fb.DislikedIngredients)
	}
	if slices.Contains(fb.LikedIngredients, "rice") || slices.Contains(fb.DislikedIngredients, "rice") {
		t.Error("a 3-star rating should be neutral")
	}
	if !slices.Contains(fb.LikedTags, "quick") {
		t.Errorf("LikedTags = %v", fb.LikedTags)
	}
}

func TestFeedbackPromptText(t *testing.T) {
	if got := (Feedback{}).PromptText(); got != "" {
		t.Errorf("no ratings should give empty text, got %q", got)
	}
	txt := Summarize(feedbackFixture()).PromptText()
	for _, want := range []string{"Miso salmon", "too mushy", "salmon", "mushroom"} {
		if !strings.Contains(txt, want) {
			t.Errorf("prompt text missing %q:\n%s", want, txt)
		}
	}
}
