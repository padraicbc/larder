// Package api exposes the JSON HTTP API under /api and optionally serves the built SvelteKit app.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/padraicbc/larder/internal/auth"
	"github.com/padraicbc/larder/internal/llm"
	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/nutrition"
	"github.com/padraicbc/larder/internal/store"
)

type Server struct {
	Store     *store.Store
	EnvAPIKey string // fallback if the settings UI has no key
	StaticFS  fs.FS  // built SvelteKit app; nil to disable
	// NewLLM is swappable so tests can inject a fake model.
	NewLLM func(llm.Config) (llm.Client, error)
	Now    func() time.Time

	// Barcode lookups (Open Food Facts). Tests point OFFBase at a fake server.
	OFFBase string
	HTTP    *http.Client

	Tokens        *auth.Tokens
	Limiter       *auth.Limiter
	SignupOpen    bool // when false, only the very first account can register
	SecureCookies bool // set the Secure flag (serve over HTTPS)
	TrustProxy    bool // take the client IP from X-Forwarded-For (only behind your own proxy)

	// Local is set when running on-device (Android): one account, signed in
	// through a one-time code handed to the app shell. See local.go.
	Local *LocalLogin
}

func init() {
	// Go doesn't know these; browsers want the right type for PWA install.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

func New(s *store.Store, jwtSecret []byte) *Server {
	return &Server{
		Store:      s,
		NewLLM:     llm.New,
		Now:        time.Now,
		HTTP:       &http.Client{Timeout: 8 * time.Second},
		Tokens:     auth.NewTokens(jwtSecret, 30*24*time.Hour),
		Limiter:    auth.NewLimiter(5, 15*time.Minute),
		SignupOpen: true,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// Public routes
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]bool{"ok": true}) })
	mux.HandleFunc("GET /api/auth/status", s.authStatus)
	mux.HandleFunc("POST /api/auth/register", s.register)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/logout", s.logout)

	// Everything else needs a signed-in user.
	protected := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, s.requireAuth(h)) }
	admin := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, s.requireAuth(s.requireAdmin(h))) }

	protected("GET /api/auth/me", s.me)
	protected("GET /api/backup", s.exportBackup)
	protected("POST /api/backup/preview", s.importBackup)
	protected("POST /api/backup/restore", s.importBackup)

	protected("GET /api/profile", s.getProfile)
	protected("PUT /api/profile", s.putProfile)
	protected("POST /api/profile/targets", s.calcTargets)

	protected("GET /api/settings", s.getSettings)
	admin("PUT /api/settings", s.putSettings)
	admin("POST /api/settings/test", s.testSettings)

	protected("GET /api/products", s.listProducts)
	protected("POST /api/products", s.saveProduct)
	protected("DELETE /api/products/{id}", s.deleteProduct)
	protected("PUT /api/products/{id}/ingredients", s.setProductIngredients)
	protected("GET /api/products/lookup/{barcode}", s.lookupBarcode)
	protected("POST /api/products/barcode-photo", s.readBarcodePhoto)
	protected("GET /api/log", s.getLog)
	protected("PUT /api/log/target", s.putDayTarget)
	protected("GET /api/log/history", s.getHistory)
	protected("GET /api/log/recent", s.recentLogs)
	protected("POST /api/log/{id}/repeat", s.repeatLog)
	protected("POST /api/log", s.addLog)
	protected("GET /api/rating-settings", s.getRatingSettings)
	protected("PUT /api/rating-settings", s.putRatingSettings)
	protected("GET /api/workouts", s.listWorkouts)
	protected("POST /api/workouts", s.addWorkout)
	protected("DELETE /api/workouts/{id}", s.deleteWorkout)
	protected("GET /api/workout-kinds", s.listWorkoutKinds)
	protected("DELETE /api/workout-kinds/{name}", s.deleteWorkoutKind)
	protected("GET /api/scan/barcode/{barcode}", s.scanBarcode)
	protected("POST /api/scan/ingredients", s.scanIngredients)
	protected("POST /api/scan/rate", s.rateScan)
	protected("GET /api/scan/search", s.searchScans)
	protected("GET /api/scans", s.listScans)
	protected("POST /api/scans", s.saveScan)
	protected("DELETE /api/scans/{id}", s.deleteScan)
	protected("GET /api/weight", s.listWeights)
	protected("PUT /api/weight", s.putWeight)
	protected("DELETE /api/weight/{date}", s.deleteWeight)
	protected("GET /api/meals", s.listMeals)
	protected("POST /api/meals/group", s.groupMeal)
	protected("PUT /api/meals/{id}", s.updateMeal)
	protected("DELETE /api/meals/{id}", s.deleteMeal)
	protected("POST /api/meals/{id}/log", s.logMeal)
	protected("POST /api/log/estimate", s.estimate)
	protected("POST /api/log/photo", s.estimatePhoto)
	protected("POST /api/log/label", s.readProductLabel)
	protected("DELETE /api/log/{id}", s.deleteLog)

	protected("GET /api/recipes", s.listRecipes)
	protected("GET /api/recipes/{id}", s.getRecipe)
	protected("POST /api/recipes/{id}/ratings", s.rateRecipe)
	protected("GET /api/feedback", s.getFeedback)

	protected("POST /api/plans/{id}/replace-meal", s.replaceMeal)
	protected("GET /api/plans", s.listPlans)
	protected("POST /api/plans", s.createPlan)
	protected("GET /api/plans/{id}", s.getPlan)
	protected("DELETE /api/plans/{id}", s.deletePlan)
	protected("GET /api/plans/{id}/shopping", s.getShopping)
	protected("POST /api/plans/{id}/shopping/rebuild", s.rebuildShopping)
	protected("POST /api/plans/{id}/shopping/items", s.addShoppingItem)
	protected("PATCH /api/plans/{id}/shopping/items/{key}", s.toggleShoppingItem)

	protected("GET /api/pantry", s.getPantry)
	protected("POST /api/pantry", s.addPantry)
	protected("PATCH /api/pantry/{id}", s.updatePantryItem)
	protected("DELETE /api/pantry/{id}", s.deletePantryItem)
	protected("DELETE /api/pantry", s.clearPantry)
	protected("POST /api/pantry/scan", s.scanPantry)
	protected("POST /api/pantry/cook", s.cookFromPantry)

	// Unknown /api paths get JSON 404s rather than the SPA shell.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeErr(w, 404, "no such endpoint") })
	if s.Local != nil {
		mux.HandleFunc("GET /api/auth/local", s.localLogin)
	}
	if s.StaticFS != nil {
		mux.Handle("/", spa(s.StaticFS))
	}
	return logRequests(securityHeaders(mux))
}

