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

// settingsOnlyShareStore is a ShareStore that only knows the site's settings.
type settingsOnlyShareStore struct {
	ShareStore
	set SharingSettings
}

func (s *settingsOnlyShareStore) SharingSettings(context.Context) (SharingSettings, error) {
	return s.set, nil
}

// fakeViewer answers /api/push/sets/:set like the sort viewer site.
func fakeViewer(t *testing.T, ready bool) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer site-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case !ready:
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error": "the set data isn't ready yet"}`))
		case r.URL.Path == "/api/push/sets/1234-1":
			_, _ = w.Write([]byte(`{"set_num": "1234-1", "name": "Small house", "year": 2020, "img_url": "https://img/1234.jpg", "parts": [
			  {"part_num": "3001", "color_name": "Red", "need": 4},
			  {"part_num": "3069b", "color_name": "Light Bluish Gray", "need": 2},
			  {"part_num": "3001", "color_name": "red", "need": 1},
			  {"part_num": "x99", "color_name": "Odd Glitter Mauve", "need": 1},
			  {"part_num": "x100", "color_name": "[Unknown]", "need": 3}
			]}`))
		case r.URL.Path == "/api/push/sets/old-1": // a site from before set lookups
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "no such endpoint"}`))
		case r.URL.Path == "/api/push/sets/older-1":
			http.NotFound(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestViewerLookupSet(t *testing.T) {
	ctx := context.Background()
	srv := fakeViewer(t, true)
	share := NewSharing(&settingsOnlyShareStore{set: SharingSettings{URL: srv.URL, APIKey: "site-key"}})
	// A colour the sorter has identified, beyond the usual ones.
	share.Attach(nil, nil, nil, func(context.Context) (map[string]string, error) {
		return map[string]string{"1001": "Odd Glitter Mauve"}, nil
	})

	set, err := share.LookupSet(ctx, "1234-1")
	require.NoError(t, err)
	assert.Equal(t, "Small house", set.Name)
	assert.Equal(t, 2020, set.Year)
	assert.Equal(t, "viewer", set.Source)
	assert.Equal(t, []profile.KitLine{
		{Part: "3001", Color: "5", Need: 5}, // "Red" and "red" are the same colour
		{Part: "3069b", Color: "86", Need: 2},
		{Part: "x100", Color: "", Need: 3}, // unknown colour: any colour, not a problem
		{Part: "x99", Color: "1001", Need: 1},
	}, set.Parts)
	assert.Empty(t, set.Unmapped)

	// Without the identified colour, it can't be named: any colour, and said so.
	share.Attach(nil, nil, nil, nil)
	set, err = share.LookupSet(ctx, "1234-1")
	require.NoError(t, err)
	assert.Contains(t, set.Parts, profile.KitLine{Part: "x99", Need: 1})
	assert.Len(t, set.Unmapped, 1)

	_, err = share.LookupSet(ctx, "9999-1")
	assert.ErrorIs(t, err, ErrSetNotFound)
	_, err = share.LookupSet(ctx, "old-1")
	assert.ErrorIs(t, err, ErrViewerTooOld)
	_, err = share.LookupSet(ctx, "older-1")
	assert.ErrorIs(t, err, ErrViewerTooOld)

	_, err = NewSharing(&settingsOnlyShareStore{}).LookupSet(ctx, "1234-1")
	assert.ErrorIs(t, err, ErrSharingNotSetUp)
	_, err = NewSharing(&settingsOnlyShareStore{set: SharingSettings{URL: fakeViewer(t, false).URL, APIKey: "site-key"}}).LookupSet(ctx, "1234-1")
	assert.ErrorIs(t, err, ErrViewerSetsNotReady)
}

// The viewer site comes first; the Rebrickable key is the backup when it can't answer.
func TestLookupSetViewerThenRebrickable(t *testing.T) {
	ctx := context.Background()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lego/sets/1234-1/":
			_, _ = w.Write([]byte(`{"set_num": "1234-1", "name": "From the API"}`))
		case "/lego/sets/1234-1/parts/":
			_, _ = w.Write([]byte(`{"next": null, "results": []}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer api.Close()
	viewer := fakeViewer(t, true)
	store := &memRebrickableStore{}
	siteStore := &settingsOnlyShareStore{set: SharingSettings{URL: viewer.URL, APIKey: "site-key"}}
	rb := NewRebrickable(store).WithBaseURL(api.URL).WithViewer(NewSharing(siteStore))

	set, err := rb.LookupSet(ctx, "1234")
	require.NoError(t, err)
	assert.Equal(t, "viewer", set.Source, "the viewer site first, no key needed")

	// The site isn't ready and there's no key: say why.
	siteStore.set.URL = fakeViewer(t, false).URL
	_, err = rb.LookupSet(ctx, "1234")
	assert.ErrorIs(t, err, ErrViewerSetsNotReady)

	// With a key, Rebrickable answers instead.
	require.NoError(t, rb.SaveKey(ctx, "secret"))
	set, err = rb.LookupSet(ctx, "1234")
	require.NoError(t, err)
	assert.Equal(t, "rebrickable", set.Source)
	assert.Equal(t, "From the API", set.Name)

	// Not connected and no key: where sets come from.
	siteStore.set = SharingSettings{}
	require.NoError(t, rb.SaveKey(ctx, ""))
	_, err = rb.LookupSet(ctx, "1234")
	assert.ErrorIs(t, err, ErrNoSetSource)
}
