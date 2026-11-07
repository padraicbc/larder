package api

import (
	"net/http"
	"strings"

	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/nutrition"
)

func (s *Server) ratingSettingsView(uid string) map[string]any {
	return map[string]any{
		"settings":  s.Store.RatingSettings(uid),
		"catalogue": nutrition.RuleCatalogue(),
		"rules":     nutrition.RatingRules,
	}
}

func (s *Server) getRatingSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.ratingSettingsView(userID(r)))
}

// putRatingSettings replaces this person's tweaks. Anything left out uses the default,
// and an empty body resets everything. Ratings follow immediately because they are
// never stored.
func (s *Server) putRatingSettings(w http.ResponseWriter, r *http.Request) {
	var in models.RatingSettings
	if !decode(w, r, &in) {
		return
	}
	clean := models.RatingSettings{}
	defaults := map[string]int{}
	for _, rule := range nutrition.RuleCatalogue() {
		defaults[rule.Key] = rule.Default
	}
	for k, v := range in.Points {
		if v == defaults[k] && nutrition.RuleKnown(k) {
			continue // same as the default: nothing to remember
		}
		if clean.Points == nil {
			clean.Points = map[string]int{}
		}
		clean.Points[k] = v
	}
	seen := map[string]bool{}
	for _, c := range in.Custom {
		c.Match = strings.Join(strings.Fields(c.Match), " ")
		if k := strings.ToLower(c.Match); !seen[k] {
			seen[k] = true
			clean.Custom = append(clean.Custom, c)
		}
	}
	if err := clean.Validate(nutrition.RuleKnown); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := s.Store.SetRatingSettings(userID(r), clean); err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, s.ratingSettingsView(userID(r)))
}
