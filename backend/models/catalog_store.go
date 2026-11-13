package models

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"goklippy/backend/logic/sorter"

	"github.com/jmoiron/sqlx"
)

// The catalog of every part and colour identified, across runs (sorter.CatalogStore).

// NoteCatalog counts one identified piece, once: a piece already counted is skipped.
func (s *RecordsStore) NoteCatalog(ctx context.Context, sg sorter.CatalogSighting) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := noteCatalog(ctx, tx, sg); err != nil {
		return err
	}
	return tx.Commit()
}

// noteCatalog counts the piece inside tx; false when it was already counted.
func noteCatalog(ctx context.Context, tx *sqlx.Tx, sg sorter.CatalogSighting) (bool, error) {
	res, err := tx.ExecContext(ctx, `INSERT INTO catalog_counted (piece_id) VALUES (?) ON CONFLICT (piece_id) DO NOTHING`, sg.PieceID)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	at := sg.At.UTC()
	const part = `
INSERT INTO catalog_parts (part_id, color_id, part_name, color_name, category, img_url, seen, best_score, first_seen, last_seen)
VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?)
ON CONFLICT (part_id, color_id) DO UPDATE SET
  seen = seen + 1,
  part_name = CASE WHEN excluded.part_name <> '' THEN excluded.part_name ELSE part_name END,
  color_name = CASE WHEN excluded.color_name <> '' THEN excluded.color_name ELSE color_name END,
  category = CASE WHEN excluded.category <> '' THEN excluded.category ELSE category END,
  img_url = CASE WHEN excluded.img_url <> '' THEN excluded.img_url ELSE img_url END,
  best_score = MAX(best_score, excluded.best_score),
  first_seen = MIN(first_seen, excluded.first_seen),
  last_seen = MAX(last_seen, excluded.last_seen)
`
	if _, err := tx.ExecContext(ctx, part, sg.PartID, sg.ColorID, sg.PartName, sg.ColorName, sg.Category, sg.ImgURL,
		sg.Score, at, at); err != nil {
		return false, err
	}
	if sg.RunID != 0 {
		const run = `
INSERT INTO catalog_runs (part_id, color_id, run_id, seen) VALUES (?, ?, ?, 1)
ON CONFLICT (part_id, color_id, run_id) DO UPDATE SET seen = seen + 1
`
		if _, err := tx.ExecContext(ctx, run, sg.PartID, sg.ColorID, sg.RunID); err != nil {
			return false, err
		}
	}
	return true, nil
}

var catalogSorts = map[string]string{
	"seen":  "seen %s, part_name",
	"name":  "part_name %s, color_name",
	"part":  "part_id %s, color_name",
	"color": "color_name %s, part_name",
	"last":  "last_seen %s, part_name",
}

type catalogRow struct {
	PartID    string         `db:"part_id"`
	ColorID   string         `db:"color_id"`
	PartName  string         `db:"part_name"`
	ColorName string         `db:"color_name"`
	Category  string         `db:"category"`
	ImgURL    string         `db:"img_url"`
	Seen      int            `db:"seen"`
	BestScore float64        `db:"best_score"`
	FirstSeen time.Time      `db:"first_seen"`
	LastSeen  time.Time      `db:"last_seen"`
	Photo     sql.NullString `db:"photo"` // "pieceID/idx"
}

