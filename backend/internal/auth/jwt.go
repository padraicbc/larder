package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "larder"

// Tokens issues and verifies HS256 JWTs whose subject is the user ID.
type Tokens struct {
	Secret []byte
	TTL    time.Duration
	Now    func() time.Time
}

func NewTokens(secret []byte, ttl time.Duration) *Tokens {
	return &Tokens{Secret: secret, TTL: ttl, Now: time.Now}
}

func (t *Tokens) Issue(userID string) (token string, expires time.Time, err error) {
	now := t.Now()
	expires = now.Add(t.TTL)
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expires),
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.Secret)
	return token, expires, err
}

var ErrInvalidToken = errors.New("invalid or expired session")

// Parse returns the user ID. Only HS256 is accepted, which blocks "alg: none"
// and algorithm-confusion tricks.
func (t *Tokens) Parse(token string) (string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return t.Secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.Now),
	)
	if err != nil || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
