package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/store"
)

func validMealType(t string) bool {
	return t == "breakfast" || t == "lunch" || t == "dinner" || t == "snack"
}

func (s *Server) listMeals(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.Store.Meals(userID(r)))
}

// groupMeal turns several log entries (e.g. Weetabix, oats, protein powder
// logged as separate products) into one named, reusable meal. With merge set,
// the separate entries are replaced by a single combined entry.
func (s *Server) groupMeal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string   `json:"name"`
		MealType string   `json:"mealType"`
		EntryIDs []string `json:"entryIds"`
		Merge    bool     `json:"merge"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeErr(w, 400, "give the meal a name")
		return
	}
	if len(body.EntryIDs) == 0 {
		writeErr(w, 400, "choose the foods to group")
		return
	}
	uid := userID(r)
	want := map[string]bool{}
	for _, id := range body.EntryIDs {
		want[id] = true
	}
	var entries []models.LogEntry
	for _, e := range s.Store.LogBetween(uid, "", "9999-12-31") {
		if want[e.ID] {
			entries = append(entries, e)
		}
	}
	if len(entries) != len(want) {
		writeErr(w, 404, "one of those foods is no longer in your log")
		return
	}
	if body.MealType == "" {
		body.MealType = entries[0].MealType
	}
	if !validMealType(body.MealType) {
		writeErr(w, 400, "choose breakfast, lunch, dinner or snack")
		return
	}
	date := entries[0].Date
	var total models.Macros
	var items []models.MealItem
	for _, e := range entries {
		if e.Date != date {
			writeErr(w, 400, "group foods from the same day")
			return
		}
		total = total.Add(e.Macros)
		if len(e.Items) > 0 {
			items = append(items, e.Items...)
			continue
		}
		q := "1 serving"
		if e.Servings != 1 {
			q = trimFloat(e.Servings) + " servings"
		}
		items = append(items, models.MealItem{Name: e.Name, Quantity: q})
	}
	total = roundMacros(total)
	now := s.Now()
	meal := models.SavedMeal{Name: body.Name, MealType: body.MealType, Items: items, Macros: total}
	var combined *models.LogEntry
	var merge []string
	if body.Merge {
		merge = body.EntryIDs
		combined = &models.LogEntry{ID: store.NewID(), Date: date, MealType: body.MealType, Name: body.Name, Servings: 1, Macros: total, Items: items, CreatedAt: now}
	}
	saved, err := s.Store.GroupMeal(uid, meal, merge, combined, now)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"meal": saved, "entry": combined})
}

func (s *Server) updateMeal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		MealType string `json:"mealType"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" || !validMealType(body.MealType) {
		writeErr(w, 400, "give the meal a name and choose breakfast, lunch, dinner or snack")
		return
	}
	m, err := s.Store.UpdateMeal(userID(r), r.PathValue("id"), body.Name, body.MealType, s.Now())
	if errors.Is(err, store.ErrNameTaken) {
		writeErr(w, 409, err.Error())
		return
	}
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, m)
}

func (s *Server) deleteMeal(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteMeal(userID(r), r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// logMeal logs a saved meal (scaled by servings) to the chosen day and slot.
func (s *Server) logMeal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date     string  `json:"date"`
		MealType string  `json:"mealType"`
		Servings float64 `json:"servings"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Date == "" {
		body.Date = s.today()
	}
	if !validDate(body.Date) {
		writeErr(w, 400, "date must be YYYY-MM-DD")
		return
	}
	if body.Servings <= 0 {
		body.Servings = 1
	}
	if body.MealType != "" && !validMealType(body.MealType) {
		writeErr(w, 400, "choose breakfast, lunch, dinner or snack")
		return
	}
	uid := userID(r)
	m, err := s.Store.Meal(uid, r.PathValue("id"))
	if err != nil {
		storeErr(w, err)
		return
	}
	e := models.LogEntry{
		ID: store.NewID(), Date: body.Date, MealType: orDefault(body.MealType, m.MealType), Name: m.Name,
		Servings: body.Servings, Macros: roundMacros(m.Macros.Scale(body.Servings)), Items: m.Items, CreatedAt: s.Now(),
	}
	if err := s.Store.AddLog(uid, e); err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 201, e)
}

func trimFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(strings.TrimSpace(formatFloat(f)), "0"), ".")
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) }
