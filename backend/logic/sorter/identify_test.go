package sorter

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http"
	"testing"

	"goklippy/backend/logic/brickognize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeIdentifier struct {
	got    [][]byte
	result brickognize.Result
}

func (f *fakeIdentifier) Identify(_ context.Context, jpegs [][]byte) (brickognize.Result, error) {
	f.got = jpegs
	return f.result, nil
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))))
	return buf.Bytes()
}

func TestIdentifyAndRouteWithoutProfile(t *testing.T) {
	id := &fakeIdentifier{result: brickognize.Result{
		Items:  []brickognize.Item{{ID: "3001", Name: "Brick 2 x 4"}},
		Colors: []brickognize.Color{{ID: "5", Name: "Red"}},
	}}
	sorting := NewSortingService(context.Background(), &memSortingStore{}, DefaultMachineSettings)

	res, err := IdentifyAndRoute(context.Background(), id, sorting, [][]byte{pngBytes(t)}, 0)
	require.NoError(t, err)
	require.Len(t, id.got, 1)
	assert.Equal(t, "image/jpeg", http.DetectContentType(id.got[0]), "PNG converted to JPEG")
	assert.Equal(t, "3001", res.Brickognize.Items[0].ID)
	assert.Nil(t, res.Route)
	assert.Contains(t, res.RouteError, "no sorting profile")
}

func TestIdentifyAndRouteRejectsOtherFiles(t *testing.T) {
	sorting := NewSortingService(context.Background(), &memSortingStore{}, DefaultMachineSettings)
	_, err := IdentifyAndRoute(context.Background(), &fakeIdentifier{}, sorting, [][]byte{[]byte("GIF89a....")}, 0)
	assert.ErrorIs(t, err, ErrBadImage)
}

func TestIdentifyAndRouteNoPart(t *testing.T) {
	sorting := NewSortingService(context.Background(), &memSortingStore{}, DefaultMachineSettings)
	res, err := IdentifyAndRoute(context.Background(), &fakeIdentifier{}, sorting, [][]byte{pngBytes(t)}, 0)
	require.NoError(t, err)
	assert.Contains(t, res.RouteError, "no part")
}

// memSortingStore is an empty store: no profiles, and none can be added.
type memSortingStore struct{ SortingStore }

func (memSortingStore) ListProfiles(context.Context) ([]StoredProfile, error) { return nil, nil }

func (memSortingStore) CreateProfile(context.Context, StoredProfile, []byte) (int64, error) {
	return 0, errors.New("read-only store")
}

func (memSortingStore) ColorNames(context.Context) (map[string]string, error) { return nil, nil }

func (memSortingStore) CurrentCollection(context.Context) (CollectionRef, bool, error) {
	return CollectionRef{}, false, nil
}
