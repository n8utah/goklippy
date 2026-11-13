package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"goklippy/backend/logic/sorter"

	"github.com/jmoiron/sqlx"
)

// RecordsStore keeps pieces, sort runs and incidents.
type RecordsStore struct {
	db *sqlx.DB
}

func NewRecordsStore(db *sqlx.DB) *RecordsStore {
	return &RecordsStore{db: db}
}

func (s *RecordsStore) StartRun(ctx context.Context, profileID *int64, profileName string, at time.Time) (int64, error) {
	if s.db == nil {
		return 0, ErrDBNotConfigured
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO sort_runs (profile_id, profile_name, started_at) VALUES (?, ?, ?)`,
		profileID, profileName, at.UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *RecordsStore) EndRun(ctx context.Context, id int64, at time.Time) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	_, err := s.db.ExecContext(ctx, `UPDATE sort_runs SET ended_at = ? WHERE id = ? AND ended_at IS NULL`, at.UTC(), id)
	return err
}

func (s *RecordsStore) ListRuns(ctx context.Context, limit int) ([]sorter.Run, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	const query = `
SELECT r.id, r.profile_id, r.profile_name, r.started_at, r.ended_at,
  (SELECT COUNT(*) FROM pieces p WHERE p.run_id = r.id) AS pieces,
  (SELECT COUNT(*) FROM pieces p WHERE p.run_id = r.id AND p.status = 'distributed') AS distributed
FROM sort_runs r
ORDER BY r.started_at DESC, r.id DESC
LIMIT ?
`
	var rows []struct {
		ID          int64         `db:"id"`
		ProfileID   sql.NullInt64 `db:"profile_id"`
		ProfileName string        `db:"profile_name"`
		StartedAt   time.Time     `db:"started_at"`
		EndedAt     sql.NullTime  `db:"ended_at"`
		Pieces      int           `db:"pieces"`
		Distributed int           `db:"distributed"`
	}
	if err := s.db.SelectContext(ctx, &rows, query, clampLimit(limit)); err != nil {
		return nil, err
	}
	out := make([]sorter.Run, 0, len(rows))
	for _, r := range rows {
		run := sorter.Run{ID: r.ID, ProfileName: r.ProfileName, StartedAt: r.StartedAt, Pieces: r.Pieces, Distributed: r.Distributed}
		if r.ProfileID.Valid {
			id := r.ProfileID.Int64
			run.ProfileID = &id
		}
		if r.EndedAt.Valid {
			t := r.EndedAt.Time
			run.EndedAt = &t
		}
		out = append(out, run)
	}
	return out, nil
}

type pieceRow struct {
	ID            string         `db:"id"`
	RunID         sql.NullInt64  `db:"run_id"`
	Status        string         `db:"status"`
	SeenAt        time.Time      `db:"seen_at"`
	IdentifiedAt  sql.NullTime   `db:"identified_at"`
	DistributedAt sql.NullTime   `db:"distributed_at"`
	PartID        string         `db:"part_id"`
	PartName      string         `db:"part_name"`
	PartCategory  string         `db:"part_category"`
	PartScore     float64        `db:"part_score"`
	ColorID       string         `db:"color_id"`
	ColorName     string         `db:"color_name"`
	ColorScore    float64        `db:"color_score"`
	Category      string         `db:"category"`
	CategoryName  string         `db:"category_name"`
	BinLayer      int            `db:"bin_layer"`
	BinNumber     int            `db:"bin_number"`
	RouteReason   string         `db:"route_reason"`
	PieceMm       float64        `db:"piece_mm"`
	Problems      string         `db:"problems"`
	Prediction    sql.NullString `db:"prediction"`
}

const pieceColumns = `id, run_id, status, seen_at, identified_at, distributed_at, part_id, part_name,
  part_category, part_score, color_id, color_name, color_score, category, category_name,
  bin_layer, bin_number, route_reason, piece_mm, problems, prediction`

func (r pieceRow) piece() sorter.Piece {
	p := sorter.Piece{
		ID: r.ID, Status: sorter.PieceStatus(r.Status), SeenAt: r.SeenAt,
		PartID: r.PartID, PartName: r.PartName, PartCategory: r.PartCategory, PartScore: r.PartScore,
		ColorID: r.ColorID, ColorName: r.ColorName, ColorScore: r.ColorScore,
		Category: r.Category, CategoryName: r.CategoryName,
		BinLayer: r.BinLayer, BinNumber: r.BinNumber, RouteReason: r.RouteReason, PieceMm: r.PieceMm,
	}
	if r.RunID.Valid {
		p.RunID = r.RunID.Int64
	}
	if r.IdentifiedAt.Valid {
		t := r.IdentifiedAt.Time
		p.IdentifiedAt = &t
	}
	if r.DistributedAt.Valid {
		t := r.DistributedAt.Time
		p.DistributedAt = &t
	}
	_ = json.Unmarshal([]byte(r.Problems), &p.Problems)
	if p.Problems == nil {
		p.Problems = []sorter.Problem{}
	}
	if r.Prediction.Valid && r.Prediction.String != "" {
		p.Prediction = json.RawMessage(r.Prediction.String)
		p.PartImgURL = sorter.PartImage(p.Prediction, p.PartID)
	}
	return p
}

func (s *RecordsStore) SavePiece(ctx context.Context, p sorter.Piece) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	problems, err := json.Marshal(p.Problems)
	if err != nil {
		return err
	}
	if p.Problems == nil {
		problems = []byte("[]")
	}
	var prediction any
	if len(p.Prediction) > 0 {
		prediction = string(p.Prediction)
	}
	var runID any
	if p.RunID != 0 {
		runID = p.RunID
	}
	const query = `
INSERT INTO pieces (` + pieceColumns + `)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
  run_id = excluded.run_id, status = excluded.status, identified_at = excluded.identified_at,
  distributed_at = excluded.distributed_at, part_id = excluded.part_id, part_name = excluded.part_name,
  part_category = excluded.part_category, part_score = excluded.part_score, color_id = excluded.color_id,
  color_name = excluded.color_name, color_score = excluded.color_score, category = excluded.category,
  category_name = excluded.category_name, bin_layer = excluded.bin_layer, bin_number = excluded.bin_number,
  route_reason = excluded.route_reason, piece_mm = excluded.piece_mm, problems = excluded.problems,
  prediction = excluded.prediction
`
	_, err = s.db.ExecContext(ctx, query,
		p.ID, runID, string(p.Status), p.SeenAt.UTC(), utcOrNil(p.IdentifiedAt), utcOrNil(p.DistributedAt),
		p.PartID, p.PartName, p.PartCategory, p.PartScore, p.ColorID, p.ColorName, p.ColorScore,
		p.Category, p.CategoryName, p.BinLayer, p.BinNumber, p.RouteReason, p.PieceMm, string(problems), prediction)
	return err
}

func (s *RecordsStore) AddPieceImage(ctx context.Context, pieceID string, img sorter.PieceImage) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	const query = `
INSERT INTO piece_images (piece_id, idx, path, used, taken_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (piece_id, idx) DO UPDATE SET path = excluded.path, used = excluded.used, taken_at = excluded.taken_at
`
	_, err := s.db.ExecContext(ctx, query, pieceID, img.Index, img.Path, img.Used, img.TakenAt.UTC())
	return err
}

func (s *RecordsStore) ListPieces(ctx context.Context, f sorter.PieceFilter) ([]sorter.Piece, int, error) {
	if s.db == nil {
		return nil, 0, ErrDBNotConfigured
	}
	var where []string
	var args []any
	if f.RunID != 0 {
		where = append(where, "run_id = ?")
		args = append(args, f.RunID)
	}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.ColorID != "" {
		where = append(where, "color_id = ?")
		args = append(args, f.ColorID)
	}
	if f.PartCategory != "" {
		where = append(where, "part_category = ?")
		args = append(args, f.PartCategory)
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		where = append(where, "(part_id LIKE ? OR id LIKE ? OR category_name LIKE ? OR part_name LIKE ? OR color_name LIKE ?)")
		like := "%" + q + "%"
		args = append(args, q+"%", q+"%", like, like, like)
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := s.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM pieces "+clause, args...); err != nil {
		return nil, 0, err
	}
	var rows []pieceRow
	query := "SELECT " + pieceColumns + " FROM pieces " + clause + " ORDER BY seen_at DESC LIMIT ? OFFSET ?"
	if err := s.db.SelectContext(ctx, &rows, query, append(args, clampLimit(f.Limit), max(0, f.Offset))...); err != nil {
		return nil, 0, err
	}
	out := make([]sorter.Piece, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.piece())
	}
	if err := s.attachFirstImages(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// attachFirstImages gives each listed piece its first photo, for thumbnails.
func (s *RecordsStore) attachFirstImages(ctx context.Context, pieces []sorter.Piece) error {
	if len(pieces) == 0 {
		return nil
	}
	ids := make([]any, len(pieces))
	index := make(map[string]int, len(pieces))
	for i, p := range pieces {
		ids[i] = p.ID
		index[p.ID] = i
	}
	query, args, err := sqlx.In(`
SELECT piece_id, idx, path, used, taken_at FROM piece_images
WHERE piece_id IN (?) ORDER BY piece_id, used DESC, idx`, ids)
	if err != nil {
		return err
	}
	var rows []imageRow
	if err := s.db.SelectContext(ctx, &rows, s.db.Rebind(query), args...); err != nil {
		return err
	}
	for _, r := range rows {
		p := &pieces[index[r.PieceID]]
		if len(p.Images) == 0 {
			p.Images = append(p.Images, r.image())
		}
	}
	return nil
}

type imageRow struct {
	PieceID string    `db:"piece_id"`
	Idx     int       `db:"idx"`
	Path    string    `db:"path"`
	Used    bool      `db:"used"`
	TakenAt time.Time `db:"taken_at"`
}

func (r imageRow) image() sorter.PieceImage {
	return sorter.PieceImage{Index: r.Idx, Path: r.Path, Used: r.Used, TakenAt: r.TakenAt}
}

func (s *RecordsStore) GetPiece(ctx context.Context, id string) (sorter.Piece, error) {
	if s.db == nil {
		return sorter.Piece{}, ErrDBNotConfigured
	}
	var row pieceRow
	err := s.db.GetContext(ctx, &row, "SELECT "+pieceColumns+" FROM pieces WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return sorter.Piece{}, sorter.ErrPieceNotFound
	}
	if err != nil {
		return sorter.Piece{}, err
	}
	p := row.piece()
	var imgs []imageRow
	if err := s.db.SelectContext(ctx, &imgs, `SELECT piece_id, idx, path, used, taken_at FROM piece_images WHERE piece_id = ? ORDER BY idx`, id); err != nil {
		return sorter.Piece{}, err
	}
	for _, r := range imgs {
		p.Images = append(p.Images, r.image())
	}
	if p.Review, err = s.review(ctx, id); err != nil {
		return sorter.Piece{}, err
	}
	return p, nil
}

func (s *RecordsStore) OpenIncident(ctx context.Context, inc sorter.Incident) (int64, error) {
	if s.db == nil {
		return 0, ErrDBNotConfigured
	}
	var pieceID any
	if inc.PieceID != "" {
		pieceID = inc.PieceID
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO incidents (kind, title, message, piece_id, opened_at) VALUES (?, ?, ?, ?, ?)`,
		inc.Kind, inc.Title, inc.Message, pieceID, inc.OpenedAt.UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *RecordsStore) ResolveIncident(ctx context.Context, id int64, resolution string, at time.Time) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	_, err := s.db.ExecContext(ctx, `UPDATE incidents SET resolved_at = ?, resolution = ? WHERE id = ? AND resolved_at IS NULL`,
		at.UTC(), resolution, id)
	return err
}

func (s *RecordsStore) ListIncidents(ctx context.Context, limit int) ([]sorter.Incident, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	var rows []struct {
		ID         int64          `db:"id"`
		Kind       string         `db:"kind"`
		Title      string         `db:"title"`
		Message    string         `db:"message"`
		PieceID    sql.NullString `db:"piece_id"`
		OpenedAt   time.Time      `db:"opened_at"`
		ResolvedAt sql.NullTime   `db:"resolved_at"`
		Resolution string         `db:"resolution"`
	}
	const query = `SELECT id, kind, title, message, piece_id, opened_at, resolved_at, resolution
FROM incidents ORDER BY opened_at DESC, id DESC LIMIT ?`
	if err := s.db.SelectContext(ctx, &rows, query, clampLimit(limit)); err != nil {
		return nil, err
	}
	out := make([]sorter.Incident, 0, len(rows))
	for _, r := range rows {
		inc := sorter.Incident{ID: r.ID, Kind: r.Kind, Title: r.Title, Message: r.Message, OpenedAt: r.OpenedAt, Resolution: r.Resolution}
		if r.PieceID.Valid {
			inc.PieceID = r.PieceID.String
		}
		if r.ResolvedAt.Valid {
			t := r.ResolvedAt.Time
			inc.ResolvedAt = &t
		}
		out = append(out, inc)
	}
	if err := s.attachIncidentCameras(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *RecordsStore) Stats(ctx context.Context) (sorter.Stats, error) {
	if s.db == nil {
		return sorter.Stats{}, ErrDBNotConfigured
	}
	stats := sorter.Stats{ByStatus: map[string]int{}}
	var rows []struct {
		Status string `db:"status"`
		N      int    `db:"n"`
	}
	if err := s.db.SelectContext(ctx, &rows, `SELECT status, COUNT(*) AS n FROM pieces GROUP BY status`); err != nil {
		return stats, err
	}
	for _, r := range rows {
		stats.ByStatus[r.Status] = r.N
		stats.Pieces += r.N
	}
	stats.Distributed = stats.ByStatus[string(sorter.PieceDistributed)]
	if err := s.db.GetContext(ctx, &stats.Runs, `SELECT COUNT(*) FROM sort_runs`); err != nil {
		return stats, err
	}
	return stats, nil
}

func clampLimit(limit int) int {
	if limit <= 0 || limit > 500 {
		return 100
	}
	return limit
}

func utcOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

// BinCounts is the pieces delivered to each bin since it was last emptied.
func (s *RecordsStore) BinCounts(ctx context.Context) (sorter.BinCounts, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	const query = `
SELECT p.bin_layer AS layer, p.bin_number AS bin, COUNT(*) AS n
FROM pieces p
LEFT JOIN bin_empties e ON e.layer = p.bin_layer AND e.bin = p.bin_number
WHERE p.status = 'distributed' AND p.bin_layer > 0
  AND (e.emptied_at IS NULL OR p.distributed_at > e.emptied_at)
GROUP BY p.bin_layer, p.bin_number
`
	var rows []struct {
		Layer int `db:"layer"`
		Bin   int `db:"bin"`
		N     int `db:"n"`
	}
	if err := s.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, err
	}
	out := sorter.BinCounts{}
	for _, r := range rows {
		out[sorter.BinKey{Layer: r.Layer, Bin: r.Bin}] = r.N
	}
	return out, nil
}

// EmptyBin records that a bin was emptied now.
func (s *RecordsStore) EmptyBin(ctx context.Context, bin sorter.BinKey, at time.Time) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	const query = `
INSERT INTO bin_empties (layer, bin, emptied_at) VALUES (?, ?, ?)
ON CONFLICT (layer, bin) DO UPDATE SET emptied_at = excluded.emptied_at
`
	_, err := s.db.ExecContext(ctx, query, bin.Layer, bin.Bin, at.UTC())
	return err
}

