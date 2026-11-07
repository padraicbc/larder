package nutrition

import (
	"fmt"
	"sort"
	"strings"

	"github.com/padraicbc/larder/internal/models"
)

type RatedRecipe struct {
	Title    string   `json:"title"`
	Avg      float64  `json:"avg"`
	Comments []string `json:"comments"`
}

// Feedback is what we learned from ratings. It's shown on the recipes page
// (so users can see what the app thinks they like) and fed into prompts.
type Feedback struct {
	Loved               []RatedRecipe `json:"loved"`
	Disliked            []RatedRecipe `json:"disliked"`
	LikedIngredients    []string      `json:"likedIngredients"`
	DislikedIngredients []string      `json:"dislikedIngredients"`
	LikedTags           []string      `json:"likedTags"`
	RatingCount         int           `json:"ratingCount"`
}

// Staples carry no signal about taste.
var staples = map[string]bool{"salt": true, "black pepper": true, "pepper": true, "water": true, "olive oil": true,
	"oil": true, "vegetable oil": true, "garlic": true, "onion": true}

const maxListed = 8

func Summarize(recipes map[string]models.Recipe, ratings []models.Rating) Feedback {
	type agg struct {
		sum      int
		n        int
		comments []string
	}
	by := map[string]*agg{}
	for _, r := range ratings {
		a := by[r.RecipeID]
		if a == nil {
			a = &agg{}
			by[r.RecipeID] = a
		}
		a.sum += r.Stars
		a.n++
		if c := strings.TrimSpace(r.Comment); c != "" {
			a.comments = append(a.comments, c)
		}
	}

	fb := Feedback{RatingCount: len(ratings)}
	ingScore := map[string]float64{}
	tagScore := map[string]float64{}
	for id, a := range by {
		rec, ok := recipes[id]
		if !ok {
			continue
		}
		avg := float64(a.sum) / float64(a.n)
		rr := RatedRecipe{Title: rec.Title, Avg: avg, Comments: a.comments}
		switch {
		case avg >= 4:
			fb.Loved = append(fb.Loved, rr)
		case avg <= 2:
			fb.Disliked = append(fb.Disliked, rr)
		}
		// Centre on 3 so a 5 pushes ingredients up and a 1 pushes them down.
		w := avg - 3
		for _, ing := range rec.Ingredients {
			n := Singular(ing.Name)
			if !staples[n] {
				ingScore[n] += w
			}
		}
		for _, t := range rec.Tags {
			tagScore[strings.ToLower(t)] += w
		}
	}

	byAvg := func(s []RatedRecipe, desc bool) {
		sort.Slice(s, func(i, j int) bool {
			if s[i].Avg != s[j].Avg {
				return (s[i].Avg > s[j].Avg) == desc
			}
			return s[i].Title < s[j].Title
		})
	}
	byAvg(fb.Loved, true)     // best first
	byAvg(fb.Disliked, false) // worst first
	fb.Loved = capList(fb.Loved)
	fb.Disliked = capList(fb.Disliked)

	fb.LikedIngredients = topKeys(ingScore, 2, true)
	fb.DislikedIngredients = topKeys(ingScore, -2, false)
	fb.LikedTags = topKeys(tagScore, 2, true)
	return fb
}

func capList[T any](s []T) []T {
	if len(s) > maxListed {
		return s[:maxListed]
	}
	return s
}

// topKeys returns keys whose score passes threshold, strongest first.
func topKeys(m map[string]float64, threshold float64, positive bool) []string {
	var keys []string
	for k, v := range m {
		if (positive && v >= threshold) || (!positive && v <= threshold) {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := m[keys[i]], m[keys[j]]
		if a != b {
			if positive {
				return a > b
			}
			return a < b
		}
		return keys[i] < keys[j]
	})
	return capList(keys)
}

// PromptText renders feedback as plain text for the model. Empty if nothing learned yet.
func (f Feedback) PromptText() string {
	if f.RatingCount == 0 {
		return ""
	}
	var b strings.Builder
	line := func(label string, rs []RatedRecipe) {
		if len(rs) == 0 {
			return
		}
		b.WriteString(label + ":\n")
		for _, r := range rs {
			fmt.Fprintf(&b, "- %s (%.1f/5)", r.Title, r.Avg)
			if len(r.Comments) > 0 {
				fmt.Fprintf(&b, ` — "%s"`, strings.Join(r.Comments, `"; "`))
			}
			b.WriteString("\n")
		}
	}
	line("Recipes they loved (make more like these)", f.Loved)
	line("Recipes they disliked (avoid what made these bad; read the comments)", f.Disliked)
	if len(f.LikedIngredients) > 0 {
		b.WriteString("Ingredients they tend to enjoy: " + strings.Join(f.LikedIngredients, ", ") + "\n")
	}
	if len(f.DislikedIngredients) > 0 {
		b.WriteString("Ingredients that show up in low-rated meals (use sparingly): " + strings.Join(f.DislikedIngredients, ", ") + "\n")
	}
	if len(f.LikedTags) > 0 {
		b.WriteString("Styles they like: " + strings.Join(f.LikedTags, ", ") + "\n")
	}
	return b.String()
}
