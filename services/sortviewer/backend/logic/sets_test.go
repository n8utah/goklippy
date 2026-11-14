package logic_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goklippy/services/sortviewer/backend/logic"
	"goklippy/services/sortviewer/backend/models"
)

// A tiny Rebrickable: set 100-1 has an old inventory version, a mold variant (3001old)
// of 3001, a spare, a sticker and a printed tile; 300-1 has no parts; fig-000001 is a
// minifigure's inventory.
var rebrickableFixture = map[string]string{
	"colors": `id,name,rgb,is_trans,num_parts,num_sets,y1,y2
0,Black,05131D,False,1,1,1957,2027
4,Red,C91A09,False,1,1,1957,2027
15,White,FFFFFF,False,1,1,1957,2027
71,Light Bluish Gray,A0A5A9,False,1,1,2003,2027`,
	"themes": `id,name,parent_id
1,City,
2,Town,1`,
	"parts": `part_num,name,part_cat_id,part_material
3001,Brick 2 x 4,11,Plastic
3001old,Brick 2 x 4 (old mold),11,Plastic
3023,Plate 1 x 2,14,Plastic
3069b,Tile 1 x 2,19,Plastic
3069bpr0001,Tile 1 x 2 with print,19,Plastic
stk1,Sticker Sheet,58,Plastic`,
	"part_relationships": `rel_type,child_part_num,parent_part_num
M,3001old,3001
P,3069bpr0001,3069b`,
	"sets": `set_num,name,year,theme_id,num_parts,img_url
100-1,Red House,2020,2,12,https://img/100-1.jpg
200-1,Grey Wall,2021,1,10,https://img/200-1.jpg
300-1,Ideas Book,2019,1,0,`,
	"inventories": `id,version,set_num
1,1,100-1
2,2,100-1
3,1,200-1
4,1,fig-000001`,
	"inventory_parts": `inventory_id,part_num,color_id,quantity,is_spare,img_url
1,3023,4,50,False,
2,3001,4,4,False,https://img/3001-red.jpg
2,3001old,4,2,False,
2,3023,15,2,False,https://img/3023-white.jpg
2,3023,15,1,True,
2,stk1,15,1,False,
2,3069bpr0001,0,1,False,
3,3001,71,10,False,
4,3001,0,1,False,`,
}

func fixtureSource(files map[string]string) logic.SetSource {
	return func(_ context.Context, name string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(files[name])), nil
	}
}

type setsSite struct {
	svc   *logic.Service
	store *models.Store
	admin logic.Session
}

func newSetsSite(t *testing.T, src logic.SetSource) *setsSite {
	t.Helper()
	store, err := models.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	dir := t.TempDir()
	svc := logic.NewService(store, dir)
	svc.UseSets(logic.SetsConfig{
		Path:       filepath.Join(dir, "sets.db"),
		Open:       func(path string) (logic.SetCatalog, error) { return models.OpenCatalog(path) },
		NewBuilder: func(path string) (logic.CatalogBuilder, error) { return models.NewCatalogBuilder(path) },
		Source:     src,
	})
	t.Cleanup(func() { svc.CloseSets() })
	return &setsSite{svc: svc, store: store, admin: logic.Session{AdminEmail: "nate@example.com"}}
}

// collect puts pieces into collection 1: part, BrickLink colour name, how many.
func (s *setsSite) collect(t *testing.T, pieces ...any) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	require.NoError(t, s.store.UpsertCollection(ctx, 1, "Sam's LEGO", nil, now))
	n := 0
	for i := 0; i < len(pieces); i += 3 {
		for range pieces[i+2].(int) {
			n++
			require.NoError(t, s.store.UpsertPiece(ctx, "p"+strconv.Itoa(n), logic.PushPiece{
				CollectionID: 1, SeenAt: now, PartID: pieces[i].(string), ColorID: pieces[i+1].(string), ColorName: pieces[i+1].(string),
			}, now))
		}
	}
}

