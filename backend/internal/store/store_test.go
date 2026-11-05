package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/you/nutrition-app/internal/models"
)

const uid = "u1"

func TestPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "data.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := s.Profile(uid)
	p.Name = "Sam"
	p.Diets = []string{"vegan"}
	must(t, s.SetProfile(uid, p))
	must(t, s.PutRecipes(uid, models.Recipe{ID: "r1", Title: "Soup"}))
	must(t, s.AddRating(uid, models.Rating{RecipeID: "r1", Stars: 4}))
	must(t, s.UpdatePantry(uid, func(m map[string]models.PantryItem) error {
		m["p1"] = models.PantryItem{ID: "p1", Name: "eggs"}
		return nil
	}))

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Profile(uid).Name != "Sam" || s2.Profile(uid).Diets[0] != "vegan" {
		t.Errorf("profile not persisted: %+v", s2.Profile(uid))
	}
	if r, err := s2.Recipe(uid, "r1"); err != nil || r.Title != "Soup" {
		t.Errorf("recipe not persisted: %+v %v", r, err)
	}
	if len(s2.Ratings(uid)) != 1 || len(s2.Pantry(uid)) != 1 {
		t.Error("rating or pantry not persisted")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestUsersAreIsolated(t *testing.T) {
	s, _ := Open("")
	must(t, s.PutRecipes("alice", models.Recipe{ID: "r1", Title: "Alice's soup"}))
	if _, err := s.Recipe("bob", "r1"); !errors.Is(err, ErrNotFound) {
		t.Error("bob can see alice's recipe")
	}
	if err := s.AddRating("bob", models.Rating{RecipeID: "r1", Stars: 1}); !errors.Is(err, ErrNotFound) {
		t.Error("bob can rate alice's recipe")
	}
	if len(s.Recipes("bob")) != 0 {
		t.Error("bob's recipe list should be empty")
	}
	// Reading for an unknown user returns defaults without creating data.
	if s.Profile("nobody").MealsPerDay != 3 {
		t.Error("unknown user should get the default profile")
	}
}

func TestAccounts(t *testing.T) {
	s, _ := Open("")
	now := time.Now()
	first, err := s.CreateUser("  Ada@Example.com ", "Ada", "hash", now)
	if err != nil || !first.Admin || first.Email != "ada@example.com" {
		t.Fatalf("first user = %+v, %v", first, err)
	}
	second, _ := s.CreateUser("bo@example.com", "Bo", "hash", now)
	if second.Admin {
		t.Error("only the first account is admin")
	}
	if _, err := s.CreateUser("ADA@example.com", "", "h", now); !errors.Is(err, ErrEmailTaken) {
		t.Error("duplicate email (case-insensitive) should be rejected")
	}
	if u, err := s.UserByEmail("ada@EXAMPLE.com"); err != nil || u.ID != first.ID {
		t.Error("lookup by email should ignore case")
	}
	if s.UserCount() != 2 {
		t.Errorf("UserCount = %d", s.UserCount())
	}
}

func TestJWTSecretIsStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.json")
	s, _ := Open(path)
	a, err := s.JWTSecret()
	if err != nil || len(a) != 32 {
		t.Fatal(err, len(a))
	}
	s2, _ := Open(path)
	b, _ := s2.JWTSecret()
	if string(a) != string(b) {
		t.Error("secret should survive a restart")
	}
}

// A data file from the single-user version is handed to the first account.
func TestMigratesV1File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.json")
	v1 := `{"profile":{"name":"Old me","mealsPerDay":4},"llm":{"provider":"openai","model":"m"},
	        "recipes":{"r1":{"id":"r1","title":"Old soup"}},"ratings":[],"plans":{},"shopping":{},"log":{}}`
	must(t, os.WriteFile(path, []byte(v1), 0o600))

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.LLMConfig().Model != "m" {
		t.Error("LLM settings should carry over")
	}
	u, _ := s.CreateUser("me@example.com", "", "h", time.Now())
	if s.Profile(u.ID).Name != "Old me" {
		t.Errorf("first user should adopt old data, got %+v", s.Profile(u.ID))
	}
	if _, err := s.Recipe(u.ID, "r1"); err != nil {
		t.Error("old recipes should carry over")
	}
	other, _ := s.CreateUser("you@example.com", "", "h", time.Now())
	if len(s.Recipes(other.ID)) != 0 {
		t.Error("second user must start empty")
	}
	// And it was rewritten as v2.
	s2, _ := Open(path)
	if s2.d.Version != 2 || s2.d.Unclaimed != nil {
		t.Error("file should be saved as v2 with nothing unclaimed")
	}
}

func TestLogBetween(t *testing.T) {
	s, _ := Open("")
	now := time.Now()
	for i, d := range []string{"2026-01-01", "2026-01-02", "2026-01-02", "2026-01-05"} {
		_ = s.AddLog(uid, models.LogEntry{ID: NewID(), Date: d, CreatedAt: now.Add(time.Duration(i) * time.Second)})
	}
	if got := s.LogBetween(uid, "2026-01-02", "2026-01-04"); len(got) != 2 {
		t.Errorf("want 2 entries, got %d", len(got))
	}
	all := s.LogBetween(uid, "2026-01-01", "2026-12-31")
	if len(all) != 4 || all[0].Date != "2026-01-01" || all[3].Date != "2026-01-05" {
		t.Errorf("entries not sorted: %+v", all)
	}
}

