// Package database holds the schemas, built into the server so it can apply them on start.
package database

import _ "embed"

//go:embed schema.sql
var Schema string

// CatalogSchema is the set catalog's (sets.db), applied when one is built.
//
//go:embed catalog.sql
var CatalogSchema string
