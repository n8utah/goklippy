package config

import (
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port        int               `yaml:"port"`
	Database    DatabaseConfig    `yaml:"database"`
	JWT         JWTConfig         `yaml:"jwt"`
	InitialUser InitialUserConfig `yaml:"initial_user"`
	Frontend    FrontendConfig    `yaml:"frontend"`
}

type DatabaseConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	SSLMode  string `yaml:"ssl_mode"`
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

func (cfg Config) DatabaseURL() string {
	return buildDatabaseURL(cfg.Database)
}

func applyDefaults(cfg *Config) {
	if cfg.Port == 0 {
		cfg.Port = 8080
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
}

func buildDatabaseURL(db DatabaseConfig) string {
	if db.Host == "" || db.Username == "" || db.Name == "" {
		return ""
	}

	sslMode := db.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	host := db.Host
	if db.Port != 0 {
		host = fmt.Sprintf("%s:%d", db.Host, db.Port)
	}

	uri := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(db.Username, db.Password),
		Host:   host,
		Path:   db.Name,
	}
	q := uri.Query()
	q.Set("sslmode", sslMode)
	uri.RawQuery = q.Encode()
	return uri.String()
}
