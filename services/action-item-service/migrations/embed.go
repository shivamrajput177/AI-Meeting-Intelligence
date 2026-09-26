// Package actionitemmigrations embeds Action Item Service's SQL
// migrations.
//
// Go's //go:embed can only reach files inside the embedding package's own
// directory, so each service's migrations/<service>/ directory (kept
// top-level per docs/architecture/folder-structure.md, alongside every
// other service's) is also a tiny Go package in its own right — this file
// is the entire package, existing only to make its *.sql files
// importable by cmd/action-item-service via shared/dbx.
package actionitemmigrations

import "embed"

//go:embed *.sql
var FS embed.FS
