package profiles

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Save writes a profile YAML file to the profiles directory.
func Save(profilesDir string, profile Profile) error {
	if profile.ID == "" {
		return fmt.Errorf("profile id is required")
	}
	path := filepath.Join(profilesDir, profile.ID+".yaml")
	data, err := yaml.Marshal(&profile)
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	return nil
}

// SaveHotspots updates diagram hotspots for a board profile and persists the YAML.
func SaveHotspots(profilesDir, id string, hotspots []Hotspot) error {
	profile, err := LoadByID(profilesDir, id)
	if err != nil {
		return err
	}
	profile.Diagram.Hotspots = hotspots
	return Save(profilesDir, profile)
}
