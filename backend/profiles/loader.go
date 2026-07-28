package profiles

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Profile is a supported control board definition.
type Profile struct {
	ID      string          `yaml:"id" json:"id"`
	Name    string          `yaml:"name" json:"name"`
	MCU     string          `yaml:"mcu" json:"mcu"`
	Serial  SerialConfig    `yaml:"serial" json:"serial"`
	Diagram DiagramConfig   `yaml:"diagram" json:"diagram"`
	Slots   map[string]Slot `yaml:"slots" json:"slots"`
}

type SerialConfig struct {
	Baud int `yaml:"baud" json:"baud"`
}

type DiagramConfig struct {
	Image     string    `yaml:"image" json:"image"`
	ImageType string    `yaml:"image_type" json:"image_type"`
	ImageURL  string    `yaml:"image_url,omitempty" json:"image_url,omitempty"`
	Width     float64   `yaml:"width" json:"width"`
	Height    float64   `yaml:"height" json:"height"`
	ViewBox   string    `yaml:"viewBox" json:"viewBox"`
	Hotspots  []Hotspot `yaml:"hotspots" json:"hotspots"`
}

type Hotspot struct {
	ID      string   `yaml:"id" json:"id"`
	SlotIDs []string `yaml:"slot_ids" json:"slot_ids"`
	Label   string   `yaml:"label" json:"label"`
	Shape   string   `yaml:"shape" json:"shape"`
	Accepts []string `yaml:"accepts" json:"accepts"`
	X       float64  `yaml:"x,omitempty" json:"x,omitempty"`
	Y       float64  `yaml:"y,omitempty" json:"y,omitempty"`
	Width   float64  `yaml:"width,omitempty" json:"width,omitempty"`
	Height  float64  `yaml:"height,omitempty" json:"height,omitempty"`
	CX      float64  `yaml:"cx,omitempty" json:"cx,omitempty"`
	CY      float64  `yaml:"cy,omitempty" json:"cy,omitempty"`
	R       float64  `yaml:"r,omitempty" json:"r,omitempty"`
}

type Slot struct {
	Type     string                 `yaml:"type" json:"type"`
	Group    string                 `yaml:"group" json:"group"`
	Label    string                 `yaml:"label" json:"label"`
	Pins     map[string]string      `yaml:"pins" json:"pins"`
	Defaults map[string]interface{} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
}

// Load reads a board profile YAML file.
func Load(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}

	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	if profile.ID == "" {
		return Profile{}, fmt.Errorf("profile id is required")
	}
	return profile, nil
}

// LoadByID loads a built-in profile from the profiles directory.
func LoadByID(profilesDir, id string) (Profile, error) {
	path := filepath.Join(profilesDir, id+".yaml")
	return Load(path)
}

// List returns profile ids available in a directory.
func List(profilesDir string) ([]string, error) {
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".yaml" {
			continue
		}
		ids = append(ids, name[:len(name)-len(".yaml")])
	}
	return ids, nil
}
