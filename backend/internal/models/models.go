// Package models holds the domain types shared by the store, API and LLM layers.
package models

import "time"

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
