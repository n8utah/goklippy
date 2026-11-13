// Package models is every database query of the sort viewer.
package models

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"

	"goklippy/services/sortviewer/database"
)

// Store is the sort viewer's database.
type Store struct {
	db *sqlx.DB
}

// Open opens (creating if needed) the database at path and applies the schema.
// ":memory:" gives a throwaway database for tests.
func Open(path string) (*Store, error) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, database.Schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// utcOrNil is a time for a nullable column.
func utcOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

// timeOf reads a DATETIME a subquery or aggregate returned: the driver gives a time
// when it knows the column's type, else the stored text.
func timeOf(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		for _, layout := range []string{"2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999", time.RFC3339Nano} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed, true
			}
		}
	}
	return time.Time{}, false
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
