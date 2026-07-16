# Web replacement to-do

Work GitHub issues in dependency order before continuing discretionary parity improvements.

## GitHub issues

- [x] [#8 Migrate the Go module to the canonical repository path](https://github.com/outerstellar-hq/gouterstellar-platform/issues/8) — implemented and locally verified; remote `go get` verification requires publishing the canonical module declaration.
- [x] [#4 Expose shared page rendering and extension-owned templates](https://github.com/outerstellar-hq/gouterstellar-platform/issues/4) — implemented and verified through the reports extension.
- [x] [#5 Expose authenticated identity and CSRF context to extensions](https://github.com/outerstellar-hq/gouterstellar-platform/issues/5) — implemented with structural admin enforcement and credential-free request projection.
- [x] [#6 Add date and monotonic build identity](https://github.com/outerstellar-hq/gouterstellar-platform/issues/6) — implemented across the shell, health/diagnostics, backup provenance, containers, CI, and releases.
- [x] [#7 Add an extension operations registry](https://github.com/outerstellar-hq/gouterstellar-platform/issues/7) — implemented with owner-isolated admin routes, typed remote providers, audit outcomes, operation states, and confirmed restores.
- [x] [#9 Add the Starforge worker-management extension](https://github.com/outerstellar-hq/gouterstellar-platform/issues/9) — implemented with a protected page, authenticated BFF, server-only credential, explicit unavailable state, and label management.

## Parity work already in progress

- [x] Finished the owned CSS palettes for every Java-compatible theme name without adding Tailwind or Node to the Go build.
- [x] Rebuilt the disposable Podman image and visually verified light, dark, high-contrast, and branded themes.
- [x] Re-ran the full four-core Go, lint, security, workflow, and Podman packaging gates after the theme work.
- [x] Re-ran the Java-to-Go route, behavior, accessibility, and operational parity audit after the GitHub issue queue was complete.
- [x] Kept the Java desktop application out of scope; the Go project replaces only the Java web application.

## Publication follow-up

- [ ] Review and commit the complete issue/parity change set on `codex/web-replacement`.
- [ ] Push the commit, then verify the canonical module path with a remote `go get`.
- [ ] Link the published fix to issues #4–#9 and close each ticket after its acceptance criteria are confirmed upstream.

## Completed in the current parity slice

- [x] Added the Java-shaped WebSocket refresh channel and HTMX WebSocket asset.
- [x] Added refreshable message, contact, and trash fragments with filter and pagination preservation.
- [x] Preserved WebSocket upgrades through HTTP logging and metrics middleware.
- [x] Fixed HTMX-boosted authentication transitions so authenticated pages acquire the WebSocket-enabled shell.
- [x] Proved cross-tab live refresh in the packaged Podman application.
- [x] Persisted theme, language, and density choices across navigation through a CSRF-protected preference update.
- [x] Added the public build identity, operations registry, and Starforge extension from GitHub issues #6, #7, and #9.
- [x] Passed module verification, tidy check, vet, full Podman-backed tests, golangci-lint, gosec, actionlint, and compose validation.
