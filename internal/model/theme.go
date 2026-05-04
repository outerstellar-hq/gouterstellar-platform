package model

type ThemeDefinition struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Colors map[string]string `json:"colors"`
}
