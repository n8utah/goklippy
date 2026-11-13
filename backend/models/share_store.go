package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"goklippy/backend/logic/sorter"

	"github.com/jmoiron/sqlx"
)

// ShareStore keeps what's waiting to go to the sort viewer site (share_outbox), which
// collections are shared, and the sharing settings.
type ShareStore struct {
	db *sqlx.DB
}

func NewShareStore(db *sqlx.DB) *ShareStore {
	return &ShareStore{db: db}
}

// mark queues a thing for sending if its collection is shared. A thing already waiting
// keeps its place: it's read afresh when sent anyway.
func (s *ShareStore) mark(ctx context.Context, kind, ref, sharedCollection string, args ...any) error {
	query := `INSERT INTO share_outbox (kind, ref, queued_at) SELECT ?, ?, ? WHERE EXISTS (` +
		sharedCollection + `) ON CONFLICT (kind, ref) DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, append([]any{kind, ref, time.Now().UTC()}, args...)...)
	return err
}

func (s *ShareStore) MarkCollection(ctx context.Context, id int64) error {
	return s.mark(ctx, sorter.ShareCollection, itoa(id), `SELECT 1 FROM collections WHERE id = ? AND shared`, id)
}

func (s *ShareStore) MarkRun(ctx context.Context, id int64) error {
	return s.mark(ctx, sorter.ShareRun, itoa(id),
		`SELECT 1 FROM sort_runs r JOIN collections c ON c.id = r.collection_id WHERE r.id = ? AND c.shared`, id)
}

const pieceShared = `SELECT 1 FROM pieces p JOIN sort_runs r ON r.id = p.run_id
  JOIN collections c ON c.id = r.collection_id WHERE p.id = ? AND c.shared`

func (s *ShareStore) MarkPiece(ctx context.Context, id string) error {
	return s.mark(ctx, sorter.SharePiece, id, pieceShared, id)
}

func (s *ShareStore) MarkPhoto(ctx context.Context, id string) error {
	return s.mark(ctx, sorter.SharePhoto, id, pieceShared, id)
}

// MarkClearUnknowns queues clearing a collection's unknowns on the site, and drops the
// marks of pieces that no longer exist (the unknowns just deleted here).
func (s *ShareStore) MarkClearUnknowns(ctx context.Context, collectionID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM share_outbox WHERE kind IN (?, ?) AND ref NOT IN (SELECT id FROM pieces)`,
		sorter.SharePiece, sorter.SharePhoto)
	if err != nil {
		return err
	}
	return s.mark(ctx, sorter.ShareClearUnknowns, itoa(collectionID), `SELECT 1 FROM collections WHERE id = ? AND shared`, collectionID)
}

// SetShared turns sharing a collection on (and queues everything already in it: the
// backfill) or off (and drops what was waiting for it).
func (s *ShareStore) SetShared(ctx context.Context, collectionID int64, shared bool) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE collections SET shared = ? WHERE id = ?`, shared, collectionID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sorter.ErrCollectionNotFound
	}
	now := time.Now().UTC()
	runs := `SELECT id FROM sort_runs WHERE collection_id = ?`
	pieces := `SELECT p.id FROM pieces p JOIN sort_runs r ON r.id = p.run_id WHERE r.collection_id = ?`
	if !shared {
		_, err := tx.ExecContext(ctx, `DELETE FROM share_outbox WHERE
  (kind IN (?, ?) AND ref = ?) OR (kind = ? AND ref IN (`+runs+`)) OR (kind IN (?, ?) AND ref IN (`+pieces+`))`,
			sorter.ShareCollection, sorter.ShareClearUnknowns, itoa(collectionID),
			sorter.ShareRun, collectionID, sorter.SharePiece, sorter.SharePhoto, collectionID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	// The backfill, in sending order: the collection, its runs, its pieces, their photos.
	steps := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO share_outbox (kind, ref, queued_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
			[]any{sorter.ShareCollection, itoa(collectionID), now}},
		{`INSERT INTO share_outbox (kind, ref, queued_at) SELECT ?, CAST(id AS TEXT), ? FROM sort_runs
  WHERE collection_id = ? ORDER BY started_at ON CONFLICT DO NOTHING`, []any{sorter.ShareRun, now, collectionID}},
		{`INSERT INTO share_outbox (kind, ref, queued_at) SELECT ?, p.id, ? FROM pieces p JOIN sort_runs r ON r.id = p.run_id
  WHERE r.collection_id = ? ORDER BY p.seen_at ON CONFLICT DO NOTHING`, []any{sorter.SharePiece, now, collectionID}},
		{`INSERT INTO share_outbox (kind, ref, queued_at) SELECT ?, p.id, ? FROM pieces p JOIN sort_runs r ON r.id = p.run_id
  WHERE r.collection_id = ? AND EXISTS (SELECT 1 FROM piece_images i WHERE i.piece_id = p.id)
  ORDER BY p.seen_at ON CONFLICT DO NOTHING`, []any{sorter.SharePhoto, now, collectionID}},
	}
	for _, st := range steps {
		if _, err := tx.ExecContext(ctx, st.query, st.args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// NextMarks is the oldest marks, in sending order.
func (s *ShareStore) NextMarks(ctx context.Context, limit int) ([]sorter.ShareMark, error) {
	var out []sorter.ShareMark
	err := s.db.SelectContext(ctx, &out,
		`SELECT seq, kind, ref, attempts FROM share_outbox ORDER BY seq LIMIT ?`, limit)
	return out, err
}

func (s *ShareStore) DeleteMark(ctx context.Context, seq int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM share_outbox WHERE seq = ?`, seq)
	return err
}

