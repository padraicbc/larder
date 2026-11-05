// Package auth handles passwords, JWTs and login rate limiting.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// PBKDF2-SHA256 from the standard library (OWASP 2023 recommends 600k iterations).
// The iteration count is stored in each hash, so you can raise it later and
// old hashes still verify. Tests lower it for speed.
var Iterations = 600_000

const (
	MinPasswordLen = 8
	MaxPasswordLen = 256
	saltLen        = 16
	keyLen         = 32
	scheme         = "pbkdf2-sha256"
)

var ErrWeakPassword = fmt.Errorf("password must be %d–%d characters", MinPasswordLen, MaxPasswordLen)

func ValidatePassword(pw string) error {
	if n := utf8.RuneCountInString(pw); n < MinPasswordLen || n > MaxPasswordLen {
		return ErrWeakPassword
	}
	return nil
}

// HashPassword returns "pbkdf2-sha256$<iter>$<salt>$<key>" (base64, no padding).
func HashPassword(pw string) (string, error) {
	if err := ValidatePassword(pw); err != nil {
		return "", err
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, pw, salt, Iterations, keyLen)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("%s$%d$%s$%s", scheme, Iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// CheckPassword reports whether pw matches the encoded hash, in constant time.
func CheckPassword(encoded, pw string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != scheme {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[2])
	want, err2 := enc.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is checked when an email doesn't exist, so a login attempt takes
// the same time whether or not the account is real.
var dummyHash = func() string {
	h, _ := HashPassword("not-a-real-password")
	return h
}()

func CheckDummy(pw string) { _ = CheckPassword(dummyHash, pw) }

var ErrInvalidCredentials = errors.New("email or password is incorrect")
