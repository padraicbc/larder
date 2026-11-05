package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/you/nutrition-app/internal/auth"
	"github.com/you/nutrition-app/internal/llm"
	"github.com/you/nutrition-app/internal/models"
	"github.com/you/nutrition-app/internal/store"
)

// client is a tiny test helper: c.do("POST", "/api/x", body, &out) returns the status.
// Each client has its own cookie jar, so it behaves like one browser.
type client struct {
	t     *testing.T
	srv   *httptest.Server
	http  *http.Client
	token string // if set, sent as a Bearer token instead of relying on cookies
}

func init() {
	auth.Iterations = 1000 // fast password hashing in tests
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newTestServer(t *testing.T) (*client, *Server) {
	t.Helper()
	st, _ := store.Open("")
	s := New(st, []byte("test-secret-test-secret-32-bytes"))
	s.Now = func() time.Time { return time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC) }
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return newClient(t, srv), s
}

func newClient(t *testing.T, srv *httptest.Server) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, srv: srv, http: &http.Client{Jar: jar}}
}

// signUp registers a fresh account and returns a client signed in as it.
func signUp(t *testing.T, srv *httptest.Server, email string) *client {
	t.Helper()
	c := newClient(t, srv)
	if code := c.do("POST", "/api/auth/register", map[string]string{"email": email, "password": "password123"}, nil); code != 201 {
		t.Fatalf("register %s: %d", email, code)
	}
	return c
}

func (c *client) do(method, path string, body, out any) int {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.srv.URL+path, r)
	return c.send(req, out)
}

func (c *client) send(req *http.Request, out any) int {
	c.t.Helper()
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		data, _ := io.ReadAll(res.Body)
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s %s: decode %q: %v", req.Method, req.URL.Path, data, err)
		}
	}
	return res.StatusCode
}

type planResp struct {
	Plan    models.MealPlan          `json:"plan"`
	Recipes map[string]recipeSummary `json:"recipes"`
}

// One test walks the main user journey end to end with the mock model.
func TestUserJourney(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "journey@example.com")

	// 1. profile
	var prof models.Profile
	c.do("GET", "/api/profile", nil, &prof)
	prof.Diets = []string{"vegetarian", " vegetarian ", ""} // gets cleaned
	prof.HouseholdSize = 2
	if code := c.do("PUT", "/api/profile", prof, &prof); code != 200 {
		t.Fatalf("put profile: %d", code)
	}
	if len(prof.Diets) != 1 {
		t.Errorf("diets not cleaned: %v", prof.Diets)
	}

	// 2. generate a plan
	var pr planResp
	if code := c.do("POST", "/api/plans", map[string]any{"days": 2}, &pr); code != 201 {
		t.Fatalf("create plan: %d", code)
	}
	if len(pr.Plan.Days) != 2 || len(pr.Recipes) == 0 {
		t.Fatalf("plan looks wrong: %+v", pr.Plan)
	}
	recipeID := pr.Plan.Days[0].RecipeIDs[0]

	// 3. shopping list: toggle an item (key contains "|", so escape it)
	var list models.ShoppingList
	c.do("GET", "/api/plans/"+pr.Plan.ID+"/shopping", nil, &list)
	if len(list.Items) == 0 {
		t.Fatal("empty shopping list")
	}
	key := url.PathEscape(list.Items[0].Key)
	c.do("PATCH", "/api/plans/"+pr.Plan.ID+"/shopping/items/"+key, map[string]bool{"checked": true}, &list)
	if !list.Items[0].Checked {
		t.Error("item not checked")
	}
	c.do("POST", "/api/plans/"+pr.Plan.ID+"/shopping/items", map[string]string{"name": "coffee"}, &list)
	c.do("POST", "/api/plans/"+pr.Plan.ID+"/shopping/rebuild", nil, &list)
	var checked, manual int
	for _, it := range list.Items {
		if it.Checked {
			checked++
		}
		if it.Manual {
			manual++
		}
	}
	if checked != 1 || manual != 1 {
		t.Errorf("rebuild lost state: checked=%d manual=%d", checked, manual)
	}

	// 4. rate a recipe and see it in feedback
	if code := c.do("POST", "/api/recipes/"+recipeID+"/ratings", map[string]any{"stars": 5, "comment": "great"}, nil); code != 201 {
		t.Fatalf("rate: %d", code)
	}
	var fb struct {
		Loved []struct{ Title string } `json:"loved"`
	}
	c.do("GET", "/api/feedback", nil, &fb)
	if len(fb.Loved) != 1 || fb.Loved[0].Title != pr.Recipes[recipeID].Title {
		t.Errorf("feedback = %+v", fb)
	}

	// 5. log the recipe (1.5 servings) and a manual entry, then read the day
	var entry models.LogEntry
	c.do("POST", "/api/log", map[string]any{"recipeId": recipeID, "servings": 1.5}, &entry)
	wantCal := pr.Recipes[recipeID].Macros.Calories * 1.5
	if entry.Macros.Calories != wantCal || entry.Date != "2026-03-10" {
		t.Errorf("log entry = %+v (want %v kcal)", entry, wantCal)
	}
	c.do("POST", "/api/log", map[string]any{"name": "Apple", "macros": map[string]float64{"calories": 95}}, nil)

	var day dayLog
	c.do("GET", "/api/log?date=2026-03-10", nil, &day)
	if len(day.Entries) != 2 || day.Totals.Calories != wantCal+95 {
		t.Errorf("day = %+v", day)
	}

	var hist struct {
		Days []struct {
			Date   string        `json:"date"`
			Totals models.Macros `json:"totals"`
		} `json:"days"`
	}
	c.do("GET", "/api/log/history?days=7", nil, &hist)
	if len(hist.Days) != 7 || hist.Days[6].Date != "2026-03-10" || hist.Days[6].Totals.Calories == 0 {
		t.Errorf("history = %+v", hist.Days)
	}

	// 6. delete things
	if code := c.do("DELETE", "/api/log/"+entry.ID, nil, nil); code != 204 {
		t.Errorf("delete log: %d", code)
	}
	if code := c.do("DELETE", "/api/plans/"+pr.Plan.ID, nil, nil); code != 204 {
		t.Errorf("delete plan: %d", code)
	}
}

