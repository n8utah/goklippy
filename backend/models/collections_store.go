package models

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"goklippy/backend/logic/sorter"
)

// unknownPiece matches pieces with no part: unidentified, failed to identify, or lost
// before they were identified. Pieces still on their way through have no part yet
// either, so they're left out.
const unknownPiece = `part_id = '' AND status NOT IN ('tracking', 'identifying')`

const collectionColumns = `c.id, c.name, c.current, c.bins_started_at, c.archived_at, c.created_at,
  c.profile_id, COALESCE((SELECT sp.name FROM sorting_profiles sp WHERE sp.id = c.profile_id), '') AS profile_name,
  (SELECT COUNT(*) FROM sort_runs r WHERE r.collection_id = c.id) AS runs,
  (SELECT COUNT(*) FROM pieces p JOIN sort_runs r ON r.id = p.run_id WHERE r.collection_id = c.id) AS pieces,
  (SELECT COUNT(*) FROM pieces p JOIN sort_runs r ON r.id = p.run_id WHERE r.collection_id = c.id AND p.part_id != '') AS identified,
  (SELECT COUNT(*) FROM pieces p JOIN sort_runs r ON r.id = p.run_id WHERE r.collection_id = c.id AND ` + unknownPiece + `) AS unknown,
  (SELECT MAX(COALESCE(r.ended_at, r.started_at)) FROM sort_runs r WHERE r.collection_id = c.id) AS last_sorted_at`

type collectionRow struct {
	ID            int64        `db:"id"`
	Name          string       `db:"name"`
	Current       bool         `db:"current"`
	BinsStartedAt sql.NullTime `db:"bins_started_at"`
	ArchivedAt    sql.NullTime `db:"archived_at"`
	CreatedAt     time.Time     `db:"created_at"`
	ProfileID     sql.NullInt64 `db:"profile_id"`
	ProfileName   string        `db:"profile_name"`
	Runs          int          `db:"runs"`
	Pieces        int          `db:"pieces"`
	Identified    int          `db:"identified"`
	Unknown       int          `db:"unknown"`
	LastSortedAt  any          `db:"last_sorted_at"` // a subquery's column: time or text
}

func (r collectionRow) collection() sorter.Collection {
	c := sorter.Collection{
		ID: r.ID, Name: r.Name, Current: r.Current, CreatedAt: r.CreatedAt,
		Runs: r.Runs, Pieces: r.Pieces, Identified: r.Identified, Unknown: r.Unknown,
		ProfileName: r.ProfileName,
	}
	if r.ProfileID.Valid {
		id := r.ProfileID.Int64
		c.ProfileID = &id
	}
	if r.BinsStartedAt.Valid {
		t := r.BinsStartedAt.Time
		c.BinsStartedAt = &t
	}
	if r.ArchivedAt.Valid {
		t := r.ArchivedAt.Time
		c.ArchivedAt = &t
	}
	if t, ok := timeOf(r.LastSortedAt); ok {
		c.LastSortedAt = &t
	}
	return c
}

// ListCollections lists every collection: unarchived first, then newest first.
func (s *RecordsStore) ListCollections(ctx context.Context) ([]sorter.Collection, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	var rows []collectionRow
	query := `SELECT ` + collectionColumns + ` FROM collections c ORDER BY c.archived_at IS NOT NULL, c.created_at DESC, c.id DESC`
	if err := s.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, err
	}
	out := make([]sorter.Collection, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.collection())
	}
	return out, nil
}

func (s *RecordsStore) GetCollection(ctx context.Context, id int64) (sorter.Collection, error) {
	if s.db == nil {
		return sorter.Collection{}, ErrDBNotConfigured
	}
	var row collectionRow
	err := s.db.GetContext(ctx, &row, `SELECT `+collectionColumns+` FROM collections c WHERE c.id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return sorter.Collection{}, sorter.ErrCollectionNotFound
	}
	if err != nil {
		return sorter.Collection{}, err
	}
	return row.collection(), nil
}

// CreateCollection makes a collection sorting by the profile (nil: none yet).
func (s *RecordsStore) CreateCollection(ctx context.Context, name string, profileID *int64, at time.Time) (int64, error) {
	if s.db == nil {
		return 0, ErrDBNotConfigured
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO collections (name, profile_id, created_at) VALUES (?, ?, ?)`, name, profileID, at.UTC())
	if err != nil {
		return 0, nameTaken(err)
	}
	return res.LastInsertId()
}

func (s *RecordsStore) RenameCollection(ctx context.Context, id int64, name string) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	res, err := s.db.ExecContext(ctx, `UPDATE collections SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return nameTaken(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sorter.ErrCollectionNotFound
	}
	return nil
}

func (s *RecordsStore) ArchiveCollection(ctx context.Context, id int64, at *time.Time) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	res, err := s.db.ExecContext(ctx, `UPDATE collections SET archived_at = ? WHERE id = ?`, utcOrNil(at), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sorter.ErrCollectionNotFound
	}
	return nil
}

// CollectionBins counts the collection's delivered pieces per bin since its bins last
// started afresh (the bucket under BinKey{}).
func (s *RecordsStore) CollectionBins(ctx context.Context, id int64) (sorter.BinCounts, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	const query = `
SELECT p.bin_layer AS layer, p.bin_number AS bin, COUNT(*) AS n
FROM pieces p JOIN sort_runs r ON r.id = p.run_id JOIN collections c ON c.id = r.collection_id
WHERE r.collection_id = ? AND p.status = 'distributed'
  AND (c.bins_started_at IS NULL OR p.distributed_at >= c.bins_started_at)
GROUP BY p.bin_layer, p.bin_number
`
	var rows []struct {
		Layer int `db:"layer"`
		Bin   int `db:"bin"`
		N     int `db:"n"`
	}
	if err := s.db.SelectContext(ctx, &rows, query, id); err != nil {
		return nil, err
	}
	counts := sorter.BinCounts{}
	for _, r := range rows {
		counts[sorter.BinKey{Layer: r.Layer, Bin: r.Bin}] += r.N
	}
	return counts, nil
}

// ClearUnknowns deletes the collection's unknown pieces (their photos' rows and reviews
// go with them) and returns their ids.
func (s *RecordsStore) ClearUnknowns(ctx context.Context, id int64) ([]string, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	where := `run_id IN (SELECT id FROM sort_runs WHERE collection_id = ?) AND ` + unknownPiece
	var ids []string
	if err := tx.SelectContext(ctx, &ids, `SELECT id FROM pieces WHERE `+where, id); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM pieces WHERE `+where, id); err != nil {
		return nil, err
	}
	return ids, tx.Commit()
}

func nameTaken(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return sorter.ErrCollectionName
	}
	return err
}
