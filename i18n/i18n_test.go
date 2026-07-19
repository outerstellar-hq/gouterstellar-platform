package i18n

import (
	"fmt"
	"strings"
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

func TestNewNormalizesConfiguredLocaleCodes(t *testing.T) {
	translator, err := New(Options{
		FS:            fstest.MapFS{"en.properties": {Data: []byte("ok=yes\n")}},
		DefaultLocale: " en ",
		Languages:     []Language{{Code: " en ", DisplayName: "English"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if translator.Default().Locale() != "en" {
		t.Fatalf("default locale = %q", translator.Default().Locale())
	}
	if languages := translator.Languages(); len(languages) != 1 || languages[0].Code != "en" {
		t.Fatalf("languages = %#v", languages)
	}
}

func TestNewRejectsInconsistentMessageContracts(t *testing.T) {
	tests := map[string]struct {
		english string
		german  string
		want    string
	}{
		"indexed placeholder": {english: "greeting=Hello {0}\n", german: "greeting=Hallo\n", want: "different parameter contract"},
		"printf verb":         {english: "count=Count %d\n", german: "count=Anzahl %s\n", want: "different parameter contract"},
		"unsupported verb":    {english: "greeting=Hello %1$s\n", german: "greeting=Hallo %1$s\n", want: "unsupported format verb"},
		"unescaped percent":   {english: "progress=100% complete for %s\n", german: "progress=100%% fertig für %s\n", want: "unescaped percent"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := New(Options{
				FS: fstest.MapFS{
					"en.properties": {Data: []byte(test.english)},
					"de.properties": {Data: []byte(test.german)},
				},
				DefaultLocale: "en",
				Languages:     []Language{{Code: "en"}, {Code: "de"}},
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestNewAllowsLiteralPercentWithoutFormatting(t *testing.T) {
	translator, err := New(Options{
		FS:            fstest.MapFS{"en.properties": {Data: []byte("completion=100% complete\n")}},
		DefaultLocale: "en",
		Languages:     []Language{{Code: "en"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := translator.Default().Translate("completion"); got != "100% complete" {
		t.Fatalf("translation = %q", got)
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

func TestParsePropertiesSupportsJavaSeparatorsAndContinuations(t *testing.T) {
	parsed, err := parseProperties([]byte("space separated\ncontinued = first\\\n  second\nliteral = ${HOME}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if parsed["space"] != "separated" || parsed["continued"] != "firstsecond" || parsed["literal"] != "${HOME}" {
		t.Fatalf("unexpected properties: %#v", parsed)
	}
}
