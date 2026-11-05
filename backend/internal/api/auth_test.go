package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/you/nutrition-app/internal/models"
)

func TestProtectedRoutesNeedAuth(t *testing.T) {
	c, _ := newTestServer(t)
	for _, path := range []string{"/api/profile", "/api/plans", "/api/pantry", "/api/auth/me", "/api/settings"} {
		if code := c.do("GET", path, nil, nil); code != 401 {
			t.Errorf("GET %s without auth = %d, want 401", path, code)
		}
	}
	if code := c.do("GET", "/api/health", nil, nil); code != 200 {
		t.Error("health should be public")
	}
	if code := c.do("GET", "/api/nope", nil, nil); code != 404 {
		t.Error("unknown API paths should 404")
	}
}

func TestRegisterLoginLogout(t *testing.T) {
	c, _ := newTestServer(t)

	var st map[string]bool
	c.do("GET", "/api/auth/status", nil, &st)
	if !st["firstUser"] || !st["signupOpen"] {
		t.Errorf("status before any users = %v", st)
	}

	var reg authResponse
	if code := c.do("POST", "/api/auth/register", map[string]string{"email": "Ada@Example.com", "password": "password123", "name": "Ada"}, &reg); code != 201 {
		t.Fatalf("register = %d", code)
	}
	if reg.User.Email != "ada@example.com" || !reg.User.Admin || reg.Token == "" {
		t.Errorf("register response = %+v", reg)
	}

	var me models.PublicUser
	if code := c.do("GET", "/api/auth/me", nil, &me); code != 200 || me.Name != "Ada" {
		t.Errorf("me via cookie = %d %+v", code, me)
	}

	c.do("POST", "/api/auth/logout", nil, nil)
	if code := c.do("GET", "/api/auth/me", nil, nil); code != 401 {
		t.Errorf("after logout = %d", code)
	}

	var login authResponse
	if code := c.do("POST", "/api/auth/login", map[string]string{"email": "ADA@example.com", "password": "password123"}, &login); code != 200 {
		t.Fatalf("login = %d", code)
	}

	// The token also works as a Bearer header for API clients.
	api := newClient(t, c.srv)
	api.token = login.Token
	if code := api.do("GET", "/api/auth/me", nil, nil); code != 200 {
		t.Errorf("bearer auth = %d", code)
	}
	api.token = login.Token + "x"
	if code := api.do("GET", "/api/auth/me", nil, nil); code != 401 {
		t.Errorf("tampered bearer = %d", code)
	}
}

func TestSessionCookieIsLockedDown(t *testing.T) {
	c, _ := newTestServer(t)
	req, _ := http.NewRequest("POST", c.srv.URL+"/api/auth/register", strings.NewReader(`{"email":"a@example.com","password":"password123"}`))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	ck := res.Cookies()
	if len(ck) != 1 || !ck[0].HttpOnly || ck[0].SameSite != http.SameSiteStrictMode || ck[0].Path != "/api" {
		t.Errorf("cookie = %+v", ck)
	}
	if res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing security headers")
	}
}

func TestRegisterValidation(t *testing.T) {
	c, s := newTestServer(t)
	cases := []struct {
		email, password string
		want            int
	}{
		{"not-an-email", "password123", 400},
		{"a@example.com", "short", 400},
		{"Bob <bob@example.com>", "password123", 400},
		{"ok@example.com", "password123", 201},
		{"OK@example.com", "password123", 409},
	}
	for _, tc := range cases {
		var e apiError
		if got := c.do("POST", "/api/auth/register", map[string]string{"email": tc.email, "password": tc.password}, &e); got != tc.want {
			t.Errorf("register(%q, %q) = %d (%s), want %d", tc.email, tc.password, got, e.Error, tc.want)
		}
	}

	s.SignupOpen = false
	if code := c.do("POST", "/api/auth/register", map[string]string{"email": "late@example.com", "password": "password123"}, nil); code != 403 {
		t.Errorf("closed signup = %d", code)
	}
}

