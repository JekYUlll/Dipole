// Package migrations exposes the versioned MySQL schema migrations.
package migrations

import "embed"

// Files contains paired *.up.sql and *.down.sql migration files.
//
//go:embed *.sql
var Files embed.FS
