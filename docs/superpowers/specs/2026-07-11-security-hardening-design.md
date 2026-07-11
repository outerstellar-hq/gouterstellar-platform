# Security Hardening Design (Workstream B)

**Date:** 2026-07-11
**Status:** Approved
**Workstream:** B of 5

## Problem

The audit identified 5 security gaps:
1. Authorization middleware never enforced — PermissionResolver built but unused
2. Logout doesn't delete server-side sessions
3. No auth-route-specific rate limiting
4. No CSP nonce support
5. Login/logout/failed-login audit events missing

## Scope

Fix the 5 gaps above. Out of scope: CSRF on API routes (design tradeoff for bearer-token APIs), constant-time CSRF comparison (low risk given single-request lifecycle), fail-open auth (architectural decision for the middleware chain).

## 1. Authorization middleware

Add `filter.RequirePermission(resolver, domain, action)` middleware. Applied to `GroupAdmin` via `GroupMiddleware` in main.go. The middleware calls `PermissionResolver.PermissionsFor(user)` and checks if any returned permission `.Implies(requested)`.

New method on PermissionResolver: `Allowed(user *model.User, perm model.Permission) bool` — checks `PermissionsFor` + `Implies`.

For admin routes, apply `RequirePermission(resolver, "*", "*")` — effectively "must have admin wildcard." This replaces the hand-coded `currentUser.Role != model.RoleAdmin` checks in user_admin handlers. Those per-handler checks become redundant (but harmless — defense in depth).

Also add admin permission check to `GroupAdmin` middleware so read-only admin endpoints (ListUsers, ShowAudit, ExportUsers, ExportAudit) that currently lack role checks are protected.

## 2. Logout session deletion

Add `SecurityService.Logout(ctx, rawToken)` that hashes the token and calls `sessionRepo.DeleteByTokenHash`. Update both `AuthHandler.HandleLogout` and `AuthAPI.Logout` to call it. Add `USER_LOGOUT` audit event.

The handlers need access to the raw session token — read it from the cookie via `web.GetSessionToken(r)`.

## 3. Auth-route rate limiting

Add a second `RateLimiter` instance with stricter settings (e.g., 3 rps, burst 5) applied only to auth-sensitive routes. Apply it as a `GroupPublicUI` middleware that checks if the path starts with `/auth/`.

Actually simpler: create `filter.AuthRateLimiter(rps, burst)` that only activates on auth paths (`/auth/login`, `/auth/register`, `/auth/reset`, `/api/v1/auth/login`, `/api/v1/auth/register`). Apply it globally but have it pass through for non-auth paths.

## 4. CSP nonce

Add per-request nonce generation to `SecurityHeaders`. Generate 16 random bytes, base64-encode, inject as `'nonce-<value>'` into the CSP header. Thread the nonce through request context so templates can use it (future: for inline scripts).

Set a default CSP policy in config: `default-src 'self'; script-src 'self' 'nonce-{nonce}'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'`.

## 5. Login/logout audit events

Add to `SecurityService`:
- `auditLog` call in `Authenticate` on success → `USER_LOGIN`
- `auditLog` call in `Authenticate` on failure → `USER_LOGIN_FAILED`
- `auditLog` call in new `Logout` method → `USER_LOGOUT`

## Out of scope

- CSRF on API routes (bearer-token tradeoff)
- Instance-level permission isolation (requires data model changes)
- Constant-time CSRF comparison
- HSTS preload
- Sliding session policy changes
