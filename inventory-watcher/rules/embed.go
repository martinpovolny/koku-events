package rules

import "embed"

// FS contains the default JSON Decision Models used to seed a new database.
// Database rows remain authoritative after the initial seed.
//
//go:embed *.json
var FS embed.FS
