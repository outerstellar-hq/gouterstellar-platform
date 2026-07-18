# Repository boundary

This repository is a set of application-neutral Go libraries.

- Never add an executable (`package main`) or application startup wiring.
- Never add product-specific handlers, routes, templates, assets, migrations,
  configuration, deployment files, or plugins.
- Consumer applications import these libraries. This repository must never
  import a consumer application.
- Shared UI structure belongs in `ui`; product page content, CSS, state,
  authentication, and routing remain in the consumer repository.
- Internationalization policy and translation bundles remain consumer-owned;
  `i18n` only supplies generic loading and lookup behavior.
- A new package must be independently useful to multiple applications and must
  expose a small, application-neutral API.

Before delivery, run `make check` and verify the consumer integration in the
consumer repository.
