package applog

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir, 24*time.Hour)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

// Records through slog are kept with their attributes and read back newest first,
// filtered by level, text and time; Debug isn't kept.
func TestLogsAreKeptAndFiltered(t *testing.T) {
	s, _ := open(t)
	log := slog.New(s.Handler(slog.LevelInfo)).With("component", "sorter")
	tick := func() { time.Sleep(3 * time.Millisecond) } // distinct times
	log.Debug("too chatty")
	log.Info("sort run started", "run", 35)
	tick()
	log.Warn("feeder move refused", "channel", "c3", "error", "busy")
	tick()
	log.WithGroup("mcu").Error("mcu reported shutdown", "port", "COM4")

	all, more := s.Query(Filter{})
	require.Len(t, all, 3, "debug isn't kept")
	assert.False(t, more)
	assert.Equal(t, "mcu reported shutdown", all[0].Msg, "newest first")
	assert.Equal(t, "COM4", all[0].Attrs["mcu.port"])
	assert.Equal(t, "sorter", all[0].Attrs["component"])
	assert.Equal(t, "35", all[2].Attrs["run"])

	warn, _ := s.Query(Filter{MinLevel: "warn"})
	assert.Len(t, warn, 2)
	found, _ := s.Query(Filter{Search: "C3"})
	require.Len(t, found, 1, "attributes are searched too, any case")
	assert.Equal(t, "feeder move refused", found[0].Msg)

	page, more := s.Query(Filter{Limit: 2})
	assert.Len(t, page, 2)
	assert.True(t, more)
	older, more := s.Query(Filter{Before: page[1].Time, Limit: 2})
	assert.False(t, more)
	require.Len(t, older, 1)
	assert.Equal(t, "sort run started", older[0].Msg)
	newer, _ := s.Query(Filter{Since: all[1].Time})
	require.Len(t, newer, 1)
	assert.Equal(t, "mcu reported shutdown", newer[0].Msg)
}

// A message logged over and over is kept a few times a minute, then summed up.
func TestRepeatsAreCollapsed(t *testing.T) {
	s, _ := open(t)
	t0 := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
	for i := range 40 {
		s.Add(slog.LevelWarn, Entry{Time: t0.Add(time.Duration(i) * time.Second), Level: "WARN", Msg: "camera stopped; restarting"})
	}
	// Different requests with the same message aren't repeats of each other.
	for i := range 3 {
		s.Add(slog.LevelInfo, Entry{Time: t0.Add(time.Duration(i) * time.Second), Level: "INFO", Msg: "http request completed",
			Attrs: map[string]string{"method": "POST", "path": []string{"/a", "/b", "/c"}[i]}})
	}
	s.Add(slog.LevelInfo, Entry{Time: t0.Add(61 * time.Second), Level: "INFO", Msg: "later"})

	got, _ := s.Query(Filter{Search: "camera stopped"})
	require.Len(t, got, repeatBurst+1)
	assert.Contains(t, got[0].Attrs["repeated"], "35 more times")
	http, _ := s.Query(Filter{Search: "http"})
	assert.Len(t, http, 3)
}

// Hourly files wholly older than the window are removed.
func TestOldFilesArePruned(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, time.Now().UTC().Add(-26*time.Hour).Format(fileLayout)+fileSuffix)
	recent := filepath.Join(dir, time.Now().UTC().Add(-2*time.Hour).Format(fileLayout)+fileSuffix)
	require.NoError(t, os.WriteFile(old, []byte("{}\n"), 0o644))
	require.NoError(t, os.WriteFile(recent, []byte("{}\n"), 0o644))
	s, err := Open(dir, 24*time.Hour)
	require.NoError(t, err)
	defer s.Close()
	assert.NoFileExists(t, old)
	assert.FileExists(t, recent)

	require.NoError(t, s.Clear())
	assert.NoFileExists(t, recent)
}
