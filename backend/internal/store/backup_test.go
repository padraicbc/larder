package store

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
)

func TestBackupRoundTripAndPersistence(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.json"))
	must(t, err)
	p := s.Profile(uid)
	p.Name = "Backup owner"
	must(t, s.SetProfile(uid, p))
	must(t, s.PutRecipes(uid, models.Recipe{ID: "recipe", Title: "Porridge", Demo: true}))
	must(t, s.PutPlan(uid, models.MealPlan{ID: "plan", Demo: true, Days: []models.PlanDay{{Day: 1, RecipeIDs: []string{"recipe"}}}}, models.ShoppingList{PlanID: "plan", Items: []models.ShoppingItem{{Name: "Oats", Checked: true}}}))
	must(t, s.AddLog(uid, models.LogEntry{ID: "meal", Date: "2026-11-05", Name: "Porridge", Items: []models.MealItem{{Name: "Oats", Quantity: "60 g"}}}))
	_, err = s.PutProduct(uid, models.Product{Name: "Oats", BasisQuantity: 100, BasisUnit: "g", Macros: models.Macros{Calories: 379.5}}, time.Now())
	must(t, err)
	must(t, s.SetDayAdjustment(uid, "2026-11-05", models.DayAdjustment{Calories: 300, Note: "Walk"}))
	must(t, s.UpdatePantry(uid, func(items map[string]models.PantryItem) error {
		items["oats"] = models.PantryItem{ID: "oats", Name: "Oats"}
		return nil
	}))
	must(t, s.AddRating(uid, models.Rating{RecipeID: "recipe", Stars: 5}))
	must(t, s.SetLLMConfig(llm.Config{Provider: "openai", APIKey: "secret-api-key"}))
	peer := s.Profile("peer")
	peer.Name = "Other user's private data"
	must(t, s.SetProfile("peer", peer))
	backup, err := s.ExportBackup(uid, time.Now())
	must(t, err)
	raw, err := json.Marshal(backup)
	must(t, err)
	if bytes.Contains(raw, []byte("secret-api-key")) || bytes.Contains(raw, []byte(peer.Name)) {
		t.Fatal("backup leaked server settings or another user's data")
	}
	// Hidden demo data must still be included in a complete backup.
	if backup.Summary().Recipes != 1 || backup.Summary().Products != 1 || backup.Summary().Ratings != 1 {
		t.Fatal("backup omitted saved data")
	}
	p.Name = "Changed"
	must(t, s.SetProfile(uid, p))
	must(t, s.DeleteLog(uid, "meal"))
	parsed, err := ReadBackup(bytes.NewReader(raw))
	must(t, err)
	must(t, s.RestoreBackup(uid, parsed))
	reopened, err := Open(s.path)
	must(t, err)
	if reopened.Profile(uid).Name != "Backup owner" || len(reopened.LogBetween(uid, "2026-11-05", "2026-11-05")) != 1 || len(reopened.Products(uid)) != 1 || reopened.DayAdjustment(uid, "2026-11-05").Calories != 300 {
		t.Fatal("restore lost data")
	}
	list, err := reopened.Shopping(uid, "plan")
	if err == nil && !list.Items[0].Checked {
		t.Fatal("shopping state lost")
	}
	if reopened.Profile("peer").Name != peer.Name || reopened.LLMConfig().APIKey != "secret-api-key" {
		t.Fatal("restore changed another account or model settings")
	}
}

func TestBackupRejectsInvalidFiles(t *testing.T) {
	for _, raw := range []string{`not JSON`, `{}`, `{"format":"larder-backup","version":999,"data":{}}`, `{"format":"larder-backup","version":1,"data":{}}`, `{"format":"larder-backup","version":1,"data":{"profile":{}}}`} {
		if _, err := ReadBackup(strings.NewReader(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	s, _ := Open("")
	backup, err := s.ExportBackup(uid, time.Now())
	must(t, err)
	backup.Data.Plans["bad"] = models.MealPlan{ID: "bad", Days: []models.PlanDay{{RecipeIDs: []string{"missing"}}}}
	if err := s.RestoreBackup(uid, backup); err == nil {
		t.Fatal("accepted missing recipe reference")
	}
	if len(s.Plans(uid)) != 0 {
		t.Fatal("invalid restore changed active data")
	}
}