func TestBuildableSets(t *testing.T) {
	ctx := context.Background()
	site := newSetsSite(t, fixtureSource(rebrickableFixture))
	require.NoError(t, site.svc.BuildSets(ctx))
	st := site.svc.SetsStatus(ctx)
	require.True(t, st.Ready)
	assert.Equal(t, 2, st.Sets, "the book has nothing to count and is dropped")

	site.collect(t,
		"3001", "Red", 5,
		"3001old", "Red", 1, // the old mold counts as 3001
		"3001", "Light Bluish Gray", 3,
		"3023", "White", 2,
		"3069b", "Black", 1, // plain, so not the printed tile
		"99999", "Black", 4, // not in the set data
	)

	// 100-1 needs 6 red 3001, 2 white 3023 and a printed tile (old version, spare and
	// sticker not counted): 9, of which 8 in the right colour.
	page, err := site.svc.BuildableSets(ctx, site.admin, 1, logic.SetFilter{})
	require.NoError(t, err)
	matches, cov := page.Sets, page.Coverage
	assert.Equal(t, 2, page.Total)
	require.Len(t, matches, 2)
	assert.Equal(t, logic.SetMatch{
		SetNum: "100-1", Name: "Red House", Year: 2020, Theme: "Town", ImgURL: "https://img/100-1.jpg",
		Need: 9, Have: 8, HaveAnyColour: 8,
	}, matches[0])
	assert.Equal(t, "200-1", matches[1].SetNum)
	assert.Equal(t, 3, matches[1].Have)
	assert.Equal(t, 9, matches[1].HaveAnyColour, "nine 3001 in any colour, of ten")

	// In any colour, the wall (9 of 10) beats the house (8 of 9).
	anyPage, err := site.svc.BuildableSets(ctx, site.admin, 1, logic.SetFilter{AnyColour: true})
	anyColour := anyPage.Sets
	require.NoError(t, err)
	assert.Equal(t, []string{"200-1", "100-1"}, []string{anyColour[0].SetNum, anyColour[1].SetNum})

	assert.Equal(t, 16, cov.Pieces)
	assert.Equal(t, 12, cov.Known)
	assert.Equal(t, 12, cov.KnownColour)
	assert.Equal(t, []logic.UnknownPart{{PartNum: "99999", Pieces: 4}}, cov.Unknown)

	// Filters.
	found, err := site.svc.BuildableSets(ctx, site.admin, 1, logic.SetFilter{Search: "wall"})
	require.NoError(t, err)
	assert.Len(t, found.Sets, 1)
	found, err = site.svc.BuildableSets(ctx, site.admin, 1, logic.SetFilter{MinParts: 10})
	require.NoError(t, err)
	assert.Len(t, found.Sets, 1)

	set, lines, err := site.svc.SetDetail(ctx, site.admin, 1, "100-1")
	require.NoError(t, err)
	assert.Equal(t, 8, set.Have)
	assert.Equal(t, 8, set.HaveAnyColour)
	assert.Equal(t, []logic.SetPartLine{
		{PartNum: "3001", PartName: "Brick 2 x 4", ColorID: 4, ColorName: "Red", RGB: "C91A09",
			ImgURL: "https://img/3001-red.jpg", Need: 6, Have: 6, OtherColours: 3},
		{PartNum: "3023", PartName: "Plate 1 x 2", ColorID: 15, ColorName: "White", RGB: "FFFFFF",
			ImgURL: "https://img/3023-white.jpg", Need: 2, Have: 2},
		{PartNum: "3069bpr0001", PartName: "Tile 1 x 2 with print", ColorID: 0, ColorName: "Black",
			RGB: "05131D", Need: 1},
	}, lines)

	_, _, err = site.svc.SetDetail(ctx, site.admin, 1, "999-1")
	assert.ErrorIs(t, err, logic.ErrNotFound)

	// A viewer who wasn't given the collection can't see its sets.
	_, err = site.svc.BuildableSets(ctx, logic.Session{ViewerID: 42}, 1, logic.SetFilter{})
	assert.ErrorIs(t, err, logic.ErrNotFound)
}

func TestSetsRebuildSwapsData(t *testing.T) {
	ctx := context.Background()
	files := map[string]string{}
	for k, v := range rebrickableFixture {
		files[k] = v
	}
	site := newSetsSite(t, fixtureSource(files))
	_, err := site.svc.BuildableSets(ctx, site.admin, 1, logic.SetFilter{})
	assert.ErrorIs(t, err, logic.ErrSetsNotReady)

	require.NoError(t, site.svc.BuildSets(ctx))
	files["sets"] += "\n400-1,New Set,2026,1,1,"
	files["inventories"] += "\n5,1,400-1"
	files["inventory_parts"] += "\n5,3023,15,1,False,"
	require.NoError(t, site.svc.BuildSets(ctx))
	assert.Equal(t, 3, site.svc.SetsStatus(ctx).Sets)
}

func TestSetsBadDownloadKeepsOldData(t *testing.T) {
	ctx := context.Background()
	files := map[string]string{}
	for k, v := range rebrickableFixture {
		files[k] = v
	}
	site := newSetsSite(t, fixtureSource(files))
	require.NoError(t, site.svc.BuildSets(ctx))
	files["sets"] = "nope\n1"
	assert.Error(t, site.svc.BuildSets(ctx))
	st := site.svc.SetsStatus(ctx)
	assert.True(t, st.Ready)
	assert.Equal(t, 2, st.Sets)
	assert.Contains(t, st.Error, "set_num")
}

// TestRealRebrickableData builds from real downloads in SORTVIEWER_REBRICKABLE_DIR
// (the .csv.gz files from rebrickable.com/downloads) and reports how long it took.
func TestRealRebrickableData(t *testing.T) {
	dir := os.Getenv("SORTVIEWER_REBRICKABLE_DIR")
	if dir == "" {
		t.Skip("SORTVIEWER_REBRICKABLE_DIR not set")
	}
	ctx := context.Background()
	site := newSetsSite(t, logic.GzipFiles(dir))
	started := time.Now()
	require.NoError(t, site.svc.BuildSets(ctx))
	t.Logf("built in %s: %+v", time.Since(started).Round(time.Second), site.svc.SetsStatus(ctx))

	site.collect(t,
		"3001", "Red", 40, "3001", "Light Bluish Gray", 30, "3023", "Black", 60, "3020", "Dark Bluish Gray", 25,
		"3069b", "White", 50, "3004", "Reddish Brown", 30, "3022", "Tan", 20, "3666", "Light Bluish Gray", 30,
		"3710", "Black", 40, "3005", "Trans-Clear", 20, "4073", "Light Bluish Gray", 40, "3062b", "Yellow", 20,
	)
	started = time.Now()
	page, err := site.svc.BuildableSets(ctx, site.admin, 1, logic.SetFilter{MinParts: 25})
	matches, cov := page.Sets, page.Coverage
	require.NoError(t, err)
	t.Logf("ranked in %s; coverage %+v", time.Since(started).Round(time.Millisecond), cov)
	for _, m := range matches[:min(5, len(matches))] {
		t.Logf("%s %s: %d/%d (%d any colour)", m.SetNum, m.Name, m.Have, m.Need, m.HaveAnyColour)
	}
}
