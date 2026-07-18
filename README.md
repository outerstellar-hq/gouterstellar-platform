# gouterstellar-platform

Reusable Go libraries for Outerstellar applications. This repository does not
build or host an application: it has no executable, routes, deployment image,
database schema, product assets, or in-tree product modules.

## Libraries

### `ui`

`ui` owns the shared server-rendered application shell: document structure,
navigation, account chrome, page header, footer, accessibility landmarks, and
the template composition contract. Consumer repositories own their page
templates, handlers, data, routes, authentication, and CSS.

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
    Navigation: []ui.NavigationGroup{{
        Label: "Control",
        Items: []ui.NavigationItem{{Label: "Workers", URL: "/workers", Active: true}},
    }},
    Header: ui.Header{Title: "Workers"},
}, pageData)
```

The consumer template set must define exactly one integration seam:

```gotemplate
{{define "application-content"}}
  {{template "workers-page" .}}
{{end}}
```

The shared shell is parsed after consumer templates, so consumers cannot
accidentally replace its reserved definitions. Navigation, brand, profile,
logout, and stylesheet URLs must be same-origin absolute paths.

### `i18n`

`i18n` loads application-owned Java `.properties` bundles from any `fs.FS`.
The application declares its languages and default locale; the library does not
hardcode product language policy. Construction fails when a declared bundle is
missing or invalid.

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
```

## Dependency boundary

The direction is intentionally one-way:

```text
consumer application -> gouterstellar-platform/ui
consumer application -> gouterstellar-platform/i18n
gouterstellar-platform -X-> consumer application
```

Product implementations belong in their own repositories. Adding `package
main`, an application host, product-specific templates/assets, deployment
configuration, or an in-tree plugin is a boundary violation. CI checks this
rule in addition to tests and static analysis.

## Development

Requires Go 1.26.2 or newer.

```bash
make check
```

The equivalent commands are:

```bash
go mod verify
go mod tidy -diff
go vet ./...
go test ./... -count=1
```

## License

[MIT](LICENSE)
