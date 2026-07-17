# Web replacement to-do

Work GitHub issues in dependency order before continuing discretionary parity improvements.

## Repository boundaries

- [x] Keep this repository focused on the Outerstellar web platform.
- [x] Keep product-specific application work out of this repository.
- [x] Keep the Java desktop application out of scope; the Go project replaces only the Java web application.
- [x] Keep styling as handwritten CSS with no Node build dependency.

## GitHub issues

- [x] [#8 Migrate the Go module to the canonical repository path](https://github.com/outerstellar-hq/gouterstellar-platform/issues/8) — implemented and verified from a disposable consumer against the published upstream commit.
- [x] [#4 Expose shared page rendering and extension-owned templates](https://github.com/outerstellar-hq/gouterstellar-platform/issues/4) — implemented and verified through the reports extension.
- [x] [#5 Expose authenticated identity and CSRF context to extensions](https://github.com/outerstellar-hq/gouterstellar-platform/issues/5) — implemented with structural admin enforcement and credential-free request projection.
- [x] [#6 Add date and monotonic build identity](https://github.com/outerstellar-hq/gouterstellar-platform/issues/6) — implemented across the shell, health/diagnostics, backup provenance, containers, CI, and releases.
- [x] [#7 Add an extension operations registry](https://github.com/outerstellar-hq/gouterstellar-platform/issues/7) — implemented with owner-isolated admin routes, typed remote providers, audit outcomes, operation states, and confirmed restores.

## Confirmed parity work completed

- [x] Add authenticated, role-aware extension banner providers to the public contribution API and render their notices in the shared shell with CSRF-safe dismissal.
- [x] Add the Java-compatible configurable global request-body limit so declared oversized requests receive `413` before handlers read or allocate the body.
- [x] Add equivalent OpenAPI documentation endpoints.
- [x] Restore the Java static-resource contract with a public `/site.css` alias, strong ETags, and `304 Not Modified` handling without applying ETags to API JSON.
- [x] Match Java's content-aware error boundary: JSON 404/500 payloads with request IDs for API paths, compact HTMX failures, and themed HTML for browser routes.
- [x] Signal expired sessions consistently: `X-Session-Expired` on API/bearer responses, cleared cookies and `/auth?expired=true` redirects for browser requests.
- [x] Preserve same-origin browser destinations through unauthenticated redirects with a sanitized `returnTo` parameter.
- [x] Echo or generate `X-Request-Id` on every response and expose it together with `X-Session-Expired` through CORS.
- [x] Keep common security headers on every response while omitting browser-only CSP from `/api/` routes.
- [x] Carry the per-request CSP nonce into shared-shell script tags and remove the unnecessary `unsafe-inline` script allowance from the default policy.
- [x] Reconcile session sliding timeout, absolute timeout, cookie refresh, logout invalidation, and fixation protection against Java's session integration tests.
- [x] Verify extension-owned static assets have an embedded-filesystem registration path and preserve Java's filesystem-first, packaged-fallback behavior.
- [x] Re-run OAuth, password-reset-token, and return-to workflow comparisons against the packaged Go server.
- [x] Match Java's auth-only fixed-window rate limits, reset-specific threshold, trusted-proxy handling, and cross-IP per-account protection.
- [x] Complete the Java integration-test matrix through admin exports, sync conflicts, WebSocket protocol edges, contact sync, API keys, notifications, user management, profile, search, diagnostics, developer dashboard, extension dashboard, extension host UI, component fragments, packaged extension-host behavior, and release artifact behavior.

## Completed in the latest parity slices

- [x] Added the Java-shaped WebSocket refresh channel and HTMX WebSocket asset.
- [x] Added refreshable message, contact, and trash fragments with filter and pagination preservation.
- [x] Preserved WebSocket upgrades through HTTP logging and metrics middleware.
- [x] Fixed HTMX-boosted authentication transitions so authenticated pages acquire the WebSocket-enabled shell.
- [x] Proved cross-tab live refresh in the packaged Podman application.
- [x] Persisted theme, language, and density choices across navigation through a CSRF-protected preference update.
- [x] Added extension-owned embedded static asset registration with manifest ownership validation.
- [x] Preserved Java-compatible `STATIC_DIR` and legacy `ASSETS_DIR` filesystem overrides for platform and extension assets.
- [x] Applied the host ETag policy uniformly to core and extension asset routes.
- [x] Proved override precedence, packaged fallback, public access, and conditional `304` responses in the packaged Podman application.
- [x] Matched Java's OAuth callback error redirects, provider status codes, safe login destinations, atomic OAuth identity creation, and collision-safe usernames.
- [x] Replaced raw password-reset tokens with peppered HMAC digests, invalidated older links, consumed resets atomically, and revoked all existing sessions.
- [x] Removed the global refill throttle and matched Java's bounded auth-path, reset, trusted-proxy, and per-account rate-limit buckets.
- [x] Reverified admin CSV/JSON export shapes, pagination, formula neutralization, access control, and packaged download behavior.
- [x] Persisted stale sync clients as resolvable conflicts, returned both client/server versions with schema version 1, and enforced Java's sync field limits.
- [x] Bounded message and contact pulls to 500 rows with an accurate `hasMore` signal from database-level `limit + 1` queries.
- [x] Matched WebSocket 4401 authentication failures and Java's HTMX refresh protocol while serializing concurrent broadcasts.
- [x] Proved contact sync create/update/conflict/tombstone behavior, complete detail-field replacement, empty and future pulls, and concurrent same-ID message pushes through real PostgreSQL routes and the packaged Podman runtime.
- [x] Matched Java API-key validation, key shape, peppered HMAC storage, disabled/deleted-key rejection, device-token lifecycle, and poll API/HTMX workflows through real PostgreSQL routes and the packaged Podman runtime.
- [x] Matched Java audit actor/target/action recording, admin lifecycle and password changes, notification ownership/no-op semantics, unread state, and browser bell/admin workflows through real PostgreSQL routes and the packaged Podman runtime.
- [x] Matched Java message restore no-op/redirect behavior, persisted profile/account API lifecycle, and CSRF-protected browser password-change fragments through real PostgreSQL routes and the packaged Podman runtime.
- [x] Matched Java diagnostics route metadata, extension readiness reporting, message-search/sync pull behavior, and developer-dashboard access/disabled-route behavior through real route tests and Podman-backed PostgreSQL coverage.
- [x] Matched Java extension dashboard diagnostics for route ownership, admin sections, mounted asset routes, and readiness while confirming component fragment behavior through Podman-backed handler coverage.
- [x] Matched Java extension-host page-set behavior so core pages and chrome are hidden by default, selected core pages can be opted in, and extensions can own root without conflicting with the core home route.
- [x] Added a packaged Podman end-to-end gate that builds the release image, migrates and seeds PostgreSQL, logs in with CSRF, verifies extension-host page filtering, checks extension asset ETags, and rejects JS runtime artifacts.
- [x] Passed module verification, tidy check, vet, full Podman-backed tests, golangci-lint, gosec, actionlint, and compose validation.
