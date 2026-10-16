package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

const pragmas = `
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -2000;
PRAGMA temp_store = MEMORY;
PRAGMA foreign_keys = ON;
`

// OpenSQLite opens a database connection.
// If TURSO_DATABASE_URL and TURSO_AUTH_TOKEN environment variables are provided,
// it connects to the remote, persistent Turso Cloud SQLite (libSQL) cluster.
// Otherwise, it falls back to local SQLite at dbPath.
func OpenSQLite(dbPath string) (*sql.DB, error) {
	tursoURL := strings.TrimSpace(os.Getenv("TURSO_DATABASE_URL"))
	tursoToken := strings.TrimSpace(os.Getenv("TURSO_AUTH_TOKEN"))

	// 1. Remote Turso Cloud SQLite (libSQL)
	if tursoURL != "" && tursoToken != "" {
		log.Printf("[Database] Connecting to remote Turso Cloud SQLite (libSQL): %s\n", tursoURL)
		connString := fmt.Sprintf("%s?authToken=%s", tursoURL, tursoToken)
		db, err := sql.Open("libsql", connString)
		if err != nil {
			return nil, fmt.Errorf("opening turso libsql connection: %w", err)
		}

		// Pool configuration for remote libSQL client
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxIdleTime(15 * time.Minute)
		db.SetConnMaxLifetime(1 * time.Hour)

		// Verification with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			db.Close()
			return nil, fmt.Errorf("pinging turso cloud database (%s): %w", tursoURL, err)
		}

		log.Printf("[Database] ✓ Successfully verified active connection to Turso Cloud SQLite (%s)\n", tursoURL)
		return db, nil
	}

	// 2. Error handling if only one variable was set
	if tursoURL != "" && tursoToken == "" {
		return nil, fmt.Errorf("TURSO_DATABASE_URL is set but TURSO_AUTH_TOKEN is missing")
	}
	if tursoURL == "" && tursoToken != "" {
		return nil, fmt.Errorf("TURSO_AUTH_TOKEN is set but TURSO_DATABASE_URL is missing")
	}

	// 3. Graceful Fallback to Local SQLite (for local development or unit testing)
	cleanPath := filepath.Clean(dbPath)
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating database directory %s: %w", dir, err)
	}

	log.Printf("[Database] TURSO credentials not set; opening local SQLite at %s\n", cleanPath)
	db, err := sql.Open("sqlite", cleanPath)
	if err != nil {
		return nil, fmt.Errorf("opening local sqlite connection: %w", err)
	}

	if _, err := db.Exec(pragmas); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying sqlite pragmas: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxIdleTime(10 * time.Minute)
	db.SetConnMaxLifetime(1 * time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging local sqlite database: %w", err)
	}

	log.Printf("[Database] ✓ Successfully connected to local SQLite database: %s\n", cleanPath)
	return db, nil
}
