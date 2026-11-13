// Package database holds the schema, built into the server so it can apply it on start.
package database

import _ "embed"

//go:embed schema.sql
var Schema string
