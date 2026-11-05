package larder

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/you/nutrition-app/internal/auth"
	"github.com/you/nutrition-app/internal/webui"
)

func init() { auth.Iterations = 1000 }

var testUI = fstest.MapFS{"200.html": {Data: []byte("<html>larder shell</html>")}}

func browser() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func get(t *testing.T, c *http.Client, u string) (int, string) {
	t.Helper()
	res, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestStartSignsInAndServesApp(t *testing.T) {
	t.Cleanup(func() { Stop() })
	dir := t.TempDir()

	link, err := startWith(dir, testUI)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(link)
	if u.Hostname() != "127.0.0.1" || u.Port() == "" {
		t.Errorf("should listen on loopback only, got %s", link)
	}
	base := u.Scheme + "://" + u.Host

	c := browser()
	code, body := get(t, c, link) // follows the redirect to "/"
	if code != 200 || !strings.Contains(body, "larder shell") {
		t.Fatalf("open link = %d %q", code, body)
	}
	code, body = get(t, c, base+"/api/auth/me")
	var me struct {
		Email string `json:"email"`
		Admin bool   `json:"admin"`
		Local bool   `json:"local"`
	}
	json.Unmarshal([]byte(body), &me)
	if code != 200 || !me.Local || !me.Admin || me.Email != localEmail {
		t.Errorf("me = %d %+v", code, me)
	}

	// Another app on the phone, without the code, gets nothing.
	if code, _ := get(t, browser(), base+"/api/profile"); code != 401 {
		t.Errorf("unauthenticated request = %d", code)
	}
	// Codes are single use.
	if code, _ := get(t, browser(), link); code != 401 {
		t.Errorf("reused code = %d", code)
	}
	// Nobody can register a second account on the device.
	res, _ := http.Post(base+"/api/auth/register", "application/json", strings.NewReader(`{"email":"x@example.com","password":"password123"}`))
	if res.StatusCode != 403 {
		t.Errorf("register on device = %d", res.StatusCode)
	}

	// Start again (activity recreated): same server, fresh code.
	link2, _ := startWith(dir, testUI)
	if !strings.HasPrefix(link2, base) || link2 == link {
		t.Errorf("second Start = %s", link2)
	}
}

func TestDataSurvivesRestart(t *testing.T) {
	t.Cleanup(func() { Stop() })
	dir := t.TempDir()

	link, _ := startWith(dir, testUI)
	c := browser()
	get(t, c, link)
	base := link[:strings.Index(link, "/api/")]
	res, err := c.Post(base+"/api/pantry", "application/json", strings.NewReader(`{"items":[{"name":"eggs"}]}`))
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("add pantry: %v %v", res.StatusCode, err)
	}

	if err := Stop(); err != nil {
		t.Fatal(err)
	}
	link, _ = startWith(dir, testUI) // app relaunched
	c = browser()
	get(t, c, link)
	base = link[:strings.Index(link, "/api/")]
	_, body := get(t, c, base+"/api/pantry")
	if !strings.Contains(body, `"eggs"`) {
		t.Errorf("pantry lost after restart: %s", body)
	}
}

func TestStartChecksForEmbeddedUI(t *testing.T) {
	t.Cleanup(func() { Stop() })
	_, err := Start(t.TempDir())
	if _, embedded := webui.FS(); embedded {
		if err != nil {
			t.Errorf("web app is embedded, Start should work: %v", err)
		}
	} else if err == nil || !strings.Contains(err.Error(), "make android") {
		t.Errorf("without the web app Start must explain how to build: %v", err)
	}
}

func TestStopWhenNotRunning(t *testing.T) {
	if err := Stop(); err != nil {
		t.Error(err)
	}
}