// KitPieces lists delivered pieces in these categories from runs under the profile.
func (s *RecordsStore) KitPieces(ctx context.Context, profileID int64, categories []string) ([]sorter.KitPiece, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	if len(categories) == 0 {
		return nil, nil
	}
	query, args, err := sqlx.In(`
SELECT p.category, p.part_id, p.color_id
FROM pieces p JOIN sort_runs r ON r.id = p.run_id
WHERE r.profile_id = ? AND p.status = 'distributed' AND p.category IN (?)
ORDER BY p.seen_at`, profileID, categories)
	if err != nil {
		return nil, err
	}
	var out []sorter.KitPiece
	if err := s.db.SelectContext(ctx, &out, s.db.Rebind(query), args...); err != nil {
		return nil, err
	}
	return out, nil
}

// SaveReview stores a person's check of a piece, replacing any earlier one.
func (s *RecordsStore) SaveReview(ctx context.Context, pieceID string, r sorter.PieceReview) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	issues := r.Issues
	if issues == nil {
		issues = []string{}
	}
	raw, err := json.Marshal(issues)
	if err != nil {
		return err
	}
	const query = `
INSERT INTO piece_reviews (piece_id, part_correct, color_correct, color_id, issues, reviewed_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (piece_id) DO UPDATE SET part_correct = excluded.part_correct, color_correct = excluded.color_correct,
  color_id = excluded.color_id, issues = excluded.issues, reviewed_at = excluded.reviewed_at
`
	_, err = s.db.ExecContext(ctx, query, pieceID, r.PartCorrect, r.ColorCorrect, r.ColorID, string(raw), r.ReviewedAt.UTC())
	if err != nil && strings.Contains(err.Error(), "FOREIGN KEY") {
		return sorter.ErrPieceNotFound
	}
	return err
}

func (s *RecordsStore) review(ctx context.Context, pieceID string) (*sorter.PieceReview, error) {
	var row struct {
		PartCorrect  sql.NullBool `db:"part_correct"`
		ColorCorrect sql.NullBool `db:"color_correct"`
		ColorID      string       `db:"color_id"`
		Issues       string       `db:"issues"`
		ReviewedAt   time.Time    `db:"reviewed_at"`
	}
	err := s.db.GetContext(ctx, &row, `SELECT part_correct, color_correct, color_id, issues, reviewed_at FROM piece_reviews WHERE piece_id = ?`, pieceID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r := &sorter.PieceReview{ColorID: row.ColorID, ReviewedAt: row.ReviewedAt, Issues: []string{}}
	if row.PartCorrect.Valid {
		v := row.PartCorrect.Bool
		r.PartCorrect = &v
	}
	if row.ColorCorrect.Valid {
		v := row.ColorCorrect.Bool
		r.ColorCorrect = &v
	}
	_ = json.Unmarshal([]byte(row.Issues), &r.Issues)
	return r, nil
}
