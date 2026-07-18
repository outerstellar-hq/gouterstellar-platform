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
structure across independently owned applications.

## Decision

This repository contains application-neutral libraries only. It has no
executable, application wiring, product implementation, database schema, or
deployment artifact.

The `ui` module owns the shared HTML shell and accepts an
`application-content` template supplied by a consumer. The `i18n` module loads
consumer-owned translation catalogs. Consumer repositories own runtime wiring,
routes, authentication, page content, styling, persistence, and deployment.

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

### Library-only shared shell and utilities (chosen)

Pros: repository ownership is unambiguous; applications deploy independently;
shared template changes propagate through normal Go dependency updates; the
public surface stays small and testable.

Cons: applications must own integration tests and dependency upgrades; a shared
shell change is not instantaneous across all deployed consumers; library APIs
must remain backward compatible or use coordinated version updates.

## Consequences

UI consistency remains achievable without a host. Applications compose their
page templates into the same shell and keep their product behavior local. A
consumer integration is the acceptance test for shared-library changes.
