package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"
)

func TestLocalLoginCodes(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewLocalLogin("u1")
	code := l.Code(now)
	if !l.redeem(code, now.Add(time.Minute)) {
		t.Error("fresh code rejected")
	}
	if l.redeem(code, now.Add(time.Minute)) {
		t.Error("code worked twice")
	}
	old := l.Code(now)
	if l.redeem(old, now.Add(3*time.Minute)) {
		t.Error("expired code accepted")
	}
	if l.redeem("", now) {
		t.Error("empty code accepted")
	}
}

func TestLocalLoginRouteOnlyOnDevice(t *testing.T) {
	c, _ := newTestServer(t) // a normal server: Local is nil
	if code := c.do("GET", "/api/auth/local?code=x", nil, nil); code != 404 {
		t.Errorf("local login on a normal server = %d, want 404", code)
	}
}

func TestClientIPBehindProxy(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.2:5555"                          // the proxy
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.9") // first value is client-supplied, last is the proxy's

	if got := s.clientIP(r); got != "10.0.0.2" {
		t.Errorf("without TrustProxy = %s (must ignore the header)", got)
	}
	s.TrustProxy = true
	if got := s.clientIP(r); got != "203.0.113.9" {
		t.Errorf("with TrustProxy = %s, want the entry the proxy added", got)
	}
}

func TestSPAServing(t *testing.T) {
	ui := fstest.MapFS{
		"200.html":                {Data: []byte("shell")},
		"manifest.webmanifest":    {Data: []byte("{}")},
		"_app/immutable/app.1.js": {Data: []byte("js")},
	}
	h := spa(ui)
	cases := []struct{ path, body, cache string }{
		{"/", "shell", "no-cache"},
		{"/plans/abc", "shell", "no-cache"},
		{"/_app/immutable/app.1.js", "js", "public, max-age=31536000, immutable"},
		{"/manifest.webmanifest", "{}", "no-cache"},
		{"/../../etc/passwd", "shell", "no-cache"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Body.String() != tc.body || w.Header().Get("Cache-Control") != tc.cache {
			t.Errorf("%s -> %q (%s)", tc.path, w.Body.String(), w.Header().Get("Cache-Control"))
		}
	}
	w := httptest.NewRecorder()
	(&Server{StaticFS: ui, Store: nil}).spaOnly().ServeHTTP(w, httptest.NewRequest("GET", "/api/whatever", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown /api path served %d", w.Code)
	}
}

func TestManifestContentType(t *testing.T) {
	w := httptest.NewRecorder()
	spa(fstest.MapFS{"200.html": {}, "manifest.webmanifest": {Data: []byte("{}")}}).
		ServeHTTP(w, httptest.NewRequest("GET", "/manifest.webmanifest", nil))
	if ct := w.Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("Content-Type = %q", ct)
	}
}
