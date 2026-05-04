package i18n

type Language struct {
	DisplayName string
	Code        string
	NativeName  string
}

func AvailableLanguages() []Language {
	return []Language{
		{DisplayName: "English", Code: "en", NativeName: "English"},
		{DisplayName: "Deutsch", Code: "de", NativeName: "Deutsch"},
		{DisplayName: "Fran\u00e7ais", Code: "fr", NativeName: "Fran\u00e7ais"},
	}
}

func IsSupported(code string) bool {
	for _, lang := range AvailableLanguages() {
		if lang.Code == code {
			return true
		}
	}
	return false
}

func ForCode(code string) *Language {
	for _, lang := range AvailableLanguages() {
		if lang.Code == code {
			return &lang
		}
	}
	return nil
}
