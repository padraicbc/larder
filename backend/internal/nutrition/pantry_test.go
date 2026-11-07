package nutrition

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
)

func pantryOf(names ...string) []models.PantryItem {
	var out []models.PantryItem
	for _, n := range names {
		out = append(out, models.PantryItem{Name: n})
	}
	return out
}

func TestMatchPantry(t *testing.T) {
	pantry := pantryOf("feta", "greek yogurt", "cherry tomatoes", "eggs", "red onion")
	tests := []struct {
		ingredient string
		want       string // "" = no match
	}{
		{"crumbled feta cheese", "feta"},     // pantry name is less specific
		{"yogurt", "greek yogurt"},           // pantry name is more specific
		{"Cherry Tomato", "cherry tomatoes"}, // case + plural
		{"2 large eggs", "eggs"},             // extra words
		{"eggplant", ""},                     // no partial-word matches
		{"tomato paste", ""},                 // "cherry tomato" isn't "tomato paste"
		{"onion", "red onion"},               // generic onion covered by a red onion
		{"spring onion", ""},                 // but a different onion isn't
	}
	for _, tt := range tests {
		got, ok := MatchPantry(tt.ingredient, pantry)
		if (tt.want == "" && ok) || (tt.want != "" && got.Name != tt.want) {
			t.Errorf("MatchPantry(%q) = %q, %v; want %q", tt.ingredient, got.Name, ok, tt.want)
		}
	}
}

func TestSplitByPantry(t *testing.T) {
	r := recipeWith("eggs", "spinach", "feta", "salt", "olive oil", "smoked paprika", "lemon")
	uses, missing := SplitByPantry(r, pantryOf("eggs", "baby spinach", "feta"))
	if strings.Join(uses, ",") != "eggs,baby spinach,feta" {
		t.Errorf("uses = %v", uses)
	}
	if len(missing) != 1 || missing[0].Name != "lemon" {
		t.Errorf("missing = %+v (staples like salt/oil/paprika shouldn't count)", missing)
	}
}

func TestMergePantry(t *testing.T) {
	now := time.Now()
	n := 0
	id := func() string { n++; return fmt.Sprint("id", n) }
	p := map[string]models.PantryItem{}

	added := MergePantry(p, []models.PantryItem{
		{Name: "Eggs", Quantity: "6", Category: "Protein"},
		{Name: "spinach"},
		{Name: "  "},
	}, id, now)
	if added != 2 || len(p) != 2 {
		t.Fatalf("added %d, have %d", added, len(p))
	}
	if p["id1"].Name != "eggs" || p["id1"].Category != "protein" || p["id1"].Source != "manual" {
		t.Errorf("not normalised: %+v", p["id1"])
	}

	// Same item again (plural/case) updates instead of duplicating.
	added = MergePantry(p, []models.PantryItem{{Name: "egg", Quantity: "12"}, {Name: "Spinach", Quantity: ""}}, id, now)
	if added != 0 || len(p) != 2 {
		t.Errorf("duplicates were added: %d new, %d total", added, len(p))
	}
	if p["id1"].Quantity != "12" {
		t.Error("quantity should update")
	}
}

type fakeVision struct {
	reply  string
	images int
}

func (f *fakeVision) CompleteWithImages(_ context.Context, _, _ string, images []llm.Image) (string, error) {
	f.images = len(images)
	return f.reply, nil
}

