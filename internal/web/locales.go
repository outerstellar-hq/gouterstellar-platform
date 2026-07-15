package web

import "embed"

// LocaleFS embeds the bundled locale .properties files so the i18n service can
// load translations without touching the filesystem at runtime.
//
//go:embed locales
var LocaleFS embed.FS

// LocaleBasePath is the directory inside LocaleFS that holds the per-locale
// .properties files (e.g. "locales/en.properties").
const LocaleBasePath = "locales"