// ---------- helpers ----------

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, apiError{msg})
}

func storeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	slog.Error("store", "err", err)
	writeErr(w, http.StatusInternalServerError, "could not save; check the server log")
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func (s *Server) client() (llm.Client, llm.Config, error) {
	cfg := withEnvKey(s.Store.LLMConfig(), s.EnvAPIKey)
	c, err := s.NewLLM(cfg)
	return c, cfg, err
}

// visionClient uses the vision model if one is set.
func (s *Server) visionClient() (llm.VisionClient, error) {
	cfg := withEnvKey(s.Store.LLMConfig(), s.EnvAPIKey).ForVision()
	c, err := s.NewLLM(cfg)
	if err != nil {
		return nil, err
	}
	vc, ok := c.(llm.VisionClient)
	if !ok {
		return nil, errors.New("the configured provider can't read photos")
	}
	return vc, nil
}

func (s *Server) today() string { return s.Now().Format(time.DateOnly) }

func validDate(d string) bool {
	_, err := time.Parse(time.DateOnly, d)
	return err == nil
}

// ---------- profile ----------

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	writeJSON(w, 200, s.Store.Profile(uid))
}

func cleanList(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		k := strings.ToLower(v)
		if v != "" && !seen[k] {
			seen[k] = true
			out = append(out, v)
		}
	}
	return out
}

