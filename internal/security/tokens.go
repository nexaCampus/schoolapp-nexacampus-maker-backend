package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateSecureToken generates a cryptographically random hex-encoded token of n bytes (yielding 2*n characters).
func GenerateSecureToken(nBytes int) (string, error) {
	if nBytes <= 0 {
		nBytes = 32
	}
	bytes := make([]byte, nBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// MustGenerateSecureToken generates a secure token or panics only if system entropy fails.
func MustGenerateSecureToken(nBytes int) string {
	token, err := GenerateSecureToken(nBytes)
	if err != nil {
		panic(fmt.Sprintf("crypto/rand failure: %v", err))
	}
	return token
}
