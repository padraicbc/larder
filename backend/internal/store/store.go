// Package store persists everything to a single JSON file, partitioned per user.
// Zero dependencies and fine for a household or small group; swap for
// SQLite/Postgres behind the same methods when you outgrow it.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
)

var (
	ErrInvalidTarget = errors.New("the adjusted calorie target must be greater than zero")
	ErrNotFound      = errors.New("not found")
	ErrNameTaken     = errors.New("you already have a meal with that name")
	ErrEmailTaken    = errors.New("an account with that email already exists")
)

// userData is everything that belongs to one account.
type userData struct {
	Products       map[string]models.Product       `json:"products,omitempty"`
	DayAdjustments map[string]models.DayAdjustment `json:"dayAdjustments,omitempty"`
	Profile        models.Profile                  `json:"profile"`
	Recipes        map[string]models.Recipe        `json:"recipes"`
	Ratings        []models.Rating                 `json:"ratings"`
	Plans          map[string]models.MealPlan      `json:"plans"`
	Shopping       map[string]models.ShoppingList  `json:"shopping"`
	Log            map[string]models.LogEntry      `json:"log"`
	Pantry         map[string]models.PantryItem    `json:"pantry"`
	Meals          map[string]models.SavedMeal     `json:"meals,omitempty"`
	Weights        map[string]models.WeightEntry   `json:"weights,omitempty"` // by date
	Workouts       map[string]models.Workout       `json:"workouts,omitempty"`
	Scans          map[string]models.Scan          `json:"scans,omitempty"`
	RatingRules    models.RatingSettings           `json:"ratingRules"`            // this person's tweaks to the rating rules; empty = defaults
	WorkoutKinds   []string                        `json:"workoutKinds,omitempty"` // custom kinds the user added
}

func newUserData() *userData {
	return &userData{
		Profile:        models.DefaultProfile(),
		Products:       map[string]models.Product{},
		DayAdjustments: map[string]models.DayAdjustment{},
		Recipes:        map[string]models.Recipe{},
		Plans:          map[string]models.MealPlan{},
		Shopping:       map[string]models.ShoppingList{},
		Log:            map[string]models.LogEntry{},
		Pantry:         map[string]models.PantryItem{},
		Meals:          map[string]models.SavedMeal{},
		Weights:        map[string]models.WeightEntry{},
		Workouts:       map[string]models.Workout{},
		Scans:          map[string]models.Scan{},
	}
}

// fillNil repairs maps that are missing in older files.
func (u *userData) fillNil() {
	u.stripRatings() // a rating left in an older file is ignored, not trusted
	if u.Products == nil {
		u.Products = map[string]models.Product{}
	}
	if u.DayAdjustments == nil {
		u.DayAdjustments = map[string]models.DayAdjustment{}
	}
	if u.Recipes == nil {
		u.Recipes = map[string]models.Recipe{}
	}
	if u.Plans == nil {
		u.Plans = map[string]models.MealPlan{}
	}
	if u.Shopping == nil {
		u.Shopping = map[string]models.ShoppingList{}
	}
	if u.Log == nil {
		u.Log = map[string]models.LogEntry{}
	}
	if u.Pantry == nil {
		u.Pantry = map[string]models.PantryItem{}
	}
	if u.Scans == nil {
		u.Scans = map[string]models.Scan{}
	}
	if u.Workouts == nil {
		u.Workouts = map[string]models.Workout{}
	}
	if u.Weights == nil {
		u.Weights = map[string]models.WeightEntry{}
	}
	if u.Meals == nil {
		u.Meals = map[string]models.SavedMeal{}
	}
	// Preserve legacy demo provenance even if the originating plan is deleted.
	for id, p := range u.Plans {
		if !demoPlan(p) {
			continue
		}
		p.Demo = true
		u.Plans[id] = p
		for _, day := range p.Days {
			for _, rid := range day.RecipeIDs {
				if r, ok := u.Recipes[rid]; ok {
					r.Demo = true
					u.Recipes[rid] = r
				}
			}
		}
	}
}

