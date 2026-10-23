// Package migrations embeds the SQL schema so the binary is self-contained.
package migrations

import _ "embed"

//go:embed 001_init.sql
var InitSQL string
