// Package jwtutil issues and verifies the JWT access token, and generates
// the opaque refresh token stored (hashed) in auth.refresh_tokens.
//
// Design note (documented in docs/PROJECT_PLAN.md §5 and
// docs/architecture/observability-security.md §2): access tokens are
// signed HS256 with a shared secret for Phase 1's simplicity. Every
// service that needs to *verify* a token (today: only the API Gateway)
// needs the same secret via JWT_SIGNING_KEY. Upgrading to RS256 (a
// private key held only by Auth Service, a public key distributed to
// verifiers) is a drop-in change to GenerateAccessToken/ParseAccessToken's
// signing method and is called out as a Phase 6 hardening item — HS256 is
// a deliberate, temporary simplification, not an oversight.
package jwtutil

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

// Claims is the payload carried in every access token. It mirrors
// RequestContext from the (now-removed) proto contracts: org_id and role
// are what every downstream authorization check keys off of.
type Claims struct {
	OrgID string `json:"org_id"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateAccessToken signs a short-lived JWT for the given user/org/role.
// The token's jti (JWT ID) is a fresh UUID, used as the revocation key in
// Redis (see internal/routes/middleware.Auth).
func GenerateAccessToken(secret []byte, userID, orgID, role string, ttl time.Duration) (token string, jti string, err error) {
	jti = uuid.NewString()
	now := time.Now()
	claims := Claims{
		OrgID: orgID,
		Role:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		return "", "", err
	}
	return signed, jti, nil
}

// ParseAccessToken verifies signature and expiry and returns the claims.
// It does NOT check revocation — that's a separate Redis lookup by jti,
// kept out of this package so it stays a pure, side-effect-free parse.
func ParseAccessToken(secret []byte, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// NewOpaqueRefreshToken returns a random, URL-safe 256-bit token to hand
// to the client, plus its SHA-256 hex digest to store in
// auth.refresh_tokens.token_hash. The raw token is never persisted.
func NewOpaqueRefreshToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashRefreshToken(raw), nil
}

// HashRefreshToken hashes a raw refresh token the same way
// NewOpaqueRefreshToken does, so a token presented on /auth/refresh can be
// looked up by its hash.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
