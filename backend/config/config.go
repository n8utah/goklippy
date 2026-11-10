package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port        int               `yaml:"port"`
	Database    DatabaseConfig    `yaml:"database"`
	JWT         JWTConfig         `yaml:"jwt"`
	InitialUser InitialUserConfig `yaml:"initial_user"`
	Frontend    FrontendConfig    `yaml:"frontend"`
	MCU         MCUConfig         `yaml:"mcu"`
	Klipper     KlipperConfig     `yaml:"klipper"`
	Sorter      SorterConfig      `yaml:"sorter"`
}

// SorterConfig controls the LEGO sorter's control loop.
type SorterConfig struct {
	// Simulate runs the sorter against simulated hardware instead of the MCU, for
	// working on the sorter without the machine attached.
	Simulate bool `yaml:"simulate"`
	// DataDir holds piece photos, relative to where the backend runs (backend/).
	DataDir string `yaml:"data_dir"`
}

type MCUConfig struct {
	DefaultBaud   int `yaml:"default_baud"`
	ScanTimeoutMs int `yaml:"scan_timeout_ms"`
}

type KlipperConfig struct {
	ConfigRoot   string `yaml:"config_root"`
	ProfilesDir  string `yaml:"profiles_dir"`
}

// DatabaseConfig locates the SQLite database. Paths are relative to where the backend
// runs (backend/).
type DatabaseConfig struct {
	Path       string `yaml:"path"`        // the database file; created if missing
	SchemaPath string `yaml:"schema_path"` // database/schema.sql, applied on startup
}

type JWTConfig struct {
	AccessSecret   string `yaml:"access_secret"`
	RefreshSecret  string `yaml:"refresh_secret"`
	AccessTTLMin   int    `yaml:"access_ttl_minutes"`
	RefreshTTLHour int    `yaml:"refresh_ttl_hours"`
}

type InitialUserConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
	Role     string `yaml:"role"`
}

type FrontendConfig struct {
	Mode    string `yaml:"mode"`
	DevURL  string `yaml:"dev_url"`
	DistDir string `yaml:"dist_dir"`
}

func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, fmt.Errorf("config path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config yaml: %w", err)
	}

	applyDefaults(&cfg)

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "../private/goklippy.db"
	}
	if cfg.Sorter.DataDir == "" {
		cfg.Sorter.DataDir = "../private/data"
	}
	if cfg.Database.SchemaPath == "" {
		cfg.Database.SchemaPath = "../database/schema.sql"
	}

	if cfg.JWT.AccessSecret == "" {
		cfg.JWT.AccessSecret = "dev-access-secret-change-me"
	}
	if cfg.JWT.RefreshSecret == "" {
		cfg.JWT.RefreshSecret = "dev-refresh-secret-change-me"
	}
	if cfg.JWT.AccessTTLMin == 0 {
		cfg.JWT.AccessTTLMin = 15
	}
	if cfg.JWT.RefreshTTLHour == 0 {
		cfg.JWT.RefreshTTLHour = 168
	}

	if cfg.InitialUser.Role == "" {
		cfg.InitialUser.Role = "admin"
	}
	if cfg.InitialUser.Email == "" {
		cfg.InitialUser.Email = "admin@admin.com"
	}
	if cfg.InitialUser.Password == "" {
		cfg.InitialUser.Password = "admin"
	}

	if cfg.Frontend.Mode == "" {
		cfg.Frontend.Mode = "dev"
	}
	if cfg.Frontend.DevURL == "" {
		cfg.Frontend.DevURL = "http://127.0.0.1:4173"
	}
	if cfg.Frontend.DistDir == "" {
		cfg.Frontend.DistDir = "../frontend/dist"
	}

	if cfg.MCU.DefaultBaud == 0 {
		cfg.MCU.DefaultBaud = 250000
	}
	if cfg.MCU.ScanTimeoutMs == 0 {
		cfg.MCU.ScanTimeoutMs = 5000
	}
	if cfg.Klipper.ProfilesDir == "" {
		cfg.Klipper.ProfilesDir = "profiles"
	}
}
