package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenWithTursoFallsBackToLocalSQLite(t *testing.T) {
	t.Setenv("TURSO_DATABASE_URL", "")
	t.Setenv("TURSO_AUTH_TOKEN", "")

	dbPath := filepath.Join(t.TempDir(), "fallback.db")
	db, err := OpenWithTurso(dbPath, "", "")
	if err != nil {
		t.Fatalf("expected the local fallback to succeed, got %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("pinging the local database: %v", err)
	}
}

func TestOpenWithTursoRejectsPartialCredentials(t *testing.T) {
	t.Setenv("TURSO_DATABASE_URL", "")
	t.Setenv("TURSO_AUTH_TOKEN", "")

	dbPath := filepath.Join(t.TempDir(), "partial.db")

	if _, err := OpenWithTurso(dbPath, "libsql://example.turso.io", ""); err == nil {
		t.Fatal("expected an error when the auth token is missing")
	} else if !strings.Contains(err.Error(), "TURSO_AUTH_TOKEN") {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := OpenWithTurso(dbPath, "", "token-only"); err == nil {
		t.Fatal("expected an error when the database URL is missing")
	} else if !strings.Contains(err.Error(), "TURSO_DATABASE_URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReconcileSchoolDomainsRewritesLegacyURLs(t *testing.T) {
	t.Setenv("TURSO_DATABASE_URL", "")
	t.Setenv("TURSO_AUTH_TOKEN", "")

	dbPath := filepath.Join(t.TempDir(), "domains.db")
	db, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	if err := MigrateMaster(db); err != nil {
		t.Fatalf("migrating: %v", err)
	}

	// Simulate a legacy row that still points at the path-style URL.
	if _, err := db.Exec(
		`UPDATE schools SET vercel_fallback_domain = ? WHERE subdomain_slug = 'greenwood'`,
		"https://school-webapp-self.vercel.app/s/greenwood",
	); err != nil {
		t.Fatalf("seeding legacy row: %v", err)
	}

	updated, err := ReconcileSchoolDomains(db, "nexacampus.tech", "school-webapp-self.vercel.app")
	if err != nil {
		t.Fatalf("reconciling: %v", err)
	}
	if updated < 1 {
		t.Fatalf("expected at least one updated tenant, got %d", updated)
	}

	var fallback, student, admin string
	err = db.QueryRow(
		`SELECT vercel_fallback_domain, student_domain, admin_domain FROM schools WHERE subdomain_slug = 'greenwood'`,
	).Scan(&fallback, &student, &admin)
	if err != nil {
		t.Fatalf("reading reconciled row: %v", err)
	}

	if fallback != "https://greenwood.school-webapp-self.vercel.app" {
		t.Errorf("fallback domain = %q", fallback)
	}
	if student != "https://greenwood.nexacampus.tech" {
		t.Errorf("student domain = %q", student)
	}
	if admin != "https://greenwood.admin.nexacampus.tech" {
		t.Errorf("admin domain = %q", admin)
	}
}

func TestMasterMigrationSeedsCanonicalDomains(t *testing.T) {
	t.Setenv("TURSO_DATABASE_URL", "")
	t.Setenv("TURSO_AUTH_TOKEN", "")

	dbPath := filepath.Join(t.TempDir(), "seed.db")
	db, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	if err := MigrateMaster(db); err != nil {
		t.Fatalf("migrating: %v", err)
	}

	var fallback string
	if err := db.QueryRow(
		`SELECT vercel_fallback_domain FROM schools WHERE subdomain_slug = 'greenwood'`,
	).Scan(&fallback); err != nil {
		t.Fatalf("reading seeded row: %v", err)
	}
	if !strings.HasSuffix(fallback, ".school-webapp-self.vercel.app") {
		t.Errorf("seeded tenant URL = %q, want a subdomain of the app host", fallback)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected the database file to exist: %v", err)
	}
}
