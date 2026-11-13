package models

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"goklippy/services/sortviewer/backend/logic"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

const pieceColumns = `id, collection_id, run_id, status, seen_at, sorted_at, part_id, part_name, part_category,
  part_score, part_img_url, color_id, color_name, category, category_name, bin_layer, bin_number, virtual_bin, photo`

type pieceRow struct {
	ID           string        `db:"id"`
	CollectionID int64         `db:"collection_id"`
	RunID        sql.NullInt64 `db:"run_id"`
	Status       string        `db:"status"`
	SeenAt       time.Time     `db:"seen_at"`
	SortedAt     sql.NullTime  `db:"sorted_at"`
	PartID       string        `db:"part_id"`
	PartName     string        `db:"part_name"`
	PartCategory string        `db:"part_category"`
	PartScore    float64       `db:"part_score"`
	PartImgURL   string        `db:"part_img_url"`
	ColorID      string        `db:"color_id"`
	ColorName    string        `db:"color_name"`
	Category     string        `db:"category"`
	CategoryName string        `db:"category_name"`
	BinLayer     int           `db:"bin_layer"`
	BinNumber    int           `db:"bin_number"`
	VirtualBin   bool          `db:"virtual_bin"`
	Photo        string        `db:"photo"`
}

func (r pieceRow) piece() logic.Piece {
	p := logic.Piece{
		ID: r.ID, CollectionID: r.CollectionID, RunID: r.RunID.Int64, Status: r.Status, SeenAt: r.SeenAt,
		PartID: r.PartID, PartName: r.PartName, PartCategory: r.PartCategory, PartScore: r.PartScore,
		PartImgURL: r.PartImgURL, ColorID: r.ColorID, ColorName: r.ColorName,
		Category: r.Category, CategoryName: r.CategoryName,
		BinLayer: r.BinLayer, BinNumber: r.BinNumber, VirtualBin: r.VirtualBin, HasPhoto: r.Photo != "",
	}
	if r.SortedAt.Valid {
		t := r.SortedAt.Time
		p.SortedAt = &t
	}
	return p
}

// UpsertPiece saves a piece as the sorter sent it; its photo, if any, is kept.
func (s *Store) UpsertPiece(ctx context.Context, id string, p logic.PushPiece, at time.Time) error {
	var runID any
	if p.RunID != 0 {
		runID = p.RunID
	}
	const query = `
INSERT INTO pieces (id, collection_id, run_id, status, seen_at, sorted_at, part_id, part_name, part_category,
  part_score, part_img_url, color_id, color_name, category, category_name, bin_layer, bin_number, virtual_bin, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
  collection_id = excluded.collection_id, run_id = excluded.run_id, status = excluded.status,
  seen_at = excluded.seen_at, sorted_at = excluded.sorted_at, part_id = excluded.part_id,
  part_name = excluded.part_name, part_category = excluded.part_category, part_score = excluded.part_score,
  part_img_url = excluded.part_img_url, color_id = excluded.color_id, color_name = excluded.color_name,
  category = excluded.category, category_name = excluded.category_name, bin_layer = excluded.bin_layer,
  bin_number = excluded.bin_number, virtual_bin = excluded.virtual_bin, updated_at = excluded.updated_at
`
	_, err := s.db.ExecContext(ctx, query, id, p.CollectionID, runID, p.Status, p.SeenAt.UTC(), utcOrNil(p.SortedAt),
		p.PartID, p.PartName, p.PartCategory, p.PartScore, p.PartImgURL, p.ColorID, p.ColorName,
		p.Category, p.CategoryName, p.BinLayer, p.BinNumber, p.VirtualBin, at.UTC())
	return err
}

