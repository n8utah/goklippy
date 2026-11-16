package sorter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goklippy/backend/logic/sorter/profile"
)

type memRebrickableStore struct{ key string }

func (m *memRebrickableStore) RebrickableKey(context.Context) (string, error) { return m.key, nil }
func (m *memRebrickableStore) SaveRebrickableKey(_ context.Context, k string) error {
	m.key = k
	return nil
}

func TestRebrickableLookupSet(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "key secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/lego/sets/1234-1/":
			_, _ = w.Write([]byte(`{"set_num": "1234-1", "name": "Small house", "year": 2020, "num_parts": 9, "set_img_url": "https://img/1234.jpg"}`))
		case r.URL.Path == "/lego/sets/1234-1/parts/" && r.URL.Query().Get("page") == "":
			_, _ = w.Write([]byte(`{"next": "` + srv.URL + `/lego/sets/1234-1/parts/?page=2", "results": [
			  {"part": {"part_num": "3001", "external_ids": {"BrickLink": ["3001"]}}, "color": {"id": 4, "name": "Red", "external_ids": {"BrickLink": {"ext_ids": [5], "ext_descrs": [["Red"]]}}}, "quantity": 4, "is_spare": false},
			  {"part": {"part_num": "3001", "external_ids": {"BrickLink": ["3001"]}}, "color": {"id": 4, "name": "Red", "external_ids": {"BrickLink": {"ext_ids": [5]}}}, "quantity": 1, "is_spare": true},
			  {"part": {"part_num": "3069b", "external_ids": {}}, "color": {"id": 0, "name": "Black", "external_ids": {"BrickLink": {"ext_ids": [11]}}}, "quantity": 2, "is_spare": false}
			]}`))
		case r.URL.Path == "/lego/sets/1234-1/parts/":
			_, _ = w.Write([]byte(`{"next": null, "results": [
			  {"part": {"part_num": "3001", "external_ids": {"BrickLink": ["3001"]}}, "color": {"id": 4, "name": "Red", "external_ids": {"BrickLink": {"ext_ids": [5]}}}, "quantity": 2, "is_spare": false},
			  {"part": {"part_num": "x99", "external_ids": {"BrickLink": ["x99a"]}}, "color": {"id": 9999, "name": "Odd", "external_ids": {}}, "quantity": 1, "is_spare": false}
			]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	store := &memRebrickableStore{}
	rb := NewRebrickable(store).WithBaseURL(srv.URL)
	ctx := context.Background()

	_, err := rb.LookupSet(ctx, "1234")
	assert.ErrorIs(t, err, ErrNoSetSource)

	require.NoError(t, rb.SaveKey(ctx, " secret "))
	set, err := rb.LookupSet(ctx, "1234")
	require.NoError(t, err)
	assert.Equal(t, "1234-1", set.Number)
	assert.Equal(t, "Small house", set.Name)
	assert.Equal(t, []profile.KitLine{
		{Part: "3001", Color: "5", Need: 6}, // both pages, no spare
		{Part: "3069b", Color: "11", Need: 2},
		{Part: "x99a", Color: "", Need: 1}, // colour unmapped: any colour
	}, set.Parts)
	assert.Len(t, set.Unmapped, 1)

	_, err = rb.LookupSet(ctx, "9999")
	assert.ErrorIs(t, err, ErrSetNotFound)
	_, err = rb.LookupSet(ctx, "not a set!")
	assert.ErrorIs(t, err, profile.ErrInvalid)

	require.NoError(t, rb.SaveKey(ctx, "wrong"))
	_, err = rb.LookupSet(ctx, "1234-1")
	assert.ErrorIs(t, err, ErrInvalidSettings)
}
