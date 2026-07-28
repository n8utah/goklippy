package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSaveHotspots(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join("..", "profiles", "ramps14.yaml")
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ramps14.yaml"), data, 0o644))

	hotspots := []Hotspot{
		{
			ID:      "test_hotspot",
			SlotIDs: []string{"stepper_x"},
			Label:   "Test X",
			Shape:   "rect",
			X:       10,
			Y:       20,
			Width:   30,
			Height:  40,
			Accepts: []string{"stepper"},
		},
	}

	require.NoError(t, SaveHotspots(dir, "ramps14", hotspots))

	loaded, err := LoadByID(dir, "ramps14")
	require.NoError(t, err)
	require.Len(t, loaded.Diagram.Hotspots, 1)
	require.Equal(t, "test_hotspot", loaded.Diagram.Hotspots[0].ID)
	require.Equal(t, "RAMPS 1.4", loaded.Name)
}
