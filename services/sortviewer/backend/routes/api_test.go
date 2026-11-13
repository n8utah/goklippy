package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goklippy/services/sortviewer/backend/config"
	"goklippy/services/sortviewer/backend/logic"
	"goklippy/services/sortviewer/backend/models"
)

type testSite struct {
	t   *testing.T
	srv *httptest.Server
	svc *logic.Service
	key string
}

func newSite(t *testing.T) *testSite {
	t.Helper()
	store, err := models.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	svc := logic.NewService(store, t.TempDir())
	cfg := config.Config{AppURL: "http://localhost", DevAdmin: "nate@example.com"}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := &API{Svc: svc, Admin: logic.NewAdminSignIn(logic.OIDCConfig{}), Cfg: cfg, Commit: "test"}
	api.Register(r, func(c *gin.Context) { c.String(http.StatusOK, "ui") })
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	key, err := svc.CreateAPIKey(context.Background(), "bench sorter")
	require.NoError(t, err)
	return &testSite{t: t, srv: srv, svc: svc, key: key}
}

// client is a browser with its own cookies.
type client struct {
	site *testSite
	http *http.Client
	auth string // Authorization header, for the sorter
}

func (s *testSite) browser() *client {
	jar := newJar()
	return &client{site: s, http: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}}
}

