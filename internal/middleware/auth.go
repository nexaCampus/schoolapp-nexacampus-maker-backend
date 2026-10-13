package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/internal/security"
)

type contextKey string

const (
	AdminClaimsKey contextKey = "admin_claims"
)

// JWTAuth guards admin endpoints using an HMAC-SHA256 JWT secret.
func JWTAuth(secretKey []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				sendAuthError(w, http.StatusUnauthorized, "missing or malformed authorization header")
				return
			}

			tokenStr := strings.TrimSpace(authHeader[7:])
			claims, err := security.ValidateJWT(tokenStr, secretKey)
			if err != nil {
				sendAuthError(w, http.StatusUnauthorized, err.Error())
				return
			}

			// Store claims in context
			ctx := context.WithValue(r.Context(), AdminClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAdminClaims extracts the decoded claims from the request context.
func GetAdminClaims(ctx context.Context) (*security.AdminClaims, bool) {
	claims, ok := ctx.Value(AdminClaimsKey).(*security.AdminClaims)
	return claims, ok
}

func sendAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "unauthorized",
		"message": message,
	})
}
