package models

import (
	"context"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

var ErrDBNotConfigured = errors.New("database is not configured")

func OpenDB(databaseURL string) (*sqlx.DB, error) {
	if databaseURL == "" {
		return nil, nil
	}

	db, err := sqlx.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