type fileData struct {
	Version   int                    `json:"version"`
	JWTSecret string                 `json:"jwtSecret,omitempty"`
	LLM       llm.Config             `json:"llm"`
	Lookup    models.LookupSettings  `json:"lookup"`
	Accounts  map[string]models.User `json:"accounts"` // by user ID
	Users     map[string]*userData   `json:"users"`    // by user ID
	// Unclaimed holds data from a v1 (single-user) file until the first account adopts it.
	Unclaimed *userData `json:"unclaimed,omitempty"`
}

// v1 files had one user's data at the top level.
type v1File struct {
	Profile  *models.Profile                `json:"profile"`
	Recipes  map[string]models.Recipe       `json:"recipes"`
	Ratings  []models.Rating                `json:"ratings"`
	Plans    map[string]models.MealPlan     `json:"plans"`
	Shopping map[string]models.ShoppingList `json:"shopping"`
	Log      map[string]models.LogEntry     `json:"log"`
}

type Store struct {
	mu   sync.RWMutex
	path string // "" = memory only (tests)
	d    fileData
}

func emptyFile() fileData {
	return fileData{Version: 2, LLM: llm.DefaultConfig(), Accounts: map[string]models.User{}, Users: map[string]*userData{}}
}

// Open loads path, creating it if missing. Pass "" for an in-memory store.
func Open(path string) (*Store, error) {
	s := &Store{path: path, d: emptyFile()}
	if path == "" {
		return s, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, s.saveLocked()
	}
	if err != nil {
		return nil, err
	}
	s.d = fileData{LLM: llm.DefaultConfig()} // Version stays 0 for v1 files
	if err := json.Unmarshal(b, &s.d); err != nil {
		return nil, err
	}
	if s.d.Accounts == nil {
		s.d.Accounts = map[string]models.User{}
	}
	if s.d.Users == nil {
		s.d.Users = map[string]*userData{}
	}
	for _, u := range s.d.Users {
		u.fillNil()
	}
	if s.d.Version < 2 {
		var old v1File
		if err := json.Unmarshal(b, &old); err != nil {
			return nil, err
		}
		if old.Profile != nil {
			u := &userData{Profile: *old.Profile, Recipes: old.Recipes, Ratings: old.Ratings,
				Plans: old.Plans, Shopping: old.Shopping, Log: old.Log}
			u.fillNil()
			s.d.Unclaimed = u
		}
		s.d.Version = 2
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// saveLocked writes atomically. Caller holds the write lock.
// stripRatings removes computed ratings. They are worked out from the ingredients and
// nutrition every time they are shown, so storing one would only let it go stale.
func (u *userData) stripRatings() {
	for id, p := range u.Products {
		if p.Rating != nil {
			p.Rating = nil
			u.Products[id] = p
		}
	}
	for id, sc := range u.Scans {
		if sc.Rating != nil {
			sc.Rating = nil
			u.Scans[id] = sc
		}
	}
}

func (s *Store) saveLocked() error {
	for _, u := range s.d.Users {
		u.stripRatings() // whatever a caller passed in, a rating is never written to disk
	}
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// view returns a user's data for reading (an empty default if they have none yet).
// Caller holds at least the read lock and must not mutate the result.
func (s *Store) view(uid string) *userData {
	if u, ok := s.d.Users[uid]; ok {
		return u
	}
	return newUserData()
}

// mut returns a user's data for writing, creating it. Caller holds the write lock.
func (s *Store) mut(uid string) *userData {
	u, ok := s.d.Users[uid]
	if !ok {
		u = newUserData()
		s.d.Users[uid] = u
	}
	return u
}

// ---- accounts ----

func NormalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// CreateUser adds an account. The first account becomes admin and adopts any
// data left over from a single-user (v1) data file.
func (s *Store) CreateUser(email, name, passwordHash string, now time.Time) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	email = NormalizeEmail(email)
	for _, u := range s.d.Accounts {
		if u.Email == email {
			return models.User{}, ErrEmailTaken
		}
	}
	u := models.User{ID: NewID(), Email: email, Name: strings.TrimSpace(name), PasswordHash: passwordHash,
		Admin: len(s.d.Accounts) == 0, CreatedAt: now}
	s.d.Accounts[u.ID] = u
	if u.Admin && s.d.Unclaimed != nil {
		s.d.Users[u.ID] = s.d.Unclaimed
		s.d.Unclaimed = nil
	} else {
		s.d.Users[u.ID] = newUserData()
	}
	return u, s.saveLocked()
}

func (s *Store) UserByEmail(email string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	email = NormalizeEmail(email)
	for _, u := range s.d.Accounts {
		if u.Email == email {
			return u, nil
		}
	}
	return models.User{}, ErrNotFound
}

func (s *Store) UserByID(id string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.d.Accounts[id]
	if !ok {
		return u, ErrNotFound
	}
	return u, nil
}

func (s *Store) UserCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.d.Accounts)
}

