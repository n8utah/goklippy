package sorter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type incidentStoreStub struct {
	RecordsStore
	cams     map[int64][]IncidentCamera
	paths    map[string]string
	resolved []int64
}

func (s *incidentStoreStub) AddIncidentCamera(_ context.Context, id int64, cam IncidentCamera, path string) error {
	s.cams[id] = append(s.cams[id], cam)
	s.paths[string(cam.Channel)] = path
	return nil
}

func (s *incidentStoreStub) IncidentCameraPath(_ context.Context, _ int64, ch Channel) (string, error) {
	if p := s.paths[string(ch)]; p != "" {
		return p, nil
	}
	return "", ErrIncidentImageNotFound
}

func (s *incidentStoreStub) ClearIncidentCameras(context.Context) (int, error) {
	n := len(s.cams)
	s.cams = map[int64][]IncidentCamera{}
	return n, nil
}

func (s *incidentStoreStub) DeleteIncidentsExcept(context.Context, int64) ([]int64, error) {
	return s.resolved, nil
}

// Pictures are written under incidents/<id>/, served from there, and cleared on their
// own or with their incident.
func TestIncidentPicturesAreKeptAndCleared(t *testing.T) {
	dir := t.TempDir()
	store := &incidentStoreStub{cams: map[int64][]IncidentCamera{}, paths: map[string]string{}, resolved: []int64{7}}
	r := NewRecords(store, dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.Run(ctx)

	r.saveIncidentCameras(7, []incidentShot{
		{cam: IncidentCamera{Channel: ChannelC1, State: IncidentChannelState{Doing: "waiting"}}},
		{cam: IncidentCamera{Channel: ChannelC4}, jpeg: []byte("jpeg!")},
	})
	r.Flush()
	require.Len(t, store.cams[7], 2)
	assert.False(t, store.cams[7][0].Image)
	assert.True(t, store.cams[7][1].Image)
	full, err := r.IncidentImage(ctx, 7, ChannelC4)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "incidents", "7", "c4.jpg"), full)
	assert.Equal(t, PhotoUsage{Files: 1, Bytes: 5}, r.IncidentPictures())

	n, err := r.ClearIncidents(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.NoDirExists(t, filepath.Join(dir, "incidents", "7"))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "incidents", "8"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "incidents", "8", "c2.jpg"), []byte("xy"), 0o644))
	freed, err := r.ClearIncidentPictures(ctx)
	require.NoError(t, err)
	assert.Equal(t, PhotoUsage{Files: 1, Bytes: 2}, freed)
	assert.NoDirExists(t, filepath.Join(dir, "incidents"))
}
