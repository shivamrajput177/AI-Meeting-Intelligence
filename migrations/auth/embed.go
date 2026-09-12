// Package authmigrations embeds Auth Service's SQL migrations — see
// migrations/org/embed.go for why each service's migrations directory is
// also a tiny Go package.
package authmigrations

import "embed"

//go:embed *.sql
var FS embed.FS
