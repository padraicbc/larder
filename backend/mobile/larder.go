// Package larder is the Android entry point, built into an .aar with
// `gomobile bind`. Java/Kotlin sees it as the class larder.Larder:
//
//	String url = Larder.start(getFilesDir().getAbsolutePath());
//	webView.loadUrl(url);
//
// The whole Go server (API + embedded web app) runs inside the app on
// 127.0.0.1. Only exported functions with gomobile-friendly types (strings,
// errors) are allowed here.
package larder

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/you/nutrition-app/internal/api"
	"github.com/you/nutrition-app/internal/auth"
	"github.com/you/nutrition-app/internal/models"
	"github.com/you/nutrition-app/internal/store"
	"github.com/you/nutrition-app/internal/webui"
)

const localEmail = "me@device.local"

type instance struct {
	http *http.Server
	api  *api.Server
	base string
}

var (
	mu      sync.Mutex
	running *instance
)

// Start launches the server on first call (later calls reuse it) and returns
// a URL that signs the WebView in. Each URL works once, for two minutes.
// Data is kept in dataDir/larder.json (pass the app's private files dir).
func Start(dataDir string) (string, error) {
	ui, ok := webui.FS()
	if !ok {
		return "", errors.New("web app not embedded: build with `make android`")
	}
	return startWith(dataDir, ui)
}

func startWith(dataDir string, ui fs.FS) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if running == nil {
		inst, err := launch(dataDir, ui)
		if err != nil {
			return "", err
		}
		running = inst
	}
	return running.base + "/api/auth/local?code=" + running.api.Local.Code(time.Now()), nil
}

// Stop shuts the server down. Safe to call when it isn't running.
func Stop() error {
	mu.Lock()
	defer mu.Unlock()
	if running == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := running.http.Shutdown(ctx)
	running = nil
	return err
}

func launch(dataDir string, ui fs.FS) (*instance, error) {
	st, err := store.Open(filepath.Join(dataDir, "larder.json"))
	if err != nil {
		return nil, err
	}
	u, err := localUser(st)
	if err != nil {
		return nil, err
	}
	secret, err := st.JWTSecret() // lives in the app's private storage
	if err != nil {
		return nil, err
	}

	s := api.New(st, secret)
	s.SignupOpen = false // the one local account already exists
	s.Local = api.NewLocalLogin(u.ID)
	s.StaticFS = ui

	// Port 0: the OS picks a free port. Loopback only: nothing off the phone can connect.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return &instance{http: srv, api: s, base: "http://" + ln.Addr().String()}, nil
}

// localUser returns the phone's single account, creating it on first run.
// Its password is random and never shown: sign-in only happens via Start.
func localUser(st *store.Store) (models.User, error) {
	if u, err := st.UserByEmail(localEmail); err == nil {
		return u, nil
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return models.User{}, err
	}
	hash, err := auth.HashPassword(hex.EncodeToString(b))
	if err != nil {
		return models.User{}, err
	}
	return st.CreateUser(localEmail, "", hash, time.Now())
}