// FailMark notes a failed send of a mark.
func (s *ShareStore) FailMark(ctx context.Context, seq int64, reason string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE share_outbox SET attempts = attempts + 1, last_error = ? WHERE seq = ?`, reason, seq)
	return err
}

func (s *ShareStore) CountMarks(ctx context.Context) (int, error) {
	var n int
	err := s.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM share_outbox`)
	return n, err
}

// GetRun is one run, for sending.
func (s *ShareStore) GetRun(ctx context.Context, id int64) (sorter.Run, error) {
	var row struct {
		ID           int64        `db:"id"`
		CollectionID int64        `db:"collection_id"`
		ProfileName  string       `db:"profile_name"`
		StartedAt    time.Time    `db:"started_at"`
		EndedAt      sql.NullTime `db:"ended_at"`
	}
	err := s.db.GetContext(ctx, &row,
		`SELECT id, collection_id, profile_name, started_at, ended_at FROM sort_runs WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return sorter.Run{}, sorter.ErrNotShared
	}
	if err != nil {
		return sorter.Run{}, err
	}
	run := sorter.Run{ID: row.ID, CollectionID: row.CollectionID, ProfileName: row.ProfileName, StartedAt: row.StartedAt}
	if row.EndedAt.Valid {
		t := row.EndedAt.Time
		run.EndedAt = &t
	}
	return run, nil
}

// PieceCollection is the collection a piece belongs to.
func (s *ShareStore) PieceCollection(ctx context.Context, pieceID string) (int64, error) {
	var id int64
	err := s.db.GetContext(ctx, &id,
		`SELECT r.collection_id FROM pieces p JOIN sort_runs r ON r.id = p.run_id WHERE p.id = ?`, pieceID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, sorter.ErrNotShared
	}
	return id, err
}

const sharingSettingsName = "sharing"

func (s *ShareStore) SharingSettings(ctx context.Context) (sorter.SharingSettings, error) {
	var data string
	err := s.db.GetContext(ctx, &data, `SELECT data FROM sorter_settings WHERE name = ?`, sharingSettingsName)
	if errors.Is(err, sql.ErrNoRows) {
		return sorter.SharingSettings{}, nil
	}
	if err != nil {
		return sorter.SharingSettings{}, err
	}
	var out sorter.SharingSettings
	err = json.Unmarshal([]byte(data), &out)
	return out, err
}

func (s *ShareStore) SaveSharingSettings(ctx context.Context, set sorter.SharingSettings) error {
	data, err := json.Marshal(set)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO sorter_settings (name, data, updated_at) VALUES (?, ?, ?)
ON CONFLICT (name) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		sharingSettingsName, string(data), time.Now().UTC())
	return err
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
