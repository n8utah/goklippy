package logic

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Reading Rebrickable's downloads (https://rebrickable.com/downloads/) into a new set
// catalog. Each file is a CSV with a header row.

// SetSource opens one of Rebrickable's CSV files by name ("sets", "inventory_parts", …),
// uncompressed.
type SetSource func(ctx context.Context, name string) (io.ReadCloser, error)

const rebrickableDownloads = "https://cdn.rebrickable.com/media/downloads/"

// stickerCategory is Rebrickable's part category for sticker sheets, which aren't
// counted: a sorted collection never has them.
const stickerCategory = 58

var downloadClient = &http.Client{Timeout: 10 * time.Minute}

// RebrickableDownloads fetches the files from Rebrickable's download site.
func RebrickableDownloads(ctx context.Context, name string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rebrickableDownloads+name+".csv.gz", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "goklippy-sortviewer")
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", name, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: %s", name, resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: %w", name, err)
	}
	return gzReadCloser{gz, resp.Body}, nil
}

// GzipFiles reads the same files from a folder of downloaded .csv.gz files.
func GzipFiles(dir string) SetSource {
	return func(_ context.Context, name string) (io.ReadCloser, error) {
		f, err := os.Open(filepath.Join(dir, name+".csv.gz"))
		if err != nil {
			return nil, err
		}
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return gzReadCloser{gz, f}, nil
	}
}

type gzReadCloser struct {
	*gzip.Reader
	body io.Closer
}

func (g gzReadCloser) Close() error {
	return errors.Join(g.Reader.Close(), g.body.Close())
}

type importPart struct {
	id       int64
	category int64
}

// importRebrickable reads every file into b: colours, themes, parts (each pointing at
// the canonical part of its mold/alternate group), sets, and each set's latest
// inventory without spares or stickers.
func importRebrickable(ctx context.Context, src SetSource, b CatalogBuilder) error {
	err := eachRow(ctx, src, "colors", []string{"id", "name", "rgb", "is_trans"}, func(r row) error {
		return b.AddColor(r.int("id"), r.str("name"), ColorKey(r.str("name")), r.str("rgb"), r.bool("is_trans"))
	})
	if err != nil {
		return err
	}
	err = eachRow(ctx, src, "themes", []string{"id", "name", "parent_id"}, func(r row) error {
		return b.AddTheme(r.int("id"), r.str("name"), r.int("parent_id"))
	})
	if err != nil {
		return err
	}

	parts := map[string]importPart{}
	names := map[string]string{}
	var order []string
	err = eachRow(ctx, src, "parts", []string{"part_num", "name", "part_cat_id"}, func(r row) error {
		num := r.str("part_num")
		if _, dup := parts[num]; dup || num == "" {
			return nil
		}
		order = append(order, num)
		parts[num] = importPart{id: int64(len(order)), category: r.int("part_cat_id")}
		names[num] = r.str("name")
		return nil
	})
	if err != nil {
		return err
	}
	groups := newPartGroups()
	err = eachRow(ctx, src, "part_relationships", []string{"rel_type", "child_part_num", "parent_part_num"}, func(r row) error {
		// M: a mold variant, A: an alternate. Both are the same part to a builder;
		// prints (P) and the rest aren't.
		child, parent := r.str("child_part_num"), r.str("parent_part_num")
		_, okChild := parts[child]
		_, okParent := parts[parent]
		if rel := r.str("rel_type"); (rel == "M" || rel == "A") && okChild && okParent {
			groups.join(child, parent)
		}
		return nil
	})
	if err != nil {
		return err
	}
	canon := func(num string) int64 { return parts[groups.root(num)].id }
	for _, num := range order {
		p := parts[num]
		if err := b.AddPart(p.id, num, names[num], p.category, canon(num)); err != nil {
			return err
		}
	}

	setIDs := map[string]int64{}
	err = eachRow(ctx, src, "sets", []string{"set_num", "name", "year", "theme_id", "img_url"}, func(r row) error {
		num := r.str("set_num")
		if _, dup := setIDs[num]; dup || num == "" {
			return nil
		}
		id := int64(len(setIDs) + 1)
		setIDs[num] = id
		return b.AddSet(id, num, r.str("name"), int(r.int("year")), r.int("theme_id"), r.str("img_url"))
	})
	if err != nil {
		return err
	}

	// A set can have several inventory versions; the latest is the one counted.
	type latest struct{ inventory, version int64 }
	newest := map[string]latest{}
	err = eachRow(ctx, src, "inventories", []string{"id", "version", "set_num"}, func(r row) error {
		num := r.str("set_num")
		if _, ok := setIDs[num]; !ok {
			return nil // a minifigure's inventory
		}
		if v := r.int("version"); v > newest[num].version {
			newest[num] = latest{inventory: r.int("id"), version: v}
		}
		return nil
	})
	if err != nil {
		return err
	}
	inventorySet := make(map[int64]int64, len(newest))
	for num, l := range newest {
		inventorySet[l.inventory] = setIDs[num]
	}

	return eachRow(ctx, src, "inventory_parts", []string{"inventory_id", "part_num", "color_id", "quantity", "is_spare", "img_url"}, func(r row) error {
		setID, ok := inventorySet[r.int("inventory_id")]
		if !ok || r.bool("is_spare") {
			return nil
		}
		num := r.str("part_num")
		part, ok := parts[num]
		if !ok || part.category == stickerCategory {
			return nil
		}
		qty := int(r.int("quantity"))
		if qty <= 0 {
			return nil
		}
		partID, colorID := canon(num), r.int("color_id")
		if err := b.AddSetPart(setID, partID, colorID, qty); err != nil {
			return err
		}
		if img := r.str("img_url"); img != "" {
			return b.AddImage(partID, colorID, img)
		}
		return nil
	})
}

