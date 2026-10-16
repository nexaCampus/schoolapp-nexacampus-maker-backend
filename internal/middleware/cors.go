package middleware

import (
	"net/http"
	"strings"
)

// CORS handles Cross-Origin Resource Sharing headers and preflight OPTIONS requests.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	wildcard := false
	originsMap := make(map[string]struct{})
	for _, o := range allowedOrigins {
		if o == "*" {
			wildcard = true
		}
		originsMap[strings.ToLower(strings.TrimSpace(o))] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" {
				if wildcard {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := originsMap[strings.ToLower(origin)]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			} else if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Tenant-Slug, X-School-Slug, X-Portal-Role, X-Backend-Host")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders applies defensive HTTP headers to all incoming responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		next.ServeHTTP(w, r)
	})
}
