# ADR 0001: Library-only repository boundary

Status: Accepted

## Context

The original system combined a deployable web application with an in-process
extension mechanism. That design made a single host easy to assemble, but it
also made repository ownership ambiguous: product code looked structurally
welcome beside shared infrastructure. Automated contributors repeatedly chose
the shortest compile-time path and placed product implementations in the shared
repository.

The useful original goal was not the host. It was consistent user-interface
structure and reusable application mechanics across independently owned
applications. Restricting the replacement to UI and internationalization
discarded authentication and security behavior that consumers immediately had
to reimplement.

## Decision

This repository contains application-neutral libraries only. It has no
executable, application wiring, product implementation, product database
schema, or deployment artifact.

The `ui` module owns the shared HTML shell and accepts an
`application-content` template supplied by a consumer. The `i18n` module loads
consumer-owned translation catalogs.

The `auth` module owns reusable password, token, session, JWT, and TOTP
mechanics. The `web` module owns reusable HTTP security conventions. The
`migration` module runs consumer-owned embedded migration sets, and the
`observability` module constructs standard OpenTelemetry instrumentation.

Consumer repositories own runtime wiring, routes, user and role models,
identity policy, login pages, persistence adapters, migration files, page
content, styling, and deployment. Libraries may depend on maintained upstream
Go modules when they eliminate security-sensitive custom code or enforce a
shared convention. A wrapper must add policy or integration leverage; a pure
forwarder is not a supported module.

Dependencies point from consumer applications to this module and never in the
opposite direction.

## Alternatives considered

### Keep the deployable host and strengthen contribution rules

Pros: centralized runtime policy, one integration-test surface, and fast
in-process composition.

Cons: the physical repository still advertises product extension points;
ownership depends on contributors interpreting policy correctly; releases and
runtime failures remain coupled.

### Promote the old application internals as public packages

Pros: preserves more code and exposes authentication, persistence, and web
layers for reuse.

Cons: those APIs were shaped around one concrete application. Publishing them
would freeze accidental coupling and make the old host architecture appear to
be the supported integration path.

### Limit the repository to the shared shell and translation utilities

Pros: extremely small dependency graph and public surface.

Cons: consumers must reimplement password hashing, session lifecycle, CSRF,
token handling, migrations, and tracing; security conventions drift between
applications; useful implementation knowledge disappears behind an artificial
restriction.

### Application-neutral library suite (chosen)

Pros: repository ownership is unambiguous; applications deploy independently;
shared behavior propagates through normal Go dependency updates; consumers
reuse security mechanics without inheriting another product's models, routes,
schema, or runtime; maintained upstream modules replace custom protocol code.

Cons: applications must own adapters, integration tests, and dependency
upgrades; shared changes are not instantaneous across deployed consumers;
library interfaces must remain backward compatible or use coordinated version
updates; the module dependency graph is larger than a UI-only repository.

## Consequences

UI consistency and shared application conventions remain achievable without a
host. Applications compose their page templates into the same shell and use
the same authentication, HTTP security, migration, and observability modules
while keeping product behavior local. A consumer integration is the acceptance
test for shared-library changes.