func (s *testSite) sorter(key string) *client {
	return &client{site: s, http: http.DefaultClient, auth: "Bearer " + key}
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.site.t.Helper()
	var r *bytes.Reader
	contentType := "application/json"
	switch b := body.(type) {
	case nil:
		r = bytes.NewReader(nil)
	case []byte:
		r, contentType = bytes.NewReader(b), "image/jpeg"
	default:
		data, _ := json.Marshal(b)
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.site.srv.URL+path, r)
	require.NoError(c.site.t, err)
	req.Header.Set("Content-Type", contentType)
	if c.auth != "" {
		req.Header.Set("Authorization", c.auth)
	}
	resp, err := c.http.Do(req)
	require.NoError(c.site.t, err)
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func jpegBytes(t *testing.T, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, x%h, color.RGBA{200, 30, 30, 255})
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

const pieceA = "0a1b2c3d-0000-4000-8000-000000000001"
const pieceB = "0a1b2c3d-0000-4000-8000-000000000002"

// pushSort sends a small sort into collection 7: a run, an identified piece with a
// photo, and an unknown.
func (s *testSite) pushSort() {
	t := s.t
	sorter := s.sorter(s.key)
	plan := map[string]any{
		"profile": map[string]any{"name": "Bricks and plates"},
		"bins": []any{map[string]any{"layer": 1, "bin": 1, "categories": []any{map[string]any{"id": "r-bricks", "name": "Bricks"}}}},
	}
	code, _ := sorter.do("PUT", "/api/push/collections/7", map[string]any{"name": "Sam's LEGO", "plan": plan})
	require.Equal(t, 200, code)
	code, _ = sorter.do("PUT", "/api/push/runs/3", map[string]any{"collection_id": 7, "profile_name": "Bricks and plates", "started_at": "2026-11-13T10:00:00Z"})
	require.Equal(t, 200, code)
	piece := map[string]any{"collection_id": 7, "run_id": 3, "status": "distributed", "seen_at": "2026-11-13T10:01:00Z",
		"sorted_at": "2026-11-13T10:01:05Z", "part_id": "3001", "part_name": "Brick 2 x 4", "color_id": "5",
		"color_name": "Red", "category": "r-bricks", "category_name": "Bricks", "bin_layer": 1, "bin_number": 1}
	code, _ = sorter.do("PUT", "/api/push/pieces/"+pieceA, piece)
	require.Equal(t, 200, code)
	code, _ = sorter.do("PUT", "/api/push/pieces/"+pieceA, piece) // again: no change
	require.Equal(t, 200, code)
	code, _ = sorter.do("PUT", "/api/push/pieces/"+pieceA+"/photo", jpegBytes(t, 1600, 1200))
	require.Equal(t, 200, code)
	code, _ = sorter.do("PUT", "/api/push/pieces/"+pieceB, map[string]any{"collection_id": 7, "run_id": 3,
		"status": "distributed", "seen_at": "2026-11-13T10:02:00Z", "sorted_at": "2026-11-13T10:02:05Z"})
	require.Equal(t, 200, code)
}

func TestPushNeedsAValidKey(t *testing.T) {
	s := newSite(t)
	code, _ := s.sorter("sv_wrong").do("GET", "/api/push/ping", nil)
	assert.Equal(t, 401, code)
	code, _ = s.browser().do("PUT", "/api/push/collections/1", map[string]any{"name": "x"})
	assert.Equal(t, 401, code)
	code, _ = s.sorter(s.key).do("GET", "/api/push/ping", nil)
	assert.Equal(t, 200, code)

	// A revoked key stops working.
	keys, err := s.svc.APIKeys(context.Background())
	require.NoError(t, err)
	require.NoError(t, s.svc.RevokeAPIKey(context.Background(), keys[0].ID))
	code, _ = s.sorter(s.key).do("GET", "/api/push/ping", nil)
	assert.Equal(t, 401, code)
}

func TestPushedSortShowsToItsViewersOnly(t *testing.T) {
	s := newSite(t)
	s.pushSort()
	admin := s.browser()
	code, _ := admin.do("GET", "/auth/login", nil) // dev admin: signed straight in
	require.Equal(t, http.StatusFound, code)

	code, _ = admin.do("POST", "/api/admin/viewers", map[string]any{"email": "Sam@Example.com", "name": "Sam",
		"password": "lego-lego", "collections": []int{7}})
	require.Equal(t, 201, code)
	code, _ = admin.do("POST", "/api/admin/viewers", map[string]any{"email": "alex@example.com", "password": "bricks-bricks"})
	require.Equal(t, 201, code)

	sam := s.browser()
	code, _ = sam.do("GET", "/api/collections", nil)
	assert.Equal(t, 401, code, "signed out")
	code, _ = sam.do("POST", "/api/auth/login", map[string]any{"email": "sam@example.com", "password": "wrong-one"})
	assert.Equal(t, 401, code)
	code, _ = sam.do("POST", "/api/auth/login", map[string]any{"email": "SAM@example.com", "password": "lego-lego"})
	require.Equal(t, 200, code, "email in any case")

	_, body := sam.do("GET", "/api/collections", nil)
	cols := body["collections"].([]any)
	require.Len(t, cols, 1)
	col := cols[0].(map[string]any)
	assert.Equal(t, "Sam's LEGO", col["name"])
	assert.EqualValues(t, 2, col["pieces"])
	assert.EqualValues(t, 1, col["identified"])
	assert.EqualValues(t, 1, col["unknown"])
	assert.Equal(t, true, col["sorting"], "the run hasn't ended")

	_, body = sam.do("GET", "/api/collections/7/parts", nil)
	parts := body["parts"].([]any)
	require.Len(t, parts, 1)
	assert.Equal(t, pieceA, parts[0].(map[string]any)["photo_piece_id"])
	_, body = sam.do("GET", "/api/collections/7/pieces?unknown=1", nil)
	assert.EqualValues(t, 1, body["total"])
	_, body = sam.do("GET", "/api/collections/7/plan", nil)
	assert.Len(t, body["counts"], 2, "bin 1, and the bucket")

	req, _ := http.NewRequest("GET", s.srv.URL+"/api/photos/"+pieceA, nil)
	resp, err := sam.http.Do(req)
	require.NoError(t, err)
	img, err := jpeg.DecodeConfig(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, 800, img.Width, "shrunk to 800 on its longest side")

	// Alex wasn't given it: it doesn't exist as far as Alex can tell.
	alex := s.browser()
	code, _ = alex.do("POST", "/api/auth/login", map[string]any{"email": "alex@example.com", "password": "bricks-bricks"})
	require.Equal(t, 200, code)
	_, body = alex.do("GET", "/api/collections", nil)
	assert.Empty(t, body["collections"])
	for _, path := range []string{"/api/collections/7", "/api/collections/7/pieces", "/api/collections/7/plan", "/api/photos/" + pieceA} {
		code, _ = alex.do("GET", path, nil)
		assert.Equal(t, 404, code, path)
	}
	code, _ = alex.do("GET", "/api/admin/viewers", nil)
	assert.Equal(t, 403, code, "viewers aren't admins")
	code, _ = alex.do("PUT", "/api/push/collections/7", map[string]any{"name": "mine"})
	assert.Equal(t, 401, code, "viewers can't push")

	// The sorter clears the unknowns to re-run them.
	code, body = s.sorter(s.key).do("POST", "/api/push/collections/7/clear-unknowns", nil)
	require.Equal(t, 200, code)
	assert.EqualValues(t, 1, body["cleared"])
	_, body = sam.do("GET", "/api/collections/7", nil)
	assert.EqualValues(t, 0, body["unknown"])
}

func TestPushRejectsBadInput(t *testing.T) {
	s := newSite(t)
	sorter := s.sorter(s.key)
	code, _ := sorter.do("PUT", "/api/push/collections/1", map[string]any{"name": "  "})
	assert.Equal(t, 400, code)
	code, _ = sorter.do("PUT", "/api/push/pieces/..%5C..%5Cetc", map[string]any{"collection_id": 1, "seen_at": "2026-11-13T10:00:00Z"})
	assert.Equal(t, 400, code, "piece ids are letters, digits and dashes")
	code, _ = sorter.do("PUT", "/api/push/pieces/"+pieceA+"/photo", []byte("not a photo"))
	assert.Equal(t, 404, code, "no such piece yet")
	code, _ = sorter.do("PUT", "/api/push/pieces/"+pieceA, map[string]any{"collection_id": 1, "seen_at": "2026-11-13T10:00:00Z"})
	require.Equal(t, 200, code)
	code, _ = sorter.do("PUT", "/api/push/pieces/"+pieceA+"/photo", []byte("not a photo"))
	assert.Equal(t, 400, code)
}

func TestCrossSiteChangesAreRefused(t *testing.T) {
	s := newSite(t)
	req, _ := http.NewRequest("POST", s.srv.URL+"/api/auth/login", strings.NewReader(`{"email":"a@b.c","password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 403, resp.StatusCode)
}

func TestLoginIsRateLimited(t *testing.T) {
	s := newSite(t)
	b := s.browser()
	last := 0
	for i := 0; i < 7; i++ {
		last, _ = b.do("POST", "/api/auth/login", map[string]any{"email": "nobody@example.com", "password": "guess-guess"})
	}
	assert.Equal(t, 429, last)
}