func TestValidation(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "me@example.com")
	cases := []struct {
		method, path string
		body         any
		want         int
	}{
		{"POST", "/api/plans", map[string]int{"days": 9}, 400},
		{"POST", "/api/recipes/nope/ratings", map[string]int{"stars": 3}, 404},
		{"POST", "/api/log", map[string]string{"name": "x", "date": "10/03/2026"}, 400},
		{"POST", "/api/log", map[string]string{"name": "  "}, 400},
		{"GET", "/api/log?date=yesterday", nil, 400},
		{"POST", "/api/profile/targets", map[string]any{"sex": "male"}, 400},
		{"PUT", "/api/profile", map[string]int{"mealsPerDay": 0}, 400},
		{"PUT", "/api/settings", map[string]any{"llm": map[string]string{"provider": "openai"}}, 400},
		{"GET", "/api/plans/ghost", nil, 404},
		{"POST", "/api/log/estimate", map[string]string{"text": ""}, 400},
	}
	for _, tc := range cases {
		var e apiError
		if got := c.do(tc.method, tc.path, tc.body, &e); got != tc.want {
			t.Errorf("%s %s = %d (%s), want %d", tc.method, tc.path, got, e.Error, tc.want)
		}
	}
}

func TestStarsMustBeOneToFive(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "me@example.com")
	var pr planResp
	c.do("POST", "/api/plans", map[string]int{"days": 1}, &pr)
	id := pr.Plan.Days[0].RecipeIDs[0]
	for _, stars := range []int{0, 6} {
		if code := c.do("POST", "/api/recipes/"+id+"/ratings", map[string]int{"stars": stars}, nil); code != 400 {
			t.Errorf("stars=%d gave %d", stars, code)
		}
	}
}

func TestSettingsKeepKeyWhenMasked(t *testing.T) {
	c0, s := newTestServer(t)
	c := signUp(t, c0.srv, "admin@example.com")
	cfg := llm.Config{Provider: "openai", BaseURL: "http://localhost:11434/v1", Model: "llama3.1", APIKey: "sk-real-9999"}
	var got struct {
		LLM llm.Config `json:"llm"`
	}
	c.do("PUT", "/api/settings", map[string]any{"llm": cfg}, &got)
	if got.LLM.APIKey != "••••9999" {
		t.Errorf("response should mask key, got %q", got.LLM.APIKey)
	}
	// Browser sends the masked value back while changing the model.
	got.LLM.Model = "qwen2.5"
	c.do("PUT", "/api/settings", map[string]any{"llm": got.LLM}, nil)
	if stored := s.Store.LLMConfig(); stored.APIKey != "sk-real-9999" || stored.Model != "qwen2.5" {
		t.Errorf("stored = %+v", stored)
	}
}

func TestSettingsTestEndpoint(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "me@example.com")
	var out map[string]any
	if code := c.do("POST", "/api/settings/test", map[string]any{"llm": map[string]string{"provider": "mock"}}, &out); code != 200 {
		t.Errorf("mock ping failed: %d %v", code, out)
	}
}

func TestCalcTargets(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "me@example.com")
	var m models.Macros
	c.do("POST", "/api/profile/targets", models.Profile{Sex: "female", Age: 35, HeightCM: 168, WeightKG: 65, Activity: "light", Goal: "maintain"}, &m)
	if m.Calories < 1700 || m.Calories > 2200 || m.Protein == 0 {
		t.Errorf("unexpected targets %+v", m)
	}
}
