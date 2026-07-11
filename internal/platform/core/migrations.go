package core

import "embed"

// Migrations embeds the SQL migration files shipped by the core extension.
// Files live under internal/platform/core/migrations/*.sql.
//
//go:embed migrations/*.sql
var Migrations embed.FS
