// Package i18n loads application-owned Java properties bundles and provides
// concurrent, locale-aware translation without prescribing supported languages.
package i18n

import (
	"bufio"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	placeholderRE = regexp.MustCompile(`\{(\d+)\}`)
	javaFormatRE  = regexp.MustCompile(`%[sd]`)
)

// Language describes one locale supplied by an application.
type Language struct {
	Code        string
	DisplayName string
	NativeName  string
}

// Options defines an application-owned translation catalog. Every declared
// language must have a <code>.properties file below BasePath.
type Options struct {
	FS            fs.FS
	BasePath      string
	DefaultLocale string
	Languages     []Language
}

// Translator is an immutable application catalog safe for concurrent use.
type Translator struct {
	defaultLocale string
	languages     []Language
	translations  map[string]map[string]string
}

// Localizer resolves translations for one immutable locale.
type Localizer struct {
	translator *Translator
	locale     string
}

// New validates and eagerly loads the complete application catalog.
func New(opts Options) (*Translator, error) {
	if opts.FS == nil {
		return nil, fmt.Errorf("translation filesystem is required")
	}
	if strings.TrimSpace(opts.DefaultLocale) == "" {
		return nil, fmt.Errorf("default locale is required")
	}
	if len(opts.Languages) == 0 {
		return nil, fmt.Errorf("at least one language is required")
	}

	translations := make(map[string]map[string]string, len(opts.Languages))
	languages := append([]Language(nil), opts.Languages...)
	defaultFound := false
	for _, language := range languages {
		code := strings.TrimSpace(language.Code)
		if code == "" || strings.ContainsAny(code, `/\\`) {
			return nil, fmt.Errorf("invalid language code %q", language.Code)
		}
		if _, exists := translations[code]; exists {
			return nil, fmt.Errorf("duplicate language code %q", code)
		}
		bundle, err := loadBundle(opts.FS, opts.BasePath, code)
		if err != nil {
			return nil, err
		}
		translations[code] = bundle
		defaultFound = defaultFound || code == opts.DefaultLocale
	}
	if !defaultFound {
		return nil, fmt.Errorf("default locale %q is not declared", opts.DefaultLocale)
	}

	return &Translator{
		defaultLocale: opts.DefaultLocale,
		languages:     languages,
		translations:  translations,
	}, nil
}

// ForLocale returns an immutable locale-bound reader.
func (t *Translator) ForLocale(locale string) (Localizer, error) {
	if _, supported := t.translations[locale]; !supported {
		return Localizer{}, fmt.Errorf("unsupported locale %q", locale)
	}
	return Localizer{translator: t, locale: locale}, nil
}

// Default returns a reader bound to the configured fallback locale.
func (t *Translator) Default() Localizer {
	return Localizer{translator: t, locale: t.defaultLocale}
}

// Languages returns an independent copy of the application catalog metadata.
func (t *Translator) Languages() []Language {
	return append([]Language(nil), t.languages...)
}

// Locale returns the locale fixed to this reader.
func (l Localizer) Locale() string {
	return l.locale
}

// Translate resolves a key in this locale and then in the default locale.
func (l Localizer) Translate(key string, params ...any) string {
	return l.translator.translate(l.locale, key, key, params)
}

// TranslateOrDefault uses defaultValue when neither locale contains key.
func (l Localizer) TranslateOrDefault(key, defaultValue string, params ...any) string {
	return l.translator.translate(l.locale, key, defaultValue, params)
}

func (t *Translator) translate(locale, key, fallback string, params []any) string {
	if bundle, ok := t.translations[locale]; ok {
		if value, found := bundle[key]; found {
			return injectParams(value, params)
		}
	}
	if bundle := t.translations[t.defaultLocale]; bundle != nil {
		if value, found := bundle[key]; found {
			return injectParams(value, params)
		}
	}
	return injectParams(fallback, params)
}

func loadBundle(fsys fs.FS, basePath, locale string) (map[string]string, error) {
	filename := path.Join(strings.TrimSpace(basePath), locale+".properties")
	data, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return nil, fmt.Errorf("load locale %q from %q: %w", locale, filename, err)
	}
	return parseProperties(data)
}

func parseProperties(data []byte) (map[string]string, error) {
	properties := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		separator := strings.IndexAny(line, "=:")
		if separator < 0 {
			continue
		}
		key := strings.TrimSpace(line[:separator])
		properties[key] = decodePropertyValue(strings.TrimSpace(line[separator+1:]))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse properties: %w", err)
	}
	return properties, nil
}

func decodePropertyValue(value string) string {
	var decoded strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 >= len(value) {
			decoded.WriteByte(value[i])
			continue
		}
		i++
		switch value[i] {
		case 'u':
			if i+4 < len(value) {
				if codepoint, err := strconv.ParseUint(value[i+1:i+5], 16, 16); err == nil {
					decoded.WriteRune(rune(codepoint))
					i += 4
					continue
				}
			}
			decoded.WriteString(`\u`)
		case 't':
			decoded.WriteByte('\t')
		case 'n':
			decoded.WriteByte('\n')
		case 'r':
			decoded.WriteByte('\r')
		case 'f':
			decoded.WriteByte('\f')
		default:
			decoded.WriteByte(value[i])
		}
	}
	return decoded.String()
}

func injectParams(message string, params []any) string {
	if len(params) == 0 {
		return message
	}
	result := placeholderRE.ReplaceAllStringFunc(message, func(match string) string {
		index, err := strconv.Atoi(placeholderRE.FindStringSubmatch(match)[1])
		if err != nil || index >= len(params) {
			return match
		}
		return fmt.Sprint(params[index])
	})
	if javaFormatRE.MatchString(result) {
		return fmt.Sprintf(result, params...)
	}
	return result
}
