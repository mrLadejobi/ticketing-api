package migrations

import "embed"

// FS embeds all versioned goose SQL migration files.
//
//go:embed *.sql
var FS embed.FS

