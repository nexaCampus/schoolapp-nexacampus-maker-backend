package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// ReconcileSchoolDomains rewrites the stored tenant URLs of every school so that
// the canonical subdomain form is persisted in the database:
//
//	student_domain        -> https://<slug>.<baseDomain>          (DNS-bound custom domain)
//	admin_domain          -> https://<slug>.admin.<baseDomain>    (DNS-bound admin domain)
//	vercel_fallback_domain-> https://<slug>.<appBaseDomain>        (canonical app-host portal)
//
// Rows written before the subdomain migration still hold the legacy path-style
// value ("https://<app-host>/school/<slug>"), so this runs on every boot and
// keeps both local SQLite and Turso databases in sync with the configuration.
func ReconcileSchoolDomains(db *sql.DB, baseDomain, appBaseDomain string) (int, error) {
	if db == nil {
		return 0, nil
	}

	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))
	if baseDomain == "" {
		baseDomain = "nexacampus.school"
	}
	appBaseDomain = strings.ToLower(strings.TrimSpace(appBaseDomain))
	if appBaseDomain == "" {
		appBaseDomain = "nexacampus.school"
	}

	rows, err := db.Query(`SELECT id, subdomain_slug, student_domain, admin_domain, vercel_fallback_domain FROM schools`)
	if err != nil {
		return 0, fmt.Errorf("reading schools for domain reconciliation: %w", err)
	}
	defer rows.Close()

	type pendingUpdate struct {
		id            string
		studentDomain string
		adminDomain   string
		appDomain     string
	}

	updates := []pendingUpdate{}
	for rows.Next() {
		var id, slug, studentDomain, adminDomain, fallbackDomain string
		if err := rows.Scan(&id, &slug, &studentDomain, &adminDomain, &fallbackDomain); err != nil {
			continue
		}
		cleanSlug := strings.ToLower(strings.TrimSpace(slug))
		if cleanSlug == "" {
			continue
		}
		updates = append(updates, pendingUpdate{
			id:            id,
			studentDomain: fmt.Sprintf("https://%s.%s", cleanSlug, baseDomain),
			adminDomain:   fmt.Sprintf("https://%s.admin.%s", cleanSlug, baseDomain),
			appDomain:     fmt.Sprintf("https://%s.%s", cleanSlug, appBaseDomain),
		})
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterating schools for domain reconciliation: %w", err)
	}

	updated := 0
	now := time.Now().UTC()
	for _, u := range updates {
		res, err := db.Exec(
			`UPDATE schools SET student_domain = ?, admin_domain = ?, vercel_fallback_domain = ?, updated_at = ? WHERE id = ?`,
			u.studentDomain, u.adminDomain, u.appDomain, now, u.id,
		)
		if err != nil {
			return updated, fmt.Errorf("reconciling domains for school %s: %w", u.id, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			updated++
		}
	}

	if updated > 0 {
		log.Printf("[Database] Reconciled tenant URLs for %d school(s) to the %s subdomain form", updated, appBaseDomain)
	}
	return updated, nil
}
