package platform

import (
	"fmt"
	"io/fs"
	"strings"
)

// Extension is the contract every compile-time extension satisfies.
type Extension interface {
	Manifest() Manifest
	Contribute(ctx *ContributionContext) error
}

// PlatformMode controls which route groups are mounted and who can own root UI routes.
type PlatformMode string

const (
	FullPlatform  PlatformMode = "full"
	ExtensionHost PlatformMode = "extension-host"
	Headless      PlatformMode = "headless"
)

// Manifest declares an extension's identity, mode, route ownership, and migrations.
type Manifest struct {
	ID         string
	Label      string
	Mode       PlatformMode
	Ownership  RouteOwnership
	Migrations []MigrationSet
}

// RouteOwnership declares the path prefixes an extension is allowed to register under.
type RouteOwnership struct {
	UI     []string
	API    []string
	Admin  []string
	Assets []string
}

// MigrationSet declares an isolated migration history for one extension.
type MigrationSet struct {
	ExtensionID string
	FS          fs.FS
	Directory   string
	Table       string
}

// Validate checks the manifest is well-formed: non-empty ID, valid mode,
// at least one ownership prefix (unless headless).
func (m Manifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("manifest ID must not be empty")
	}
	switch m.Mode {
	case FullPlatform, ExtensionHost, Headless:
	default:
		return fmt.Errorf("manifest %s: invalid mode %q (want full, extension-host, or headless)", m.ID, m.Mode)
	}
	if m.Mode != Headless {
		if len(m.Ownership.UI) == 0 && len(m.Ownership.API) == 0 &&
			len(m.Ownership.Admin) == 0 && len(m.Ownership.Assets) == 0 {
			return fmt.Errorf("manifest %s: must declare at least one ownership prefix", m.ID)
		}
	}
	return nil
}
