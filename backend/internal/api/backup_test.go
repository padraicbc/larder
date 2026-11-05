package api

import (
	"github.com/you/nutrition-app/internal/models"
	"github.com/you/nutrition-app/internal/store"
	"testing"
)

func TestBackupPreviewAndRestoreAreScoped(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "backup@example.com")
	c.do("POST", "/api/log", map[string]any{"name": "Porridge", "servings": 1, "macros": models.Macros{Calories: 350}}, nil)
	var backup store.Backup
	if code := c.do("GET", "/api/backup", nil, &backup); code != 200 {
		t.Fatal(code)
	}
	c.do("POST", "/api/log", map[string]any{"name": "Coffee", "servings": 1}, nil)
	var summary store.BackupSummary
	if code := c.do("POST", "/api/backup/preview", backup, &summary); code != 200 || summary.Logs != 1 {
		t.Fatalf("preview: %d %+v", code, summary)
	}
	var day dayLog
	c.do("GET", "/api/log", nil, &day)
	if len(day.Entries) != 2 {
		t.Fatal("preview modified active data")
	}
	if code := c.do("POST", "/api/backup/restore", map[string]string{"not": "a backup"}, nil); code != 400 {
		t.Errorf("invalid backup = %d", code)
	}
	c.do("GET", "/api/log", nil, &day)
	if len(day.Entries) != 2 {
		t.Fatal("invalid restore changed active data")
	}
	peer := signUp(t, c0.srv, "peer-backup@example.com")
	peer.do("POST", "/api/log", map[string]any{"name": "Private meal", "servings": 1}, nil)
	if code := c.do("POST", "/api/backup/restore", backup, &summary); code != 200 {
		t.Fatal(code)
	}
	c.do("GET", "/api/log", nil, &day)
	if len(day.Entries) != 1 || day.Entries[0].Name != "Porridge" {
		t.Fatal("restore did not replace logs")
	}
	peer.do("GET", "/api/log", nil, &day)
	if len(day.Entries) != 1 || day.Entries[0].Name != "Private meal" {
		t.Fatal("restore changed another account")
	}
	if code := c.do("GET", "/api/auth/me", nil, nil); code != 200 {
		t.Fatal("restore broke the active sign-in")
	}
	if code := c0.do("GET", "/api/backup", nil, nil); code != 401 {
		t.Errorf("unauthenticated export = %d", code)
	}
	if code := c0.do("POST", "/api/backup/restore", backup, nil); code != 401 {
		t.Errorf("unauthenticated restore = %d", code)
	}
}
