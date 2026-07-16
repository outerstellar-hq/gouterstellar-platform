package i18n

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata
var testFS embed.FS

func TestTranslate(t *testing.T) {
	svc := NewI18nService(testFS, "testdata")
	assert.Equal(t, "Hello World", svc.Translate("greeting"))
	assert.Equal(t, "greeting.missing", svc.Translate("greeting.missing"))
}

func TestTranslateOrDefault(t *testing.T) {
	svc := NewI18nService(testFS, "testdata")
	assert.Equal(t, "Hello World", svc.TranslateOrDefault("greeting", "fallback"))
	assert.Equal(t, "fallback", svc.TranslateOrDefault("missing.key", "fallback"))
}

func TestParameterInjection(t *testing.T) {
	result := injectParams("Hello {0}, you have {1} messages", []interface{}{"Alice", 5})
	assert.Equal(t, "Hello Alice, you have 5 messages", result)

	result = injectParams("No placeholders", nil)
	assert.Equal(t, "No placeholders", result)

	result = injectParams("Out of range {2}", []interface{}{"a", "b"})
	assert.Equal(t, "Out of range {2}", result)

	result = injectParams("Version %s has %d messages", []interface{}{"3.6.11", 4})
	assert.Equal(t, "Version 3.6.11 has 4 messages", result)
}

func TestParsePropertiesDecodesJavaEscapes(t *testing.T) {
	props := parseProperties([]byte("accent=Param\\u00e8tres\nline=first\\nsecond\nseparator=one\\:two\n"))

	assert.Equal(t, "Paramètres", props["accent"])
	assert.Equal(t, "first\nsecond", props["line"])
	assert.Equal(t, "one:two", props["separator"])
}

func TestLocaleSwitching(t *testing.T) {
	svc := NewI18nService(testFS, "testdata")
	assert.Equal(t, "en", svc.Locale())
	assert.Equal(t, "Hello World", svc.Translate("greeting"))

	svc.SetLocale("de")
	assert.Equal(t, "de", svc.Locale())
	assert.Equal(t, "Hallo Welt", svc.Translate("greeting"))

	svc.SetLocale("fr")
	assert.Equal(t, "fr", svc.Locale())
	assert.Equal(t, "Hello World", svc.Translate("greeting"))
}

func TestTranslateForLocaleInjectsParametersWithoutChangingGlobalLocale(t *testing.T) {
	svc := NewI18nService(testFS, "testdata")

	assert.Equal(t, "Hallo Alice", svc.TranslateForLocale("de", "welcome", "Alice"))
	assert.Equal(t, "en", svc.Locale())
}

func TestLanguageHelpers(t *testing.T) {
	assert.True(t, IsSupported("en"))
	assert.True(t, IsSupported("de"))
	assert.True(t, IsSupported("fr"))
	assert.False(t, IsSupported("xx"))

	lang := ForCode("en")
	if assert.NotNil(t, lang) {
		assert.Equal(t, "English", lang.DisplayName)
	}

	assert.Nil(t, ForCode("xx"))
}
