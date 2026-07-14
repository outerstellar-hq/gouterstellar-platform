package i18n

import (
	"bufio"
	"embed"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type Translatable interface {
	UpdateTexts()
}

type I18nService struct {
	mu             sync.RWMutex
	locale         string
	translations   map[string]map[string]string
	dynamicBundles map[string]map[string]string
	listeners      []Translatable
	fsys           embed.FS
	basePath       string
}

var placeholderRe = regexp.MustCompile(`\{(\d+)\}`)

func NewI18nService(fsys embed.FS, basePath string) *I18nService {
	svc := &I18nService{
		locale:         "en",
		translations:   make(map[string]map[string]string),
		dynamicBundles: make(map[string]map[string]string),
		fsys:           fsys,
		basePath:       basePath,
	}
	// Preload every supported locale so TranslateForLocale can resolve any
	// language under the read lock without mutating the service locale. The
	// default locale is loaded first; others are best-effort (missing files
	// yield an empty bundle and fall back to "en" at lookup time).
	svc.translations["en"] = svc.loadLocale("en")
	for _, lang := range AvailableLanguages() {
		if lang.Code == "en" {
			continue
		}
		if _, ok := svc.translations[lang.Code]; !ok {
			svc.translations[lang.Code] = svc.loadLocale(lang.Code)
		}
	}
	return svc
}

func (s *I18nService) SetLocale(locale string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if locale == s.locale {
		return
	}

	s.locale = locale
	if _, ok := s.translations[locale]; !ok {
		s.translations[locale] = s.loadLocale(locale)
	}
	s.dynamicBundles[locale] = make(map[string]string)

	for _, l := range s.listeners {
		l.UpdateTexts()
	}

	slog.Info("i18n locale changed", "locale", locale)
}

func (s *I18nService) Locale() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.locale
}

func (s *I18nService) Translate(key string, params ...interface{}) string {
	return s.TranslateOrDefault(key, key, params...)
}

func (s *I18nService) TranslateOrDefault(key, defaultVal string, params ...interface{}) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	locale := s.locale

	if dyn, ok := s.dynamicBundles[locale]; ok {
		if val, found := dyn[key]; found {
			return injectParams(val, params)
		}
	}

	if bundle, ok := s.translations[locale]; ok {
		if val, found := bundle[key]; found {
			return injectParams(val, params)
		}
	}

	if locale != "en" {
		if bundle, ok := s.translations["en"]; ok {
			if val, found := bundle[key]; found {
				return injectParams(val, params)
			}
		}
	}

	return injectParams(defaultVal, params)
}

// TranslateForLocale translates a key for the given locale without changing
// the service's current locale. This is safe under concurrent renders that
// use different languages: it reads the locale's bundle under the read lock
// and never mutates the service-wide locale. If the requested locale's
// bundle is not loaded, it falls back to the default locale ("en"); if the
// key is absent from the locale's bundle, it also falls back to "en".
func (s *I18nService) TranslateForLocale(locale, key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bundle, ok := s.translations[locale]
	if !ok {
		// Fall back to default locale ("en").
		bundle, ok = s.translations["en"]
		if !ok {
			return key
		}
	}

	if val, found := bundle[key]; found {
		return val
	}

	// Fall back to the default locale's bundle for the key.
	if locale != "en" {
		if defaultBundle, ok := s.translations["en"]; ok {
			if val, found := defaultBundle[key]; found {
				return val
			}
		}
	}

	return key
}

func (s *I18nService) HasKey(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	locale := s.locale

	if dyn, ok := s.dynamicBundles[locale]; ok {
		if _, found := dyn[key]; found {
			return true
		}
	}

	if bundle, ok := s.translations[locale]; ok {
		if _, found := bundle[key]; found {
			return true
		}
	}

	return false
}

func (s *I18nService) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]bool)
	var keys []string

	addKeys := func(m map[string]string) {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}

	if dyn, ok := s.dynamicBundles[s.locale]; ok {
		addKeys(dyn)
	}
	if bundle, ok := s.translations[s.locale]; ok {
		addKeys(bundle)
	}

	sort.Strings(keys)
	return keys
}

func (s *I18nService) AddListener(l Translatable) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, l)
}

func (s *I18nService) RemoveListener(l Translatable) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, listener := range s.listeners {
		if listener == l {
			s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
			return
		}
	}
}

func (s *I18nService) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.translations = make(map[string]map[string]string)
	s.translations["en"] = s.loadLocale("en")

	if s.locale != "en" {
		s.translations[s.locale] = s.loadLocale(s.locale)
	}

	s.dynamicBundles[s.locale] = make(map[string]string)

	for _, l := range s.listeners {
		l.UpdateTexts()
	}

	slog.Info("i18n translations reloaded")
}

func (s *I18nService) LoadFromProperties(data []byte, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	props := parseProperties(data)

	locale := s.locale
	if _, ok := s.dynamicBundles[locale]; !ok {
		s.dynamicBundles[locale] = make(map[string]string)
	}

	for k, v := range props {
		if key != "" {
			s.dynamicBundles[locale][key+"."+k] = v
		} else {
			s.dynamicBundles[locale][k] = v
		}
	}

	slog.Info("i18n loaded dynamic properties", "key", key, "count", len(props))
}

func (s *I18nService) loadLocale(locale string) map[string]string {
	path := s.basePath + "/" + locale + ".properties"
	data, err := s.fsys.ReadFile(path)
	if err != nil {
		if !strings.Contains(err.Error(), "file does not exist") && !strings.HasSuffix(err.Error(), "no such file") {
			slog.Warn("i18n failed to load locale", "locale", locale, "error", err)
		}
		return make(map[string]string)
	}
	return parseProperties(data)
}

func parseProperties(data []byte) map[string]string {
	props := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		idx := strings.IndexAny(line, "=:")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		props[key] = value
	}
	return props
}

func injectParams(template string, params []interface{}) string {
	if len(params) == 0 {
		return template
	}
	return placeholderRe.ReplaceAllStringFunc(template, func(match string) string {
		sub := placeholderRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		var idx int
		if _, err := fmt.Sscanf(sub[1], "%d", &idx); err != nil {
			return match
		}
		if idx >= 0 && idx < len(params) {
			return fmt.Sprintf("%v", params[idx])
		}
		return match
	})
}
