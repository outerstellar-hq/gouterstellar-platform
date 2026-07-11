package platform

import (
	"fmt"
	"strings"
)

// Diagnostics is a read-only summary of what an extension contributed.
// Used by contract tests (Tier 1).
type Diagnostics struct {
	routes        []RouteRegistration
	navLabels     []string
	ownershipErrs []error
}

func (d Diagnostics) RoutePatterns() []string {
	patterns := make([]string, len(d.routes))
	for i, r := range d.routes {
		patterns[i] = fmt.Sprintf("%s %s", r.Method, r.Pattern)
	}
	return patterns
}

func (d Diagnostics) NavigationLabels() []string {
	return d.navLabels
}

func (d Diagnostics) OwnershipViolations() error {
	if len(d.ownershipErrs) == 0 {
		return nil
	}
	msgs := make([]string, len(d.ownershipErrs))
	for i, e := range d.ownershipErrs {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("ownership violations: %s", strings.Join(msgs, "; "))
}

// CheckExtension validates a single extension's manifest and contributions
// without starting the platform. The ContributionContext's owner is expected
// to match the extension's manifest ID (NewContributionContext stamps every
// route registration with it). The returned Diagnostics summarises the
// contributed routes, navigation labels, and any ownership violations.
func CheckExtension(ext Extension, ctx *ContributionContext) (*Diagnostics, error) {
	m := ext.Manifest()
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("manifest %s: %w", m.ID, err)
	}

	if err := ext.Contribute(ctx); err != nil {
		return nil, fmt.Errorf("contribute %s: %w", m.ID, err)
	}

	routes := ctx.Routes.All()
	// Key ownership by the owner actually stamped on the routes (ctx.Routes.owner)
	// rather than by manifest ID, so a caller-built context whose owner differs
	// from the manifest ID still validates correctly.
	owner := ctx.Routes.owner
	ownership := map[string]RouteOwnership{owner: m.Ownership}
	ownershipErrs := validateRoutes(routes, FullPlatform, ownership)

	diag := &Diagnostics{
		routes:        routes,
		navLabels:     navLabelSlice(ctx.Navigation.Items()),
		ownershipErrs: ownershipErrs,
	}

	if len(ownershipErrs) > 0 {
		return diag, diag.OwnershipViolations()
	}

	return diag, nil
}

// TestHostContext creates a ContributionContext suitable for contract tests.
func TestHostContext() *ContributionContext {
	return NewContributionContext("test-host")
}

func navLabelSlice(items []NavigationItem) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}
