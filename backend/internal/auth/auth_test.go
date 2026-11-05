package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func init() { Iterations = 1000 } // keep tests fast

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "pbkdf2-sha256$1000$") {
		t.Errorf("unexpected format %q", h)
	}
	if !CheckPassword(h, "correct horse") {
		t.Error("right password rejected")
	}
	if CheckPassword(h, "correct horsE") {
		t.Error("wrong password accepted")
	}
	h2, _ := HashPassword("correct horse")
	if h == h2 {
		t.Error("salts should differ")
	}
}

func TestPasswordRules(t *testing.T) {
	for _, pw := range []string{"", "short", strings.Repeat("x", 257)} {
		if _, err := HashPassword(pw); err == nil {
			t.Errorf("%d-char password should be rejected", len(pw))
		}
	}
	if _, err := HashPassword("ünïcødé!"); err != nil {
		t.Error("8 unicode characters should be fine:", err)
	}
}

func TestCheckPasswordRejectsGarbage(t *testing.T) {
	for _, h := range []string{"", "plain", "bcrypt$1$a$b", "pbkdf2-sha256$x$a$b", "pbkdf2-sha256$10$!!$!!"} {
		if CheckPassword(h, "whatever1") {
			t.Errorf("accepted bad hash %q", h)
		}
	}
}

func TestOldHashesStillVerifyAfterRaisingIterations(t *testing.T) {
	h, _ := HashPassword("password123")
	old := Iterations
	Iterations = 2000
	defer func() { Iterations = old }()
	if !CheckPassword(h, "password123") {
		t.Error("hash made with fewer iterations should still verify")
	}
}

func fixedTokens(now time.Time) *Tokens {
	tk := NewTokens([]byte("test-secret-test-secret-32-bytes"), time.Hour)
	tk.Now = func() time.Time { return now }
	return tk
}

func TestTokens(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tk := fixedTokens(now)
	tok, exp, err := tk.Issue("user-1")
	if err != nil || !exp.Equal(now.Add(time.Hour)) {
		t.Fatal(err, exp)
	}
	if uid, err := tk.Parse(tok); err != nil || uid != "user-1" {
		t.Errorf("Parse = %q, %v", uid, err)
	}

	t.Run("expired", func(t *testing.T) {
		later := fixedTokens(now.Add(2 * time.Hour))
		if _, err := later.Parse(tok); err == nil {
			t.Error("expired token accepted")
		}
	})
	t.Run("wrong secret", func(t *testing.T) {
		other := fixedTokens(now)
		other.Secret = []byte("another-secret-another-secret-32")
		if _, err := other.Parse(tok); err == nil {
			t.Error("token signed with another secret accepted")
		}
	})
	t.Run("tampered payload", func(t *testing.T) {
		parts := strings.Split(tok, ".")
		parts[1] = parts[1][:len(parts[1])-2] + "AA"
		if _, err := tk.Parse(strings.Join(parts, ".")); err == nil {
			t.Error("tampered token accepted")
		}
	})
	t.Run("alg none", func(t *testing.T) {
		none := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
			Subject: "admin", Issuer: issuer, ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		})
		s, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if _, err := tk.Parse(s); err == nil {
			t.Error("alg=none accepted")
		}
	})
	t.Run("no expiry", func(t *testing.T) {
		s, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "u", Issuer: issuer}).SignedString(tk.Secret)
		if _, err := tk.Parse(s); err == nil {
			t.Error("token without exp accepted")
		}
	})
	t.Run("wrong issuer", func(t *testing.T) {
		s, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			Subject: "u", Issuer: "someone-else", ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		}).SignedString(tk.Secret)
		if _, err := tk.Parse(s); err == nil {
			t.Error("foreign issuer accepted")
		}
	})
}

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewLimiter(3, 10*time.Minute)
	l.Now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if ok, _ := l.Allowed("k"); !ok {
			t.Fatalf("attempt %d blocked too early", i+1)
		}
		l.Fail("k")
	}
	ok, wait := l.Allowed("k")
	if ok || wait != 10*time.Minute {
		t.Errorf("want blocked for 10m, got ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.Allowed("other"); !ok {
		t.Error("other keys should be unaffected")
	}

	now = now.Add(11 * time.Minute)
	if ok, _ := l.Allowed("k"); !ok {
		t.Error("should unblock after the window")
	}

	l.Fail("k")
	l.Reset("k")
	if ok, _ := l.Allowed("k"); !ok {
		t.Error("Reset should clear failures")
	}
}
