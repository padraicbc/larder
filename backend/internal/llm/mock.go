package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Mock returns canned, valid JSON so the whole app works with no model attached.
// It recognises the TASK: marker that the prompt builders put in every prompt.
type Mock struct{}

var reInt = func(key string) *regexp.Regexp { return regexp.MustCompile(key + `:\s*(\d+)`) }

func findInt(s, key string, def int) int {
	m := reInt(key).FindStringSubmatch(s)
	if len(m) < 2 {
		return def
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return def
	}
	return n
}

func (Mock) Complete(_ context.Context, system, user string) (string, error) {
	prompt := system + "\n" + user
	switch {
	case strings.Contains(prompt, "TASK: replace_meal"):
		var recipe map[string]any
		for _, line := range strings.Split(user, "\n") {
			if strings.HasPrefix(line, "ORIGINAL_MEAL: ") {
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "ORIGINAL_MEAL: ")), &recipe); err != nil {
					return "", err
				}
				break
			}
		}
		if recipe == nil {
			return "", fmt.Errorf("mock: missing original meal")
		}
		recipe["title"] = "Herby chickpea and rice bowl"
		if strings.Contains(user, `"title":"Herby chickpea and rice bowl"`) {
			recipe["title"] = "Lentil and roast vegetable bowl"
		}
		recipe["description"] = "A fresh demo meal idea; configure a real model for tailored nutrition."
		recipe["ingredients"] = []map[string]any{ing("chickpeas", 150, "g", "protein"), ing("rice", 75, "g", "grains"), ing("spinach", 50, "g", "produce")}
		recipe["steps"] = []string{"Cook rice.", "Warm chickpeas with spinach and serve over rice."}
		b, err := json.Marshal(map[string]any{"recipe": recipe})
		return string(b), err
	case strings.Contains(prompt, "TASK: meal_plan"):
		return mockPlan(findInt(prompt, "DAYS", 3), findInt(prompt, "MEALS_PER_DAY", 3)), nil
	case strings.Contains(prompt, "TASK: estimate_macros"):
		return `{"name":"Estimated meal","macros":{"calories":450,"protein":25,"carbs":45,"fat":18,"fiber":6},"confidence":"low","notes":"Mock provider: configure a real model in Settings for real estimates."}`, nil
	case strings.Contains(prompt, "TASK: pantry_cook"):
		return mockCook(findInt(prompt, "COUNT", 1)), nil
	case strings.Contains(prompt, "TASK: ping"):
		return `{"ok":true}`, nil
	}
	return "", fmt.Errorf("mock: no canned reply for this prompt")
}

type mockRecipe struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	MealType    string           `json:"mealType"`
	Servings    int              `json:"servings"`
	PrepMinutes int              `json:"prepMinutes"`
	Ingredients []map[string]any `json:"ingredients"`
	Steps       []string         `json:"steps"`
	Macros      map[string]int   `json:"macros"`
	Tags        []string         `json:"tags"`
}

func ing(name string, q float64, unit, cat string) map[string]any {
	return map[string]any{"name": name, "quantity": q, "unit": unit, "category": cat}
}

