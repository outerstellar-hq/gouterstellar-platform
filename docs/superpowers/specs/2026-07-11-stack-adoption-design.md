# Stack Adoption Design (Workstream E)

**Date:** 2026-07-11
**Status:** Approved
**Workstream:** E of 5

## Problem

The brief recommended templ, HTMX, and Huma. None are adopted. Workstreams A-D fixed the foundations on html/template. This workstream adopts the remaining stack pragmatically.

## Scope decisions

### templ: DEFER
Rewriting all 14+ working page templates as .templ files is high effort, high risk (we just fixed them), and low immediate value. The rendering pipeline works well with html/template. templ adoption is deferred to a future cycle when there's a compelling need (e.g., type-safe component composition).

### HTMX: ADOPT (minimal)
HTMX provides the biggest user experience improvement with the least disruption:
1. Add htmx.min.js to static/
2. Add `hx-*` attributes to existing templates for dynamic interactions (inline search, pagination without full page reload, notification updates)
3. The /components/* partial endpoints already exist and work (fixed in Workstream A)
4. Wire HTMX into base.html

### Huma + OpenAPI: ADOPT (for new APIs only)
Adopting Huma for ALL existing API handlers would mean rewriting 30+ endpoints. Instead:
1. Add Huma dependency
2. Create a Huma router mounted at `/api/v1` 
3. Migrate the reports extension's API endpoint to Huma as a reference
4. Generate OpenAPI spec at `/openapi.json`
5. Existing API handlers stay as-is (Chi handlers) — migration is gradual

## Implementation

### HTMX adoption
1. Download htmx.min.js (v2.x) to static/js/htmx.min.js
2. Add `<script src="/static/js/htmx.min.js"></script>` to base.html
3. Add hx-get attributes to:
   - Search page: `hx-get="/search?q=..." hx-target="#results"` for live search
   - Contacts pagination: `hx-get="/components/contact-list?page=2" hx-target="..."`
   - Message pagination: `hx-get="/components/message-list?page=2" hx-target="..."`
4. Add hx-boost to nav links for SPA-like page transitions

### Huma + OpenAPI
1. `go get github.com/danielgtaylor/huma/v2`
2. Create a Huma router in the reports extension
3. Define typed input/output structs for the reports summary endpoint
4. Generate OpenAPI at `/openapi.json`
5. Add Swagger UI at `/docs` (optional)

## Out of scope
- Migrating existing 30+ API endpoints to Huma (gradual)
- Rewriting templates as .templ files (deferred)
- WebSocket-based HTMX extensions
