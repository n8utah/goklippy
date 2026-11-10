//go:build mage

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/sh"
	"gopkg.in/yaml.v3"
)

type runtimeConfig struct {
	Database runtimeDatabaseConf `yaml:"database"`
}

type runtimeDatabaseConf struct {
	Path string `yaml:"path"`
}

// Build compiles backend and frontend artifacts.
func Build() error {
	if err := BuildBackend(); err != nil {
		return err
	}
	return BuildFrontend()
}

// BuildBackend compiles the Go backend.
func BuildBackend() error {
	fmt.Println("Building backend...")
	return sh.RunV("go", "-C", "backend", "build", "./...")
}

// TestBackend runs backend unit tests.
func TestBackend() error {
	fmt.Println("Testing backend...")
	return sh.RunV("go", "-C", "backend", "test", "./...")
}

// McuProbe runs the MCU identify CLI against --port (pass args after mage target).
func McuProbe() error {
	args := os.Args[2:]
	cmd := append([]string{"run", "./cmd/mcu-probe"}, args...)
	return sh.RunV("go", append([]string{"-C", "backend"}, cmd...)...)
}

// BuildFrontend installs dependencies and builds the Vite app.
func BuildFrontend() error {
	fmt.Println("Building frontend...")
	if err := os.Chdir("frontend"); err != nil {
		return err
	}
	defer os.Chdir("..")

	if err := sh.RunV("npm", "install"); err != nil {
		return err
	}
	return sh.RunV("npm", "run", "build")
}

// TestFrontend is a placeholder until test scripts are added.
func TestFrontend() error {
	fmt.Println("No frontend tests configured yet.")
	return nil
}

// Deploy is intentionally a placeholder until server targets are finalized.
func Deploy() error {
	fmt.Println("Deploy pipeline not configured yet for this project.")
	return nil
}

// AtlasPush prompts for an environment and applies the shared schema to that environment's
// SQLite database. The backend already creates new tables on startup; use this for changes
// to existing tables (new or altered columns), which CREATE TABLE IF NOT EXISTS can't make.
func AtlasPush() error {
	env, configPath, err := promptEnvironment()
	if err != nil {
		return err
	}

	databaseURL, err := loadDatabaseURL(configPath)
	if err != nil {
		return err
	}

	fmt.Printf("Applying schema.sql to %s environment (%s)\n", env, databaseURL)
	fmt.Println("Atlas will now show schema changes and prompt for confirmation before applying.")
	return sh.RunV(
		"atlas",
		"schema",
		"apply",
		"--to",
		"file://database/schema.sql",
		"--url",
		databaseURL,
		"--dev-url",
		atlasDevURL,
	)
}

func promptEnvironment() (string, string, error) {
	configs := map[string]string{
		"dev":      filepath.FromSlash("private/config-dev.yaml"),
		"autotest": filepath.FromSlash("private/config-autotest.yaml"),
		"prod":     filepath.FromSlash("private/config-prod.yaml"),
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Select environment to target:")
	fmt.Println("1) dev")
	fmt.Println("2) autotest")
	fmt.Println("3) prod")
	fmt.Print("Enter choice [1-3] (default 1): ")

	raw, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	choice := strings.TrimSpace(raw)

	env := "dev"
	switch choice {
	case "", "1":
		env = "dev"
	case "2":
		env = "autotest"
	case "3":
		env = "prod"
	default:
		return "", "", fmt.Errorf("invalid choice %q", choice)
	}

	return env, configs[env], nil
}

func loadRuntimeConfig(configPath string) (runtimeConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return runtimeConfig{}, fmt.Errorf("read config %s: %w", configPath, err)
	}

	var cfg runtimeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return runtimeConfig{}, fmt.Errorf("parse config %s: %w", configPath, err)
	}

	return cfg, nil
}

// atlasDevURL is the scratch database Atlas diffs against: an in-memory SQLite.
const atlasDevURL = "sqlite://dev?mode=memory"

// defaultDatabasePath matches the backend's default (config.applyDefaults).
const defaultDatabasePath = "../private/goklippy.db"

// loadDatabaseURL is the environment's SQLite file as an Atlas URL. The config path is
// relative to backend/, where the backend runs; mage runs from the repo root.
func loadDatabaseURL(configPath string) (string, error) {
	cfg, err := loadRuntimeConfig(configPath)
	if err != nil {
		return "", err
	}
	path := cfg.Database.Path
	if path == "" {
		path = defaultDatabasePath
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join("backend", path)
	}
	return "sqlite://" + filepath.ToSlash(filepath.Clean(path)), nil
}
