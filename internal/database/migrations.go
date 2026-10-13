package database

import (
	"database/sql"
	"fmt"

	"github.com/nexaCampus/schoolapp-nexacampus-maker-backend/migrations"
)

// MigrateMaster applies the master schema to a database.
func MigrateMaster(db *sql.DB) error {
	if _, err := db.Exec(migrations.MasterSchemaSQL); err != nil {
		return fmt.Errorf("executing master schema migrations: %w", err)
	}
	return nil
}

// MigrateTenant applies the tenant schema to an isolated tenant database.
func MigrateTenant(db *sql.DB) error {
	if _, err := db.Exec(migrations.TenantSchemaSQL); err != nil {
		return fmt.Errorf("executing tenant schema migrations: %w", err)
	}
	return nil
}

// SeedTenantAdmin ensures the tenant admin account exists with the designated credentials.
func SeedTenantAdmin(db *sql.DB, adminID, username, passwordHash string) error {
	query := `
		INSERT INTO admins (id, username, password_hash, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(username) DO UPDATE SET password_hash = excluded.password_hash
	`
	_, err := db.Exec(query, adminID, username, passwordHash)
	return err
}

// SeedSchoolMeta initializes basic branding in school_meta.
func SeedSchoolMeta(db *sql.DB, schoolName, primaryColor, secondaryColor string) error {
	query := `
		INSERT INTO school_meta (id, name, tagline, theme_primary_color, theme_secondary_color, updated_at)
		VALUES (1, ?, 'Excellence in Global Learning & Research', ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name
	`
	_, err := db.Exec(query, schoolName, primaryColor, secondaryColor)
	return err
}