var mockLibrary = map[string][]mockRecipe{
	"breakfast": {
		{"Peanut butter overnight oats", "Make-ahead oats with banana and chia.", "breakfast", 1, 5,
			[]map[string]any{ing("rolled oats", 60, "g", "grains"), ing("oat milk", 200, "ml", "dairy"), ing("peanut butter", 1, "tbsp", "pantry"), ing("banana", 1, "", "produce"), ing("chia seeds", 1, "tbsp", "pantry")},
			[]string{"Stir everything except banana in a jar.", "Refrigerate overnight.", "Slice banana on top before eating."},
			map[string]int{"calories": 480, "protein": 16, "carbs": 68, "fat": 17, "fiber": 11}, []string{"make-ahead", "quick"}},
		{"Spiced tofu scramble", "Turmeric tofu with spinach on toast.", "breakfast", 1, 15,
			[]map[string]any{ing("firm tofu", 150, "g", "protein"), ing("spinach", 40, "g", "produce"), ing("wholegrain bread", 2, "slice", "grains"), ing("turmeric", 0.5, "tsp", "pantry"), ing("olive oil", 1, "tsp", "pantry")},
			[]string{"Crumble tofu into a hot oiled pan.", "Add turmeric and a pinch of salt, cook 5 min.", "Wilt in spinach and serve on toast."},
			map[string]int{"calories": 420, "protein": 28, "carbs": 34, "fat": 18, "fiber": 7}, []string{"high-protein"}},
	},
	"lunch": {
		{"Smashed chickpea salad wraps", "Lemony chickpeas, crunchy veg, tortilla.", "lunch", 1, 10,
			[]map[string]any{ing("chickpeas", 240, "g", "pantry"), ing("lemon", 0.5, "", "produce"), ing("cucumber", 0.5, "", "produce"), ing("tortilla", 1, "", "grains"), ing("tahini", 1, "tbsp", "pantry")},
			[]string{"Mash chickpeas with tahini and lemon.", "Dice cucumber and fold in.", "Wrap in the tortilla."},
			map[string]int{"calories": 540, "protein": 22, "carbs": 70, "fat": 18, "fiber": 15}, []string{"no-cook", "quick"}},
		{"Red lentil soup", "Cumin-scented lentils with carrot.", "lunch", 2, 30,
			[]map[string]any{ing("red lentils", 150, "g", "pantry"), ing("carrot", 2, "", "produce"), ing("onion", 1, "", "produce"), ing("vegetable stock", 1, "l", "pantry"), ing("cumin", 1, "tsp", "pantry")},
			[]string{"Soften diced onion and carrot.", "Add lentils, cumin and stock; simmer 20 min.", "Blend half for a creamy texture."},
			map[string]int{"calories": 380, "protein": 21, "carbs": 58, "fat": 5, "fiber": 12}, []string{"batch-cook"}},
	},
	"dinner": {
		{"Black bean tacos", "Smoky beans, quick slaw, lime.", "dinner", 2, 20,
			[]map[string]any{ing("black beans", 400, "g", "pantry"), ing("corn tortillas", 6, "", "grains"), ing("red cabbage", 150, "g", "produce"), ing("lime", 1, "", "produce"), ing("smoked paprika", 1, "tsp", "pantry")},
			[]string{"Warm beans with paprika and a splash of water.", "Toss cabbage with lime juice and salt.", "Fill warm tortillas."},
			map[string]int{"calories": 520, "protein": 21, "carbs": 88, "fat": 8, "fiber": 22}, []string{"family"}},
		{"Ginger tofu stir-fry", "Crispy tofu, broccoli, rice.", "dinner", 2, 25,
			[]map[string]any{ing("firm tofu", 300, "g", "protein"), ing("broccoli", 300, "g", "produce"), ing("brown rice", 150, "g", "grains"), ing("ginger", 15, "g", "produce"), ing("soy sauce", 2, "tbsp", "pantry")},
			[]string{"Cook rice.", "Pan-fry cubed tofu until golden.", "Add broccoli, ginger and soy; toss 4 min."},
			map[string]int{"calories": 560, "protein": 32, "carbs": 70, "fat": 16, "fiber": 9}, []string{"high-protein"}},
	},
	"snack": {
		{"Apple with almond butter", "Simple and filling.", "snack", 1, 2,
			[]map[string]any{ing("apple", 1, "", "produce"), ing("almond butter", 1, "tbsp", "pantry")},
			[]string{"Slice apple and dip."},
			map[string]int{"calories": 190, "protein": 4, "carbs": 25, "fat": 9, "fiber": 5}, []string{"quick"}},
	},
}

func mockPlan(days, meals int) string {
	if days < 1 {
		days = 1
	}
	types := []string{"breakfast", "lunch", "dinner", "snack", "snack"}
	if meals < 1 {
		meals = 3
	}
	if meals > len(types) {
		meals = len(types)
	}
	type day struct {
		Day   int          `json:"day"`
		Meals []mockRecipe `json:"meals"`
	}
	out := struct {
		Title string `json:"title"`
		Days  []day  `json:"days"`
	}{Title: fmt.Sprintf("%d-day plan (mock)", days)}
	for d := 1; d <= days; d++ {
		dd := day{Day: d}
		for m := 0; m < meals; m++ {
			lib := mockLibrary[types[m]]
			dd.Meals = append(dd.Meals, lib[(d+m)%len(lib)])
		}
		out.Days = append(out.Days, dd)
	}
	b, _ := json.Marshal(out)
	return "Here is your plan:\n```json\n" + string(b) + "\n```"
}

