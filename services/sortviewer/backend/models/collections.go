package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"goklippy/services/sortviewer/backend/logic"
)

const collectionColumns = `c.id, c.name, c.plan, c.created_at, c.updated_at,
  (SELECT COUNT(*) FROM runs r WHERE r.collection_id = c.id) AS runs,
  (SELECT COUNT(*) FROM pieces p WHERE p.collection_id = c.id) AS pieces,
  (SELECT COUNT(*) FROM pieces p WHERE p.collection_id = c.id AND p.part_id != '') AS identified,
  (SELECT COUNT(*) FROM pieces p WHERE p.collection_id = c.id AND p.part_id = '') AS unknown,
  EXISTS (SELECT 1 FROM runs r WHERE r.collection_id = c.id AND r.ended_at IS NULL) AS sorting,
  (SELECT MAX(p.seen_at) FROM pieces p WHERE p.collection_id = c.id) AS last_piece_at`

type collectionRow struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	Plan        string    `db:"plan"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	Runs        int       `db:"runs"`
	Pieces      int       `db:"pieces"`
	Identified  int       `db:"identified"`
	Unknown     int       `db:"unknown"`
	Sorting     bool      `db:"sorting"`
	LastPieceAt any       `db:"last_piece_at"`
}

func (r collectionRow) collection() logic.Collection {
	c := logic.Collection{
		ID: r.ID, Name: r.Name, Plan: json.RawMessage(r.Plan), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Runs: r.Runs, Pieces: r.Pieces, Identified: r.Identified, Unknown: r.Unknown, Sorting: r.Sorting,
	}
	if t, ok := timeOf(r.LastPieceAt); ok {
		c.LastPieceAt = &t
	}
	return c
}

// ListCollections lists every collection (viewerID 0) or the ones a viewer may see,
// newest activity first.
func (s *Store) ListCollections(ctx context.Context, viewerID int64) ([]logic.Collection, error) {
	query := `SELECT ` + collectionColumns + ` FROM collections c
WHERE ? = 0 OR c.id IN (SELECT collection_id FROM viewer_collections WHERE viewer_id = ?)
ORDER BY c.updated_at DESC, c.id DESC`
	var rows []collectionRow
	if err := s.db.SelectContext(ctx, &rows, query, viewerID, viewerID); err != nil {
		return nil, err
	}
	out := make([]logic.Collection, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.collection())
	}
	return out, nil
}

func (s *Store) GetCollection(ctx context.Context, id int64) (logic.Collection, error) {
	var row collectionRow
	err := s.db.GetContext(ctx, &row, `SELECT `+collectionColumns+` FROM collections c WHERE c.id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return logic.Collection{}, logic.ErrNotFound
	}
	if err != nil {
		return logic.Collection{}, err
	}
	return row.collection(), nil
}

// CanSee reports whether a viewer has been given the collection.
func (s *Store) CanSee(ctx context.Context, viewerID, collectionID int64) (bool, error) {
	var n int
	err := s.db.GetContext(ctx, &n,
		`SELECT COUNT(*) FROM viewer_collections WHERE viewer_id = ? AND collection_id = ?`, viewerID, collectionID)
	return n > 0, err
}

// UpsertCollection creates or renames a collection; a plan, when given, replaces its plan.
func (s *Store) UpsertCollection(ctx context.Context, id int64, name string, plan json.RawMessage, at time.Time) error {
	planText := "{}"
	if len(plan) > 0 {
		planText = string(plan)
	}
	const query = `
INSERT INTO collections (id, name, plan, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
  name = excluded.name,
  plan = CASE WHEN ? THEN excluded.plan ELSE collections.plan END,
  updated_at = excluded.updated_at
`
	_, err := s.db.ExecContext(ctx, query, id, name, planText, at.UTC(), at.UTC(), len(plan) > 0)
	return err
}

// EnsureCollection makes a placeholder for a collection pushed to before it was named
// (a run or piece arriving first); the collection push names it later.
func (s *Store) EnsureCollection(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO collections (id, name, created_at, updated_at) VALUES (?, ?, ?, ?) ON CONFLICT (id) DO NOTHING`,
		id, "Collection "+itoa(id), at.UTC(), at.UTC())
	return err
}

// TouchCollection marks a collection as just updated, so lists put it first.
func (s *Store) TouchCollection(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE collections SET updated_at = ? WHERE id = ?`, at.UTC(), id)
	return err
}

// DeleteCollection removes a collection with its runs and pieces, and returns its
// pieces' photo files for the caller to remove.
func (s *Store) DeleteCollection(ctx context.Context, id int64) ([]string, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var photos []string
	if err := tx.SelectContext(ctx, &photos, `SELECT photo FROM pieces WHERE collection_id = ? AND photo != ''`, id); err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM collections WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, logic.ErrNotFound
	}
	return photos, tx.Commit()
}

func (s *Store) UpsertRun(ctx context.Context, id int64, r logic.PushRun) error {
	const query = `
INSERT INTO runs (id, collection_id, profile_name, started_at, ended_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
  collection_id = excluded.collection_id, profile_name = excluded.profile_name,
  started_at = excluded.started_at, ended_at = excluded.ended_at
`
	_, err := s.db.ExecContext(ctx, query, id, r.CollectionID, r.ProfileName, r.StartedAt.UTC(), utcOrNil(r.EndedAt))
	return err
}

// ListRuns is a collection's runs, newest first.
func (s *Store) ListRuns(ctx context.Context, collectionID int64) ([]logic.Run, error) {
	var rows []struct {
		ID          int64        `db:"id"`
		ProfileName string       `db:"profile_name"`
		StartedAt   time.Time    `db:"started_at"`
		EndedAt     sql.NullTime `db:"ended_at"`
		Pieces      int          `db:"pieces"`
	}
	const query = `
SELECT r.id, r.profile_name, r.started_at, r.ended_at,
  (SELECT COUNT(*) FROM pieces p WHERE p.run_id = r.id) AS pieces
FROM runs r WHERE r.collection_id = ? ORDER BY r.started_at DESC, r.id DESC`
	if err := s.db.SelectContext(ctx, &rows, query, collectionID); err != nil {
		return nil, err
	}
	out := make([]logic.Run, 0, len(rows))
	for _, r := range rows {
		run := logic.Run{ID: r.ID, CollectionID: collectionID, ProfileName: r.ProfileName, StartedAt: r.StartedAt, Pieces: r.Pieces}
		if r.EndedAt.Valid {
			t := r.EndedAt.Time
			run.EndedAt = &t
		}
		out = append(out, run)
	}
	return out, nil
}
