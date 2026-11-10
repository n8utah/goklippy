package models

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

var ErrDBNotConfigured = errors.New("database is not configured")

// OpenDB opens (creating if needed) the SQLite database at path. Use ":memory:" for
// a throwaway database in tests.
func OpenDB(path string) (*sqlx.DB, error) {
	if path == "" {
		return nil, ErrDBNotConfigured
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create database folder: %w", err)
		}
	}

	// WAL lets readers run during a write; busy_timeout waits out a lock instead of
	// failing; times are written in a format SQLite's date functions understand.
	params := url.Values{}
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "busy_timeout(5000)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Set("_time_format", "sqlite")
	db, err := sqlx.Open("sqlite", "file:"+path+"?"+params.Encode())
	if err != nil {
		return nil, err
	}
	// One connection: SQLite has a single writer anyway, and an in-memory database
	// exists per connection.
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ApplySchemaFile runs the schema file (every statement IF NOT EXISTS), so a new
// database gets its tables and an existing one gains any new tables.
func ApplySchemaFile(ctx context.Context, db *sqlx.DB, path string) error {
	schema, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("apply schema %s: %w", path, err)
	}
	return nil
}
