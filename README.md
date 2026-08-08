# gouterstellar-platform

Reusable Go libraries for independently deployed Outerstellar applications.
This repository is not an application host: it has no executable, routes,
product schema, deployment image, product assets, or in-tree plugins.

## Library catalog

| Module | Shared responsibility | Proven implementation underneath |
| --- | --- | --- |
| `auth` | Argon2id passwords, opaque tokens, server-side sessions, principals, JWTs, TOTP | `alexedwards/argon2id`, `alexedwards/scs`, `golang-jwt/jwt`, `pquerna/otp` |
| `durablefile` | complete, crash-resistant file replacement with explicit Unix modes | `natefinch/atomic`, Go standard library |
| `web` | masked CSRF tokens, strict bounded JSON, CSP nonces, security headers, body limits, sensitive-response caching | `gorilla/csrf`, `net/http` |
| `ui` | shared server-rendered application shell and composition contract | `html/template`, `embed` |
| `i18n` | application-owned Java `.properties` catalog loading and lookup | `magiconair/properties` |
| `migration` | application-owned embedded migration sets | `golang-migrate/migrate` with `iofs` |
| `observability` | OTLP tracing plus HTTP, gRPC, and safe PGX instrumentation | OpenTelemetry Go, `exaring/otelpgx` |

The platform modules are deliberately not reimplementations of those upstream
projects. They add the shared conventions callers otherwise repeat: bounded
password verification, password-cost upgrade detection, session rotation,
current-principal resolution, secure cookie defaults, strict JWT claim and
algorithm checks, one TOTP profile, safe CSRF defaults, embedded migration
loading, and explicit tracing lifecycle.

Use an upstream package directly when no platform policy is needed. In
particular, applications should use `pgx` for PostgreSQL queries and
transactions, a maintained SCS store for session persistence, and a mature
authorization engine such as Casbin when their policy exceeds simple roles.

## Authentication

Applications retain their own user model and database. The shared session
module stores a stable string subject, then asks the application to resolve
that subject on every request so disabled accounts and changed roles take
effect immediately.

```go
passwords, err := auth.NewPasswords(auth.PasswordConfig{})
if err != nil {
    return err
}
hash, err := passwords.Hash(password)

verification, err := passwords.VerifyWithRehash(hash, loginPassword)
if err != nil {
    return err
}
if verification.Matched && verification.NeedsRehash {
    replacementHash, err := passwords.Hash(loginPassword)
    if err != nil {
        return err
    }
    // Persist replacementHash with the successful login transaction.
}

tokenHasher, err := auth.NewTokenHasher(tokenPepper)
if err != nil {
    return err
}
resetToken, err := tokenHasher.NewToken("reset_")
if err != nil {
    return err
}
// Return resetToken.Plaintext once; persist only resetToken.Digest.

sessions, err := auth.NewSessions(store, auth.PrincipalResolverFunc(
    func(ctx context.Context, subject string) (auth.Principal, error) {
        user, err := users.FindCurrent(ctx, subject)
        if errors.Is(err, ErrUserDisabled) {
            return auth.Principal{}, auth.ErrUnauthenticated
        }
        if err != nil {
            return auth.Principal{}, err
        }
        return auth.NewPrincipal(
            user.ID,
            auth.SecurityVersion(user.SecurityVersion),
            user.Roles,
            nil,
        ), nil
    },
), auth.SessionConfig{CookieName: "example_session"})
if err != nil {
    return err
}

handler := sessions.Middleware(auth.RequireAuthenticated(applicationHandler))
```

Call `SignIn` only from a handler already inside `sessions.Middleware`. Pass the
security version captured in the same read that verified the credentials:

```go
err = sessions.SignIn(request.Context(), auth.SessionIdentity{
    Subject:         user.ID,
    SecurityVersion: auth.SecurityVersion(user.SecurityVersion),
})
```