func (s *Server) putProfile(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	var p models.Profile
	if !decode(w, r, &p) {
		return
	}
	p.Diets, p.Excluded, p.Allergies, p.Cuisines = cleanList(p.Diets), cleanList(p.Excluded), cleanList(p.Allergies), cleanList(p.Cuisines)
	if p.MealsPerDay < 1 || p.MealsPerDay > 6 {
		writeErr(w, 400, "meals per day must be 1–6")
		return
	}
	if p.HouseholdSize < 1 {
		p.HouseholdSize = 1
	}
	if err := s.Store.SetProfile(uid, p); err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) calcTargets(w http.ResponseWriter, r *http.Request) {
	var p models.Profile
	if !decode(w, r, &p) {
		return
	}
	t, ok := nutrition.CalculateTargets(p)
	if !ok {
		writeErr(w, 400, "add age, height and weight to calculate targets")
		return
	}
	writeJSON(w, 200, t)
}

// ---------- settings ----------

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"llm":          s.Store.LLMConfig().Masked(),
		"lookup":       s.Store.Lookup(),
		"envKeyLoaded": s.EnvAPIKey != "",
		"canEdit":      currentUser(r).Admin,
	})
}

// incomingConfig keeps the stored key when the browser sends back the masked value.
func (s *Server) incomingConfig(c llm.Config) llm.Config {
	if llm.IsMaskedKey(c.APIKey) {
		c.APIKey = s.Store.LLMConfig().APIKey
	}
	return c
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LLM    llm.Config             `json:"llm"`
		Lookup *models.LookupSettings `json:"lookup"`
	}
	if !decode(w, r, &body) {
		return
	}
	c := s.incomingConfig(body.LLM)
	if _, err := s.NewLLM(withEnvKey(c, s.EnvAPIKey)); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := s.Store.SetLLMConfig(c); err != nil {
		storeErr(w, err)
		return
	}
	if body.Lookup != nil {
		if err := s.Store.SetLookup(*body.Lookup); err != nil {
			storeErr(w, err)
			return
		}
	}
	writeJSON(w, 200, map[string]any{"llm": c.Masked(), "lookup": s.Store.Lookup(), "envKeyLoaded": s.EnvAPIKey != "", "canEdit": true})
}

func withEnvKey(c llm.Config, env string) llm.Config {
	if c.APIKey == "" {
		c.APIKey = env
	}
	return c
}

// testSettings checks a config (unsaved) by asking for a tiny JSON reply.
func (s *Server) testSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LLM llm.Config `json:"llm"`
	}
	if !decode(w, r, &body) {
		return
	}
	c := withEnvKey(s.incomingConfig(body.LLM), s.EnvAPIKey)
	client, err := s.NewLLM(c)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	start := time.Now()
	var out map[string]any
	err = llm.CompleteJSON(r.Context(), client, "Reply with only JSON.", `TASK: ping. Reply exactly {"ok": true}`, &out)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "latencyMs": time.Since(start).Milliseconds()})
}

// ---------- food log ----------

type dayLog struct {
	BaseTargets models.Macros        `json:"baseTargets"`
	Adjustment  models.DayAdjustment `json:"adjustment"`
	Date        string               `json:"date"`
	Entries     []models.LogEntry    `json:"entries"`
	Totals      models.Macros        `json:"totals"`
	Targets     models.Macros        `json:"targets"`
}

func (s *Server) getLog(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	date := r.URL.Query().Get("date")
	if date == "" {
		date = s.today()
	}
	if !validDate(date) {
		writeErr(w, 400, "date must be YYYY-MM-DD")
		return
	}
	entries := s.Store.LogBetween(uid, date, date)
	if entries == nil {
		entries = []models.LogEntry{}
	}
	base := s.Store.Profile(uid).Targets
	adjustment := s.Store.DayAdjustment(uid, date)
	writeJSON(w, 200, dayLog{Date: date, Entries: entries, Totals: nutrition.SumEntries(entries), Targets: adjustedTargets(base, adjustment), BaseTargets: base, Adjustment: adjustment})
}

