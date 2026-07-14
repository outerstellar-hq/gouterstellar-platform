# Full Parity Implementation Plan

14 items organized into 5 phases. Each phase is independently shippable.

## Phase 1: Critical data bugs (4 items)

### 1.1: Fix password change/reset — `UpdatePasswordHash` query
- Add `-- name: UpdatePasswordHash :exec` query to `queries/users.sql`: `UPDATE plt_users SET password_hash = $2 WHERE id = $1`
- Run `sqlc generate`
- Add `UpdatePasswordHash(ctx, userID, hash) error` to `UserRepository` interface + impl
- Fix `SecurityService.ChangePassword` (line 146): replace `s.userRepo.CreateUser(ctx, ...)` with `s.userRepo.UpdatePasswordHash(ctx, userID, hash)`
- Fix `PasswordResetService.ResetPassword` (line 95): same replacement
- Fix password reset email body: send a clickable URL (`{app_base_url}/auth/reset?token={token}`) instead of raw token
- Add web confirm-reset route: `GET /auth/reset/confirm` + `POST /auth/reset/confirm` to `AuthHandler`

### 1.2: Fix email circuit breaker
- Change `EmailService.Send` interface to return `error`
- Update all 4 implementations (Smtp, Console, NoOp, Resilient) to return errors
- Fix `resilient_email_service.go`: `closedSend`/`halfOpenSend` check the error from `delegate.Send` and call `recordFailure(err != nil)`
- Fix `smtp_email_service.go`: StartTLS branch uses `smtp.Dial` + explicit `StartTLS` when configured; falls back to `smtp.SendMail` when not. Alternatively, just remove the dead branch and document that `smtp.SendMail` handles opportunistic TLS.

### 1.3: Wire notification creation
- Inject `NotificationService` into `MessageService` and `ContactService`
- After successful message create → `notificationService.Create(ctx, userID, "New Message", content[:100], "message")`
- After successful contact create → `notificationService.Create(ctx, userID, "New Contact", name, "contact")`
- After admin actions (user enabled/disabled, role change) → notification to the affected user
- Inject into `PasswordResetService` to notify on successful password reset

### 1.4: Implement outbox processing
- The outbox processor's `processEntry` currently logs and marks PROCESSED with no side effect
- Implement: parse the sync payload, broadcast a WebSocket refresh event (`refresh:messages` / `refresh:contacts`), and optionally send push notifications to registered device tokens
- This makes the outbox the central fan-out mechanism: domain write → outbox entry → worker picks up → WebSocket refresh + push notification

## Phase 2: Missing API/UI surfaces (3 items)

### 2.1: Message CRUD routes
- Add `POST /messages/create` (create server message) to `MessagesHandler` — register in core contribute.go
- Add `POST /messages/{syncId}/delete` (soft delete)
- Add `POST /messages/{syncId}/restore` (undo delete)
- Add `POST /messages/{syncId}/resolve` (conflict resolution — takes `strategy=mine|server`)
- Add message create/delete forms to `messages.html` template with CSRF tokens
- Wire the existing `ResolveConflict` service method to the new route

### 2.2: Contact detail HTML page
- Change `ContactsHandler.Detail` from `writeJSON` to `renderer.RenderPage(w, r, "contact_detail", page)`
- Create `pages/contact_detail.html` template showing contact fields + edit form + delete button
- Create `ContactDetailPage` viewmodel with full contact fields (name, emails, phones, social, company)

### 2.3: Session management UI
- Add `ListForUser(ctx, userID) ([]SessionInfo, error)` to `SessionRepository` + new sqlc query
- Add `GET /settings/sessions` page showing active sessions with "Revoke" buttons
- Add `POST /settings/sessions/{id}/revoke` to delete a specific session
- Add this to the settings page as a new tab or sub-page

## Phase 3: Missing platform features (3 items)

### 3.1: Wire i18n service
- Instantiate `I18nService` in `wire.go` with embedded `.properties` files
- Add locale middleware: reads `user.Language` from request context, sets locale on the i18n service
- Ship base English `.properties` file with nav labels, button text, page titles
- Add `{{ translate .Language "key" }}` template function to renderer
- Apply user's language preference in `base.html`

### 3.2: Implement Segment analytics
- Create `SegmentAnalyticsService` implementing `AnalyticsService` interface
- Uses HTTP API (no SDK dependency) — POST batch events to `https://api.segment.io/v1/batch`
- Wire in `wire.go` when `cfg.Segment.Enabled && cfg.Segment.WriteKey != ""`
- The existing `analytics.Track(...)` calls throughout handlers now actually fire

### 3.3: WebSocket user scoping
- Change `WsEventPublisher.PublishRefresh` to accept a `userID` parameter
- Only broadcast to clients whose `UserID` matches (or broadcast to all if userID is empty for system-wide events)
- Update all callers: `messageService` passes the acting user's ID, so only that user's WebSocket connections get the refresh
- Add per-user client tracking in the publisher

## Phase 4: OAuth providers (1 item)

### 4.1: Implement Google OAuth provider
- Create `GoogleOAuthProvider` in `internal/security/google_oauth_provider.go`
- Implement `AuthorizationURL` → `https://accounts.google.com/o/oauth2/v2/auth?...`
- Implement `ExchangeCode` → POST to `https://oauth2.googleapis.com/token`, then GET userinfo from `https://www.googleapis.com/oauth2/v2/userinfo`
- Add config: `oauth.google.client_id`, `oauth.google.client_secret`, `oauth.google.redirect_uri`
- Register in `wire.go` and add to `resolveProvider` in `oauth.go`
- Add GitHub provider similarly if time permits

## Phase 5: Polish (3 items)

### 5.1: Apply user preferences in rendering
- The theme preference IS applied via `buildShell`. Verify `language` and `layout` are also applied:
  - Layout: add `class="{{ .Layout }}"` to body in base.html (the CSS already has layout-sidebar/layout-topbar classes)
  - Language: handled by i18n wiring in 3.1

### 5.2: Per-IP auth rate limiting
- Change `AuthRateLimiter` from a single shared limiter to per-IP limiters (same `sync.Map` pattern as the global `RateLimiter`)
- Each IP gets its own 3 rps / burst 5 limiter for auth routes
- Global limiter still applies as a second layer

### 5.3: Email notifications for events
- When `email_notifications_enabled` is true and email service is configured:
  - Send a notification email when a new message is received
  - Send a notification email when account is disabled by admin
  - Send a welcome email on registration
- Gate on user preference: check `user.EmailNotificationsEnabled` before sending