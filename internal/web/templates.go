package web

import "embed"

//go:embed template
var templateFS embed.FS

func TemplateFS() embed.FS {
	return templateFS
}
