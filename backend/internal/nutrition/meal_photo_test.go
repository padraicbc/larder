package nutrition

import (
	"context"
	"testing"

	"github.com/padraicbc/larder/internal/llm"
)

type mealVisionReply string

func (m mealVisionReply) CompleteWithImages(context.Context, string, string, []llm.Image) (string, error) {
	return string(m), nil
}

func TestMealPhotoRejectsUnusableResults(t *testing.T) {
	image := []llm.Image{{MediaType: "image/png", Data: []byte{1}}}
	for _, reply := range []string{
		`{"items":[]}`, `{"items":[{"name":"  "}]}`,
		`{"items":[{"name":"eggs","quantity":"2"}],"macros":{"calories":-1}}`,
		`not json`,
	} {
		if _, err := EstimateMealPhoto(context.Background(), mealVisionReply(reply), image); err == nil {
			t.Errorf("accepted %s", reply)
		}
	}
	for _, images := range [][]llm.Image{nil, append(image, image...)} {
		if _, err := EstimateMealPhoto(context.Background(), llm.Mock{}, images); err == nil {
			t.Error("accepted wrong image count")
		}
	}
}
