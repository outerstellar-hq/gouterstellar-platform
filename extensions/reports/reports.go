package reports

import (
	"embed"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

//go:embed templates/pages/*.html templates/partials/*.html
var templatesFS embed.FS

//go:embed assets/*.css
var assetsFS embed.FS

// Extension is the reports extension. It demonstrates the full extension model:
// manifest, ownership, route contribution, navigation, its own migration, and
// data access through a capability interface — all WITHOUT importing internal/.
type Extension struct {
	messages extplatform.MessageCounter
	pages    *extplatform.PageRegistry
}

// New creates the reports extension with the given message counter capability.
func New(messages extplatform.MessageCounter) *Extension {
	return &Extension{messages: messages}
}

// Manifest declares the reports extension's identity, mode, route ownership, and
// migrations. It owns /reports (not / — that belongs to the core extension) so
// there is no ownership conflict. It runs in ExtensionHost mode, proving a
// third-party-style extension can mount alongside the platform core.
func (e *Extension) Manifest() extplatform.Manifest {
	return extplatform.Manifest{
		ID:    "reports",
		Label: "Reports",
		Mode:  extplatform.ExtensionHost,
		Ownership: extplatform.RouteOwnership{
			UI:     []string{"/reports", "/extension/reports"},
			API:    []string{"/api/reports", "/api/v1/reports"},
			Admin:  []string{"/admin/reports"},
			Assets: []string{"/extensions/reports/assets"},
		},
		Migrations: []extplatform.MigrationSet{{
			ExtensionID: "reports",
			FS:          migrationsFS,
			Directory:   "migrations",
			Table:       "schema_migrations_reports",
		}},
	}
}
