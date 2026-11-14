package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"

	"goklippy/services/sortviewer/backend/logic"
	"goklippy/services/sortviewer/database"
)

// Catalog is the set catalog (sets.db): every set's parts, only read once built.
type Catalog struct {
	db *sqlx.DB
}

// OpenCatalog opens a built catalog read-only. It's immutable, so SQLite takes no locks
// and the file can be replaced once this is closed.
func OpenCatalog(path string) (*Catalog, error) {
	params := url.Values{}
	params.Set("mode", "ro")
	params.Set("immutable", "1")
	// Mapped reads are several times faster than SQLite's page cache for the big joins.
	params.Add("_pragma", "mmap_size(268435456)")
	db, err := sqlx.Open("sqlite", "file:"+path+"?"+params.Encode())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var n int
	if err := db.GetContext(ctx, &n, `SELECT COUNT(*) FROM meta WHERE key = 'built_at'`); err != nil || n == 0 {
		_ = db.Close()
		if err == nil {
			err = errors.New("it was never finished")
		}
		return nil, fmt.Errorf("open the set data: %w", err)
	}
	return &Catalog{db: db}, nil
}

func (c *Catalog) Close() error { return c.db.Close() }

// Built is when the catalog was built and how many sets it has.
func (c *Catalog) Built(ctx context.Context) (time.Time, int, error) {
	var at string
	if err := c.db.GetContext(ctx, &at, `SELECT value FROM meta WHERE key = 'built_at'`); err != nil {
		return time.Time{}, 0, err
	}
	built, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return time.Time{}, 0, err
	}
	var n int
	err = c.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM sets`)
	return built, n, err
}

// haveCTE turns the collection's counts (JSON [{p, c, n}], ?1) into h: pieces per
// canonical part and colour (color_id NULL when the colour isn't known here), and hs:
// pieces per canonical part in any colour.
const haveCTE = `
h0 AS (
  SELECT value->>'p' AS part_num, value->>'c' AS color_key, value->>'n' AS n FROM json_each(?1)
),
h AS (
  SELECT p.canon_id AS part_id, c.id AS color_id, SUM(h0.n) AS n
  FROM h0 JOIN parts p ON p.part_num = h0.part_num
  LEFT JOIN colors c ON c.name_key = h0.color_key AND h0.color_key != ''
  GROUP BY p.canon_id, c.id
),
hs AS (
  SELECT part_id, SUM(n) AS n FROM h GROUP BY part_id
)`

func haveJSON(have []logic.HaveLine) (string, error) {
	if have == nil {
		have = []logic.HaveLine{}
	}
	data, err := json.Marshal(have)
	return string(data), err
}

type setMatchRow struct {
	SetNum        string `db:"set_num"`
	Name          string `db:"name"`
	Year          int    `db:"year"`
	Theme         string `db:"theme"`
	ImgURL        string `db:"img_url"`
	Need          int    `db:"need"`
	Have          int    `db:"have"`
	HaveAnyColour int    `db:"have_any_colour"`
}

func (r setMatchRow) match() logic.SetMatch {
	return logic.SetMatch{
		SetNum: r.SetNum, Name: r.Name, Year: r.Year, Theme: r.Theme, ImgURL: r.ImgURL,
		Need: r.Need, Have: r.Have, HaveAnyColour: r.HaveAnyColour,
	}
}

// ScoreSets is every set the collection has at least one part of, with how much of it.
// The joins start from the collection's parts (CROSS JOIN fixes the order), so only the
// set lines with those parts are read.
func (c *Catalog) ScoreSets(ctx context.Context, have []logic.HaveLine) ([]logic.SetMatch, error) {
	haveArg, err := haveJSON(have)
	if err != nil {
		return nil, err
	}
	// exact sums, per set, the right-colour pieces up to what each line needs; shape does
	// the same per part across its colours.
	const query = `WITH ` + haveCTE + `,
exact AS (
  SELECT sp.set_id, SUM(MIN(sp.qty, h.n)) AS have
  FROM h CROSS JOIN set_parts sp ON sp.part_id = h.part_id AND sp.color_id = h.color_id
  GROUP BY sp.set_id
),
shape AS (
  SELECT ss.set_id, SUM(MIN(ss.qty, hs.n)) AS have
  FROM hs CROSS JOIN set_shapes ss ON ss.part_id = hs.part_id
  GROUP BY ss.set_id
)
SELECT s.set_num, s.name, s.year, COALESCE(t.name, '') AS theme, s.img_url, s.need,
  COALESCE(e.have, 0) AS have, sh.have AS have_any_colour
