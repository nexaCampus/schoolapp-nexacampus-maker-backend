package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken      = errors.New("invalid or expired authentication token")
	ErrInvalidSigningKey = errors.New("jwt secret key must be at least 32 bytes")
)

// AdminClaims represents custom JWT claims for tenant administrators.
type AdminClaims struct {
	AdminID    string `json:"admin_id"`
	Username   string `json:"username"`
	TenantSlug string `json:"tenant_slug"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a signed HMAC-SHA256 JWT token for an authenticated administrator.
func GenerateJWT(adminID, username, tenantSlug string, secretKey []byte, duration time.Duration) (string, error) {
	if len(secretKey) < 32 {
		return "", ErrInvalidSigningKey
	}

	now := time.Now().UTC()
	claims := AdminClaims{
		AdminID:    adminID,
		Username:   username,
		TenantSlug: tenantSlug,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   adminID,
			Issuer:    "nexacampus-auth",
			Audience:  jwt.ClaimStrings{"nexacampus-tenant-" + tenantSlug},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(secretKey)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return signedToken, nil
}

// ValidateJWT verifies and decodes an HMAC-SHA256 JWT token.
func ValidateJWT(tokenString string, secretKey []byte) (*AdminClaims, error) {
	if len(secretKey) < 32 {
		return nil, ErrInvalidSigningKey
	}

	token, err := jwt.ParseWithClaims(tokenString, &AdminClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*AdminClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