The application owns this opaque value and rotates it in the same transaction
as a password reset, account compromise response, or global sign-out. A random
128-bit-or-larger value avoids reusing an old version. Every request compares
the stored version with the current principal, so an old session committed
concurrently with the rotation still fails closed. Missing versions are also
rejected; adopting this contract intentionally signs out unversioned sessions.

`SignIn` renews the session token before changing privilege. `SignOut` destroys
the server-side session and expires the cookie. Production cookies are Secure
by default, and SameSite defaults to Lax so common top-level OAuth callbacks
work without making cookies cross-site. Applications can explicitly select
Strict or None; None requires Secure cookies. Local plain-HTTP development must
opt into `AllowInsecureCookies` explicitly. Rotate the application-owned
security version atomically with security-sensitive changes; request-time
principal resolution rejects every session carrying the stale version.

`auth.JWTs` is intended for short-lived API bearer tokens. It requires a
256-bit HMAC secret, issuer, audience, issued-at, expiry, and a fixed HS256
allow-list. Lifetime is bounded to 15 minutes and clock leeway to one minute;
verification also rejects a correctly signed token whose declared lifetime
exceeds the configured profile.
Browser login should normally use server-side sessions instead.

## HTTP security

```go
csrfMiddleware, err := web.NewCSRF(web.CSRFConfig{
    AuthKey:    csrfAuthenticationKey,
    CookieName: "example_csrf",
})
if err != nil {
    return err
}

browserRoutes := http.NewServeMux()
browserRoutes.Handle("/", cookieAuthenticatedBrowserHandler)

root := http.NewServeMux()
root.Handle("/api/", bearerTokenAPIHandler)
root.Handle("/", csrfMiddleware(browserRoutes))

handler := web.SecurityHeaders(web.SecurityHeadersConfig{HSTS: true})(root)
```

CSRF protects routes authenticated by ambient browser credentials such as
session cookies. Do not wrap APIs authenticated exclusively by an
`Authorization: Bearer` token; keep those routes on a separate router as above.
Cookie-authenticated JSON requests send `web.CSRFToken(request)` in the
`X-CSRF-Token` header. Server-rendered forms use `web.CSRFField(request)`. Use
`web.CSPNonce(request.Context())` in an inline script or style only when the
configured CSP permits it.

For JSON endpoints, `web.DecodeJSON` combines the request size limit with one
strict decode. Unknown fields, malformed trailing bytes, and a second JSON
value are rejected instead of being silently ignored:

```go
var input createWorkerRequest
if err := web.DecodeJSON(w, r, 32<<10, &input); err != nil {
    http.Error(w, "invalid JSON body", http.StatusBadRequest)
    return
}
```

## Shared UI shell

```go
//go:embed templates/*.html
var templates embed.FS

renderer, err := ui.NewRenderer(ui.Options{
    Templates: templates,
    Patterns:  []string{"templates/*.html"},
})
if err != nil {
    return err
}

err = renderer.Render(w, ui.Shell{
    Title:       "Workers",
    ProductName: "Example",
    BrandURL:    "/",
    Stylesheets: []string{"/static/app.css"},
    Labels: ui.ShellLabels{
        SkipToContent:      translations.Text("shell.skip_to_content"),
        PrimaryNavigation: translations.Text("shell.primary_navigation"),
        SignOut:            translations.Text("shell.sign_out"),
    },
    Header:      ui.Header{Title: "Workers"},
}, pageData)
```

The consumer template set defines the integration seam:

```gotemplate
{{define "application-content"}}
  {{template "workers-page" .}}
{{end}}
```

The shared shell is parsed after consumer templates, so consumers cannot
replace its reserved definitions. Product pages, handlers, data, routes, and
CSS remain in the consumer.

## Internationalization