func (s *Store) GetPiece(ctx context.Context, id string) (logic.Piece, string, error) {
	var row pieceRow
	err := s.db.GetContext(ctx, &row, `SELECT `+pieceColumns+` FROM pieces WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return logic.Piece{}, "", logic.ErrNotFound
	}
	if err != nil {
		return logic.Piece{}, "", err
	}
	return row.piece(), row.Photo, nil
}

// SetPiecePhoto records a piece's photo file and returns the one it replaced ("" if none).
func (s *Store) SetPiecePhoto(ctx context.Context, id, photo string) (string, error) {
	var old string
	err := s.db.GetContext(ctx, &old, `SELECT photo FROM pieces WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", logic.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE pieces SET photo = ? WHERE id = ?`, photo, id)
	return old, err
}

// ListPieces lists a collection's pieces, newest first, with how many match.
func (s *Store) ListPieces(ctx context.Context, collectionID int64, f logic.PieceFilter) ([]logic.Piece, int, error) {
	where := []string{"collection_id = ?"}
	args := []any{collectionID}
	if f.Unknown {
		where = append(where, "part_id = ''")
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		like := "%" + q + "%"
		where = append(where, "(part_id LIKE ? OR part_name LIKE ? OR color_name LIKE ? OR category_name LIKE ?)")
		args = append(args, q+"%", like, like, like)
	}
	clause := " WHERE " + strings.Join(where, " AND ")
	var total int
	if err := s.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM pieces"+clause, args...); err != nil {
		return nil, 0, err
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 60
	}
	var rows []pieceRow
	query := "SELECT " + pieceColumns + " FROM pieces" + clause + " ORDER BY seen_at DESC, id LIMIT ? OFFSET ?"
	if err := s.db.SelectContext(ctx, &rows, query, append(args, limit, max(0, f.Offset))...); err != nil {
		return nil, 0, err
	}
	out := make([]logic.Piece, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.piece())
	}
	return out, total, nil
}

// PartCounts is a collection's identified pieces by part and colour, most first.
func (s *Store) PartCounts(ctx context.Context, collectionID int64) ([]logic.PartCount, error) {
	const query = `
SELECT part_id, MAX(part_name) AS part_name, MAX(part_category) AS part_category, color_id,
  MAX(color_name) AS color_name, MAX(part_img_url) AS img_url, COUNT(*) AS count,
  COALESCE(MAX(CASE WHEN photo != '' THEN id END), '') AS photo_piece_id
FROM pieces WHERE collection_id = ? AND part_id != ''
GROUP BY part_id, color_id
ORDER BY count DESC, part_name, color_name`
	var out []logic.PartCount
	if err := s.db.SelectContext(ctx, &out, query, collectionID); err != nil {
		return nil, err
	}
	if out == nil {
		out = []logic.PartCount{}
	}
	return out, nil
}

// BinCounts is how many of a collection's pieces each bin has had since `since`
// (when its bins last started afresh; nil for all). Layer 0 is the bucket.
func (s *Store) BinCounts(ctx context.Context, collectionID int64, since *time.Time) ([]logic.BinCount, error) {
	query := `SELECT bin_layer AS layer, bin_number AS bin, COUNT(*) AS pieces FROM pieces
WHERE collection_id = ? AND sorted_at IS NOT NULL`
	args := []any{collectionID}
	if since != nil {
		query += ` AND sorted_at >= ?`
		args = append(args, since.UTC())
	}
	query += ` GROUP BY bin_layer, bin_number ORDER BY bin_layer, bin_number`
	var out []logic.BinCount
	if err := s.db.SelectContext(ctx, &out, query, args...); err != nil {
		return nil, err
	}
	if out == nil {
		out = []logic.BinCount{}
	}
	return out, nil
}

// ClearUnknowns deletes a collection's unknown pieces and returns their photo files.
func (s *Store) ClearUnknowns(ctx context.Context, collectionID int64) (int, []string, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, nil, err
	}
	defer tx.Rollback()
	var photos []string
	if err := tx.SelectContext(ctx, &photos,
		`SELECT photo FROM pieces WHERE collection_id = ? AND part_id = '' AND photo != ''`, collectionID); err != nil {
		return 0, nil, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM pieces WHERE collection_id = ? AND part_id = ''`, collectionID)
	if err != nil {
		return 0, nil, err
	}
	n, _ := res.RowsAffected()
	return int(n), photos, tx.Commit()
}

// ClearPhotos forgets a collection's photos (every collection's for id 0) and returns
// the files for the caller to remove.
func (s *Store) ClearPhotos(ctx context.Context, collectionID int64) ([]string, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var photos []string
	const where = ` WHERE photo != '' AND (? = 0 OR collection_id = ?)`
	if err := tx.SelectContext(ctx, &photos, `SELECT photo FROM pieces`+where, collectionID, collectionID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pieces SET photo = ''`+where, collectionID, collectionID); err != nil {
		return nil, err
	}
	return photos, tx.Commit()
}
