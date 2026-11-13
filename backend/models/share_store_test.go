package models

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goklippy/backend/logic/sorter"
)

// fakeViewer stands in for the sort viewer site: it records what it's sent, and can be
// told to fail.
type fakeViewer struct {
	mu       sync.Mutex
	calls    []string // "PUT /api/push/pieces/<id>"
	bodies   map[string][]byte
	down     bool
	refuse   string // a path prefix answered 400
	keySeen  string
	photoDim image.Point
}

func (f *fakeViewer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keySeen = r.Header.Get("Authorization")
	if f.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if f.refuse != "" && strings.HasPrefix(r.URL.Path, f.refuse) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad piece"}`))
		return
	}
	body, _ := io.ReadAll(r.Body)
	call := r.Method + " " + r.URL.Path
	f.calls = append(f.calls, call)
	f.bodies[call] = body
	if strings.HasSuffix(r.URL.Path, "/photo") {
		if cfg, err := jpeg.DecodeConfig(bytes.NewReader(body)); err == nil {
			f.photoDim = image.Pt(cfg.Width, cfg.Height)
		}
	}
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (f *fakeViewer) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeViewer) body(call string, out any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_ = json.Unmarshal(f.bodies[call], out)
}

type shareRig struct {
	t       *testing.T
	ctx     context.Context
	records *RecordsStore
	store   *ShareStore
	share   *sorter.Sharing
	site    *fakeViewer
	dataDir string
}

// newShareRig wires sharing like the server does (wrapped stores) against a fake site.
func newShareRig(t *testing.T) *shareRig {
	t.Helper()
	db := openTestDB(t)
	ctx := context.Background()
	store := NewShareStore(db)
	share := sorter.NewSharing(store)
	plain := NewRecordsStore(db)
	recordsStore := sharedRecordsForTest{plain, share}
	dataDir := t.TempDir()
	records := sorter.NewRecords(recordsStore, dataDir)
	sortingStore := NewSortingStore(db)
	sorting := sorter.NewSortingService(ctx, sortingStore, testSettings)
	cols, err := sorter.NewCollections(records, sorting, func() int64 { return 0 })
	require.NoError(t, err)
	share.Attach(recordsStore, cols, records.ImagePath, sortingStore.ColorNames)
	site := &fakeViewer{bodies: map[string][]byte{}}
	srv := httptest.NewServer(site)
	t.Cleanup(srv.Close)
	require.NoError(t, share.SaveSettings(ctx, srv.URL, "sv_test"))
	return &shareRig{t: t, ctx: ctx, records: plain, store: store, share: share, site: site, dataDir: dataDir}
}

// sharedRecordsForTest marks pieces and runs like the server's sharingRecords.
type sharedRecordsForTest struct {
	*RecordsStore
	share *sorter.Sharing
}

func (s sharedRecordsForTest) SavePiece(ctx context.Context, p sorter.Piece) error {
	err := s.RecordsStore.SavePiece(ctx, p)
	if err == nil {
		s.share.PieceChanged(ctx, p.ID)
	}
	return err
}

func (s sharedRecordsForTest) SaveReview(ctx context.Context, id string, r sorter.PieceReview) error {
	err := s.RecordsStore.SaveReview(ctx, id, r)
	if err == nil {
		s.share.PieceChanged(ctx, id)
	}
	return err
}

func (r *shareRig) piece(runID int64, part string) sorter.Piece {
	now := time.Now().UTC().Truncate(time.Second)
	p := sorter.Piece{ID: sorter.NewPieceID(), RunID: runID, Status: sorter.PieceDistributed, SeenAt: now,
		DistributedAt: &now, PartID: part, PartName: "Brick " + part, ColorID: "5", ColorName: "Red",
		BinLayer: 1, BinNumber: 2, Problems: []sorter.Problem{}}
	require.NoError(r.t, sharedRecordsForTest{r.records, r.share}.SavePiece(r.ctx, p))
	return p
}

