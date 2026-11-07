package nutrition

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
)

// Things nearly every kitchen has. They never count as "missing".
var pantryStaples = map[string]bool{
	"salt": true, "black pepper": true, "pepper": true, "water": true, "ice": true,
	"oil": true, "olive oil": true, "vegetable oil": true, "neutral oil": true, "cooking spray": true,
	"cumin": true, "paprika": true, "smoked paprika": true, "chili flake": true, "chilli flake": true,
	"red pepper flake": true, "oregano": true, "dried oregano": true, "turmeric": true, "cinnamon": true,
	"garlic powder": true, "onion powder": true, "curry powder": true, "dried thyme": true, "bay leaf": true,
	"ground coriander": true, "chili powder": true, "italian seasoning": true,
}

func IsStaple(name string) bool { return pantryStaples[Singular(name)] }

var nonWord = regexp.MustCompile(`[^a-z0-9 ]+`)

func words(s string) []string {
	return strings.Fields(nonWord.ReplaceAllString(strings.ToLower(s), " "))
}

// containsWords reports whether every word of needle appears, in order and
// adjacent, in hay ("feta" in "crumbled feta cheese").
func containsWords(hay, needle []string) bool {
	if len(needle) == 0 || len(needle) > len(hay) {
		return false
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		ok := true
		for j := range needle {
			if hay[i+j] != needle[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func singularWords(s string) []string {
	ws := words(s)
	for i, w := range ws {
		ws[i] = Singular(w)
	}
	return ws
}

// MatchPantry finds the pantry item that covers an ingredient, if any.
// Either name may be more specific: "feta" covers "crumbled feta", and
// "greek yogurt" covers "yogurt".
func MatchPantry(ingredient string, pantry []models.PantryItem) (models.PantryItem, bool) {
	iw := singularWords(ingredient)
	for _, p := range pantry {
		pw := singularWords(p.Name)
		if containsWords(iw, pw) || containsWords(pw, iw) {
			return p, true
		}
	}
	return models.PantryItem{}, false
}

// SplitByPantry returns which pantry items a recipe uses and which
// non-staple ingredients you'd still need.
func SplitByPantry(r models.Recipe, pantry []models.PantryItem) (uses []string, missing []models.Ingredient) {
	seen := map[string]bool{}
	for _, ing := range r.Ingredients {
		if p, ok := MatchPantry(ing.Name, pantry); ok {
			if !seen[p.Name] {
				seen[p.Name] = true
				uses = append(uses, p.Name)
			}
			continue
		}
		if !IsStaple(ing.Name) {
			missing = append(missing, ing)
		}
	}
	return uses, missing
}

// MergePantry adds incoming items, updating quantity/category when an item with
// the same (singular) name already exists. Returns how many were new.
func MergePantry(pantry map[string]models.PantryItem, incoming []models.PantryItem, newID func() string, now time.Time) int {
	byName := map[string]string{}
	for id, p := range pantry {
		byName[Singular(p.Name)] = id
	}
	added := 0
	for _, in := range incoming {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			continue
		}
		key := Singular(name)
		if id, ok := byName[key]; ok {
			p := pantry[id]
			if q := strings.TrimSpace(in.Quantity); q != "" {
				p.Quantity = q
			}
			if in.Category != "" {
				p.Category = strings.ToLower(in.Category)
			}
			p.AddedAt = now
			pantry[id] = p
			continue
		}
		it := models.PantryItem{
			ID: newID(), Name: strings.ToLower(name), Quantity: strings.TrimSpace(in.Quantity),
			Category: strings.ToLower(orDefault(in.Category, GuessCategory(name))), Source: orDefault(in.Source, "manual"), AddedAt: now,
		}
		pantry[it.ID] = it
		byName[key] = it.ID
		added++
	}
	return added
}

// ---------- photo scanning ----------

type DetectedItem struct {
	Name       string `json:"name"`
	Quantity   string `json:"quantity"`
	Category   string `json:"category"`
	Confidence string `json:"confidence"`
	Have       bool   `json:"have"` // already in the pantry
}

const scanSystem = `You identify food in photos of fridges, cupboards, pantries, shopping and receipts.
Respond with ONLY JSON.`

func BuildScanPrompt() (string, string) {
	return scanSystem, `TASK: pantry_scan
List every distinct food or drink ingredient you can see across all photos.
- Use plain generic names a recipe would use ("greek yogurt", not the brand).
- Estimate the amount in everyday words ("half a bag", "6", "about 500 g").
- Skip things you can't identify; say "low" confidence when unsure.
- Merge duplicates across photos.
- category is one of: produce, protein, dairy, grains, pantry, frozen, drinks, other.

Return: {"items": [{"name": "spinach", "quantity": "1 bag", "category": "produce", "confidence": "high|medium|low"}]}`
}

const MaxScanImages = 4

func ScanPantry(ctx context.Context, c llm.VisionClient, images []llm.Image, pantry []models.PantryItem) ([]DetectedItem, error) {
	if len(images) == 0 {
		return nil, errors.New("add at least one photo")
	}
	if len(images) > MaxScanImages {
		return nil, fmt.Errorf("up to %d photos at a time", MaxScanImages)
	}
	var out struct {
		Items []DetectedItem `json:"items"`
	}
	system, user := BuildScanPrompt()
	if err := llm.CompleteJSONWithImages(ctx, c, system, user, images, &out); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	items := []DetectedItem{}
	for _, it := range out.Items {
		it.Name = strings.ToLower(strings.TrimSpace(it.Name))
		if it.Name == "" || seen[Singular(it.Name)] {
			continue
		}
		seen[Singular(it.Name)] = true
		it.Category = strings.ToLower(orDefault(it.Category, "other"))
		_, it.Have = MatchPantry(it.Name, pantry)
		items = append(items, it)
	}
	return items, nil
}

// ---------- cooking from the pantry ----------

type CookRequest struct {
	MealType string `json:"mealType"`
	Count    int    `json:"count"`  // 1..3 ideas
	Strict   bool   `json:"strict"` // only pantry + staples
	Notes    string `json:"notes"`
}

const cookSystem = `You are a resourceful home cook. You invent realistic recipes from what someone
already has, with accurate per-serving macros. Hard constraints are NEVER broken.
Respond with ONLY JSON.`

func BuildCookPrompt(p models.Profile, fb Feedback, pantry []models.PantryItem, req CookRequest) (string, string) {
	var b strings.Builder
	fmt.Fprintf(&b, "TASK: pantry_cook\nCOUNT: %d\nMEAL: %s\n\n", req.Count, orDefault(req.MealType, "any"))
	b.WriteString("WHAT THEY HAVE:\n")
	for _, it := range pantry {
		if it.Quantity != "" {
			fmt.Fprintf(&b, "- %s (%s)\n", it.Name, it.Quantity)
		} else {
			fmt.Fprintf(&b, "- %s\n", it.Name)
		}
	}
	b.WriteString("Also assume: salt, pepper, cooking oil, water and common dried spices.\n\n")
	if req.Strict {
		b.WriteString("Use ONLY the items above plus those basics. Do not add anything else.\n")
	} else {
		b.WriteString("Build each recipe mostly from the items above. You may add at most 3 other ingredients if they make a much better dish.\n")
	}
	b.WriteString("Prefer using things that spoil first (fresh produce, dairy, opened items).\n")
	b.WriteString("Give each idea a clearly different style.\n\n")

	meals := p.MealsPerDay
	if meals <= 0 {
		meals = 3
	}
	fmt.Fprintf(&b, "One serving should be roughly %.0f kcal and %.0fg protein (their daily target split over %d meals).\n",
		p.Targets.Calories/float64(meals), p.Targets.Protein/float64(meals), meals)
	b.WriteString("\nHARD CONSTRAINTS:\n")
	fmt.Fprintf(&b, "- Diets: %s\n", orNone(p.Diets))
	fmt.Fprintf(&b, "- Never use: %s\n", orNone(p.Excluded))
	fmt.Fprintf(&b, "- Allergies (zero tolerance): %s\n", orNone(p.Allergies))
	if p.MaxPrepMins > 0 {
		fmt.Fprintf(&b, "- Max prep time: %d minutes\n", p.MaxPrepMins)
	}
	if fbText := fb.PromptText(); fbText != "" {
		b.WriteString("\nWHAT WE'VE LEARNED FROM THEIR RATINGS:\n" + fbText)
	}
	if n := strings.TrimSpace(req.Notes); n != "" {
		b.WriteString("\nTHEY SAID: " + n + "\n")
	}
	b.WriteString(`
Return: {"recipes": [ { "title", "description", "mealType", "servings", "prepMinutes",
"ingredients": [{"name","quantity","unit","category"}], "steps": [], "macros": {"calories","protein","carbs","fat","fiber"}, "tags": [] } ]}
List EVERY ingredient you use, including ones from their list. Macros are PER SERVING.`)
	return cookSystem, b.String()
}

// CookFromPantry generates recipe ideas, checks diet/allergy safety (and in strict
// mode, that nothing extra was added), retries once, then attaches pantry usage.
func CookFromPantry(ctx context.Context, c llm.Client, p models.Profile, fb Feedback, pantry []models.PantryItem, req CookRequest) ([]models.Recipe, error) {
	if len(pantry) == 0 {
		return nil, errors.New("add some ingredients to your pantry first")
	}
	if req.Count < 1 {
		req.Count = 1
	}
	if req.Count > 3 {
		req.Count = 3
	}
	system, user := BuildCookPrompt(p, fb, pantry, req)

	var out struct {
		Recipes []models.Recipe `json:"recipes"`
	}
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		out.Recipes = nil
		if err := llm.CompleteJSON(ctx, c, system, user, &out); err != nil {
			return nil, err
		}
		var problems []string
		if len(out.Recipes) == 0 {
			problems = append(problems, "no recipes returned")
		}
		for i := range out.Recipes {
			r := &out.Recipes[i]
			if strings.TrimSpace(r.Title) == "" || len(r.Ingredients) == 0 {
				problems = append(problems, "a recipe is missing its title or ingredients")
				continue
			}
			r.Warnings = CheckRecipe(*r, p)
			r.UsesPantry, r.Missing = SplitByPantry(*r, pantry)
			for _, w := range r.Warnings {
				problems = append(problems, r.Title+": "+w)
			}
			if req.Strict {
				for _, m := range r.Missing {
					problems = append(problems, fmt.Sprintf("%s: uses %q, which they don't have", r.Title, m.Name))
				}
			}
		}
		if len(problems) == 0 || attempt == MaxAttempts {
			if len(out.Recipes) == 0 {
				return nil, errors.New("the model didn't return any recipes")
			}
			break
		}
		user += BuildFixPrompt(problems)
	}

	recipes := out.Recipes[:0]
	for _, r := range out.Recipes {
		if strings.TrimSpace(r.Title) == "" || len(r.Ingredients) == 0 {
			continue
		}
		if r.Servings <= 0 {
			r.Servings = 1
		}
		r.MealType = strings.ToLower(orDefault(r.MealType, orDefault(req.MealType, "dinner")))
		for i := range r.Ingredients {
			r.Ingredients[i].Category = strings.ToLower(orDefault(r.Ingredients[i].Category, "other"))
		}
		r.Source = "pantry"
		recipes = append(recipes, r)
	}
	return recipes, nil
}
