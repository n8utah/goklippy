package sorter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type photoStoreStub struct {
	RecordsStore
	cleared int
}

func (s *photoStoreStub) ClearPieceImages(context.Context) (int, error) {
	s.cleared++
	return 1, nil
}

// Clearing photos removes the files under pieces/ and forgets them, leaving the rest of
// the data folder alone.
func TestClearPhotos(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "pieces", "ab", "abc", "00.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(photo), 0o755))
	require.NoError(t, os.WriteFile(photo, []byte("12345"), 0o644))
	other := filepath.Join(dir, "backgrounds", "c2.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(other), 0o755))
	require.NoError(t, os.WriteFile(other, []byte("x"), 0o644))

	store := &photoStoreStub{}
	r := NewRecords(store, dir)
	assert.Equal(t, PhotoUsage{Files: 1, Bytes: 5}, r.Photos())
	freed, err := r.ClearPhotos(context.Background())
	require.NoError(t, err)
	assert.Equal(t, PhotoUsage{Files: 1, Bytes: 5}, freed)
	assert.Equal(t, 1, store.cleared)
	assert.NoFileExists(t, photo)
	assert.FileExists(t, other)
	assert.Equal(t, PhotoUsage{}, r.Photos())
}