func TestScanPantry(t *testing.T) {
	fv := &fakeVision{reply: `{"items":[{"name":"Spinach","quantity":"1 bag"},{"name":"spinach "},{"name":""},{"name":"Milk","category":"Dairy"}]}`}
	img := []llm.Image{{MediaType: "image/jpeg", Data: []byte{1}}}
	items, err := ScanPantry(context.Background(), fv, img, pantryOf("milk"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want deduped 2 items, got %+v", items)
	}
	if items[0].Name != "spinach" || items[0].Category != "other" || items[0].Have {
		t.Errorf("spinach = %+v", items[0])
	}
	if !items[1].Have || items[1].Category != "dairy" {
		t.Errorf("milk should be marked as already in pantry: %+v", items[1])
	}

	if _, err := ScanPantry(context.Background(), fv, nil, nil); err == nil {
		t.Error("no images should error")
	}
	if _, err := ScanPantry(context.Background(), fv, make([]llm.Image, 5), nil); err == nil {
		t.Error("5 images should error")
	}
}

func TestScanWithMock(t *testing.T) {
	items, err := ScanPantry(context.Background(), llm.Mock{}, []llm.Image{{MediaType: "image/png", Data: []byte{1}}}, nil)
	if err != nil || len(items) == 0 {
		t.Errorf("mock scan: %v %v", items, err)
	}
}

const cookBacon = `{"recipes":[{"title":"Bacon eggs","ingredients":[{"name":"eggs"},{"name":"bacon"}]}]}`
const cookEggs = `{"recipes":[{"title":"Herby eggs","ingredients":[{"name":"eggs"},{"name":"salt"}]}]}`
const cookLemon = `{"recipes":[{"title":"Lemon eggs","ingredients":[{"name":"eggs"},{"name":"lemon"}]}]}`

func TestCookFromPantry(t *testing.T) {
	ctx := context.Background()
	pantry := pantryOf("eggs", "spinach")

	t.Run("retries on diet violation", func(t *testing.T) {
		m := &scripted{replies: []string{cookBacon, cookEggs}}
		rs, err := CookFromPantry(ctx, m, veg, Feedback{}, pantry, CookRequest{Count: 1})
		if err != nil || len(rs) != 1 || rs[0].Title != "Herby eggs" {
			t.Fatalf("got %+v, %v", rs, err)
		}
		if rs[0].Source != "pantry" || rs[0].UsesPantry[0] != "eggs" || len(rs[0].Missing) != 0 {
			t.Errorf("pantry fields: %+v", rs[0])
		}
	})

	t.Run("strict mode retries when extras are added", func(t *testing.T) {
		m := &scripted{replies: []string{cookLemon, cookEggs}}
		rs, err := CookFromPantry(ctx, m, models.Profile{}, Feedback{}, pantry, CookRequest{Strict: true})
		if err != nil || rs[0].Title != "Herby eggs" || len(m.prompts) != 2 {
			t.Errorf("got %+v after %d calls, %v", rs, len(m.prompts), err)
		}
		if !strings.Contains(m.prompts[1], `"lemon"`) {
			t.Error("retry prompt should name the extra ingredient")
		}
	})

	t.Run("flexible mode keeps extras as missing", func(t *testing.T) {
		m := &scripted{replies: []string{cookLemon}}
		rs, _ := CookFromPantry(ctx, m, models.Profile{}, Feedback{}, pantry, CookRequest{})
		if len(m.prompts) != 1 || len(rs[0].Missing) != 1 || rs[0].Missing[0].Name != "lemon" {
			t.Errorf("want one call with lemon missing, got %d calls, %+v", len(m.prompts), rs)
		}
	})

	t.Run("prompt", func(t *testing.T) {
		p := models.Profile{Allergies: []string{"sesame"}, MealsPerDay: 4, Targets: models.Macros{Calories: 2000, Protein: 120}}
		pantry := []models.PantryItem{{Name: "tofu", Quantity: "1 block"}}
		_, user := BuildCookPrompt(p, Feedback{}, pantry, CookRequest{Count: 2, Strict: true, MealType: "dinner", Notes: "spicy"})
		for _, want := range []string{"TASK: pantry_cook", "COUNT: 2", "MEAL: dinner", "tofu (1 block)", "ONLY", "sesame", "500 kcal", "30g protein", "spicy"} {
			if !strings.Contains(user, want) {
				t.Errorf("prompt missing %q", want)
			}
		}
	})

	t.Run("empty pantry", func(t *testing.T) {
		if _, err := CookFromPantry(ctx, &scripted{}, models.Profile{}, Feedback{}, nil, CookRequest{}); err == nil {
			t.Error("want error")
		}
	})

	t.Run("mock", func(t *testing.T) {
		rs, err := CookFromPantry(ctx, llm.Mock{}, models.Profile{}, Feedback{}, pantryOf("eggs", "spinach", "feta"), CookRequest{Count: 3})
		if err != nil || len(rs) != 3 {
			t.Errorf("mock cook: %d recipes, %v", len(rs), err)
		}
	})
}

func TestGuessCategory(t *testing.T) {
	for name, want := range map[string]string{
		"Cherry Tomatoes": "produce", "baby spinach": "produce", "eggs": "protein", "chicken thighs": "protein",
		"feta": "dairy", "greek yogurt": "dairy", "oat milk": "grains", "brown rice": "grains",
		"frozen peas": "frozen", "orange juice": "drinks", "peanut butter": "pantry", "tahini": "pantry",
	} {
		if got := GuessCategory(name); got != want {
			t.Errorf("GuessCategory(%q) = %q, want %q", name, got, want)
		}
	}
}
