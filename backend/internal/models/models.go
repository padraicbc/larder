// Package models holds the domain types shared by the store, API and LLM layers.
package models

import (
	"fmt"
	"strings"
	"time"
)

// Macros are always per serving unless stated otherwise.
type Macros struct {
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Carbs    float64 `json:"carbs"`
	Fat      float64 `json:"fat"`
	Fiber    float64 `json:"fiber"`
}

func (m Macros) Add(o Macros) Macros {
	return Macros{
		Calories: m.Calories + o.Calories,
		Protein:  m.Protein + o.Protein,
		Carbs:    m.Carbs + o.Carbs,
		Fat:      m.Fat + o.Fat,
		Fiber:    m.Fiber + o.Fiber,
	}
}

func (m Macros) Scale(f float64) Macros {
	return Macros{m.Calories * f, m.Protein * f, m.Carbs * f, m.Fat * f, m.Fiber * f}
}

type Profile struct {
	Name          string   `json:"name"`
	Sex           string   `json:"sex"` // "male" | "female"
	Age           int      `json:"age"`
	HeightCM      float64  `json:"heightCm"`
	WeightKG      float64  `json:"weightKg"`
	Activity      string   `json:"activity"` // sedentary | light | moderate | active | very_active
	Goal          string   `json:"goal"`     // lose | maintain | gain
	Targets       Macros   `json:"targets"`
	Diets         []string `json:"diets"` // vegetarian, vegan, pescatarian, gluten-free, dairy-free, keto, halal...
	Excluded      []string `json:"excluded"`
	Allergies     []string `json:"allergies"`
	Cuisines      []string `json:"cuisines"`
	MealsPerDay   int      `json:"mealsPerDay"`
	HouseholdSize int      `json:"householdSize"`
	MaxPrepMins   int      `json:"maxPrepMins"`
}

// SavedMeal is a named group of foods (e.g. 2 Weetabix + oats + protein
// powder) that can be logged again in one go or fixed into a meal plan slot.
type SavedMeal struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	MealType  string     `json:"mealType"` // usual slot: breakfast | lunch | dinner | snack
	Items     []MealItem `json:"items"`
	Macros    Macros     `json:"macros"` // total for one serving of the whole meal
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// WeightEntry is one body-weight reading; there is at most one per day.
type WeightEntry struct {
	Date      string    `json:"date"` // YYYY-MM-DD in the user's local day
	Kg        float64   `json:"kg"`
	CreatedAt time.Time `json:"createdAt"`
}

// Workout is one training session on a day (several per day are allowed).
type Workout struct {
	ID        string    `json:"id"`
	Date      string    `json:"date"` // YYYY-MM-DD in the user's local day
	Kind      string    `json:"kind"` // e.g. Running, Weights
	Minutes   int       `json:"minutes"`
	Calories  int       `json:"calories,omitempty"` // optional, as estimated by a tracker; 0 = not recorded
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// DefaultWorkoutKinds are always offered; users add their own on top.
var DefaultWorkoutKinds = []string{"Running", "Weights", "Walking", "Cycling", "Swimming", "HIIT", "Yoga", "Football"}

// FixedMeal is a meal slot with a set recipe in one plan. Macros are per serving.
type FixedMeal struct {
	MealType    string       `json:"mealType"`
	Title       string       `json:"title"`
	Ingredients []Ingredient `json:"ingredients"`
	Macros      Macros       `json:"macros"`
}

func DefaultProfile() Profile {
	return Profile{
		Activity:      "moderate",
		Goal:          "maintain",
		Targets:       Macros{Calories: 2000, Protein: 120, Carbs: 220, Fat: 70, Fiber: 30},
		MealsPerDay:   3,
		HouseholdSize: 1,
		Diets:         []string{},
		Excluded:      []string{},
		Allergies:     []string{},
		Cuisines:      []string{},
	}
}

type Ingredient struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Category string  `json:"category"` // produce, protein, dairy, grains, pantry, frozen, other
}

type Recipe struct {
	Demo        bool         `json:"demo,omitempty"`
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	MealType    string       `json:"mealType"` // breakfast | lunch | dinner | snack
	Servings    int          `json:"servings"`
	PrepMinutes int          `json:"prepMinutes"`
	Ingredients []Ingredient `json:"ingredients"`
	Steps       []string     `json:"steps"`
	Macros      Macros       `json:"macros"` // per serving
	Tags        []string     `json:"tags"`
	Warnings    []string     `json:"warnings,omitempty"`
	Source      string       `json:"source,omitempty"`     // "plan" | "pantry"
	UsesPantry  []string     `json:"usesPantry,omitempty"` // pantry items this recipe uses
	Missing     []Ingredient `json:"missing,omitempty"`    // what you'd still need to buy
	CreatedAt   time.Time    `json:"createdAt"`
}

type Rating struct {
	ID        string    `json:"id"`
	RecipeID  string    `json:"recipeId"`
	Stars     int       `json:"stars"` // 1..5
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
}

type PlanDay struct {
	Day       int      `json:"day"`
	RecipeIDs []string `json:"recipeIds"`
}

type MealPlan struct {
	Demo      bool      `json:"demo,omitempty"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Notes     string    `json:"notes"`
	Days      []PlanDay `json:"days"`
	CreatedAt time.Time `json:"createdAt"`
}

type ShoppingItem struct {
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Category string  `json:"category"`
	Checked  bool    `json:"checked"`
	Manual   bool    `json:"manual"`
}

type ShoppingList struct {
	PlanID string         `json:"planId"`
	Items  []ShoppingItem `json:"items"`
}

type Product struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Ingredients        string    `json:"ingredients"`
	BasisQuantity      float64   `json:"basisQuantity"`
	BasisUnit          string    `json:"basisUnit"`
	Macros             Macros    `json:"macros"`
	FromNutritionLabel bool      `json:"fromNutritionLabel"`
	Confidence         string    `json:"confidence"`
	Notes              string    `json:"notes"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	Barcode            string    `json:"barcode,omitempty"` // EAN/UPC digits, checksum-verified
	Source             string    `json:"source,omitempty"`  // "" (label/manual) | "openfoodfacts"
	// Rating is computed on the fly from Ingredients and Macros; never stored.
	Rating *ProductRating `json:"rating,omitempty"`
}