func (s *RecordsStore) ListCatalog(ctx context.Context, f sorter.CatalogFilter) ([]sorter.CatalogEntry, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	var where []string
	var args []any
	if f.PartID != "" {
		where = append(where, "part_id = ?")
		args = append(args, f.PartID)
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		like := "%" + q + "%"
		where = append(where, "(part_id LIKE ? OR part_name LIKE ? OR color_name LIKE ? OR category LIKE ?)")
		args = append(args, like, like, like, like)
	}
	if f.ColorID != "" {
		where = append(where, "color_id = ?")
		args = append(args, f.ColorID)
	}
	if f.Category != "" {
		where = append(where, "category = ?")
		args = append(args, f.Category)
	}
	if f.RunID != 0 {
		where = append(where, "EXISTS (SELECT 1 FROM catalog_runs r WHERE r.part_id = catalog_parts.part_id AND r.color_id = catalog_parts.color_id AND r.run_id = ?)")
		args = append(args, f.RunID)
	}
	if f.CollectionID != 0 {
		where = append(where, `EXISTS (SELECT 1 FROM catalog_runs r JOIN sort_runs s ON s.id = r.run_id
  WHERE r.part_id = catalog_parts.part_id AND r.color_id = catalog_parts.color_id AND s.collection_id = ?)`)
		args = append(args, f.CollectionID)
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	order, ok := catalogSorts[f.Sort]
	if !ok {
		order = catalogSorts["seen"]
	}
	dir := "ASC"
	if f.Desc {
		dir = "DESC"
	}
	// The camera photo: the latest piece of the part and colour with photos still kept,
	// one sent to Brickognize if it has one.
	query := `SELECT part_id, color_id, part_name, color_name, category, img_url, seen, best_score, first_seen, last_seen,
  (SELECT pi.piece_id || '/' || pi.idx FROM pieces p JOIN piece_images pi ON pi.piece_id = p.id
   WHERE p.part_id = catalog_parts.part_id AND p.color_id = catalog_parts.color_id
   ORDER BY p.seen_at DESC, pi.used DESC, pi.idx LIMIT 1) AS photo
FROM catalog_parts ` + clause + " ORDER BY " + strings.Replace(order, "%s", dir, 1)
	var rows []catalogRow
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	runs, err := s.catalogRuns(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]sorter.CatalogEntry, 0, len(rows))
	for _, r := range rows {
		e := sorter.CatalogEntry{
			PartID: r.PartID, ColorID: r.ColorID, PartName: r.PartName, ColorName: r.ColorName,
			Category: r.Category, ImgURL: r.ImgURL, Seen: r.Seen, BestScore: r.BestScore,
			FirstSeen: r.FirstSeen, LastSeen: r.LastSeen, Runs: runs[r.PartID+"\x00"+r.ColorID],
		}
		if e.Runs == nil {
			e.Runs = []sorter.CatalogRun{}
		}
		if id, idx, ok := strings.Cut(r.Photo.String, "/"); ok {
			e.PhotoPieceID = id
			e.PhotoIndex, _ = strconv.Atoi(idx)
		}
		out = append(out, e)
	}
	return out, nil
}

// catalogRuns is each part and colour's runs, newest first.
func (s *RecordsStore) catalogRuns(ctx context.Context) (map[string][]sorter.CatalogRun, error) {
	var rows []struct {
		PartID       string        `db:"part_id"`
		ColorID      string        `db:"color_id"`
		RunID        int64         `db:"run_id"`
		CollectionID sql.NullInt64 `db:"collection_id"`
		Seen         int           `db:"seen"`
	}
	const query = `SELECT c.part_id, c.color_id, c.run_id, r.collection_id, c.seen
FROM catalog_runs c LEFT JOIN sort_runs r ON r.id = c.run_id ORDER BY c.run_id DESC`
	if err := s.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, err
	}
	out := map[string][]sorter.CatalogRun{}
	for _, r := range rows {
		k := r.PartID + "\x00" + r.ColorID
		out[k] = append(out[k], sorter.CatalogRun{RunID: r.RunID, CollectionID: r.CollectionID.Int64, Seen: r.Seen})
	}
	return out, nil
}

func (s *RecordsStore) CatalogFacets(ctx context.Context) (sorter.CatalogFacets, error) {
	out := sorter.CatalogFacets{Colors: []sorter.CatalogFacet{}, Categories: []sorter.CatalogFacet{}, Runs: []sorter.CatalogFacet{}}
	if s.db == nil {
		return out, ErrDBNotConfigured
	}
	type facet struct {
		ID    string `db:"id"`
		Name  string `db:"name"`
		Count int    `db:"n"`
	}
	load := func(dst *[]sorter.CatalogFacet, query string) error {
		var rows []facet
		if err := s.db.SelectContext(ctx, &rows, query); err != nil {
			return err
		}
		for _, r := range rows {
			*dst = append(*dst, sorter.CatalogFacet{ID: r.ID, Name: r.Name, Count: r.Count})
		}
		return nil
	}
	if err := load(&out.Colors, `SELECT color_id AS id, MAX(color_name) AS name, SUM(seen) AS n
FROM catalog_parts GROUP BY color_id ORDER BY n DESC, name`); err != nil {
		return out, err
	}
	if err := load(&out.Categories, `SELECT '' AS id, category AS name, SUM(seen) AS n
FROM catalog_parts WHERE category <> '' GROUP BY category ORDER BY n DESC, name`); err != nil {
		return out, err
	}
	var runs []struct {
		RunID     int64        `db:"run_id"`
		StartedAt sql.NullTime `db:"started_at"`
		N         int          `db:"n"`
	}
	if err := s.db.SelectContext(ctx, &runs, `SELECT c.run_id, r.started_at, SUM(c.seen) AS n
FROM catalog_runs c LEFT JOIN sort_runs r ON r.id = c.run_id
GROUP BY c.run_id ORDER BY c.run_id DESC`); err != nil {
		return out, err
	}
	for _, r := range runs {
		f := sorter.CatalogFacet{ID: strconv.FormatInt(r.RunID, 10), Name: "Run " + strconv.FormatInt(r.RunID, 10), Count: r.N}
		if r.StartedAt.Valid {
			t := r.StartedAt.Time
			f.At = &t
		}
		out.Runs = append(out.Runs, f)
	}
	if err := s.db.GetContext(ctx, &out.Parts, `SELECT COUNT(*) FROM catalog_parts`); err != nil {
		return out, err
	}
	if err := s.db.GetContext(ctx, &out.Pieces, `SELECT COALESCE(SUM(seen), 0) FROM catalog_parts`); err != nil {
		return out, err
	}
	return out, nil
}