func (s *Server) getHistory(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days < 1 || days > 90 {
		days = 7
	}
	endStr := r.URL.Query().Get("end")
	if endStr == "" {
		endStr = s.today()
	}
	end, err := time.Parse(time.DateOnly, endStr)
	if err != nil {
		writeErr(w, 400, "end must be YYYY-MM-DD")
		return
	}
	start := end.AddDate(0, 0, -(days - 1))
	entries := s.Store.LogBetween(uid, start.Format(time.DateOnly), endStr)
	byDay := map[string][]models.LogEntry{}
	for _, e := range entries {
		byDay[e.Date] = append(byDay[e.Date], e)
	}
	type point struct {
		Targets models.Macros `json:"targets"`
		Date    string        `json:"date"`
		Totals  models.Macros `json:"totals"`
	}
	out := make([]point, 0, days)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		k := d.Format(time.DateOnly)
		out = append(out, point{Date: k, Totals: nutrition.SumEntries(byDay[k]), Targets: adjustedTargets(s.Store.Profile(uid).Targets, s.Store.DayAdjustment(uid, k))})
	}
	writeJSON(w, 200, map[string]any{"days": out, "targets": s.Store.Profile(uid).Targets})
}

type addLogReq struct {
	Items    []models.MealItem `json:"items"`
	Date     string            `json:"date"`
	MealType string            `json:"mealType"`
	Name     string            `json:"name"`
	Servings float64           `json:"servings"`
	Macros   models.Macros     `json:"macros"` // per serving
	RecipeID string            `json:"recipeId"`
}

func (s *Server) addLog(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	var req addLogReq
	if !decode(w, r, &req) {
		return
	}
	if req.Date == "" {
		req.Date = s.today()
	}
	if !validDate(req.Date) {
		writeErr(w, 400, "date must be YYYY-MM-DD")
		return
	}
	if req.Servings <= 0 {
		req.Servings = 1
	}
	if req.RecipeID != "" {
		rec, err := s.Store.Recipe(uid, req.RecipeID)
		if err != nil {
			storeErr(w, err)
			return
		}
		req.Macros = rec.Macros
		if req.Name == "" {
			req.Name = rec.Title
		}
		if req.MealType == "" {
			req.MealType = rec.MealType
		}
	}
	if strings.TrimSpace(req.Name) == "" {
		writeErr(w, 400, "give the entry a name")
		return
	}
	e := models.LogEntry{
		ID:        store.NewID(),
		Date:      req.Date,
		MealType:  orDefault(req.MealType, "snack"),
		Name:      strings.TrimSpace(req.Name),
		Servings:  req.Servings,
		Macros:    roundMacros(req.Macros.Scale(req.Servings)),
		RecipeID:  req.RecipeID,
		Items:     req.Items,
		CreatedAt: s.Now(),
	}
	if err := s.Store.AddLog(uid, e); err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 201, e)
}

func roundMacros(m models.Macros) models.Macros {
	r := func(v float64) float64 { return math.Round(v*10) / 10 }
	return models.Macros{Calories: math.Round(m.Calories), Protein: r(m.Protein), Carbs: r(m.Carbs), Fat: r(m.Fat), Fiber: r(m.Fiber)}
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func (s *Server) deleteLog(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	if err := s.Store.DeleteLog(uid, r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) estimate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if !decode(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeErr(w, 400, "describe what you ate")
		return
	}
	c, _, err := s.client()
	if err != nil {
		writeErr(w, 400, "model not configured: "+err.Error())
		return
	}
	est, err := nutrition.EstimateMacros(r.Context(), c, body.Text)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	est.Macros = roundMacros(est.Macros)
	writeJSON(w, 200, est)
}

// ---------- recipes & ratings ----------

type recipeSummary struct {
	models.Recipe
	AvgRating   float64 `json:"avgRating"`
	RatingCount int     `json:"ratingCount"`
}

func (s *Server) ratingStats(uid string) map[string][2]float64 {
	stats := map[string][2]float64{} // id -> [sum, count]
	for _, rt := range s.Store.Ratings(uid) {
		st := stats[rt.RecipeID]
		st[0] += float64(rt.Stars)
		st[1]++
		stats[rt.RecipeID] = st
	}
	return stats
}

func summarize(r models.Recipe, stats map[string][2]float64) recipeSummary {
	st := stats[r.ID]
	sum := recipeSummary{Recipe: r, RatingCount: int(st[1])}
	if st[1] > 0 {
		sum.AvgRating = math.Round(st[0]/st[1]*10) / 10
	}
	return sum
}

func (s *Server) listRecipes(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	stats := s.ratingStats(uid)
	out := []recipeSummary{}
	for _, rec := range s.Store.Recipes(uid) {
		out = append(out, summarize(rec, stats))
	}
	// newest first
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].Title < out[j].Title
	})
	writeJSON(w, 200, out)
}

