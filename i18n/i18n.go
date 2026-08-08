// Package i18n loads application-owned Java properties bundles and provides
// concurrent, locale-aware translation without prescribing supported languages.
package i18n

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/magiconair/properties"
)

var (
	placeholderRE = regexp.MustCompile(`\{(\d+)\}`)
	javaFormatRE  = regexp.MustCompile(`%[sd]`)
	javaVerbRE    = regexp.MustCompile(`^%(?:\d+\$)?[-#+0,(<]*\d*(?:\.\d+)?[bBhHsScCdoxXeEfgGaAtTn]`)
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
	translations  map[string]map[string]compiledMessage
}

// Localizer resolves translations for one immutable locale.
type Localizer struct {
	translator *Translator
	locale     string
}

type messageSignature struct {
	indexed []int
	verbs   []byte
}

type messagePlaceholder struct {
	start int
	end   int
	index int
}

type compiledMessage struct {
	template     string
	placeholders []messagePlaceholder
	signature    messageSignature
	formatted    bool
}

// New validates and eagerly loads the complete application catalog.
func New(opts Options) (*Translator, error) {
	if opts.FS == nil {
		return nil, fmt.Errorf("translation filesystem is required")
	}
	defaultLocale := strings.TrimSpace(opts.DefaultLocale)
	if defaultLocale == "" {
		return nil, fmt.Errorf("default locale is required")
	}
	if len(opts.Languages) == 0 {
		return nil, fmt.Errorf("at least one language is required")
	}

	rawTranslations := make(map[string]map[string]string, len(opts.Languages))
	languages := append([]Language(nil), opts.Languages...)
	defaultFound := false
	for index := range languages {
		language := &languages[index]
		code := strings.TrimSpace(language.Code)
		if code == "" || strings.ContainsAny(code, `/\\`) {
			return nil, fmt.Errorf("invalid language code %q", language.Code)
		}
		language.Code = code
		if _, exists := rawTranslations[code]; exists {
			return nil, fmt.Errorf("duplicate language code %q", code)
		}
		bundle, err := loadBundle(opts.FS, opts.BasePath, code)
		if err != nil {
			return nil, err
		}
		rawTranslations[code] = bundle
		defaultFound = defaultFound || code == defaultLocale
	}
	if !defaultFound {
		return nil, fmt.Errorf("default locale %q is not declared", defaultLocale)
	}
	translations, err := compileCatalog(languages, defaultLocale, rawTranslations)
	if err != nil {
		return nil, err
	}

	return &Translator{
		defaultLocale: defaultLocale,
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
		if message, found := bundle[key]; found {
			return message.render(params)
		}
	}
	if bundle := t.translations[t.defaultLocale]; bundle != nil {
		if message, found := bundle[key]; found {
			return message.render(params)
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
	loader := properties.Loader{Encoding: properties.UTF8, DisableExpansion: true}
	parsed, err := loader.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parse properties: %w", err)
	}
	return parsed.Map(), nil
}

func compileCatalog(languages []Language, defaultLocale string, translations map[string]map[string]string) (map[string]map[string]compiledMessage, error) {
	compiled := make(map[string]map[string]compiledMessage, len(translations))
	defaultMessages, err := compileBundle(defaultLocale, translations[defaultLocale])
	if err != nil {
		return nil, err
	}
	compiled[defaultLocale] = defaultMessages

	for _, language := range languages {
		if language.Code == defaultLocale {
			continue
		}
		messages, err := compileBundle(language.Code, translations[language.Code])
		if err != nil {
			return nil, err
		}
		for key, message := range messages {
			defaultMessage, comparable := defaultMessages[key]
			if comparable && !sameMessageSignature(message.signature, defaultMessage.signature) {
				return nil, fmt.Errorf("locale %q key %q has a different parameter contract from default locale %q", language.Code, key, defaultLocale)
			}
		}
		compiled[language.Code] = messages
	}
	return compiled, nil
}

func compileBundle(locale string, translations map[string]string) (map[string]compiledMessage, error) {
	compiled := make(map[string]compiledMessage, len(translations))
	for key, message := range translations {
		compiledMessage, err := compileMessage(message)
		if err != nil {
			return nil, fmt.Errorf("validate locale %q key %q: %w", locale, key, err)
		}
		compiled[key] = compiledMessage
	}
	return compiled, nil
}

func compileMessage(message string) (compiledMessage, error) {
	signature, err := parseMessageSignature(message)
	if err != nil {
		return compiledMessage{}, err
	}
	matches := placeholderRE.FindAllStringSubmatchIndex(message, -1)
	placeholders := make([]messagePlaceholder, 0, len(matches))
	for _, match := range matches {
		index, err := strconv.Atoi(message[match[2]:match[3]])
		if err != nil {
			return compiledMessage{}, fmt.Errorf("parse placeholder %q: %w", message[match[0]:match[1]], err)
		}
		placeholders = append(placeholders, messagePlaceholder{start: match[0], end: match[1], index: index})
	}
	return compiledMessage{
		template:     message,
		placeholders: placeholders,
		signature:    signature,
		formatted:    javaFormatRE.MatchString(message),
	}, nil
}

func parseMessageSignature(message string) (messageSignature, error) {
	indexes := make(map[int]struct{})
	for _, match := range placeholderRE.FindAllStringSubmatch(message, -1) {
		index, err := strconv.Atoi(match[1])
		if err != nil {
			return messageSignature{}, fmt.Errorf("parse placeholder %q: %w", match[0], err)
		}
		indexes[index] = struct{}{}
	}
	signature := messageSignature{indexed: make([]int, 0, len(indexes))}
	for index := range indexes {
		signature.indexed = append(signature.indexed, index)
	}
	sort.Ints(signature.indexed)

	formatted := javaFormatRE.MatchString(message)
	for index := 0; index < len(message); index++ {
		if message[index] != '%' || index+1 >= len(message) {
			continue
		}
		switch message[index+1] {
		case '%':
			index++
		case 's', 'd':
			signature.verbs = append(signature.verbs, message[index+1])
			index++
		default:
			if verb := javaVerbRE.FindString(message[index:]); verb != "" {
				return messageSignature{}, fmt.Errorf("unsupported format verb %q; use only %%s, %%d, and %%%%", verb)
			}
			if formatted {
				return messageSignature{}, fmt.Errorf("unescaped percent in formatted message; use %%%% for a literal percent")
			}
		}
	}
	return signature, nil
}

func sameMessageSignature(first, second messageSignature) bool {
	return slices.Equal(first.indexed, second.indexed) && slices.Equal(first.verbs, second.verbs)
}

func (message compiledMessage) render(params []any) string {
	if len(params) == 0 {
		return message.template
	}
	var result strings.Builder
	result.Grow(len(message.template))
	last := 0
	for _, placeholder := range message.placeholders {
		result.WriteString(message.template[last:placeholder.start])
		if placeholder.index >= len(params) {
			result.WriteString(message.template[placeholder.start:placeholder.end])
		} else {
			_, _ = fmt.Fprint(&result, params[placeholder.index])
		}
		last = placeholder.end
	}
	result.WriteString(message.template[last:])
	if message.formatted {
		return fmt.Sprintf(result.String(), params...)
	}
	return result.String()
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
