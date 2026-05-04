package web

import (
	"encoding/json"
	"html/template"
	"net/url"
	"strconv"
	"strings"
	"time"
)

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
		"urlEncode": func(s string) string {
			return url.QueryEscape(s)
		},
	}
}
