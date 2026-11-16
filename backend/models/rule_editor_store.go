package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"goklippy/backend/logic/sorter"
)

// UpdateProfile replaces a profile's name, source, summary and contents.
func (s *SortingStore) UpdateProfile(ctx context.Context, p sorter.StoredProfile, data []byte) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	summary, err := json.Marshal(p.Summary)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE sorting_profiles SET name = ?, source = ?, summary = ?, data = ? WHERE id = ?`,
		p.Name, p.Source, string(summary), string(data), p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sorter.ErrProfileNotFound
	}
	return nil
}

// ProfileCollections is the collections that sort by a profile.
func (s *SortingStore) ProfileCollections(ctx context.Context, profileID int64) ([]int64, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	var ids []int64
	err := s.db.SelectContext(ctx, &ids, `SELECT id FROM collections WHERE profile_id = ? ORDER BY id`, profileID)
	return ids, err
}

// CatalogPartInfo is a part's name and type from the catalog (any colour); ok is false
// when it hasn't been seen.
func (s *SortingStore) CatalogPartInfo(ctx context.Context, partID string) (name, category string, ok bool, err error) {
	if s.db == nil {
		return "", "", false, ErrDBNotConfigured
	}
	var row struct {
		Name     string `db:"part_name"`
		Category string `db:"category"`
	}
	err = s.db.GetContext(ctx, &row, `SELECT part_name, category FROM catalog_parts
WHERE part_id = ? ORDER BY seen DESC LIMIT 1`, partID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return row.Name, row.Category, true, nil
}

// CatalogPartKinds is every part name and type the catalog has seen, with how many
// pieces of each: what the rule editor's pickers offer.
func (s *SortingStore) CatalogPartKinds(ctx context.Context) ([]sorter.PartKind, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	var rows []sorter.PartKind
	err := s.db.SelectContext(ctx, &rows, `SELECT MAX(part_name) AS part_name, category, SUM(seen) AS seen
FROM catalog_parts WHERE category <> '' GROUP BY part_id, category`)
	return rows, err
}

const rebrickableSettingsName = "rebrickable"

// RebrickableKey is the saved Rebrickable API key, or "".
func (s *SortingStore) RebrickableKey(ctx context.Context) (string, error) {
	if s.db == nil {
		return "", ErrDBNotConfigured
	}
	var data string
	err := s.db.GetContext(ctx, &data, `SELECT data FROM sorter_settings WHERE name = ?`, rebrickableSettingsName)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var out struct {
		APIKey string `json:"api_key"`
	}
	err = json.Unmarshal([]byte(data), &out)
	return out.APIKey, err
}

func (s *SortingStore) SaveRebrickableKey(ctx context.Context, key string) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	data, err := json.Marshal(map[string]string{"api_key": key})
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO sorter_settings (name, data, updated_at) VALUES (?, ?, ?)
ON CONFLICT (name) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		rebrickableSettingsName, string(data), time.Now().UTC())
	return err
}
