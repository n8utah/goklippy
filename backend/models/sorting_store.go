package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"goklippy/backend/logic/sorter"

	"github.com/jmoiron/sqlx"
)

// SortingStore keeps sorting profiles and their bin assignments.
type SortingStore struct {
	db *sqlx.DB
}

func NewSortingStore(db *sqlx.DB) *SortingStore {
	return &SortingStore{db: db}
}

type profileRow struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	Source    string    `db:"source"`
	Summary   string    `db:"summary"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"created_at"`
}

func (r profileRow) stored() (sorter.StoredProfile, error) {
	p := sorter.StoredProfile{ID: r.ID, Name: r.Name, Source: r.Source, Active: r.Active, CreatedAt: r.CreatedAt}
	if err := json.Unmarshal([]byte(r.Summary), &p.Summary); err != nil {
		return p, fmt.Errorf("profile %d summary: %w", r.ID, err)
	}
	return p, nil
}

func (s *SortingStore) ListProfiles(ctx context.Context) ([]sorter.StoredProfile, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	const query = `
SELECT id, name, source, summary, active, created_at
FROM sorting_profiles
ORDER BY active DESC, created_at DESC, id DESC
`
	var rows []profileRow
	if err := s.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, err
	}
	out := make([]sorter.StoredProfile, 0, len(rows))
	for _, r := range rows {
		p, err := r.stored()
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *SortingStore) ProfileData(ctx context.Context, id int64) (sorter.StoredProfile, []byte, error) {
	if s.db == nil {
		return sorter.StoredProfile{}, nil, ErrDBNotConfigured
	}
	const query = `
SELECT id, name, source, summary, active, created_at, data
FROM sorting_profiles
WHERE id = ?
`
	var row struct {
		profileRow
		Data string `db:"data"`
	}
	err := s.db.GetContext(ctx, &row, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return sorter.StoredProfile{}, nil, sorter.ErrProfileNotFound
	}
	if err != nil {
		return sorter.StoredProfile{}, nil, err
	}
	p, err := row.profileRow.stored()
	return p, []byte(row.Data), err
}

func (s *SortingStore) CreateProfile(ctx context.Context, p sorter.StoredProfile, data []byte) (int64, error) {
	if s.db == nil {
		return 0, ErrDBNotConfigured
	}
	summary, err := json.Marshal(p.Summary)
	if err != nil {
		return 0, err
	}
	const query = `INSERT INTO sorting_profiles (name, source, summary, data) VALUES (?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, p.Name, p.Source, string(summary), string(data))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SortingStore) DeleteProfile(ctx context.Context, id int64) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM sorting_profiles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sorter.ErrProfileNotFound
	}
	return nil
}

func (s *SortingStore) ActivateProfile(ctx context.Context, id int64) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE sorting_profiles SET active = FALSE WHERE active`); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE sorting_profiles SET active = TRUE WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sorter.ErrProfileNotFound
	}
	return tx.Commit()
}

func (s *SortingStore) ActiveProfileID(ctx context.Context) (int64, bool, error) {
	if s.db == nil {
		return 0, false, ErrDBNotConfigured
	}
	var id int64
	err := s.db.GetContext(ctx, &id, `SELECT id FROM sorting_profiles WHERE active LIMIT 1`)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// CurrentCollection is the collection the bins hold, if one has been picked.
func (s *SortingStore) CurrentCollection(ctx context.Context) (sorter.CollectionRef, bool, error) {
	if s.db == nil {
		return sorter.CollectionRef{}, false, ErrDBNotConfigured
	}
	var c sorter.CollectionRef
	err := s.db.QueryRowxContext(ctx, `SELECT id, name FROM collections WHERE current LIMIT 1`).Scan(&c.ID, &c.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	return c, true, nil
}

// SelectCollection makes the collection current with its bins started afresh at at:
// its bin assignments under every profile are cleared.
func (s *SortingStore) SelectCollection(ctx context.Context, id int64, at time.Time) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE collections SET current = FALSE WHERE current`); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE collections SET current = TRUE, bins_started_at = ? WHERE id = ?`, at.UTC(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sorter.ErrCollectionNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM bin_categories WHERE collection_id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SortingStore) BinCategories(ctx context.Context, collectionID, profileID int64) (sorter.Assignments, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	const query = `
SELECT layer, bin, category_id
FROM bin_categories
WHERE collection_id = ? AND profile_id = ?
ORDER BY layer, bin, position
`
	var rows []struct {
		Layer    int    `db:"layer"`
		Bin      int    `db:"bin"`
		Category string `db:"category_id"`
	}
	if err := s.db.SelectContext(ctx, &rows, query, collectionID, profileID); err != nil {
		return nil, err
	}
	out := sorter.Assignments{}
	for _, r := range rows {
		key := sorter.BinKey{Layer: r.Layer, Bin: r.Bin}
		out[key] = append(out[key], r.Category)
	}
	return out, nil
}

func (s *SortingStore) ReplaceBinCategories(ctx context.Context, collectionID, profileID int64, bins sorter.Assignments) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const clear = `DELETE FROM bin_categories WHERE collection_id = ? AND profile_id = ?`
	if _, err := tx.ExecContext(ctx, clear, collectionID, profileID); err != nil {
		return err
	}
	keys := make([]sorter.BinKey, 0, len(bins))
	for k := range bins {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Layer != keys[j].Layer {
			return keys[i].Layer < keys[j].Layer
		}
		return keys[i].Bin < keys[j].Bin
	})
	for _, k := range keys {
		if err := insertBinCategories(ctx, tx, collectionID, profileID, k, bins[k]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SortingStore) SetBinCategories(ctx context.Context, collectionID, profileID int64, bin sorter.BinKey, categories []string) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const clear = `DELETE FROM bin_categories WHERE collection_id = ? AND profile_id = ? AND layer = ? AND bin = ?`
	if _, err := tx.ExecContext(ctx, clear, collectionID, profileID, bin.Layer, bin.Bin); err != nil {
		return err
	}
	if err := insertBinCategories(ctx, tx, collectionID, profileID, bin, categories); err != nil {
		return err
	}
	return tx.Commit()
}

func insertBinCategories(ctx context.Context, tx *sqlx.Tx, collectionID, profileID int64, bin sorter.BinKey, categories []string) error {
	const insert = `
INSERT INTO bin_categories (collection_id, profile_id, layer, bin, category_id, position)
VALUES (?, ?, ?, ?, ?, ?)
`
	for i, c := range categories {
		if _, err := tx.ExecContext(ctx, insert, collectionID, profileID, bin.Layer, bin.Bin, c, i); err != nil {
			return err
		}
	}
	return nil
}