func TestFirstUserCanAlwaysRegister(t *testing.T) {
	c, s := newTestServer(t)
	s.SignupOpen = false
	if code := c.do("POST", "/api/auth/register", map[string]string{"email": "owner@example.com", "password": "password123"}, nil); code != 201 {
		t.Errorf("first account with signup closed = %d", code)
	}
}

func TestLoginFailuresAndRateLimit(t *testing.T) {
	c, s := newTestServer(t)
	signUp(t, c.srv, "ada@example.com")
	s.Limiter.Max = 3

	bad := map[string]string{"email": "ada@example.com", "password": "wrong-password"}
	var e apiError
	if code := c.do("POST", "/api/auth/login", bad, &e); code != 401 {
		t.Errorf("wrong password = %d", code)
	}
	var e2 apiError
	c.do("POST", "/api/auth/login", map[string]string{"email": "ghost@example.com", "password": "whatever1"}, &e2)
	if e.Error != e2.Error {
		t.Errorf("unknown email and wrong password should look the same: %q vs %q", e.Error, e2.Error)
	}

	c.do("POST", "/api/auth/login", bad, nil)
	c.do("POST", "/api/auth/login", bad, nil)
	good := map[string]string{"email": "ada@example.com", "password": "password123"}
	if code := c.do("POST", "/api/auth/login", good, nil); code != 429 {
		t.Errorf("after 3 failures even the right password should be blocked, got %d", code)
	}

	// The window passes.
	s.Limiter.Now = func() time.Time { return time.Now().Add(time.Hour) }
	if code := c.do("POST", "/api/auth/login", good, nil); code != 200 {
		t.Errorf("after the window = %d", code)
	}
}

func TestUsersCantSeeEachOthersData(t *testing.T) {
	c, _ := newTestServer(t)
	alice := signUp(t, c.srv, "alice@example.com")
	bob := signUp(t, c.srv, "bob@example.com")

	var pr planResp
	alice.do("POST", "/api/plans", map[string]int{"days": 1}, &pr)
	recipeID := pr.Plan.Days[0].RecipeIDs[0]
	alice.do("POST", "/api/pantry", map[string]any{"items": []map[string]string{{"name": "eggs"}}}, nil)

	checks := []struct{ method, path string }{
		{"GET", "/api/plans/" + pr.Plan.ID},
		{"GET", "/api/plans/" + pr.Plan.ID + "/shopping"},
		{"DELETE", "/api/plans/" + pr.Plan.ID},
		{"GET", "/api/recipes/" + recipeID},
	}
	for _, ch := range checks {
		if code := bob.do(ch.method, ch.path, nil, nil); code != 404 {
			t.Errorf("bob %s %s = %d, want 404", ch.method, ch.path, code)
		}
	}
	if code := bob.do("POST", "/api/recipes/"+recipeID+"/ratings", map[string]int{"stars": 1}, nil); code != 404 {
		t.Errorf("bob rating alice's recipe = %d", code)
	}
	var plans, pantry []any
	bob.do("GET", "/api/plans", nil, &plans)
	bob.do("GET", "/api/pantry", nil, &pantry)
	if len(plans) != 0 || len(pantry) != 0 {
		t.Error("bob sees alice's plans or pantry")
	}
	// Alice still has hers.
	if code := alice.do("GET", "/api/plans/"+pr.Plan.ID, nil, nil); code != 200 {
		t.Error("alice lost her plan")
	}
}

func TestOnlyAdminChangesModelSettings(t *testing.T) {
	c, _ := newTestServer(t)
	admin := signUp(t, c.srv, "admin@example.com")
	member := signUp(t, c.srv, "member@example.com")

	body := map[string]any{"llm": map[string]string{"provider": "mock"}}
	if code := member.do("PUT", "/api/settings", body, nil); code != 403 {
		t.Errorf("member PUT settings = %d", code)
	}
	if code := member.do("POST", "/api/settings/test", body, nil); code != 403 {
		t.Errorf("member test settings = %d", code)
	}
	var got map[string]any
	member.do("GET", "/api/settings", nil, &got)
	if got["canEdit"] != false {
		t.Error("member should see canEdit=false")
	}
	if code := admin.do("PUT", "/api/settings", body, nil); code != 200 {
		t.Errorf("admin PUT settings = %d", code)
	}
}
