package i18n

import (
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
	if got := translator.Translate("greeting", "Alex"); got != "Hello Alex" {
		t.Fatalf("Translate() = %q", got)
	}
	if err := translator.SetLocale("de"); err != nil {
		t.Fatal(err)
	}
	if got := translator.Translate("greeting", "Alex"); got != "Hallo Alex" {
		t.Fatalf("Translate() = %q", got)
	}
	if got := translator.TranslateForLocale("de", "version", "3.6.11", 4); got != "Version 3.6.11 has 4 messages" {
		t.Fatalf("fallback translation = %q", got)
	}
	if translator.Locale() != "de" {
		t.Fatal("TranslateForLocale changed the active locale")
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

func TestSetLocaleRejectsUnsupportedLocale(t *testing.T) {
	translator, err := New(Options{
		FS:            fstest.MapFS{"en.properties": {Data: []byte("ok=yes\n")}},
		DefaultLocale: "en",
		Languages:     []Language{{Code: "en"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := translator.SetLocale("fr"); err == nil {
		t.Fatal("SetLocale() accepted an unsupported locale")
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
