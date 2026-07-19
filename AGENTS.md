# Repository boundary

This repository is a set of application-neutral Go libraries.

- Never add an executable (`package main`) or application startup wiring.
- Never add product-specific handlers, routes, templates, assets, migrations,
  configuration, deployment files, or plugins.
- Consumer applications import these libraries. This repository must never
  import a consumer application.
- Shared UI structure belongs in `ui`; product page content, CSS, state, and
  routing remain in the consumer repository.
- Shared authentication and HTTP-security mechanics belong in `auth` and
  `web`. Consumer repositories still own users, roles, identity policy,
  persistence adapters, login pages, redirects, and route wiring.
- Internationalization policy and translation bundles remain consumer-owned;
  `i18n` only supplies generic loading and lookup behavior.
- A new package must be independently useful to multiple applications and must
  expose a small, application-neutral API.
- Prefer the Go standard library, then a maintained upstream module. Add a
  platform module only when it enforces shared policy or removes integration
  complexity; do not add pass-through wrappers.

Before delivery, run `make check` in this repository. Consumer adoption and
integration verification belong to a separately scoped task in the consumer's
own repository; never cross that repository boundary as part of platform work.