// CompleteWithImages pretends to see a typical fridge.
func (m Mock) CompleteWithImages(ctx context.Context, system, user string, images []Image) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("mock: no images")
	}
	if strings.Contains(system+user, "TASK: product_label") {
		return `{"name":"Rolled oats (demo)","ingredients":"Wholegrain rolled oats","basisQuantity":100,"basisUnit":"g","fromNutritionLabel":true,"macros":{"calories":380,"protein":13,"carbs":60,"fat":7,"fiber":10},"confidence":"low","notes":"Demo data; configure a vision model to read real labels."}`, nil
	}
	if strings.Contains(system+user, "TASK: ingredients_photo") {
		return `{"name":"Chocolate hoops (demo)","barcode":"","ingredients":"Wholegrain wheat, sugar, glucose-fructose syrup, cocoa powder, palm oil, colour (E150d), flavouring, emulsifier (E471), salt, vitamin C (E300)","hasNutrition":false,"macros":{},"notes":"Demo data; configure a vision model to read real labels."}`, nil
	}
	if strings.Contains(system+user, "TASK: barcode_photo") {
		return `{"barcode":""}`, nil // demo mode has no vision model to read digits
	}
	if strings.Contains(system+user, "TASK: meal_photo") {
		return `{"name":"Eggs on toast","items":[{"name":"eggs","quantity":"2 large"},{"name":"toast","quantity":"1 slice"}],"macros":{"calories":320,"protein":19,"carbs":22,"fat":17,"fiber":3},"confidence":"medium","notes":"Demo estimate. Check portions before logging."}`, nil
	}
	if strings.Contains(system+user, "TASK: pantry_scan") {
		return `{"items":[
 {"name":"eggs","quantity":"6","category":"protein","confidence":"high"},
 {"name":"spinach","quantity":"1 bag","category":"produce","confidence":"high"},
 {"name":"feta","quantity":"half a block","category":"dairy","confidence":"medium"},
 {"name":"cherry tomatoes","quantity":"1 punnet","category":"produce","confidence":"high"},
 {"name":"greek yogurt","quantity":"1 tub","category":"dairy","confidence":"medium"}]}`, nil
	}
	return m.Complete(ctx, system, user)
}

func mockCook(count int) string {
	if count < 1 {
		count = 1
	}
	all := []mockRecipe{
		{"Spinach and feta omelette", "Fluffy eggs folded over wilted spinach and salty feta.", "breakfast", 1, 10,
			[]map[string]any{ing("eggs", 3, "", "protein"), ing("spinach", 50, "g", "produce"), ing("feta", 30, "g", "dairy"), ing("olive oil", 1, "tsp", "pantry")},
			[]string{"Wilt spinach in an oiled pan.", "Pour in beaten eggs, cook until nearly set.", "Crumble feta over, fold and serve."},
			map[string]int{"calories": 390, "protein": 27, "carbs": 4, "fat": 29, "fiber": 2}, []string{"quick", "pantry"}},
		{"Blistered tomato yogurt bowl", "Charred cherry tomatoes over garlicky yogurt with toast.", "lunch", 1, 15,
			[]map[string]any{ing("cherry tomatoes", 200, "g", "produce"), ing("greek yogurt", 150, "g", "dairy"), ing("sourdough", 1, "slice", "grains"), ing("olive oil", 1, "tbsp", "pantry")},
			[]string{"Blister tomatoes in a hot pan with oil.", "Spread yogurt in a bowl, spoon tomatoes over.", "Serve with toast."},
			map[string]int{"calories": 420, "protein": 19, "carbs": 38, "fat": 21, "fiber": 4}, []string{"pantry"}},
		{"Shakshuka-ish eggs", "Eggs poached in a quick tomato and spinach sauce.", "dinner", 2, 20,
			[]map[string]any{ing("eggs", 4, "", "protein"), ing("cherry tomatoes", 300, "g", "produce"), ing("spinach", 80, "g", "produce"), ing("cumin", 1, "tsp", "pantry"), ing("crusty bread", 2, "slice", "grains")},
			[]string{"Soften tomatoes with cumin until saucy.", "Stir in spinach, make wells and crack in eggs.", "Cover until whites set; serve with bread."},
			map[string]int{"calories": 360, "protein": 20, "carbs": 30, "fat": 17, "fiber": 5}, []string{"one-pan", "pantry"}},
	}
	if count > len(all) {
		count = len(all)
	}
	b, _ := json.Marshal(map[string]any{"recipes": all[:count]})
	return string(b)
}
