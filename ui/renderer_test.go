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
	err = renderer.Render(&rendered, Shell{Language: "en", Title: "Workers", ProductName: "Example", ProductSubtitle: "Operations", Stylesheets: []string{"/assets/app.css"}, Favicon: "/assets/favicon.svg", AutoRefreshSeconds: 15, Status: &Status{Label: "Home control plane", Title: "Primary", Detail: "2 workers online", Online: true}, Navigation: []NavigationGroup{{Label: "Control", Items: []NavigationItem{{Label: "Workers", URL: "/workers", Count: "2", Active: true}}}}, User: &User{DisplayName: "Operator", RoleLabel: "Administrator", Initial: "O", ProfileURL: "/profile", LogoutURL: "/logout"}, Header: Header{Context: "Primary / Control", Title: "Workers", Status: "Live database view"}, Footer: Footer{Primary: "Version 1", Secondary: "Database is authoritative"}}, struct{ Message string }{"consumer page"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<title>Workers · Example</title>", `<link rel="icon" href="/assets/favicon.svg">`, `href="/workers" aria-current="page"`, "consumer page", "Operator", "Live database view", "Version 1"} {
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
		"favicon":    {Title: "Example", ProductName: "Example", Favicon: "https://example.com/icon.svg"},
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

func TestShellOmitsIconLinkWithoutFavicon(t *testing.T) {
	renderer, err := NewRenderer(Options{
		Templates: fstest.MapFS{"page.html": &fstest.MapFile{Data: []byte(`{{define "application-content"}}content{{end}}`)}},
		Patterns:  []string{"*.html"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	if err := renderer.Render(&rendered, Shell{Title: "Example", ProductName: "Example"}, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.String(), `rel="icon"`) {
		t.Fatal("shell emitted an icon link although Favicon is empty")
	}
}

func newContentRenderer(t *testing.T) *Renderer {
	t.Helper()
	renderer, err := NewRenderer(Options{
		Templates: fstest.MapFS{"page.html": &fstest.MapFile{Data: []byte(`{{define "application-content"}}content{{end}}`)}},
		Patterns:  []string{"*.html"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return renderer
}

var layoutNavigation = []NavigationGroup{
	{Label: "Fleet", Items: []NavigationItem{
		{Label: "Workers", URL: "/workers", Description: "Every worker and its load."},
		{Label: "Queues", URL: "/queues", Active: true, Count: "3", CountAriaLabel: "queues"},
	}},
	{Label: "Settings", Items: []NavigationItem{{Label: "Profile", URL: "/profile"}}},
}

// TestTopbarLayoutRendersGroupsAsMenus: the top bar puts the brand and one
// menu per group in a header above the page; each menu is a button plus a
// panel of links carrying their descriptions, and the group holding the
// current page is marked active.
func TestTopbarLayoutRendersGroupsAsMenus(t *testing.T) {
	var rendered bytes.Buffer
	shell := Shell{Title: "Queues", ProductName: "Example", Layout: LayoutTopbar, Navigation: layoutNavigation,
		Status: &Status{Label: "Plane", Title: "Primary", Detail: "online"}}
	if err := newContentRenderer(t).Render(&rendered, shell, nil); err != nil {
		t.Fatal(err)
	}
	html := rendered.String()
	for _, want := range []string{
		`<body class="layout-topbar">`,
		`<header class="app-topbar"><a class="brand" href="/">`,
		`<nav class="app-menu" aria-label="Primary navigation">`,
		`<div class="nav-menu active">`,
		`<button type="button" class="nav-menu-label"><span>Fleet</span></button>`,
		`<strong>Workers</strong><small>Every worker and its load.</small>`,
		`href="/queues" aria-current="page"`,
		`aria-label="queues">3</span>`,
		`<div class="nav-plane" aria-label="Plane">`,
		`<main id="main-content"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("top-bar shell missing %q", want)
		}
	}
	if strings.Count(html, `class="nav-menu-label"`) != 2 || strings.Count(html, `<div class="nav-menu active">`) != 1 {
		t.Error("want two menus with only the current page's group active")
	}
	if strings.Contains(html, "app-nav") || strings.Contains(html, "layout-sidebar") {
		t.Error("top-bar shell still renders the sidebar")
	}
}

// TestSidebarLayoutIsTheDefault: without a layout the shell keeps the
// sidebar, and descriptions stay out of it.
func TestSidebarLayoutIsTheDefault(t *testing.T) {
	var rendered bytes.Buffer
	if err := newContentRenderer(t).Render(&rendered, Shell{Title: "Queues", ProductName: "Example", Navigation: layoutNavigation}, nil); err != nil {
		t.Fatal(err)
	}
	html := rendered.String()
	for _, want := range []string{`<body class="layout-sidebar">`, `<nav class="app-nav" aria-label="Primary navigation"><a class="brand"`, `<span class="nav-label">Fleet</span>`, `<div class="nav-spacer"></div>`} {
		if !strings.Contains(html, want) {
			t.Errorf("sidebar shell missing %q", want)
		}
	}
	if strings.Contains(html, "Every worker and its load.") || strings.Contains(html, "nav-menu") {
		t.Error("the sidebar renders top-bar menu markup")
	}
}

func TestShellRejectsUnknownLayout(t *testing.T) {
	if err := (Shell{Title: "Example", ProductName: "Example", Layout: "drawer"}).Validate(); err == nil {
		t.Fatal("expected unknown layout error")
	}
}

func TestNavigationGroupActiveFollowsChildren(t *testing.T) {
	group := NavigationGroup{Items: []NavigationItem{{Label: "Parent", URL: "/p", Children: []NavigationItem{{Label: "Child", URL: "/c", Active: true}}}}}
	if !group.Active() {
		t.Fatal("a group whose nested item is current is not active")
	}
	if (NavigationGroup{Items: []NavigationItem{{Label: "Other", URL: "/o"}}}).Active() {
		t.Fatal("a group without the current page reports active")
	}
}
