package platform

import "strings"

const coreExtensionID = "platform-core"

func filterExtensionHostCoreRoutes(routes []RouteRegistration, pages map[PlatformPageSet]struct{}) []RouteRegistration {
	filtered := routes[:0]
	for _, route := range routes {
		if route.Owner != coreExtensionID || coreRouteAllowedInExtensionHost(route, pages) {
			filtered = append(filtered, route)
		}
	}
	return filtered
}

func filterExtensionHostCoreNav(items []NavigationItem, pages map[PlatformPageSet]struct{}) []NavigationItem {
	filtered := items[:0]
	for _, item := range items {
		if coreNavAllowedInExtensionHost(item, pages) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func coreRouteAllowedInExtensionHost(route RouteRegistration, pages map[PlatformPageSet]struct{}) bool {
	if route.Group == GroupAPI || route.Group == GroupAssets {
		return true
	}
	path := route.Pattern
	if isCoreInfrastructureRoute(path) || isCoreAuthRoute(path) || isCoreShellComponentRoute(path) {
		return true
	}
	switch {
	case route.Group == GroupAdmin:
		return hasPage(pages, PlatformPageAdmin) && isCoreAdminRoute(path)
	case hasPage(pages, PlatformPageHome) && isPathUnder(path, "/", "/messages", "/components/message-list"):
		return true
	case hasPage(pages, PlatformPageContacts) && isPathUnder(path, "/contacts"):
		return true
	case hasPage(pages, PlatformPageSearch) && isPathUnder(path, "/search"):
		return true
	case hasPage(pages, PlatformPageSettings) && isPathUnder(path, "/settings"):
		return true
	case hasPage(pages, PlatformPageNotifications) && isPathUnder(path, "/notifications", "/components/notification-bell"):
		return true
	case hasPage(pages, PlatformPageProfile) && isPathUnder(path, "/auth/profile", "/auth/change-password", "/auth/account", "/auth/api-keys"):
		return true
	default:
		return false
	}
}

func coreNavAllowedInExtensionHost(item NavigationItem, pages map[PlatformPageSet]struct{}) bool {
	switch item.URL {
	case "/":
		return hasPage(pages, PlatformPageHome)
	case "/messages/trash":
		return hasPage(pages, PlatformPageHome)
	case "/contacts":
		return hasPage(pages, PlatformPageContacts)
	case "/search":
		return hasPage(pages, PlatformPageSearch)
	case "/settings":
		return hasPage(pages, PlatformPageSettings)
	case "/notifications":
		return hasPage(pages, PlatformPageNotifications)
	case "/auth/profile":
		return hasPage(pages, PlatformPageProfile)
	case "/admin/extensions", "/admin/users", "/admin/audit":
		return hasPage(pages, PlatformPageAdmin)
	default:
		return true
	}
}

func hasPage(pages map[PlatformPageSet]struct{}, page PlatformPageSet) bool {
	_, ok := pages[page]
	return ok
}

func isCoreInfrastructureRoute(path string) bool {
	return isPathUnder(path,
		"/health", "/health/live", "/health/ready", "/robots.txt", "/sitemap.xml",
		"/debug/routes", "/openapi.json", "/ui/openapi.json", "/ui-protected/openapi.json",
		"/components/openapi.json", "/components-protected/openapi.json", "/ws",
	)
}

func isCoreAuthRoute(path string) bool {
	if path == "/logout" {
		return true
	}
	return path == "/auth" ||
		isPathUnder(path,
			"/auth/login", "/auth/register", "/auth/reset", "/auth/totp", "/auth/oauth",
			"/auth/components/forms", "/auth/components/result", "/auth/components/totp-verify", "/auth/components/reset-confirm",
		)
}

func isCoreShellComponentRoute(path string) bool {
	return isPathUnder(path,
		"/components/footer-status", "/components/navigation/page", "/components/navigation/preferences",
		"/components/sidebar", "/components/messages", "/components/polls",
	)
}

func isCoreAdminRoute(path string) bool {
	return path == "/admin" || isPathUnder(path, "/admin/extensions", "/admin/users", "/admin/audit")
}

func isPathUnder(path string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if prefix == "/" {
			if path == "/" || strings.HasPrefix(path, "/messages") {
				return true
			}
			continue
		}
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