// JWTSecret returns the stored signing secret, creating one if needed.
func (s *Store) JWTSecret() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.d.JWTSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		s.d.JWTSecret = hex.EncodeToString(b)
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	}
	return hex.DecodeString(s.d.JWTSecret)
}

// ---- server-wide settings ----

func (s *Store) LLMConfig() llm.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.d.LLM
}

func (s *Store) SetLLMConfig(c llm.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.LLM = c
	return s.saveLocked()
}

func (s *Store) Lookup() models.LookupSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.d.Lookup
}

func (s *Store) SetLookup(l models.LookupSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.Lookup = l
	return s.saveLocked()
}

// ---- profile ----

func (s *Store) Profile(uid string) models.Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.view(uid).Profile
}

func (s *Store) SetProfile(uid string, p models.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mut(uid).Profile = p
	return s.saveLocked()
}

// ---- recipes & ratings ----

func (s *Store) PutRecipes(uid string, rs ...models.Recipe) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	for _, r := range rs {
		u.Recipes[r.ID] = r
	}
	return s.saveLocked()
}

func (s *Store) Recipe(uid, id string) (models.Recipe, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.view(uid).Recipes[id]
	if !ok || (s.hideDemo() && s.demoRecipe(s.view(uid), r)) {
		return models.Recipe{}, ErrNotFound
	}
	return r, nil
}

// Recipes returns a copy of the user's recipe map.
func (s *Store) Recipes(uid string) map[string]models.Recipe {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.view(uid).Recipes
	out := make(map[string]models.Recipe, len(src))
	for k, v := range src {
		if s.hideDemo() && s.demoRecipe(s.view(uid), v) {
			continue
		}
		out[k] = v
	}
	return out
}

func (s *Store) AddRating(uid string, r models.Rating) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	if _, ok := u.Recipes[r.RecipeID]; !ok {
		return ErrNotFound
	}
	u.Ratings = append(u.Ratings, r)
	return s.saveLocked()
}

func (s *Store) Ratings(uid string) []models.Rating {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]models.Rating(nil), s.view(uid).Ratings...)
}

// ---- plans & shopping ----

func (s *Store) PutPlan(uid string, p models.MealPlan, list models.ShoppingList) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	u.Plans[p.ID] = p
	u.Shopping[p.ID] = list
	return s.saveLocked()
}

func (s *Store) Plan(uid, id string) (models.MealPlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.view(uid).Plans[id]
	if !ok || (s.hideDemo() && demoPlan(p)) {
		return models.MealPlan{}, ErrNotFound
	}
	return p, nil
}

// Plans returns newest first.
func (s *Store) Plans(uid string) []models.MealPlan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.view(uid).Plans
	out := make([]models.MealPlan, 0, len(src))
	for _, p := range src {
		if s.hideDemo() && demoPlan(p) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *Store) DeletePlan(uid, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	if _, ok := u.Plans[id]; !ok {
		return ErrNotFound
	}
	delete(u.Plans, id)
	delete(u.Shopping, id)
	return s.saveLocked()
}

func (s *Store) Shopping(uid, planID string) (models.ShoppingList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.view(uid).Shopping[planID]
	if !ok || (s.hideDemo() && demoPlan(s.view(uid).Plans[planID])) {
		return l, ErrNotFound
	}
	return l, nil
}

// UpdateShopping applies fn to the list under lock and saves.
func (s *Store) UpdateShopping(uid, planID string, fn func(*models.ShoppingList) error) (models.ShoppingList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	l, ok := u.Shopping[planID]
	if !ok {
		return l, ErrNotFound
	}
	if err := fn(&l); err != nil {
		return l, err
	}
	u.Shopping[planID] = l
	return l, s.saveLocked()
}

