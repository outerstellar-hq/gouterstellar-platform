package web

import (
	"encoding/json"
	"html/template"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rygel/gouterstellar-platform/pkg/i18n"
)

// i18nHolder stores the wired I18nService so the "translate" template func
// (registered at parse time, before Wire runs) can reach it at render time.
// SetGlobalI18nService is called once during wiring; TranslateForTemplate is
// read on every render.
var (
	i18nMu  sync.RWMutex
	i18nSvc = i18n.NewI18nService(LocaleFS, LocaleBasePath)
)

// SetGlobalI18nService installs the i18n service used by the translate template
// function. It is intended to be called once during application wiring.
func SetGlobalI18nService(svc *i18n.I18nService) {
	i18nMu.Lock()
	i18nSvc = svc
	i18nMu.Unlock()
}

// TranslateForTemplate resolves a key for the given language code, falling back
// to English and then to the key itself. It is nil-safe so templates render
// even when the i18n service has not been wired. It translates for the given
// locale WITHOUT mutating the service's process-wide locale, making it safe
// under concurrent renders that use different languages.
func TranslateForTemplate(lang, key string, params ...interface{}) string {
	i18nMu.RLock()
	svc := i18nSvc
	i18nMu.RUnlock()
	if svc == nil {
		return key
	}
	if lang == "" || !i18n.IsSupported(lang) {
		return svc.TranslateForLocale("en", key, params...)
	}
	return svc.TranslateForLocale(lang, key, params...)
}

func TemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
		"formatTime": func(t time.Time) string { return t.Format("2006-01-02 15:04") },
		"formatDate": func(t time.Time) string { return t.Format("2006-01-02") },
		"timeAgo": func(t time.Time) string {
			d := time.Since(t)
			if d < time.Minute {
				return "just now"
			}
			if d < time.Hour {
				return strconv.Itoa(int(d.Minutes())) + "m ago"
			}
			if d < 24*time.Hour {
				return strconv.Itoa(int(d.Hours())) + "h ago"
			}
			return strconv.Itoa(int(d.Hours()/24)) + "d ago"
		},
		"upper":    strings.ToUpper,
		"lower":    strings.ToLower,
		"trim":     strings.TrimSpace,
		"json":     func(v interface{}) (string, error) { b, err := json.Marshal(v); return string(b), err },
		"safeHTML": func(s string) template.HTML { return template.HTML(s) }, // #nosec G203 -- intentional: used for trusted server-rendered HTML only
		"safeURL":  func(s string) template.URL { return template.URL(s) },   // #nosec G203 -- only used for server-generated QR data URIs
		"urlEncode": func(s string) string {
			return url.QueryEscape(s)
		},
		"translate": TranslateForTemplate,
	}
}
