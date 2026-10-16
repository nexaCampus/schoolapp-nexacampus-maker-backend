package security

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// SlugRegex matches 3 to 32 lowercase alphanumeric characters with hyphens,
	// starting and ending with an alphanumeric character.
	SlugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`)

	// ReservedSlugs is a strict blocklist preventing collision with core services.
	ReservedSlugs = map[string]struct{}{
		"admin":        {},
		"api":          {},
		"app":          {},
		"assets":       {},
		"auth":         {},
		"billing":      {},
		"cdn":          {},
		"control":      {},
		"dashboard":    {},
		"ftp":          {},
		"healthz":      {},
		"mail":         {},
		"maker":        {},
		"metrics":      {},
		"nexa":         {},
		"nexacampus":   {},
		"orchestrator": {},
		"root":         {},
		"schoolmaker":  {},
		"smtp":         {},
		"static":       {},
		"status":       {},
		"system":       {},
		"www":          {},
	}

	ErrInvalidSlugFormat = errors.New("invalid slug: must be 2-32 characters, lowercase alphanumeric with optional internal hyphens")
	ErrReservedSlug      = errors.New("slug is reserved for platform infrastructure and cannot be claimed")
	ErrPathTraversal     = errors.New("potential path traversal detected in file operation")
)

// ValidateSlug verifies the format and reservation status of a proposed tenant slug.
func ValidateSlug(slug string) error {
	trimmed := strings.TrimSpace(slug)

	// Check reservation first (case-insensitive)
	if _, reserved := ReservedSlugs[strings.ToLower(trimmed)]; reserved {
		return ErrReservedSlug
	}

	// Must strictly match lowercase format
	if !SlugRegex.MatchString(trimmed) {
		return ErrInvalidSlugFormat
	}

	return nil
}

// SanitizeTenantPath constructs and verifies a safe path inside a base directory,
// strictly preventing directory traversal attacks.
func SanitizeTenantPath(baseDir, slug string) (string, error) {
	if err := ValidateSlug(slug); err != nil {
		return "", err
	}

	cleanBase := filepath.Clean(baseDir)
	absBase, err := filepath.Abs(cleanBase)
	if err != nil {
		return "", fmt.Errorf("resolving absolute base directory: %w", err)
	}

	targetPath := filepath.Join(absBase, slug)
	cleanTarget := filepath.Clean(targetPath)

	// Ensure target remains strictly within the base directory
	rel, err := filepath.Rel(absBase, cleanTarget)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return "", ErrPathTraversal
	}

	return cleanTarget, nil
}
