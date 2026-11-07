package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/padraicbc/larder/internal/auth"
	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/store"
)

const sessionCookie = "larder_session"

type ctxKey struct{}

// userID returns the signed-in user's ID (set by requireAuth).
func userID(r *http.Request) string { return currentUser(r).ID }

func currentUser(r *http.Request) models.User {
	u, _ := r.Context().Value(ctxKey{}).(models.User)
	return u
}

// tokenFrom accepts "Authorization: Bearer <jwt>" (API clients, scripts)
// or the session cookie (the web app).
func tokenFrom(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	return ""
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, err := s.Tokens.Parse(tokenFrom(r))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "sign in to continue")
			return
		}
		u, err := s.Store.UserByID(uid) // account may have been deleted
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "sign in to continue")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).Admin {
			writeErr(w, http.StatusForbidden, "only the admin can change this")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type authResponse struct {
	User      models.PublicUser `json:"user"`
	Token     string            `json:"token"` // for API clients; the browser uses the cookie
	ExpiresAt time.Time         `json:"expiresAt"`
}

func (s *Server) startSession(w http.ResponseWriter, u models.User, code int) {
	token, exp, err := s.setSessionCookie(w, u)
	if err != nil {
		writeErr(w, 500, "could not start a session")
		return
	}
	writeJSON(w, code, authResponse{u.Public(), token, exp})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, u models.User) (string, time.Time, error) {
	token, exp, err := s.Tokens.Issue(u.ID)
	if err != nil {
		return "", exp, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/api",
		Expires:  exp,
		HttpOnly: true,                    // JS can't read it, so XSS can't steal it
		SameSite: http.SameSiteStrictMode, // not sent on cross-site requests (CSRF)
		Secure:   s.SecureCookies,
	})
	return token, exp, nil
}

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	n := s.Store.UserCount()
	writeJSON(w, 200, map[string]bool{"signupOpen": s.SignupOpen || n == 0, "firstUser": n == 0})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func validEmail(e string) bool {
	if len(e) > 254 {
		return false
	}
	a, err := mail.ParseAddress(e)
	return err == nil && a.Address == e && strings.Contains(e, ".")
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	email := store.NormalizeEmail(c.Email)
	if !validEmail(email) {
		writeErr(w, 400, "enter a valid email address")
		return
	}
	if err := auth.ValidatePassword(c.Password); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if !s.SignupOpen && s.Store.UserCount() > 0 {
		writeErr(w, 403, "sign-ups are closed on this server")
		return
	}
	hash, err := auth.HashPassword(c.Password)
	if err != nil {
		writeErr(w, 500, "could not create account")
		return
	}
	u, err := s.Store.CreateUser(email, c.Name, hash, s.Now())
	if errors.Is(err, store.ErrEmailTaken) {
		writeErr(w, 409, err.Error())
		return
	}
	if err != nil {
		storeErr(w, err)
		return
	}
	s.startSession(w, u, 201)
}

// clientIP is the address used for login rate limiting. Behind a reverse proxy
// every request comes from the proxy, so with TrustProxy we use the last
// X-Forwarded-For entry, which is the one your proxy appended.
func (s *Server) clientIP(r *http.Request) string {
	if s.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	email := store.NormalizeEmail(c.Email)
	key := s.clientIP(r) + "|" + email
	if ok, wait := s.Limiter.Allowed(key); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeErr(w, http.StatusTooManyRequests, "too many attempts; try again in "+wait.Round(time.Minute).String())
		return
	}
	u, err := s.Store.UserByEmail(email)
	if err != nil {
		auth.CheckDummy(c.Password) // same timing whether or not the email exists
		s.Limiter.Fail(key)
		writeErr(w, 401, auth.ErrInvalidCredentials.Error())
		return
	}
	if !auth.CheckPassword(u.PasswordHash, c.Password) {
		s.Limiter.Fail(key)
		writeErr(w, 401, auth.ErrInvalidCredentials.Error())
		return
	}
	s.Limiter.Reset(key)
	s.startSession(w, u, 200)
}

// logout clears the cookie. JWTs are stateless, so a copied token stays valid
// until it expires; see README for adding a token version if you need revocation.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/api", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.SecureCookies})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, struct {
		models.PublicUser
		Local bool `json:"local"` // on-device: no sign-out, data lives on the phone
	}{currentUser(r).Public(), s.Local != nil})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
