package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
)

// Recoverer catches runtime panics, logs stack traces, and sends a 500 JSON envelope
// to prevent crashing single-core shared server processes.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				log.Printf("[PANIC RECOVERED] %s %s: %v\nStack:\n%s", r.Method, r.URL.Path, rvr, string(debug.Stack()))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "internal_server_error",
					"message": "An unexpected server condition occurred. Process execution was defensively recovered.",
				})
			}
		}()

		next.ServeHTTP(w, r)
	})
}
