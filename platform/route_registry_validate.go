package platform

import (
	"fmt"
	"strings"
)

// routeError describes a single invalid route registration.
type routeError struct {
	Owner   string
	Method  string
	Pattern string
	Detail  string
}

func (e routeError) Error() string {
	return fmt.Sprintf("%s %s (%s): %s", e.Method, e.Pattern, e.Owner, e.Detail)
}

// validateRoutes checks a flat slice of registrations (potentially merged
// from multiple registries/extensions) against ownership rules and the active
// platform mode. It returns ALL errors found, not just the first.
func validateRoutes(routes []RouteRegistration, mode PlatformMode, ownership map[string]RouteOwnership) []error {
	var errs []error

	type routeKey struct {
		method  string
		pattern string
	}
	seen := make(map[routeKey][]string)

	for _, r := range routes {
		if !strings.HasPrefix(r.Pattern, "/") {
			errs = append(errs, routeError{
				Owner: r.Owner, Method: r.Method, Pattern: r.Pattern,
				Detail: fmt.Sprintf("route path must be absolute: %q", r.Pattern),
			})
			continue
		}

		if mode == Headless && isHTMLGroup(r.Group) {
			errs = append(errs, routeError{
				Owner: r.Owner, Method: r.Method, Pattern: r.Pattern,
				Detail: "headless mode rejects HTML route",
			})
			continue
		}

		ownerPrefixes := getPrefixes(ownership, r.Owner, r.Group)
		if !routeInsideOwnership(r.Pattern, ownerPrefixes) {
			errs = append(errs, routeError{
				Owner: r.Owner, Method: r.Method, Pattern: r.Pattern,
				Detail: fmt.Sprintf("outside ownership (allowed: %s)", strings.Join(ownerPrefixes, ", ")),
			})
		}

		key := routeKey{r.Method, r.Pattern}
		seen[key] = append(seen[key], r.Owner)
	}

	for key, owners := range seen {
		if len(owners) > 1 {
			errs = append(errs, fmt.Errorf(
				"route conflict: %s %s is owned by both %s",
				key.method, key.pattern, strings.Join(owners, " and "),
			))
		}
	}

	return errs
}

// getPrefixes returns the ownership prefixes for an owner and group.
func getPrefixes(ownership map[string]RouteOwnership, owner string, group RouteGroup) []string {
	o, ok := ownership[owner]
	if !ok {
		return nil
	}
	switch group {
	case GroupPublicUI, GroupProtectedUI:
		return o.UI
	case GroupAPI:
		return o.API
	case GroupAdmin:
		return o.Admin
	case GroupAssets:
		return o.Assets
	default:
		return nil
	}
}

// routeInsideOwnership reports whether pattern falls under one of the allowed
// prefixes. A prefix of "/" owns everything.
func routeInsideOwnership(pattern string, prefixes []string) bool {
	for _, p := range prefixes {
		if pattern == p {
			return true
		}
		if strings.HasPrefix(pattern, p+"/") {
			return true
		}
		if p == "/" {
			return true
		}
	}
	return false
}
