package ui

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"
)

func TestRendererComposesConsumerContentIntoSharedShell(t *testing.T) {
	renderer, err := NewRenderer(Options{Templates: fstest.MapFS{"page.html": &fstest.MapFile{Data: []byte(`{{define "application-content"}}<p>{{.Message}}</p>{{end}}`)}}, Patterns: []string{"*.html"}})
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	err = renderer.Render(&rendered, Shell{Language: "en", Title: "Workers", ProductName: "Example", ProductSubtitle: "Operations", Stylesheets: []string{"/assets/app.css"}, AutoRefreshSeconds: 15, Status: &Status{Label: "Home control plane", Title: "Primary", Detail: "2 workers online", Online: true}, Navigation: []NavigationGroup{{Label: "Control", Items: []NavigationItem{{Label: "Workers", URL: "/workers", Count: "2", Active: true}}}}, User: &User{DisplayName: "Operator", RoleLabel: "Administrator", Initial: "O", ProfileURL: "/profile", LogoutURL: "/logout"}, Header: Header{Context: "Primary / Control", Title: "Workers", Status: "Live database view"}, Footer: Footer{Primary: "Version 1", Secondary: "Database is authoritative"}}, struct{ Message string }{"consumer page"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<title>Workers · Example</title>", `href="/workers" aria-current="page"`, "consumer page", "Operator", "Live database view", "Version 1"} {
		if !bytes.Contains(rendered.Bytes(), []byte(want)) {
			t.Errorf("rendered shell missing %q", want)
		}
	}
}

func TestRendererRequiresConsumerContent(t *testing.T) {
	_, err := NewRenderer(Options{Templates: fstest.MapFS{"page.html": &fstest.MapFile{Data: []byte(`{{define "other"}}x{{end}}`)}}, Patterns: []string{"*.html"}})
	if err == nil {
		t.Fatal("expected missing application-content error")
	}
}

func TestSharedShellCannotBeOverriddenByConsumer(t *testing.T) {
	consumer := fstest.MapFS{"page.html": &fstest.MapFile{Data: []byte(`{{define "application-content"}}content{{end}}{{define "shared-shell"}}consumer override{{end}}`)}}
	if _, err := NewRenderer(Options{Templates: consumer, Patterns: []string{"page.html"}}); err == nil {
		t.Fatal("consumer shared template name was accepted")
	}
}

func TestExecuteTemplateOnlyRendersApplicationOwnedNames(t *testing.T) {
	renderer, err := NewRenderer(Options{
		Templates: fstest.MapFS{"page.html": &fstest.MapFile{Data: []byte(`{{define "application-content"}}content{{end}}{{define "fragment"}}fragment {{.}}{{end}}`)}},
		Patterns:  []string{"page.html"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	if err := renderer.ExecuteTemplate(&rendered, "fragment", "value"); err != nil {
		t.Fatal(err)
	}
	if rendered.String() != "fragment value" {
		t.Fatalf("fragment = %q", rendered.String())
	}
	if err := renderer.ExecuteTemplate(&rendered, "shared-shell", nil); err == nil {
		t.Fatal("shared template executed through application seam")
	}
}

func TestRendererUsesConsumerLocalizedChromeLabels(t *testing.T) {
	renderer, err := NewRenderer(Options{
		Templates: fstest.MapFS{"page.html": &fstest.MapFile{Data: []byte(`{{define "application-content"}}Inhalt{{end}}`)}},
		Patterns:  []string{"*.html"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	err = renderer.Render(&rendered, Shell{
		Title: "Seite", ProductName: "Beispiel",
		User: &User{DisplayName: "Benutzer", ProfileURL: "/profil", LogoutURL: "/abmelden"},
		Labels: ShellLabels{
			SkipToContent: "Zum Inhalt springen", PrimaryNavigation: "Hauptnavigation", SignOut: "Abmelden",
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Zum Inhalt springen", `aria-label="Hauptnavigation"`, ">Abmelden</button>"} {
		if !strings.Contains(rendered.String(), want) {
			t.Errorf("localized shell missing %q", want)
		}
	}
}

func TestShellRejectsCrossOriginNavigation(t *testing.T) {
	err := (Shell{Title: "Example", ProductName: "Example", Navigation: []NavigationGroup{{Items: []NavigationItem{{Label: "Bad", URL: "https://example.com"}}}}}).Validate()
	if err == nil {
		t.Fatal("expected cross-origin navigation error")
	}
}

func TestShellRejectsCrossOriginChromeURLs(t *testing.T) {
	for name, shell := range map[string]Shell{
		"brand":      {Title: "Example", ProductName: "Example", BrandURL: "https://example.com"},
		"backslash":  {Title: "Example", ProductName: "Example", BrandURL: `/\untrusted.example/path`},
		"encoded":    {Title: "Example", ProductName: "Example", BrandURL: `/%2f%2funtrusted.example/path`},
		"stylesheet": {Title: "Example", ProductName: "Example", Stylesheets: []string{"//example.com/app.css"}},
		"avatar":     {Title: "Example", ProductName: "Example", User: &User{AvatarURL: "https://example.com/avatar.png", ProfileURL: "/profile", LogoutURL: "/logout"}},
		"profile":    {Title: "Example", ProductName: "Example", User: &User{ProfileURL: "https://example.com", LogoutURL: "/logout"}},
		"logout":     {Title: "Example", ProductName: "Example", User: &User{ProfileURL: "/profile", LogoutURL: "logout"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := shell.Validate(); err == nil {
				t.Fatal("expected cross-origin URL error")
			}
		})
	}
}