// partGroups joins parts that are the same to a builder. A group's root, its canonical
// part, is its shortest part number (then the first alphabetically): usually the plain
// one, "3001" over "3001old".
type partGroups struct{ parent map[string]string }

func newPartGroups() *partGroups { return &partGroups{parent: map[string]string{}} }

func (g *partGroups) root(num string) string {
	for {
		p, ok := g.parent[num]
		if !ok || p == num {
			return num
		}
		if gp, ok := g.parent[p]; ok {
			g.parent[num] = gp // halve the path
		}
		num = p
	}
}

func (g *partGroups) join(a, b string) {
	ra, rb := g.root(a), g.root(b)
	if ra == rb {
		return
	}
	if len(rb) < len(ra) || (len(rb) == len(ra) && rb < ra) {
		ra, rb = rb, ra
	}
	g.parent[rb] = ra
}

// row is one CSV record, read by column name.
type row struct {
	cols map[string]int
	rec  []string
}

func (r row) str(col string) string {
	if i, ok := r.cols[col]; ok && i < len(r.rec) {
		return r.rec[i]
	}
	return ""
}

func (r row) int(col string) int64 {
	n, _ := strconv.ParseInt(r.str(col), 10, 64)
	return n
}

func (r row) bool(col string) bool {
	v := r.str(col)
	return v == "True" || v == "true" || v == "t" || v == "1"
}

// eachRow calls fn for every record of a file, after checking it has the columns needed.
func eachRow(ctx context.Context, src SetSource, name string, need []string, fn func(row) error) error {
	rc, err := src(ctx, name)
	if err != nil {
		return err
	}
	defer rc.Close()
	cr := csv.NewReader(bufio.NewReaderSize(rc, 1<<16))
	cr.ReuseRecord = true
	cr.FieldsPerRecord = -1
	header, err := cr.Read()
	if err != nil {
		return fmt.Errorf("%s: read the header: %w", name, err)
	}
	cols := make(map[string]int, len(header))
	for i, h := range header {
		cols[h] = i
	}
	for _, c := range need {
		if _, ok := cols[c]; !ok {
			return fmt.Errorf("%s: no %q column", name, c)
		}
	}
	for n := 0; ; n++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if n%10000 == 0 && ctx.Err() != nil {
			return ctx.Err()
		}
		if err := fn(row{cols: cols, rec: rec}); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
}
