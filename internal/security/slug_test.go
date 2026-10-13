package security

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		slug    string
		wantErr error
	}{
		{"lcgvm", nil},
		{"school-101", nil},
		{"campus-west", nil},
		{"delhi-public-school", nil},
		// Invalid formats
		{"", ErrInvalidSlugFormat},
		{"ab", ErrInvalidSlugFormat},       // Too short (<4)
		{"-abc", ErrInvalidSlugFormat},     // Leading hyphen
		{"abc-", ErrInvalidSlugFormat},     // Trailing hyphen
		{"CapitalLetters", ErrInvalidSlugFormat},
		{"invalid_underscore", ErrInvalidSlugFormat},
		{"slug.with.dots", ErrInvalidSlugFormat},
		{"this-slug-is-way-too-long-exceeding-thirty-two-characters-limit", ErrInvalidSlugFormat},
		// Blocklisted / Reserved
		{"admin", ErrReservedSlug},
		{"api", ErrReservedSlug},
		{"app", ErrReservedSlug},
		{"auth", ErrReservedSlug},
		{"billing", ErrReservedSlug},
		{"root", ErrReservedSlug},
		{"www", ErrReservedSlug},
		{"maker", ErrReservedSlug},
		{"orchestrator", ErrReservedSlug},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			err := ValidateSlug(tt.slug)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateSlug(%q) = %v, want %v", tt.slug, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeTenantPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tenant-security-test")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Valid path
	validPath, err := SanitizeTenantPath(tempDir, "lcgvm")
	if err != nil {
		t.Fatalf("expected clean path, got error: %v", err)
	}
	expected := filepath.Join(tempDir, "lcgvm")
	if validPath != expected {
		t.Errorf("got %s, want %s", validPath, expected)
	}

	// Attempted traversal
	_, err = SanitizeTenantPath(tempDir, "admin")
	if err == nil {
		t.Errorf("expected reserved slug error, got nil")
	}
}