func TestPlansAndShopping(t *testing.T) {
	s, _ := Open("")
	old := models.MealPlan{ID: "old", CreatedAt: time.Now().Add(-time.Hour)}
	newer := models.MealPlan{ID: "new", CreatedAt: time.Now()}
	_ = s.PutPlan(uid, old, models.ShoppingList{PlanID: "old"})
	_ = s.PutPlan(uid, newer, models.ShoppingList{PlanID: "new", Items: []models.ShoppingItem{{Key: "k"}}})

	if ps := s.Plans(uid); ps[0].ID != "new" {
		t.Error("plans should be newest first")
	}
	l, err := s.UpdateShopping(uid, "new", func(l *models.ShoppingList) error { l.Items[0].Checked = true; return nil })
	if err != nil || !l.Items[0].Checked {
		t.Errorf("update failed: %+v %v", l, err)
	}
	if _, err := s.UpdateShopping(uid, "ghost", func(*models.ShoppingList) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Error("missing list should be ErrNotFound")
	}
	_ = s.DeletePlan(uid, "new")
	if _, err := s.Shopping(uid, "new"); !errors.Is(err, ErrNotFound) {
		t.Error("deleting a plan should delete its list")
	}
}

func TestDemoVisibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, err := Open(path)
	must(t, err)
	must(t, s.PutRecipes(uid, models.Recipe{ID: "demo", Demo: true}, models.Recipe{ID: "legacy"}, models.Recipe{ID: "real"}))
	for _, p := range []models.MealPlan{
		{ID: "demo-plan", Demo: true},
		{ID: "legacy-plan", Title: "3-day plan (mock)", Days: []models.PlanDay{{RecipeIDs: []string{"legacy"}}}},
		{ID: "real-plan"},
	} {
		must(t, s.PutPlan(uid, p, models.ShoppingList{PlanID: p.ID}))
	}
	for _, id := range []string{"demo", "legacy", "real", "manual"} {
		must(t, s.AddLog(uid, models.LogEntry{ID: id, RecipeID: id, Date: "2026-11-05"}))
	}
	cfg := s.LLMConfig()
	cfg.Provider = "openai"
	must(t, s.SetLLMConfig(cfg))
	s, err = Open(path)
	must(t, err)
	if len(s.Recipes(uid)) != 1 || len(s.Plans(uid)) != 1 || len(s.LogBetween(uid, "2026-11-05", "2026-11-05")) != 2 {
		t.Fatal("incorrect real-mode visibility")
	}
	for _, id := range []string{"demo", "legacy"} {
		if _, err := s.Recipe(uid, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("demo recipe %s accessible", id)
		}
	}
	for _, id := range []string{"demo-plan", "legacy-plan"} {
		if _, err := s.Plan(uid, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("demo plan %s accessible", id)
		}
		if _, err := s.Shopping(uid, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("demo shopping %s accessible", id)
		}
	}
	cfg.Provider = "mock"
	must(t, s.SetLLMConfig(cfg))
	if len(s.Recipes(uid)) != 3 || len(s.Plans(uid)) != 3 || len(s.LogBetween(uid, "2026-11-05", "2026-11-05")) != 4 {
		t.Fatal("demo data should reappear")
	}
}

func TestMealQuantitiesPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, err := Open(path)
	must(t, err)
	must(t, s.AddLog(uid, models.LogEntry{ID: "photo", Date: "2026-11-05", Items: []models.MealItem{{Name: "rice", Quantity: "150 g"}, {Name: "sauce", Quantity: "1 tbsp"}}}))
	s, err = Open(path)
	must(t, err)
	entries := s.LogBetween(uid, "2026-11-05", "2026-11-05")
	if len(entries) != 1 || len(entries[0].Items) != 2 || entries[0].Items[0].Quantity != "150 g" {
		t.Fatalf("lost meal quantities after reopen: %+v", entries)
	}
}

func TestDayAdjustmentsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, err := Open(path)
	must(t, err)
	must(t, s.SetDayAdjustment(uid, "2026-11-05", models.DayAdjustment{Calories: 300, Note: "Long walk"}))
	s, err = Open(path)
	must(t, err)
	if got := s.DayAdjustment(uid, "2026-11-05"); got.Calories != 300 || got.Note != "Long walk" {
		t.Fatalf("lost adjustment: %+v", got)
	}
	if s.Profile(uid).Targets.Calories != 2000 {
		t.Fatal("usual target changed")
	}
	must(t, s.SetDayAdjustment(uid, "2026-11-05", models.DayAdjustment{}))
	s, err = Open(path)
	must(t, err)
	if s.DayAdjustment(uid, "2026-11-05").Calories != 0 {
		t.Fatal("reset did not persist")
	}
}

func TestProductsPersistWithoutLogs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, err := Open(path)
	must(t, err)
	product, err := s.PutProduct(uid, models.Product{Name: "Oats", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true, Macros: models.Macros{Calories: 379.5, Protein: 13.2}}, time.Now())
	must(t, err)
	s, err = Open(path)
	must(t, err)
	products := s.Products(uid)
	if len(products) != 1 || products[0].ID != product.ID || products[0].Macros.Calories != 379.5 || products[0].BasisUnit != "g" {
		t.Fatalf("lost saved nutrition: %+v", products)
	}
	if len(s.Products("another-user")) != 0 {
		t.Fatal("another user's products visible")
	}
}