const partPieceLimit = 200

// CatalogPart is one part and colour with every kept photo of it and its pieces.
func (s *RecordsStore) CatalogPart(ctx context.Context, partID, colorID string) (sorter.CatalogPart, error) {
	if s.db == nil {
		return sorter.CatalogPart{}, ErrDBNotConfigured
	}
	entries, err := s.ListCatalog(ctx, sorter.CatalogFilter{PartID: partID})
	if err != nil {
		return sorter.CatalogPart{}, err
	}
	out := sorter.CatalogPart{Photos: []sorter.CatalogPhoto{}, Pieces: []sorter.Piece{}}
	found := false
	for _, e := range entries {
		if e.ColorID == colorID {
			out.CatalogEntry, found = e, true
		}
	}
	if !found {
		return out, sorter.ErrPartNotFound
	}
	var photos []struct {
		PieceID string        `db:"piece_id"`
		Idx     int           `db:"idx"`
		Used    bool          `db:"used"`
		RunID   sql.NullInt64 `db:"run_id"`
		TakenAt time.Time     `db:"taken_at"`
	}
	const photoQuery = `
SELECT pi.piece_id, pi.idx, pi.used, p.run_id, pi.taken_at
FROM pieces p JOIN piece_images pi ON pi.piece_id = p.id
WHERE p.part_id = ? AND p.color_id = ?
ORDER BY p.seen_at DESC, pi.used DESC, pi.idx
LIMIT 500`
	if err := s.db.SelectContext(ctx, &photos, photoQuery, partID, colorID); err != nil {
		return out, err
	}
	for _, p := range photos {
		out.Photos = append(out.Photos, sorter.CatalogPhoto{
			PieceID: p.PieceID, Index: p.Idx, Used: p.Used, RunID: p.RunID.Int64, TakenAt: p.TakenAt,
		})
	}
	var rows []pieceRow
	query := "SELECT " + pieceColumns + ` FROM pieces WHERE part_id = ? AND color_id = ?
ORDER BY seen_at DESC LIMIT ?`
	if err := s.db.SelectContext(ctx, &rows, query, partID, colorID, partPieceLimit); err != nil {
		return out, err
	}
	for _, r := range rows {
		p := r.piece()
		p.Prediction = nil // the list doesn't need Brickognize's whole answer
		out.Pieces = append(out.Pieces, p)
	}
	return out, nil
}

// ClearPieceImages forgets every piece's photos (the files are the caller's to remove)
// and returns how many there were.
func (s *RecordsStore) ClearPieceImages(ctx context.Context) (int, error) {
	if s.db == nil {
		return 0, ErrDBNotConfigured
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM piece_images`)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// ClearCatalog empties the catalog. The piece records stay; filling from them again
// counts them again.
func (s *RecordsStore) ClearCatalog(ctx context.Context) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, table := range []string{"catalog_parts", "catalog_runs", "catalog_counted"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// FillCatalog counts every identified piece in the records not counted yet.
func (s *RecordsStore) FillCatalog(ctx context.Context) (int, error) {
	if s.db == nil {
		return 0, ErrDBNotConfigured
	}
	var rows []pieceRow
	query := "SELECT " + pieceColumns + ` FROM pieces
WHERE part_id <> '' AND id NOT IN (SELECT piece_id FROM catalog_counted) ORDER BY seen_at`
	if err := s.db.SelectContext(ctx, &rows, query); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	added := 0
	for _, r := range rows {
		sg, ok := sorter.SightingOf(r.piece())
		if !ok {
			continue
		}
		counted, err := noteCatalog(ctx, tx, sg)
		if err != nil {
			return 0, err
		}
		if counted {
			added++
		}
	}
	return added, tx.Commit()
}