// photo stores a big JPEG as the piece's photo and marks it.
func (r *shareRig) photo(p sorter.Piece) {
	rel := "pieces/" + p.ID[:2] + "/" + p.ID + "/00.jpg"
	full := filepath.Join(r.dataDir, filepath.FromSlash(rel))
	require.NoError(r.t, os.MkdirAll(filepath.Dir(full), 0o755))
	var buf bytes.Buffer
	require.NoError(r.t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1600, 1200)), nil))
	require.NoError(r.t, os.WriteFile(full, buf.Bytes(), 0o644))
	require.NoError(r.t, r.records.AddPieceImage(r.ctx, p.ID, sorter.PieceImage{Path: rel, TakenAt: time.Now()}))
	r.share.PhotoChanged(r.ctx, p.ID)
}

// sendAll runs the sender until nothing's waiting.
func (r *shareRig) sendAll() {
	r.t.Helper()
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	go r.share.Run(ctx)
	require.Eventually(r.t, func() bool {
		st, err := r.share.Status(r.ctx)
		return err == nil && st.Waiting == 0
	}, 5*time.Second, 10*time.Millisecond)
}

func TestSharingSendsOnlySharedCollectionsAndBackfills(t *testing.T) {
	r := newShareRig(t)
	sam, alex := newCollection(t, r.records, "Sam"), newCollection(t, r.records, "Alex")
	samRun, err := r.records.StartRun(r.ctx, sam, nil, "", time.Now())
	require.NoError(t, err)
	alexRun, err := r.records.StartRun(r.ctx, alex, nil, "", time.Now())
	require.NoError(t, err)
	before := r.piece(samRun, "3001")
	r.photo(before)
	r.piece(alexRun, "3003")

	st, err := r.share.Status(r.ctx)
	require.NoError(t, err)
	assert.Zero(t, st.Waiting, "nothing is shared yet")

	// Sharing Sam's sends what's already there: collection, run, pieces, photos, in that
	// order; then what comes after.
	require.NoError(t, r.share.SetShared(r.ctx, sam, true))
	again := r.piece(samRun, "3020")
	r.sendAll()
	calls := r.site.seen()
	assert.Equal(t, []string{
		"PUT /api/push/collections/1",
		"PUT /api/push/runs/1",
		"PUT /api/push/pieces/" + before.ID,
		"PUT /api/push/pieces/" + before.ID + "/photo",
		"PUT /api/push/pieces/" + again.ID,
	}, calls)
	assert.Equal(t, "Bearer sv_test", r.site.keySeen)
	assert.Equal(t, image.Pt(800, 600), r.site.photoDim, "shrunk to 800 on its longest side")

	var sent sorter.SharedPiece
	r.site.body("PUT /api/push/pieces/"+before.ID, &sent)
	assert.Equal(t, sam, sent.CollectionID)
	assert.Equal(t, "3001", sent.PartID)
	assert.Equal(t, 1, sent.BinLayer)

	st, err = r.share.Status(r.ctx)
	require.NoError(t, err)
	assert.True(t, st.Connected)
	assert.True(t, st.KeySet)
	assert.NotNil(t, st.LastSent)
}

func TestSharingWaitsOutTheSiteBeingDown(t *testing.T) {
	r := newShareRig(t)
	col := newCollection(t, r.records, "Sam")
	require.NoError(t, r.share.SetShared(r.ctx, col, true))
	run, err := r.records.StartRun(r.ctx, col, nil, "", time.Now())
	require.NoError(t, err)
	r.share.RunChanged(r.ctx, run)

	r.site.mu.Lock()
	r.site.down = true
	r.site.mu.Unlock()
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	go r.share.Run(ctx)
	require.Eventually(t, func() bool {
		st, _ := r.share.Status(r.ctx)
		return st.LastError != ""
	}, 5*time.Second, 10*time.Millisecond)
	st, _ := r.share.Status(r.ctx)
	assert.False(t, st.Connected)
	assert.Equal(t, 2, st.Waiting, "kept, in order")
	p := r.piece(run, "3001") // sorting carries on meanwhile

	r.site.mu.Lock()
	r.site.down = false
	r.site.mu.Unlock()
	require.Eventually(t, func() bool {
		st, _ := r.share.Status(r.ctx)
		return st.Waiting == 0
	}, 10*time.Second, 20*time.Millisecond, "caught up once the site is back")
	assert.Equal(t, []string{"PUT /api/push/collections/1", "PUT /api/push/runs/1", "PUT /api/push/pieces/" + p.ID}, r.site.seen())
}

