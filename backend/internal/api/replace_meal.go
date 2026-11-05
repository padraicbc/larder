package api

import (
	"errors"
	"net/http"
	"reflect"

	"github.com/you/nutrition-app/internal/models"
	"github.com/you/nutrition-app/internal/nutrition"
	"github.com/you/nutrition-app/internal/store"
)

func (s *Server) replaceMeal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Day      int    `json:"day"`
		Index    int    `json:"index"`
		RecipeID string `json:"recipeId"`
		Notes    string `json:"notes"`
	}
	if !decode(w, r, &body) {
		return
	}
	uid := userID(r)
	plan, err := s.Store.Plan(uid, r.PathValue("id"))
	if err != nil {
		storeErr(w, err)
		return
	}
	dayIndex := -1
	for i, d := range plan.Days {
		if d.Day == body.Day {
			dayIndex = i
			break
		}
	}
	if dayIndex < 0 || body.Index < 0 || body.Index >= len(plan.Days[dayIndex].RecipeIDs) {
		writeErr(w, 400, "choose a meal in this plan")
		return
	}
	ids := plan.Days[dayIndex].RecipeIDs
	if ids[body.Index] != body.RecipeID {
		writeErr(w, 409, "this meal changed; refresh the plan and try again")
		return
	}
	all := s.Store.Recipes(uid)
	old, ok := all[body.RecipeID]
	if !ok {
		storeErr(w, store.ErrNotFound)
		return
	}
	kept := []models.Recipe{}
	for i, id := range ids {
		if i != body.Index {
			rec, ok := all[id]
			if !ok {
				storeErr(w, store.ErrNotFound)
				return
			}
			kept = append(kept, rec)
		}
	}
	c, cfg, err := s.client()
	if err != nil {
		writeErr(w, 400, "model not configured: "+err.Error())
		return
	}
	profile := s.Store.Profile(uid)
	recipe, err := nutrition.GenerateReplacement(r.Context(), c, profile, nutrition.Summarize(all, s.Store.Ratings(uid)), old, kept, plan.Notes+"\n"+body.Notes)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	recipe.ID, recipe.CreatedAt, recipe.Source = store.NewID(), s.Now(), "plan"
	recipe.Demo = cfg.Provider == "" || cfg.Provider == "mock"
	if !reflect.DeepEqual(profile, s.Store.Profile(uid)) {
		writeErr(w, 409, "your profile changed while generating; try again with the new targets")
		return
	}
	conflict := errors.New("plan changed while generating; refresh and try again")
	err = s.Store.UpdatePlan(uid, plan.ID, func(current *models.MealPlan, recipes map[string]models.Recipe, list *models.ShoppingList) error {
		if !reflect.DeepEqual(current.Days, plan.Days) {
			return conflict
		}
		current.Days[dayIndex].RecipeIDs[body.Index] = recipe.ID
		recipes[recipe.ID] = recipe
		fresh := nutrition.BuildShoppingList(*current, recipes, profile.HouseholdSize)
		list.PlanID = current.ID
		list.Items = nutrition.MergeChecked(list.Items, fresh)
		return nil
	})
	if errors.Is(err, conflict) {
		writeErr(w, 409, err.Error())
		return
	}
	if err != nil {
		storeErr(w, err)
		return
	}
	updated, err := s.Store.Plan(uid, plan.ID)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, s.planDetail(uid, updated))
}
