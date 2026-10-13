package security

import (
	"testing"
	"time"
)

func TestPasswordHashing(t *testing.T) {
	raw := "SecurePassword123!"
	hash, err := HashPassword(raw)
	if err != nil {
		t.Fatalf("hashing password: %v", err)
	}

	if !CheckPasswordHash(raw, hash) {
		t.Errorf("expected correct password to match hash")
	}

	if CheckPasswordHash("WrongPassword!", hash) {
		t.Errorf("expected incorrect password to fail match")
	}

	// Short password failure
	_, err = HashPassword("short")
	if err == nil {
		t.Errorf("expected short password error, got nil")
	}
}

func TestJWTGenerationAndValidation(t *testing.T) {
	secret := []byte("this-is-a-valid-32-byte-secret-key-for-testing!")
	adminID := "adm_12345"
	username := "headmaster"
	tenantSlug := "lcgvm"

	// 1. Success case
	token, err := GenerateJWT(adminID, username, tenantSlug, secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("generating JWT: %v", err)
	}

	claims, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("validating valid JWT: %v", err)
	}

	if claims.AdminID != adminID || claims.Username != username || claims.TenantSlug != tenantSlug {
		t.Errorf("claims mismatch: got %+v", claims)
	}

	// 2. Invalid secret rejection
	wrongSecret := []byte("another-different-32-byte-secret-key-here!")
	_, err = ValidateJWT(token, wrongSecret)
	if err == nil {
		t.Errorf("expected validation failure with wrong secret")
	}

	// 3. Expired token rejection
	expiredToken, err := GenerateJWT(adminID, username, tenantSlug, secret, -1*time.Minute)
	if err != nil {
		t.Fatalf("generating expired token: %v", err)
	}
	_, err = ValidateJWT(expiredToken, secret)
	if err == nil {
		t.Errorf("expected validation failure on expired token")
	}
}
