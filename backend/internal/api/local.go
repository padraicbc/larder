package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// LocalLogin signs the app shell in when the server runs on the phone.
//
// The server listens on 127.0.0.1, which other apps on the same phone can
// also reach, so the API still requires a session. The Android app gets a
// one-time code in-process (never over the network), opens
// /api/auth/local?code=…, and receives the normal session cookie.
type LocalLogin struct {
	UserID string
	TTL    time.Duration

	mu    sync.Mutex
	codes map[string]time.Time // code -> expiry
}

func NewLocalLogin(userID string) *LocalLogin {
	return &LocalLogin{UserID: userID, TTL: 2 * time.Minute, codes: map[string]time.Time{}}
}

// Code returns a fresh single-use code.
func (l *LocalLogin) Code(now time.Time) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	c := hex.EncodeToString(b)
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, exp := range l.codes { // drop stale ones
		if now.After(exp) {
			delete(l.codes, k)
		}
	}
	l.codes[c] = now.Add(l.TTL)
	return c
}

// redeem consumes a code; true if it was valid.
func (l *LocalLogin) redeem(code string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	exp, ok := l.codes[code]
	delete(l.codes, code)
	return ok && now.Before(exp)
}

func (s *Server) localLogin(w http.ResponseWriter, r *http.Request) {
	if !s.Local.redeem(r.URL.Query().Get("code"), s.Now()) {
		writeErr(w, http.StatusUnauthorized, "this sign-in link has expired; reopen the app")
		return
	}
	u, err := s.Store.UserByID(s.Local.UserID)
	if err != nil {
		storeErr(w, err)
		return
	}
	if _, _, err := s.setSessionCookie(w, u); err != nil {
		writeErr(w, 500, "could not start a session")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