FROM shape sh
JOIN sets s ON s.id = sh.set_id
LEFT JOIN exact e ON e.set_id = sh.set_id
LEFT JOIN themes t ON t.id = s.theme_id`
	var rows []setMatchRow
	if err := c.db.SelectContext(ctx, &rows, query, haveArg); err != nil {
		return nil, err
	}
	out := make([]logic.SetMatch, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.match())
	}
	return out, nil
}

type setPartRow struct {
	PartNum   string `db:"part_num"`
	PartName  string `db:"part_name"`
	ColorID   int64  `db:"color_id"`
	ColorName string `db:"color_name"`
	RGB       string `db:"rgb"`
	ImgURL    string `db:"img_url"`
	Need      int    `db:"need"`
	HaveExact int    `db:"have_exact"`
	HaveAny   int    `db:"have_any"`
}

// SetParts is one set and each of its parts against the collection, most needed first.
func (c *Catalog) SetParts(ctx context.Context, setNum string, have []logic.HaveLine) (logic.SetMatch, []logic.SetPartLine, error) {
	var set setMatchRow
	err := c.db.GetContext(ctx, &set, `
SELECT s.set_num, s.name, s.year, COALESCE(t.name, '') AS theme, s.img_url, s.need, 0 AS have, 0 AS have_any_colour
FROM sets s LEFT JOIN themes t ON t.id = s.theme_id WHERE s.set_num = ?`, setNum)
	if errors.Is(err, sql.ErrNoRows) {
		return logic.SetMatch{}, nil, logic.ErrNotFound
	}
	if err != nil {
		return logic.SetMatch{}, nil, err
	}
	haveArg, err := haveJSON(have)
	if err != nil {
		return logic.SetMatch{}, nil, err
	}
	query := `WITH ` + haveCTE + `
SELECT p.part_num, p.name AS part_name, sp.color_id, COALESCE(c.name, '') AS color_name,
  COALESCE(c.rgb, '') AS rgb, COALESCE(i.img_url, '') AS img_url, sp.qty AS need,
  COALESCE(h.n, 0) AS have_exact, COALESCE(hs.n, 0) AS have_any
FROM sets s
JOIN set_parts sp ON sp.set_id = s.id
JOIN parts p ON p.id = sp.part_id
LEFT JOIN colors c ON c.id = sp.color_id
LEFT JOIN images i ON i.part_id = sp.part_id AND i.color_id = sp.color_id
LEFT JOIN h ON h.part_id = sp.part_id AND h.color_id = sp.color_id
LEFT JOIN hs ON hs.part_id = sp.part_id
WHERE s.set_num = ?2
ORDER BY sp.qty DESC, p.name, c.name`
	var rows []setPartRow
	if err := c.db.SelectContext(ctx, &rows, query, haveArg, setNum); err != nil {
		return logic.SetMatch{}, nil, err
	}
	match := set.match()
	lines := make([]logic.SetPartLine, 0, len(rows))
	shapeNeed := map[string]int{}
	for _, r := range rows {
		got := min(r.Need, r.HaveExact)
		match.Have += got
		shapeNeed[r.PartNum] += r.Need
		lines = append(lines, logic.SetPartLine{
			PartNum: r.PartNum, PartName: r.PartName, ColorID: r.ColorID, ColorName: r.ColorName,
			RGB: r.RGB, ImgURL: r.ImgURL, Need: r.Need, Have: got, OtherColours: r.HaveAny - r.HaveExact,
		})
	}
	counted := map[string]bool{}
	for _, r := range rows {
		if !counted[r.PartNum] {
			counted[r.PartNum] = true
			match.HaveAnyColour += min(shapeNeed[r.PartNum], r.HaveAny)
		}
	}
	return match, lines, nil
}

// Coverage is how many of the collection's pieces are parts (and colours) the catalog
// knows, and the most common part numbers it doesn't.
func (c *Catalog) Coverage(ctx context.Context, have []logic.HaveLine) (logic.SetCoverage, error) {
	haveArg, err := haveJSON(have)
	if err != nil {
		return logic.SetCoverage{}, err
	}
	var cov logic.SetCoverage
	err = c.db.GetContext(ctx, &cov, `
WITH h0 AS (
  SELECT value->>'p' AS part_num, value->>'c' AS color_key, value->>'n' AS n FROM json_each(?)
)
SELECT COALESCE(SUM(h0.n), 0) AS pieces,
  COALESCE(SUM(CASE WHEN p.id IS NOT NULL THEN h0.n END), 0) AS known,
  COALESCE(SUM(CASE WHEN p.id IS NOT NULL AND EXISTS (
    SELECT 1 FROM colors c WHERE c.name_key = h0.color_key AND h0.color_key != ''
  ) THEN h0.n END), 0) AS known_colour
FROM h0 LEFT JOIN parts p ON p.part_num = h0.part_num`, haveArg)
	if err != nil {
		return logic.SetCoverage{}, err
	}
	cov.Unknown = []logic.UnknownPart{}
	err = c.db.SelectContext(ctx, &cov.Unknown, `