// ---- food log ----

func (s *Store) AddLog(uid string, e models.LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mut(uid).Log[e.ID] = e
	return s.saveLocked()
}

func (s *Store) DeleteLog(uid, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	if _, ok := u.Log[id]; !ok {
		return ErrNotFound
	}
	delete(u.Log, id)
	return s.saveLocked()
}

// LogBetween returns entries with from <= date <= to (YYYY-MM-DD compares lexically).
func (s *Store) LogBetween(uid, from, to string) []models.LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.LogEntry
	for _, e := range s.view(uid).Log {
		if rec, ok := s.view(uid).Recipes[e.RecipeID]; ok && s.hideDemo() && s.demoRecipe(s.view(uid), rec) {
			continue
		}
		if e.Date >= from && e.Date <= to {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// ---- pantry ----

// Pantry returns items sorted by category then name.
func (s *Store) Pantry(uid string) []models.PantryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.view(uid).Pantry
	out := make([]models.PantryItem, 0, len(src))
	for _, it := range src {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// UpdatePantry applies fn to the user's pantry map under lock and saves.
func (s *Store) UpdatePantry(uid string, fn func(map[string]models.PantryItem) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(s.mut(uid).Pantry); err != nil {
		return err
	}
	return s.saveLocked()
}

// These helpers run under the store lock. Legacy demo plans used a fixed suffix.
func (s *Store) hideDemo() bool {
	return s.d.LLM.Provider != "" && s.d.LLM.Provider != "mock"
}

func demoPlan(p models.MealPlan) bool {
	return p.Demo || strings.HasSuffix(p.Title, "-day plan (mock)")
}

func (s *Store) demoRecipe(u *userData, r models.Recipe) bool {
	if r.Demo {
		return true
	}
	for _, p := range u.Plans {
		if !demoPlan(p) {
			continue
		}
		for _, d := range p.Days {
			for _, id := range d.RecipeIDs {
				if id == r.ID {
					return true
				}
			}
		}
	}
	return false
}

// UpdatePlan atomically edits a plan, its recipe map and shopping list. The
// callback works on copies, so an error cannot partially replace a meal.
func (s *Store) UpdatePlan(uid, id string, fn func(*models.MealPlan, map[string]models.Recipe, *models.ShoppingList) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	p, ok := u.Plans[id]
	if !ok || (s.hideDemo() && demoPlan(p)) {
		return ErrNotFound
	}
	p.Days = append([]models.PlanDay(nil), p.Days...)
	for i := range p.Days {
		p.Days[i].RecipeIDs = append([]string(nil), p.Days[i].RecipeIDs...)
	}
	recipes := make(map[string]models.Recipe, len(u.Recipes))
	for k, v := range u.Recipes {
		recipes[k] = v
	}
	list := u.Shopping[id]
	list.Items = append([]models.ShoppingItem(nil), list.Items...)
	if err := fn(&p, recipes, &list); err != nil {
		return err
	}
	u.Plans[id], u.Recipes, u.Shopping[id] = p, recipes, list
	return s.saveLocked()
}

func (s *Store) DayAdjustment(uid, date string) models.DayAdjustment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.view(uid).DayAdjustments[date]
}

func (s *Store) SetDayAdjustment(uid, date string, adjustment models.DayAdjustment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	target := u.Profile.Targets.Calories + adjustment.Calories
	if math.IsNaN(target) || math.IsInf(target, 0) || target <= 0 {
		return ErrInvalidTarget
	}
	adjustment.Note = strings.TrimSpace(adjustment.Note)
	if adjustment.Calories == 0 && adjustment.Note == "" {
		delete(u.DayAdjustments, date)
	} else {
		u.DayAdjustments[date] = adjustment
	}
	return s.saveLocked()
}

func (s *Store) Products(uid string) []models.Product {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Product, 0, len(s.view(uid).Products))
	for _, p := range s.view(uid).Products {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// PutProduct updates the user's existing product with this ID or exact name.
func (s *Store) PutProduct(uid string, p models.Product, now time.Time) (models.Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	p.Rating = nil // ratings are computed when shown, never stored
	p.Name = strings.TrimSpace(p.Name)
	if p.ID == "" {
		// Same barcode is the same product; otherwise fall back to the exact name.
		for _, existing := range u.Products {
			if p.Barcode != "" && existing.Barcode == p.Barcode {
				p.ID = existing.ID
				break
			}
		}
		if p.ID == "" {
			for _, existing := range u.Products {
				if strings.EqualFold(existing.Name, p.Name) {
					p.ID = existing.ID
					break
				}
			}
		}
	} else if _, ok := u.Products[p.ID]; !ok {
		return models.Product{}, ErrNotFound
	}
	if old, ok := u.Products[p.ID]; ok {
		p.CreatedAt = old.CreatedAt
	} else {
		p.ID = NewID()
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	u.Products[p.ID] = p
	return p, s.saveLocked()
}

// ProductByBarcode finds the user's saved product with this (normalised) barcode.
func (s *Store) ProductByBarcode(uid, barcode string) (models.Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.view(uid).Products {
		if barcode != "" && p.Barcode == barcode {
			return p, nil
		}
	}
	return models.Product{}, ErrNotFound
}

// SetProductIngredients fills in (or corrects) a saved product's ingredients without touching the rest.
func (s *Store) SetProductIngredients(uid, id, ingredients string, now time.Time) (models.Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	p, ok := u.Products[id]
	if !ok {
		return p, ErrNotFound
	}
	p.Ingredients, p.UpdatedAt = ingredients, now
	u.Products[id] = p
	return p, s.saveLocked()
}

func (s *Store) DeleteProduct(uid, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	if _, ok := u.Products[id]; !ok {
		return ErrNotFound
	}
	delete(u.Products, id)
	return s.saveLocked()
}

// ---- saved meals ----

func (s *Store) Meals(uid string) []models.SavedMeal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.SavedMeal, 0, len(s.view(uid).Meals))
	for _, m := range s.view(uid).Meals {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func (s *Store) Meal(uid, id string) (models.SavedMeal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.view(uid).Meals[id]
	if !ok {
		return m, ErrNotFound
	}
	return m, nil
}

// putMealLocked stores m, reusing the ID of an existing meal with the same name.
func putMealLocked(u *userData, m models.SavedMeal, now time.Time) models.SavedMeal {
	m.Name = strings.TrimSpace(m.Name)
	for _, existing := range u.Meals {
		if m.ID == "" && strings.EqualFold(existing.Name, m.Name) {
			m.ID = existing.ID
			break
		}
	}
	if old, ok := u.Meals[m.ID]; ok && m.ID != "" {
		m.CreatedAt = old.CreatedAt
	} else {
		m.ID, m.CreatedAt = NewID(), now
	}
	m.UpdatedAt = now
	u.Meals[m.ID] = m
	return m
}

// GroupMeal saves m and, if merge is set, swaps the given log entries for the
// single combined entry. Either everything happens or nothing does.
func (s *Store) GroupMeal(uid string, m models.SavedMeal, merge []string, combined *models.LogEntry, now time.Time) (models.SavedMeal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	for _, id := range merge {
		if _, ok := u.Log[id]; !ok {
			return m, ErrNotFound
		}
	}
	for _, id := range merge {
		delete(u.Log, id)
	}
	if combined != nil {
		u.Log[combined.ID] = *combined
	}
	saved := putMealLocked(u, m, now)
	return saved, s.saveLocked()
}

// UpdateMeal renames a meal or changes its usual slot.
func (s *Store) UpdateMeal(uid, id, name, mealType string, now time.Time) (models.SavedMeal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	m, ok := u.Meals[id]
	if !ok {
		return m, ErrNotFound
	}
	for _, other := range u.Meals {
		if other.ID != id && strings.EqualFold(other.Name, name) {
			return m, ErrNameTaken
		}
	}
	m.Name, m.MealType, m.UpdatedAt = name, mealType, now
	u.Meals[id] = m
	return m, s.saveLocked()
}

func (s *Store) DeleteMeal(uid, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	if _, ok := u.Meals[id]; !ok {
		return ErrNotFound
	}
	delete(u.Meals, id)
	return s.saveLocked()
}

// ---- body weight ----

// Weights returns every reading, oldest first.
func (s *Store) Weights(uid string) []models.WeightEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.WeightEntry, 0, len(s.view(uid).Weights))
	for _, w := range s.view(uid).Weights {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

// SetWeight records the reading for a day, replacing any earlier one.
func (s *Store) SetWeight(uid string, w models.WeightEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mut(uid).Weights[w.Date] = w
	return s.saveLocked()
}

func (s *Store) DeleteWeight(uid, date string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	if _, ok := u.Weights[date]; !ok {
		return ErrNotFound
	}
	delete(u.Weights, date)
	return s.saveLocked()
}

// ---- workouts ----

// Workouts returns sessions with from <= date <= to, oldest first.
func (s *Store) Workouts(uid, from, to string) []models.Workout {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []models.Workout{}
	for _, w := range s.view(uid).Workouts {
		if w.Date >= from && w.Date <= to {
			out = append(out, w)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// WorkoutKinds returns the built-in kinds followed by the user's own.
func (s *Store) WorkoutKinds(uid string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append(append([]string{}, models.DefaultWorkoutKinds...), s.view(uid).WorkoutKinds...)
}

// AddWorkout saves a session. A kind that isn't known yet is remembered for
// next time (matching ignores case, and the saved spelling wins).
func (s *Store) AddWorkout(uid string, w models.Workout) (models.Workout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	all := append(append([]string{}, models.DefaultWorkoutKinds...), u.WorkoutKinds...)
	known := false
	for _, k := range all {
		if strings.EqualFold(k, w.Kind) {
			w.Kind, known = k, true
			break
		}
	}
	if !known {
		u.WorkoutKinds = append(u.WorkoutKinds, w.Kind)
	}
	u.Workouts[w.ID] = w
	return w, s.saveLocked()
}

func (s *Store) DeleteWorkout(uid, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	if _, ok := u.Workouts[id]; !ok {
		return ErrNotFound
	}
	delete(u.Workouts, id)
	return s.saveLocked()
}

// DeleteWorkoutKind forgets a custom kind. Sessions already logged keep their label.
func (s *Store) DeleteWorkoutKind(uid, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	for i, k := range u.WorkoutKinds {
		if strings.EqualFold(k, name) {
			u.WorkoutKinds = append(u.WorkoutKinds[:i:i], u.WorkoutKinds[i+1:]...)
			return s.saveLocked()
		}
	}
	return ErrNotFound
}

// ---- shop scans ----

// Scans returns saved scans, newest first.
func (s *Store) Scans(uid string) []models.Scan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Scan, 0, len(s.view(uid).Scans))
	for _, sc := range s.view(uid).Scans {
		out = append(out, sc)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// ScanByBarcode finds a saved scan with this (normalised) barcode.
func (s *Store) ScanByBarcode(uid, barcode string) (models.Scan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sc := range s.view(uid).Scans {
		if barcode != "" && sc.Barcode == barcode {
			return sc, nil
		}
	}
	return models.Scan{}, ErrNotFound
}

// PutScan saves a scan. A scan with the same barcode replaces the earlier one.
func (s *Store) PutScan(uid string, sc models.Scan, now time.Time) (models.Scan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	if sc.Barcode != "" {
		for _, existing := range u.Scans {
			if existing.Barcode == sc.Barcode {
				sc.ID = existing.ID
				break
			}
		}
	}
	sc.Rating = nil
	if _, ok := u.Scans[sc.ID]; !ok || sc.ID == "" {
		sc.ID = NewID()
	}
	sc.CreatedAt = now
	u.Scans[sc.ID] = sc
	return sc, s.saveLocked()
}

func (s *Store) DeleteScan(uid, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.mut(uid)
	if _, ok := u.Scans[id]; !ok {
		return ErrNotFound
	}
	delete(u.Scans, id)
	return s.saveLocked()
}

// ---- rating rules ----

func (s *Store) RatingSettings(uid string) models.RatingSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r := s.view(uid).RatingRules
	pts := make(map[string]int, len(r.Points))
	for k, v := range r.Points {
		pts[k] = v
	}
	return models.RatingSettings{Points: pts, Custom: append([]models.CustomRule(nil), r.Custom...)}
}

func (s *Store) SetRatingSettings(uid string, r models.RatingSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mut(uid).RatingRules = r
	return s.saveLocked()
}