func TestSharingDropsWhatTheSiteKeepsRefusing(t *testing.T) {
	r := newShareRig(t)
	col := newCollection(t, r.records, "Sam")
	run, err := r.records.StartRun(r.ctx, col, nil, "", time.Now())
	require.NoError(t, err)
	bad := r.piece(run, "3001")
	good := r.piece(run, "3003")
	r.site.refuse = "/api/push/pieces/" + bad.ID
	require.NoError(t, r.share.SetShared(r.ctx, col, true))
	r.sendAll()
	calls := r.site.seen()
	assert.Contains(t, calls, "PUT /api/push/pieces/"+good.ID, "the rest still goes")
	assert.NotContains(t, calls, "PUT /api/push/pieces/"+bad.ID)
}

func TestSharingAppliesReviewsAndClearsUnknowns(t *testing.T) {
	r := newShareRig(t)
	col := newCollection(t, r.records, "Sam")
	require.NoError(t, r.share.SetShared(r.ctx, col, true))
	run, err := r.records.StartRun(r.ctx, col, nil, "", time.Now())
	require.NoError(t, err)
	recolored := r.piece(run, "3001")
	wrong := r.piece(run, "3003")
	no, yes := false, true
	recs := sharedRecordsForTest{r.records, r.share}
	require.NoError(t, recs.SaveReview(r.ctx, recolored.ID, sorter.PieceReview{PartCorrect: &yes, ColorCorrect: &no, ColorID: "11", ReviewedAt: time.Now()}))
	require.NoError(t, recs.SaveReview(r.ctx, wrong.ID, sorter.PieceReview{PartCorrect: &no, ReviewedAt: time.Now()}))
	r.sendAll()

	var sent sorter.SharedPiece
	r.site.body("PUT /api/push/pieces/"+recolored.ID, &sent)
	assert.Equal(t, "3001", sent.PartID)
	assert.Equal(t, "11", sent.ColorID, "the reviewer's colour")
	r.site.body("PUT /api/push/pieces/"+wrong.ID, &sent)
	assert.Empty(t, sent.PartID, "a wrong part shows as unknown")

	// Clearing unknowns drops their waiting marks and tells the site.
	unknown := r.piece(run, "")
	_, err = r.records.ClearUnknowns(r.ctx, col)
	require.NoError(t, err)
	r.share.UnknownsCleared(r.ctx, col)
	r.sendAll()
	calls := r.site.seen()
	assert.NotContains(t, calls, "PUT /api/push/pieces/"+unknown.ID)
	assert.Equal(t, "POST /api/push/collections/1/clear-unknowns", calls[len(calls)-1])

	// Unsharing drops what's waiting.
	r.piece(run, "3020")
	require.NoError(t, r.share.SetShared(r.ctx, col, false))
	st, err := r.share.Status(r.ctx)
	require.NoError(t, err)
	assert.Zero(t, st.Waiting)
}

func TestSharingSettingsKeepTheKey(t *testing.T) {
	r := newShareRig(t)
	require.NoError(t, r.share.SaveSettings(r.ctx, "https://lego.n8p.xyz/", ""))
	set, err := r.store.SharingSettings(r.ctx)
	require.NoError(t, err)
	assert.Equal(t, "https://lego.n8p.xyz", set.URL)
	assert.Equal(t, "sv_test", set.APIKey, "an empty key keeps the saved one")
	assert.ErrorIs(t, r.share.SaveSettings(r.ctx, "lego.n8p.xyz", ""), sorter.ErrInvalidSettings)
}
