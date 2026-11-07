package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const BackupVersion = 1
const MaxBackupBytes = 32 << 20

type Backup struct {
	Format     string    `json:"format"`
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exportedAt"`
	Data       *userData `json:"data"`
}

type BackupSummary struct {
	ExportedAt     time.Time `json:"exportedAt"`
	Recipes        int       `json:"recipes"`
	Plans          int       `json:"plans"`
	Logs           int       `json:"logs"`
	Products       int       `json:"products"`
	Pantry         int       `json:"pantry"`
	Ratings        int       `json:"ratings"`
	DayAdjustments int       `json:"dayAdjustments"`
	Meals          int       `json:"meals"`
	Weights        int       `json:"weights"`
	Workouts       int       `json:"workouts"`
	Scans          int       `json:"scans"`
}

func (s *Store) ExportBackup(uid string, now time.Time) (Backup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Copy under lock so encoding a response cannot race with later edits.
	data, err := json.Marshal(s.view(uid))
	if err != nil {
		return Backup{}, err
	}
	var u userData
	if err = json.Unmarshal(data, &u); err != nil {
		return Backup{}, err
	}
	u.stripRatings()
	return Backup{Format: "larder-backup", Version: BackupVersion, ExportedAt: now, Data: &u}, nil
}

func ReadBackup(reader io.Reader) (Backup, error) {
	var b Backup
	raw, err := io.ReadAll(io.LimitReader(reader, MaxBackupBytes+1))
	if err != nil {
		return b, err
	}
	if len(raw) > MaxBackupBytes {
		return b, errors.New("backup is over 32 MB")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&b); err != nil {
		return b, errors.New("this is not a valid Larder backup JSON file")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return b, errors.New("backup must contain one JSON document")
	}
	if b.Format != "larder-backup" || b.Version != BackupVersion || b.Data == nil {
		return b, errors.New("unsupported backup format or version")
	}
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return b, err
	}
	profile, ok := envelope.Data["profile"]
	if !ok || bytes.Equal(bytes.TrimSpace(profile), []byte("null")) {
		return b, errors.New("backup is missing its profile")
	}
	if b.ExportedAt.IsZero() {
		return b, errors.New("backup is missing its export date")
	}
	for _, key := range []string{"recipes", "plans", "shopping", "log", "pantry", "ratings"} {
		if _, ok := envelope.Data[key]; !ok {
			return b, fmt.Errorf("backup is missing %s data", key)
		}
	}
	var profileFields map[string]json.RawMessage
	if err := json.Unmarshal(profile, &profileFields); err != nil {
		return b, errors.New("backup contains an invalid profile")
	}
	if _, ok := profileFields["targets"]; !ok {
		return b, errors.New("backup profile is missing nutrition targets")
	}
	b.Data.fillNil()
	// Reject inconsistent references before changing the active data.
	for id, r := range b.Data.Recipes {
		if id == "" || r.ID != id {
			return b, errors.New("backup contains an invalid recipe ID")
		}
	}
	for id, p := range b.Data.Plans {
		if id == "" || p.ID != id {
			return b, errors.New("backup contains an invalid plan ID")
		}
		for _, d := range p.Days {
			for _, rid := range d.RecipeIDs {
				if _, ok := b.Data.Recipes[rid]; !ok {
					return b, fmt.Errorf("backup plan refers to missing recipe %s", rid)
				}
			}
		}
	}
	for id, e := range b.Data.Log {
		if id == "" || e.ID != id {
			return b, errors.New("backup contains an invalid food log ID")
		}
	}
	for id, p := range b.Data.Products {
		if id == "" || p.ID != id || p.BasisQuantity <= 0 || (p.BasisUnit != "g" && p.BasisUnit != "ml" && p.BasisUnit != "serving") {
			return b, errors.New("backup contains an invalid product")
		}
	}
	for id, m := range b.Data.Meals {
		if id == "" || m.ID != id || strings.TrimSpace(m.Name) == "" {
			return b, errors.New("backup contains an invalid saved meal")
		}
	}
	if err := b.Data.RatingRules.Validate(nil); err != nil {
		return b, errors.New("backup has invalid rating rules: " + err.Error())
	}
	for id, sc := range b.Data.Scans {
		if id == "" || sc.ID != id || strings.TrimSpace(sc.Name) == "" {
			return b, errors.New("backup contains an invalid scan")
		}
	}
	for id, w := range b.Data.Workouts {
		if id == "" || w.ID != id || strings.TrimSpace(w.Kind) == "" || w.Minutes < 1 || w.Minutes > 1440 || w.Calories < 0 || w.Calories > 20000 {
			return b, errors.New("backup contains an invalid workout")
		}
	}
	for date, w := range b.Data.Weights {
		if date == "" || w.Date != date || !(w.Kg > 0) || w.Kg > 1000 {
			return b, errors.New("backup contains an invalid weight entry")
		}
	}
	for id, p := range b.Data.Pantry {
		if id == "" || p.ID != id {
			return b, errors.New("backup contains an invalid pantry item")
		}
	}
	return b, nil
}

func (b Backup) Summary() BackupSummary {
	u := b.Data
	return BackupSummary{ExportedAt: b.ExportedAt, Recipes: len(u.Recipes), Plans: len(u.Plans), Logs: len(u.Log), Products: len(u.Products), Pantry: len(u.Pantry), Ratings: len(u.Ratings), DayAdjustments: len(u.DayAdjustments), Meals: len(u.Meals), Weights: len(u.Weights), Workouts: len(u.Workouts), Scans: len(u.Scans)}
}

func (s *Store) RestoreBackup(uid string, b Backup) error {
	// Validate again so callers cannot bypass the import checks.
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	validated, err := ReadBackup(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, existed := s.d.Users[uid]
	s.d.Users[uid] = validated.Data
	if err = s.saveLocked(); err != nil {
		if existed {
			s.d.Users[uid] = previous
		} else {
			delete(s.d.Users, uid)
		}
		return err
	}
	return nil
}
