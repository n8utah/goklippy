package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/jmoiron/sqlx"
)

// SorterSettingsStore keeps the sorter's named JSON settings documents.
type SorterSettingsStore struct {
	db *sqlx.DB
}

func NewSorterSettingsStore(db *sqlx.DB) *SorterSettingsStore {
	return &SorterSettingsStore{db: db}
}

// GetSorterSettings returns the document saved under name, and false if there is none.
func (s *SorterSettingsStore) GetSorterSettings(ctx context.Context, name string) (json.RawMessage, bool, error) {
	if s.db == nil {
		return nil, false, ErrDBNotConfigured
	}
	const query = `SELECT data FROM sorter_settings WHERE name = ?`
	var data []byte
	err := s.db.GetContext(ctx, &data, query, name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return json.RawMessage(data), true, nil
}

// PutSorterSettings saves the document under name, replacing any earlier one.
func (s *SorterSettingsStore) PutSorterSettings(ctx context.Context, name string, data json.RawMessage) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	const query = `
INSERT INTO sorter_settings (name, data, updated_at)
VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (name) DO UPDATE SET data = excluded.data, updated_at = CURRENT_TIMESTAMP
`
	_, err := s.db.ExecContext(ctx, query, name, string(data))
	return err
}
