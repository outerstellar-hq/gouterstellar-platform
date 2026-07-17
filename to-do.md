# Web replacement to-do

Work GitHub issues in dependency order before continuing discretionary parity improvements.

## GitHub issues

- [x] [#8 Migrate the Go module to the canonical repository path](https://github.com/outerstellar-hq/gouterstellar-platform/issues/8) — implemented and verified from a disposable consumer against the published upstream commit.
- [x] [#4 Expose shared page rendering and extension-owned templates](https://github.com/outerstellar-hq/gouterstellar-platform/issues/4) — implemented and verified through the reports extension.
- [x] [#5 Expose authenticated identity and CSRF context to extensions](https://github.com/outerstellar-hq/gouterstellar-platform/issues/5) — implemented with structural admin enforcement and credential-free request projection.
- [x] [#6 Add date and monotonic build identity](https://github.com/outerstellar-hq/gouterstellar-platform/issues/6) — implemented across the shell, health/diagnostics, backup provenance, containers, CI, and releases.
- [x] [#7 Add an extension operations registry](https://github.com/outerstellar-hq/gouterstellar-platform/issues/7) — implemented with owner-isolated admin routes, typed remote providers, audit outcomes, operation states, and confirmed restores.
- [x] [#9 Add the Starforge worker-management extension](https://github.com/outerstellar-hq/gouterstellar-platform/issues/9) — implemented with a protected page, authenticated BFF, server-only credential, explicit unavailable state, and label management.
- [x] [#11 StarForge: render each pipeline through a deterministic typed template registry](https://github.com/outerstellar-hq/gouterstellar-platform/issues/11) — first Sleep Series vertical slice implemented with a closed schema registry, typed catalog client, deterministic story/episode ordering, fixed stage rail, distinct artifact states, and unavailable-state rendering.

## Parity work already in progress

- [x] Finished the owned CSS palettes for every Java-compatible theme name without adding Tailwind or Node to the Go build.
- [x] Rebuilt the disposable Podman image and visually verified light, dark, high-contrast, and branded themes.
- [x] Re-ran the full four-core Go, lint, security, workflow, and Podman packaging gates after the theme work.
- [x] Re-ran the Java-to-Go route, behavior, accessibility, and operational parity audit after the GitHub issue queue was complete.
- [x] Kept the Java desktop application out of scope; the Go project replaces only the Java web application.

## Publication follow-up

- [x] Review and commit the complete issue/parity change set on `codex/web-replacement`.
- [x] Push the commit, then verify the canonical module path with a remote `go get`.
- [x] Link [draft PR #10](https://github.com/outerstellar-hq/gouterstellar-platform/pull/10) to issues #4–#9 with automatic closing references.
- [ ] Merge PR #10 after CI and review; issues #4–#9 will close automatically on merge.

## Confirmed remaining parity gaps

- [x] Add authenticated, role-aware extension banner providers to the public contribution API and render their notices in the shared shell with CSRF-safe dismissal.
- [x] Add the Java-compatible configurable global request-body limit so declared oversized requests receive `413` before handlers read or allocate the body.
- [x] Verify whether Java's bundled Swagger UI is intentionally user-reachable and, if so, add an equivalent Go documentation page backed by the existing OpenAPI endpoints.
- [x] Restore the Java static-resource contract with a public `/site.css` alias, strong ETags, and `304 Not Modified` handling without applying ETags to API JSON.
- [x] Match Java's content-aware error boundary: JSON 404/500 payloads with request IDs for API paths, compact HTMX failures, and themed HTML for browser routes.
- [x] Signal expired sessions consistently: `X-Session-Expired` on API/bearer responses, cleared cookies and `/auth?expired=true` redirects for browser requests.
- [x] Preserve same-origin browser destinations through unauthenticated redirects with a sanitized `returnTo` parameter.
- [x] Echo or generate `X-Request-Id` on every response and expose it together with `X-Session-Expired` through CORS.
- [x] Keep common security headers on every response while omitting browser-only CSP from `/api/` routes.
- [x] Carry the per-request CSP nonce into shared-shell script tags and remove the unnecessary `unsafe-inline` script allowance from the default policy.
- [ ] Continue comparing Java integration-test behavior against the packaged Go runtime after each completed slice.

## Next parity audit queue

- [x] Reconcile session sliding timeout, absolute timeout, cookie refresh, logout invalidation, and fixation protection against Java's session integration tests.
- [x] Verify extension-owned static assets have an equally simple embedded-filesystem registration path and preserve Java's filesystem-first, packaged-fallback behavior.
- [x] Re-run OAuth, password-reset-token, and return-to workflow comparisons against the packaged Go server rather than route literals alone.
- [x] Match Java's auth-only fixed-window rate limits, reset-specific threshold, trusted-proxy handling, and cross-IP per-account protection.
- [x] Continue the Java integration-test matrix through admin exports, sync conflicts, and WebSocket protocol edge cases.
- [x] Continue the Java integration-test matrix through contact sync CRUD, contact detail synchronization, and concurrent same-ID pushes.
- [x] Continue the Java integration-test matrix through API-key lifecycle, device-token registration, and poll workflows.
- [x] Continue the Java integration-test matrix through audit-log, notifications, and user-management workflows.
- [x] Continue the Java integration-test matrix through message restore, profile API, and CSRF-protected change-password workflows.
- [x] Continue the Java integration-test matrix through message search, diagnostics, and developer-dashboard workflows.
- [ ] Continue the Java integration-test matrix through extension dashboard, extension host UI, and component fragment workflows.

## Completed in the current parity slice

- [x] Added the Java-shaped WebSocket refresh channel and HTMX WebSocket asset.
- [x] Added refreshable message, contact, and trash fragments with filter and pagination preservation.
- [x] Preserved WebSocket upgrades through HTTP logging and metrics middleware.
- [x] Fixed HTMX-boosted authentication transitions so authenticated pages acquire the WebSocket-enabled shell.
- [x] Proved cross-tab live refresh in the packaged Podman application.
- [x] Persisted theme, language, and density choices across navigation through a CSRF-protected preference update.
- [x] Added the public build identity, operations registry, and Starforge extension from GitHub issues #6, #7, and #9.
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
- [x] Passed module verification, tidy check, vet, full Podman-backed tests, golangci-lint, gosec, actionlint, and compose validation.
