package jwtutil

import (
	"testing"
	"time"
)

func TestGenerateAndParseAccessToken_RoundTrip(t *testing.T) {
	secret := []byte("test-secret")
	token, jti, err := GenerateAccessToken(secret, "user-1", "org-1", "owner", time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if jti == "" {
		t.Fatal("expected non-empty jti")
	}

	claims, err := ParseAccessToken(secret, token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.Subject != "user-1" || claims.OrgID != "org-1" || claims.Role != "owner" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.ID != jti {
		t.Fatalf("claims.ID = %q, want %q", claims.ID, jti)
	}
}

func TestParseAccessToken_RejectsWrongSecret(t *testing.T) {
	token, _, err := GenerateAccessToken([]byte("secret-a"), "user-1", "org-1", "owner", time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if _, err := ParseAccessToken([]byte("secret-b"), token); err == nil {
		t.Fatal("expected error parsing token signed with a different secret")
	}
}

func TestParseAccessToken_RejectsExpiredToken(t *testing.T) {
	secret := []byte("test-secret")
	token, _, err := GenerateAccessToken(secret, "user-1", "org-1", "owner", -time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if _, err := ParseAccessToken(secret, token); err == nil {
		t.Fatal("expected error parsing an already-expired token")
	}
}

func TestNewOpaqueRefreshToken_HashIsDeterministicAndDistinct(t *testing.T) {
	raw1, hash1, err := NewOpaqueRefreshToken()
	if err != nil {
		t.Fatalf("NewOpaqueRefreshToken: %v", err)
	}
	raw2, hash2, err := NewOpaqueRefreshToken()
	if err != nil {
		t.Fatalf("NewOpaqueRefreshToken: %v", err)
	}

	if raw1 == raw2 || hash1 == hash2 {
		t.Fatal("expected two calls to produce distinct tokens")
	}
	if HashRefreshToken(raw1) != hash1 {
		t.Fatal("HashRefreshToken(raw1) should reproduce the same hash returned at generation time")
	}
}