// LookupSettings control optional online barcode lookups (server-wide, off by default).
type LookupSettings struct {
	OpenFoodFacts bool `json:"openFoodFacts"`
}

// Scan is something checked in the shop: found by barcode or read from a photo
// of its ingredients. Nutrition is optional; a rating needs only ingredients.
type Scan struct {
	ID            string         `json:"id,omitempty"` // set once saved
	Name          string         `json:"name"`
	Barcode       string         `json:"barcode,omitempty"`
	Ingredients   string         `json:"ingredients"`
	HasNutrition  bool           `json:"hasNutrition"`
	BasisQuantity float64        `json:"basisQuantity,omitempty"` // nutrition is per this much
	BasisUnit     string         `json:"basisUnit,omitempty"`     // g | ml | serving
	Macros        Macros         `json:"macros"`
	Source        string         `json:"source"`              // saved | openfoodfacts | photo
	ProductID     string         `json:"productId,omitempty"` // set when it is already one of your products
	Notes         string         `json:"notes,omitempty"`
	Warning       string         `json:"warning,omitempty"` // ingredients missing or not in English, so the rating is provisional
	CreatedAt     time.Time      `json:"createdAt"`
	Rating        *ProductRating `json:"rating,omitempty"` // computed, never stored
}

// ProductRating scores a product 0–100 (higher is better) with the reasons why.
type ProductRating struct {
	Rules int `json:"rules"` // which version of the rating rules produced this (see nutrition.RatingRules)
	// At is when this rating was calculated. It changes on every request because
	// ratings are never stored; they are worked out each time they are shown.
	At   time.Time `json:"at"`
	Base int       `json:"base"` // every product starts here before points are added or removed
	// Provisional means the ingredients weren't available or couldn't be fully checked,
	// so additives may have been missed. Get the ingredients to firm the rating up.
	Provisional bool           `json:"provisional,omitempty"`
	Score       int            `json:"score"`
	Label       string         `json:"label"` // excellent | good | fair | poor | avoid
	Factors     []RatingFactor `json:"factors"`
}

// RatingSettings are one person's tweaks to the rating rules. Empty means the
// defaults. Points are keyed by rule (see nutrition.RuleCatalogue): a key that is
// present replaces that rule's default, and 0 switches the rule off.
type RatingSettings struct {
	Points map[string]int `json:"points,omitempty"`
	Custom []CustomRule   `json:"custom,omitempty"`
}

// Validate checks the limits on a person's tweaks. known, if given, says which rule keys exist.
func (r RatingSettings) Validate(known func(string) bool) error {
	for k, v := range r.Points {
		if known != nil && !known(k) {
			return fmt.Errorf("unknown rule %q", k)
		}
		min, max := -50, 50
		if k == "base" {
			min, max = 0, 100
		}
		if v < min || v > max {
			return fmt.Errorf("points for %q must be between %d and %d", k, min, max)
		}
	}
	if len(r.Custom) > 50 {
		return fmt.Errorf("at most 50 rules of your own")
	}
	for _, c := range r.Custom {
		m := strings.TrimSpace(c.Match)
		if n := len([]rune(m)); n < 2 || n > 60 {
			return fmt.Errorf("a rule's word or phrase must be 2 to 60 characters")
		}
		if c.Points < -50 || c.Points > 50 || c.Points == 0 {
			return fmt.Errorf("points for %q must be between -50 and 50, and not 0", m)
		}
	}
	return nil
}

// CustomRule adds points whenever Match appears as a word or phrase in the ingredients.
type CustomRule struct {
	Match  string `json:"match"`
	Points int    `json:"points"`
}

// RatingFactor is one thing that moved the score (negative = worse).
type RatingFactor struct {
	Key    string `json:"key,omitempty"` // which rule this was, so it can be tweaked in Settings
	Text   string `json:"text"`
	Points int    `json:"points"`
	Why    string `json:"why,omitempty"` // plain-language reason this matters
}

type DayAdjustment struct {
	Calories float64 `json:"calories"` // added to the normal daily calorie target
	Note     string  `json:"note"`
}

type MealItem struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
}

type LogEntry struct {
	Items     []MealItem `json:"items,omitempty"`
	ID        string     `json:"id"`
	Date      string     `json:"date"` // YYYY-MM-DD in the user's local day
	MealType  string     `json:"mealType"`
	Name      string     `json:"name"`
	Servings  float64    `json:"servings"`
	Macros    Macros     `json:"macros"` // total for this entry (already multiplied by servings)
	RecipeID  string     `json:"recipeId,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"passwordHash"`
	Admin        bool      `json:"admin"` // first account; can change model settings
	CreatedAt    time.Time `json:"createdAt"`
}

// PublicUser is what the API returns about an account.
type PublicUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
}

func (u User) Public() PublicUser { return PublicUser{u.ID, u.Email, u.Name, u.Admin} }

// PantryItem is something in the fridge or cupboard. Quantity is free text
// ("half a bag", "6") because people don't weigh their fridge.
type PantryItem struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Quantity string    `json:"quantity"`
	Category string    `json:"category"`
	Source   string    `json:"source"` // manual | photo
	AddedAt  time.Time `json:"addedAt"`
}
