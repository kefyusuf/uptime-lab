package migrations

import "embed"

// FS contains the SQL migrations owned by the API module.
//
//go:embed *.sql
var FS embed.FS