WITH h0 AS (
  SELECT value->>'p' AS part_num, value->>'n' AS n FROM json_each(?)
)
SELECT h0.part_num, SUM(h0.n) AS pieces FROM h0
WHERE NOT EXISTS (SELECT 1 FROM parts p WHERE p.part_num = h0.part_num)
GROUP BY h0.part_num ORDER BY pieces DESC, h0.part_num LIMIT 20`, haveArg)
	return cov, err
}

// CatalogBuilder writes a new catalog file in one transaction. Nothing reads it until
// Finish, so it skips the journal and syncing.
type CatalogBuilder struct {
	path  string
	db    *sqlx.DB
	tx    *sqlx.Tx
	stmts map[string]*sqlx.Stmt
}

var builderStatements = map[string]string{
	"color":    `INSERT INTO colors (id, name, name_key, rgb, is_trans) VALUES (?, ?, ?, ?, ?)`,
	"theme":    `INSERT INTO themes (id, name, parent_id) VALUES (?, ?, NULLIF(?, 0))`,
	"part":     `INSERT INTO parts (id, part_num, name, category_id, canon_id) VALUES (?, ?, ?, ?, ?)`,
	"set":      `INSERT INTO sets (id, set_num, name, year, theme_id, img_url) VALUES (?, ?, ?, ?, ?, ?)`,
	"set_part": `INSERT INTO set_parts (set_id, part_id, color_id, qty) VALUES (?, ?, ?, ?) ON CONFLICT DO UPDATE SET qty = qty + excluded.qty`,
	"image":    `INSERT OR IGNORE INTO images (part_id, color_id, img_url) VALUES (?, ?, ?)`,
}

// NewCatalogBuilder starts a new catalog at path, replacing any unfinished one there.
func NewCatalogBuilder(path string) (*CatalogBuilder, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := removeDB(path); err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Add("_pragma", "journal_mode(OFF)")
	params.Add("_pragma", "synchronous(OFF)")
	db, err := sqlx.Open("sqlite", "file:"+path+"?"+params.Encode())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	b := &CatalogBuilder{path: path, db: db, stmts: map[string]*sqlx.Stmt{}}
	if err := b.start(); err != nil {
		b.Abort()
		return nil, err
	}
	return b, nil
}

func (b *CatalogBuilder) start() error {
	ctx := context.Background()
	if _, err := b.db.ExecContext(ctx, database.CatalogSchema); err != nil {
		return fmt.Errorf("apply the catalog schema: %w", err)
	}
	tx, err := b.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	b.tx = tx
	for name, query := range builderStatements {
		stmt, err := tx.PreparexContext(ctx, query)
		if err != nil {
			return fmt.Errorf("prepare %s: %w", name, err)
		}
		b.stmts[name] = stmt
	}
	return nil
}

func (b *CatalogBuilder) exec(name string, args ...any) error {
	_, err := b.stmts[name].Exec(args...)
	return err
}

func (b *CatalogBuilder) AddColor(id int64, name, nameKey, rgb string, trans bool) error {
	return b.exec("color", id, name, nameKey, rgb, trans)
}

func (b *CatalogBuilder) AddTheme(id int64, name string, parentID int64) error {
	return b.exec("theme", id, name, parentID)
}

func (b *CatalogBuilder) AddPart(id int64, num, name string, categoryID, canonID int64) error {
	return b.exec("part", id, num, name, categoryID, canonID)
}

func (b *CatalogBuilder) AddSet(id int64, num, name string, year int, themeID int64, imgURL string) error {
	return b.exec("set", id, num, name, year, themeID, imgURL)
}

// AddSetPart adds to a set's count of a part in a colour.
func (b *CatalogBuilder) AddSetPart(setID, partID, colorID int64, qty int) error {
	return b.exec("set_part", setID, partID, colorID, qty)
}

// AddImage keeps the first picture given for a part in a colour.
func (b *CatalogBuilder) AddImage(partID, colorID int64, imgURL string) error {
	return b.exec("image", partID, colorID, imgURL)
}

// Finish works out each set's total, drops sets with nothing to count (books, gear,
// minifigure-only packs), records meta and closes the file.
func (b *CatalogBuilder) Finish(meta map[string]string) error {
	ctx := context.Background()
	steps := []string{
		`UPDATE sets SET need = COALESCE((SELECT SUM(qty) FROM set_parts WHERE set_id = sets.id), 0)`,
		`DELETE FROM sets WHERE need = 0`,
		`INSERT INTO set_shapes (set_id, part_id, qty) SELECT set_id, part_id, SUM(qty) FROM set_parts GROUP BY set_id, part_id`,
	}
	for _, q := range steps {
		if _, err := b.tx.ExecContext(ctx, q); err != nil {
			b.Abort()
			return err
		}
	}
	for k, v := range meta {
		if _, err := b.tx.ExecContext(ctx, `INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)`, k, v); err != nil {
			b.Abort()
			return err
		}
	}
	if err := b.tx.Commit(); err != nil {
		b.Abort()
		return err
	}
	b.tx = nil
	if _, err := b.db.ExecContext(ctx, `ANALYZE`); err != nil {
		b.Abort()
		return err
	}
	return b.db.Close()
}

// Abort throws the unfinished catalog away.
func (b *CatalogBuilder) Abort() {
	if b.tx != nil {
		_ = b.tx.Rollback()
		b.tx = nil
	}
	_ = b.db.Close()
	_ = removeDB(b.path)
}

// removeDB deletes a database file and its journal, if they're there.
func removeDB(path string) error {
	for _, p := range []string{path, path + "-journal", path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
