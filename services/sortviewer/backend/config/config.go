// Package config reads the sort viewer's settings from the environment (Harbor Keeper's
// Dry Dock sets them as keeper vars).
package config

import (
	"os"
	"strings"
)

type Config struct {
	Host    string // HOST: the interface to listen on ("" = all; Dry Dock sets 127.0.0.1)
	Port    string // PORT
	DataDir string // DATA_DIR, else KEEPER_DATA_DIR: database and photos
	AppURL  string // APP_URL: https://lego.n8p.xyz, for cookies and sign-in redirects

	OIDCIssuer       string   // OIDC_ISSUER: https://login.n8p.xyz
	OIDCClientID     string   // OIDC_CLIENT_ID
	OIDCClientSecret string   // OIDC_CLIENT_SECRET
	AdminEmails      []string // ADMIN_EMAILS: comma separated

	// DevAdmin (SORTVIEWER_DEV_ADMIN=<email>) signs the admin in without the Gangway,
	// for local development only. It's ignored when OIDC is set up.
	DevAdmin string

	// SetsDir (SORTVIEWER_SETS_DIR) reads the set data from downloaded Rebrickable
	// .csv.gz files in that folder instead of fetching them, for local development.
	SetsDir string
}

func FromEnv() Config {
	c := Config{
		Host:             os.Getenv("HOST"),
		Port:             env("PORT", "8090"),
		// Dry Dock gives each environment KEEPER_DATA_DIR, kept across versions.
		DataDir:          env("DATA_DIR", env("KEEPER_DATA_DIR", "../private")),
		AppURL:           strings.TrimRight(env("APP_URL", "http://localhost:8090"), "/"),
		OIDCIssuer:       os.Getenv("OIDC_ISSUER"),
		OIDCClientID:     os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		DevAdmin:         os.Getenv("SORTVIEWER_DEV_ADMIN"),
		SetsDir:          os.Getenv("SORTVIEWER_SETS_DIR"),
	}
	for _, e := range strings.Split(os.Getenv("ADMIN_EMAILS"), ",") {
		if e = strings.TrimSpace(e); e != "" {
			c.AdminEmails = append(c.AdminEmails, e)
		}
	}
	return c
}

// Secure is whether the site is served over HTTPS (cookies are marked Secure).
func (c Config) Secure() bool { return strings.HasPrefix(c.AppURL, "https://") }

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
