package i18n

import (
	"fmt"
	"sync"
	"testing"
	"testing/fstest"
)

func TestTranslatorUsesApplicationCatalogAndFallback(t *testing.T) {
	translator, err := New(Options{
		FS: fstest.MapFS{
			"locales/en.properties": {Data: []byte("greeting=Hello {0}\nversion=Version %s has %d messages\n")},
			"locales/de.properties": {Data: []byte("greeting=Hallo {0}\n")},
		},
		BasePath:      "locales",
		DefaultLocale: "en",
		Languages: []Language{
			{Code: "en", DisplayName: "English", NativeName: "English"},
			{Code: "de", DisplayName: "German", NativeName: "Deutsch"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defaultLocalizer := translator.Default()
	if got := defaultLocalizer.Translate("greeting", "Alex"); got != "Hello Alex" {
		t.Fatalf("Translate() = %q", got)
	}
	german, err := translator.ForLocale("de")
	if err != nil {
		t.Fatal(err)
	}
	if got := german.Translate("greeting", "Alex"); got != "Hallo Alex" {
		t.Fatalf("Translate() = %q", got)
	}
	if got := german.Translate("version", "3.6.11", 4); got != "Version 3.6.11 has 4 messages" {
		t.Fatalf("fallback translation = %q", got)
	}
	if german.Locale() != "de" || defaultLocalizer.Locale() != "en" {
		t.Fatal("localizer changed locale")
	}
}

func TestNewRejectsIncompleteCatalog(t *testing.T) {
	_, err := New(Options{
		FS:            fstest.MapFS{"locales/en.properties": {Data: []byte("ok=yes\n")}},
		BasePath:      "locales",
		DefaultLocale: "en",
		Languages:     []Language{{Code: "en"}, {Code: "de"}},
	})
	if err == nil {
		t.Fatal("New() accepted a missing declared locale")
	}
}

func TestForLocaleRejectsUnsupportedLocale(t *testing.T) {
	translator, err := New(Options{
		FS:            fstest.MapFS{"en.properties": {Data: []byte("ok=yes\n")}},
		DefaultLocale: "en",
		Languages:     []Language{{Code: "en"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := translator.ForLocale("fr"); err == nil {
		t.Fatal("ForLocale() accepted an unsupported locale")
	}
}

func TestLocalizersKeepConcurrentRequestsIsolated(t *testing.T) {
	translator, err := New(Options{
		FS: fstest.MapFS{
			"en.properties": {Data: []byte("greeting=Hello\n")},
			"de.properties": {Data: []byte("greeting=Hallo\n")},
		},
		DefaultLocale: "en",
		Languages:     []Language{{Code: "en"}, {Code: "de"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	english := translator.Default()
	german, err := translator.ForLocale("de")
	if err != nil {
		t.Fatal(err)
	}

	const translationsPerRequest = 1_000
	errors := make(chan error, 2)
	var requests sync.WaitGroup
	for _, request := range []struct {
		localizer Localizer
		want      string
	}{{english, "Hello"}, {german, "Hallo"}} {
		requests.Add(1)
		go func() {
			defer requests.Done()
			for range translationsPerRequest {
				if got := request.localizer.Translate("greeting"); got != request.want {
					errors <- fmt.Errorf("locale %q translated greeting as %q", request.localizer.Locale(), got)
					return
				}
			}
		}()
	}
	requests.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestParsePropertiesDecodesEscapes(t *testing.T) {
	properties, err := parseProperties([]byte("accent=Param\\u00e8tres\nline=first\\nsecond\nseparator=one\\:two\n"))
	if err != nil {
		t.Fatal(err)
	}
	if properties["accent"] != "Paramètres" || properties["line"] != "first\nsecond" || properties["separator"] != "one:two" {
		t.Fatalf("unexpected properties: %#v", properties)
	}
}