func (s *Server) getRecipe(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	rec, err := s.Store.Recipe(uid, r.PathValue("id"))
	if err != nil {
		storeErr(w, err)
		return
	}
	ratings := []models.Rating{}
	for _, rt := range s.Store.Ratings(uid) {
		if rt.RecipeID == rec.ID {
			ratings = append(ratings, rt)
		}
	}
	writeJSON(w, 200, map[string]any{"recipe": summarize(rec, s.ratingStats(uid)), "ratings": ratings})
}

func (s *Server) rateRecipe(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	var body struct {
		Stars   int    `json:"stars"`
		Comment string `json:"comment"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Stars < 1 || body.Stars > 5 {
		writeErr(w, 400, "stars must be 1–5")
		return
	}
	rt := models.Rating{ID: store.NewID(), RecipeID: r.PathValue("id"), Stars: body.Stars,
		Comment: strings.TrimSpace(body.Comment), CreatedAt: s.Now()}
	if err := s.Store.AddRating(uid, rt); err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 201, rt)
}

func (s *Server) getFeedback(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	writeJSON(w, 200, nutrition.Summarize(s.Store.Recipes(uid), s.Store.Ratings(uid)))
}

// ---------- plans ----------

type planSummary struct {
	models.MealPlan
	MealCount int `json:"mealCount"`
}

func (s *Server) listPlans(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	out := []planSummary{}
	for _, p := range s.Store.Plans(uid) {
		n := 0
		for _, d := range p.Days {
			n += len(d.RecipeIDs)
		}
		out = append(out, planSummary{p, n})
	}
	writeJSON(w, 200, out)
}

func (s *Server) recentTitles(uid string, limit int) []string {
	recipes := s.Store.Recipes(uid)
	seen := map[string]bool{}
	var out []string
	for _, p := range s.Store.Plans(uid) {
		for _, d := range p.Days {
			for _, id := range d.RecipeIDs {
				if t := recipes[id].Title; t != "" && !seen[t] {
					seen[t] = true
					out = append(out, t)
					if len(out) >= limit {
						return out
					}
				}
			}
		}
	}
	return out
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	var req nutrition.PlanRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Days == 0 {
		req.Days = 3
	}
	if req.Days < 1 || req.Days > 7 {
		writeErr(w, 400, "days must be between 1 and 7")
		return
	}
	c, cfg, err := s.client()
	if err != nil {
		writeErr(w, 400, "model not configured: "+err.Error())
		return
	}
	profile := s.Store.Profile(uid)
	for slot, id := range req.Fixed {
		if !validMealType(slot) {
			writeErr(w, 400, "choose breakfast, lunch, dinner or snack for fixed meals")
			return
		}
		m, err := s.Store.Meal(uid, id)
		if err != nil {
			writeErr(w, 400, "a meal you picked no longer exists; refresh and choose again")
			return
		}
		req.FixedMeals = append(req.FixedMeals, nutrition.FixedFromSaved(m, slot))
	}
	fb := nutrition.Summarize(s.Store.Recipes(uid), s.Store.Ratings(uid))
	gen, err := nutrition.GeneratePlan(r.Context(), c, profile, fb, s.recentTitles(uid, 20), req)
	if err != nil {
		writeErr(w, 502, "plan generation failed: "+err.Error())
		return
	}

	now := s.Now()
	demo := cfg.Provider == "" || cfg.Provider == "mock"
	plan := models.MealPlan{Demo: demo, ID: store.NewID(), Title: gen.Title, Notes: req.Notes, CreatedAt: now}
	var recipes []models.Recipe
	byID := map[string]models.Recipe{}
	byTitle := map[string]string{} // reuse one recipe when the model repeats a dish (leftovers)
	for _, d := range gen.Days {
		day := models.PlanDay{Day: d.Day}
		for _, m := range d.Meals {
			key := strings.ToLower(m.Title)
			if id, ok := byTitle[key]; ok {
				day.RecipeIDs = append(day.RecipeIDs, id)
				continue
			}
			m.Demo = demo
			m.ID = store.NewID()
			m.CreatedAt = now
			m.Source = "plan"
			byTitle[key] = m.ID
			byID[m.ID] = m
			recipes = append(recipes, m)
			day.RecipeIDs = append(day.RecipeIDs, m.ID)
		}
		plan.Days = append(plan.Days, day)
	}
	list := models.ShoppingList{PlanID: plan.ID, Items: nutrition.BuildShoppingList(plan, byID, profile.HouseholdSize)}
	if err := s.Store.PutRecipes(uid, recipes...); err != nil {
		storeErr(w, err)
		return
	}
	if err := s.Store.PutPlan(uid, plan, list); err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 201, s.planDetail(uid, plan))
}

func (s *Server) planDetail(uid string, p models.MealPlan) map[string]any {
	all := s.Store.Recipes(uid)
	stats := s.ratingStats(uid)
	recipes := map[string]recipeSummary{}
	for _, d := range p.Days {
		for _, id := range d.RecipeIDs {
			if r, ok := all[id]; ok {
				recipes[id] = summarize(r, stats)
			}
		}
	}
	return map[string]any{"plan": p, "recipes": recipes}
}

func (s *Server) getPlan(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	p, err := s.Store.Plan(uid, r.PathValue("id"))
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, s.planDetail(uid, p))
}

func (s *Server) deletePlan(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	if err := s.Store.DeletePlan(uid, r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- shopping ----------

func (s *Server) getShopping(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	l, err := s.Store.Shopping(uid, r.PathValue("id"))
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, l)
}

// rebuildShopping recomputes quantities (e.g. after household size changes), keeping ticks.
func (s *Server) rebuildShopping(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	id := r.PathValue("id")
	p, err := s.Store.Plan(uid, id)
	if err != nil {
		storeErr(w, err)
		return
	}
	fresh := nutrition.BuildShoppingList(p, s.Store.Recipes(uid), s.Store.Profile(uid).HouseholdSize)
	l, err := s.Store.UpdateShopping(uid, id, func(l *models.ShoppingList) error {
		l.Items = nutrition.MergeChecked(l.Items, fresh)
		return nil
	})
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, l)
}

func (s *Server) addShoppingItem(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	var it models.ShoppingItem
	if !decode(w, r, &it) {
		return
	}
	it.Name = strings.TrimSpace(it.Name)
	if it.Name == "" {
		writeErr(w, 400, "item needs a name")
		return
	}
	it.Key = "manual|" + store.NewID()
	it.Manual = true
	it.Category = orDefault(it.Category, "other")
	l, err := s.Store.UpdateShopping(uid, r.PathValue("id"), func(l *models.ShoppingList) error {
		l.Items = append(l.Items, it)
		nutrition.SortShopping(l.Items)
		return nil
	})
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 201, l)
}

func (s *Server) toggleShoppingItem(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	var body struct {
		Checked bool `json:"checked"`
	}
	if !decode(w, r, &body) {
		return
	}
	key := r.PathValue("key")
	l, err := s.Store.UpdateShopping(uid, r.PathValue("id"), func(l *models.ShoppingList) error {
		for i := range l.Items {
			if l.Items[i].Key == key {
				l.Items[i].Checked = body.Checked
				return nil
			}
		}
		return store.ErrNotFound
	})
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 200, l)
}

// ---------- static & middleware ----------

// spaOnly is the static part of Handler, for tests.
func (s *Server) spaOnly() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeErr(w, 404, "no such endpoint") })
	mux.Handle("/", spa(s.StaticFS))
	return mux
}

// spa serves the built app and falls back to the SPA shell (200.html) for
// client-side routes. Hashed assets are cached forever; everything else is revalidated.
func spa(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	shell, _ := fs.ReadFile(files, "200.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if st, err := fs.Stat(files, p); err == nil && !st.IsDir() {
			if strings.HasPrefix(p, "_app/immutable/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(shell)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			slog.Info("req", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start).Round(time.Millisecond))
		}
	})
}
