package i18n

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestLoadFromProperties(t *testing.T) {
	svc := NewI18nService(testFS, "testdata")

	data := []byte("custom.hello=Custom Hello\ncustom.world=Custom World\n")
	svc.LoadFromProperties(data, "app")

	assert.Equal(t, "Custom Hello", svc.Translate("app.custom.hello"))
	assert.Equal(t, "Custom World", svc.Translate("app.custom.world"))
}

func TestHasKey(t *testing.T) {
	svc := NewI18nService(testFS, "testdata")
	assert.True(t, svc.HasKey("greeting"))
	assert.False(t, svc.HasKey("nonexistent"))
}

func TestKeys(t *testing.T) {
	svc := NewI18nService(testFS, "testdata")
	keys := svc.Keys()
	require.NotEmpty(t, keys)
	assert.Contains(t, keys, "greeting")
}

func TestReload(t *testing.T) {
	svc := NewI18nService(testFS, "testdata")
	assert.Equal(t, "Hello World", svc.Translate("greeting"))
	svc.Reload()
	assert.Equal(t, "Hello World", svc.Translate("greeting"))
}

func TestListenerNotification(t *testing.T) {
	svc := NewI18nService(testFS, "testdata")
	notified := false
	listener := &testTranslatable{onUpdate: func() { notified = true }}
	svc.AddListener(listener)
	svc.SetLocale("de")
	assert.True(t, notified)

	notified = false
	svc.RemoveListener(listener)
	svc.SetLocale("en")
	assert.False(t, notified)
}

func TestLanguageHelpers(t *testing.T) {
	assert.True(t, IsSupported("en"))
	assert.True(t, IsSupported("de"))
	assert.True(t, IsSupported("fr"))
	assert.False(t, IsSupported("xx"))

	lang := ForCode("en")
	require.NotNil(t, lang)
	assert.Equal(t, "English", lang.DisplayName)

	assert.Nil(t, ForCode("xx"))
}

type testTranslatable struct {
	onUpdate func()
}

func (t *testTranslatable) UpdateTexts() {
	if t.onUpdate != nil {
		t.onUpdate()
	}
}