```go
translator, err := i18n.New(i18n.Options{
    FS:            localeFiles,
    BasePath:      "locales",
    DefaultLocale: "en",
    Languages: []i18n.Language{
        {Code: "en", DisplayName: "English", NativeName: "English"},
        {Code: "de", DisplayName: "German", NativeName: "Deutsch"},
    },
})

german, err := translator.ForLocale("de")
if err != nil {
    return err
}
label := german.Translate("navigation.workers")
```

Translation catalogs and supported-language policy remain application-owned.
Catalog loading normalizes configured locale codes and rejects translated
messages whose `{0}` or `%s`/`%d` parameter contract differs from the default
locale. Locale-bound readers are immutable, so concurrent requests cannot
change each other's language.

## Durable files

`durablefile.Write` and `durablefile.WriteReader` create missing parent
directories, stage data beside its destination, apply explicit Unix permission
modes, flush and close it, and then replace the destination atomically. On Unix
the affected directory entries are synced; on Windows replacement delegates to
`MoveFileEx` with write-through through `natefinch/atomic`. Go permission bits
do not configure Windows ACLs, so consumers writing secrets there must apply
their application-specific ACL policy.

```go
err := durablefile.Write(path, encodedState, 0o600, 0o700)
```

If replacement succeeds but a following directory sync fails, the destination
already contains the new data and the library returns a
`*durablefile.CommittedError`. Consumers can detect that outcome with
`errors.As`; the error still unwraps to the underlying sync failure.

Use `durablefile.Replace` when a consumer must determine the final destination
only after it has finished producing a temporary file, such as a content-hash
cache. Source and destination must be on the same filesystem.

## Embedded migrations and tracing

`migration.New` accepts an application-owned `fs.FS`, directory, and database
URL. The consumer registers its `golang-migrate` database driver through the
driver's normal blank import. `Runner.Up` treats an already-current schema as
success; the migration SQL and database URL stay in the consumer repository.

`observability.NewTracing` constructs an OTLP tracer provider and returns its
shutdown lifecycle without silently changing process globals. The returned
tracing lifecycle supplies provider-bound HTTP, gRPC, and PostgreSQL adapters.
`InstallGlobal` remains an explicit option for dependencies that require global
OpenTelemetry state; the provider-bound methods are the supported adapter path
and do not require global installation:

`TracingConfig.SampleRatio` is explicit: zero samples no new root spans and one
samples all new root spans. The library never silently turns a zero sampling
budget into full export.

```go
handler := tracing.HTTP("http.server", applicationHandler)
client := tracing.HTTPClient(nil)

server := grpc.NewServer(tracing.GRPCServerOption())
connection, err := grpc.NewClient(target, credentials, tracing.GRPCClientOption())

poolConfig.ConnConfig.Tracer = tracing.PostgreSQLTracer()
```

The HTTP and gRPC adapters use W3C trace context without depending on global installation.
The PostgreSQL tracer omits SQL text and parameters and reduces query span names
to their operation, while connection/pool construction and database metrics
remain consumer-owned.

## Repository seam

```text
consumer application -> gouterstellar-platform/auth
consumer application -> gouterstellar-platform/durablefile
consumer application -> gouterstellar-platform/web
consumer application -> gouterstellar-platform/ui
consumer application -> gouterstellar-platform/i18n
consumer application -> gouterstellar-platform/migration
consumer application -> gouterstellar-platform/observability
gouterstellar-platform -X-> consumer application
```

Adding `package main`, application wiring, product-specific handlers,
templates, assets, schemas, deployment configuration, or an in-tree plugin is a
repository violation. The repository-owned boundary check runs both locally and
in CI, and its explicit package/root allow-lists make every new module an
intentional architecture change.

## Development

Requires Go 1.26.2 or newer.

The Go test suite enforces the library-only seam without Make, PowerShell, or a
repository-specific executable. Run the complete local Go gate directly:

```bash
go mod verify
go mod tidy -diff
go vet ./...
go test ./... -count=1
```

CI runs the same Go checks on Linux and Windows, plus golangci-lint and the Go
race detector on Linux.

## License

[MIT](LICENSE)
