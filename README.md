# gouterstellar-platform

Reusable Go libraries for independently deployed Outerstellar applications.
This repository is not an application host: it has no executable, routes,
product schema, deployment image, product assets, or in-tree plugins.

## Library catalog

| Module | Shared responsibility | Proven implementation underneath |
| --- | --- | --- |
| `auth` | Argon2id passwords, opaque tokens, server-side sessions, principals, JWTs, TOTP | `alexedwards/argon2id`, `alexedwards/scs`, `golang-jwt/jwt`, `pquerna/otp` |
| `web` | masked CSRF tokens, CSP nonces, security headers, body limits, sensitive-response caching | `gorilla/csrf`, `net/http` |
| `ui` | shared server-rendered application shell and composition contract | `html/template`, `embed` |
| `i18n` | application-owned Java `.properties` catalog loading and lookup | Go standard library |
| `migration` | application-owned embedded migration sets | `golang-migrate/migrate` with `iofs` |
| `observability` | OTLP tracing construction and HTTP instrumentation | OpenTelemetry Go |

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

sessions, err := auth.NewSessions(store, auth.PrincipalResolverFunc(
    func(ctx context.Context, subject string) (auth.Principal, error) {
        user, err := users.FindCurrent(ctx, subject)
        if errors.Is(err, ErrUserDisabled) {
            return auth.Principal{}, auth.ErrUnauthenticated
        }
        if err != nil {
            return auth.Principal{}, err
        }
        return auth.Principal{Subject: user.ID, Roles: user.Roles}, nil
    },
), auth.SessionConfig{CookieName: "example_session"})
if err != nil {
    return err
}

handler := sessions.Middleware(auth.RequireAuthenticated(applicationHandler))
```

Call `sessions.SignIn(request.Context(), user.ID)` only from a handler already
inside `sessions.Middleware`; it renews the session token before changing
privilege. `SignOut` destroys the server-side session and expires the cookie.
Production cookies are Secure by default. Local plain-HTTP development must opt
into `AllowInsecureCookies` explicitly.

`auth.JWTs` is intended for short-lived API bearer tokens. It requires a
256-bit HMAC secret, issuer, audience, expiry, and a fixed HS256 allow-list.
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

handler := web.SecurityHeaders(web.SecurityHeadersConfig{HSTS: true})(
    csrfMiddleware(applicationHandler),
)
```

Use `web.CSRFToken(request)` for JSON clients or `web.CSRFField(request)` in
server-rendered forms. Use `web.CSPNonce(request.Context())` in an inline script
or style only when the configured CSP permits it.

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
Locale-bound readers are immutable, so concurrent requests cannot change each
other's language.

## Embedded migrations and tracing

`migration.New` accepts an application-owned `fs.FS`, directory, and database
URL. The consumer registers its `golang-migrate` database driver through the
driver's normal blank import. `Runner.Up` treats an already-current schema as
success; the migration SQL and database URL stay in the consumer repository.

`observability.NewTracing` constructs an OTLP tracer provider and returns its
shutdown lifecycle without silently changing process globals. Consumers may
call `InstallGlobal` explicitly and use `observability.HTTP` or
`observability.HTTPClient` at their HTTP seams.

## Repository seam

```text
consumer application -> gouterstellar-platform/auth
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

```bash
make check
```

The full gate verifies module checksums, module-file tidiness, the library-only
seam, vet, tests, and lint.

## License

[MIT](LICENSE)
