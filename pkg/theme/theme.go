package theme

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
)

type ColorScheme struct {
	Base    string `json:"base"`
	Hover   string `json:"hover"`
	Pressed string `json:"pressed"`
}

type ThemeColors struct {
	Colors map[string]ColorScheme `json:"colors"`
}

type ThemeService struct {
	mu         sync.RWMutex
	colors     map[string]ColorScheme
	shadeCache map[string]ColorScheme
}

func NewThemeService() *ThemeService {
	return &ThemeService{
		colors:     make(map[string]ColorScheme),
		shadeCache: make(map[string]ColorScheme),
	}
}

func NewThemeServiceFromJSON(jsonStr string) (*ThemeService, error) {
	svc := NewThemeService()
	var tc ThemeColors
	if err := json.Unmarshal([]byte(jsonStr), &tc); err != nil {
		return nil, fmt.Errorf("failed to parse theme JSON: %w", err)
	}
	for name, scheme := range tc.Colors {
		svc.colors[name] = scheme
	}
	return svc, nil
}

func NewThemeServiceFromFS(fsys fs.FS, path string) (*ThemeService, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read theme file: %w", err)
	}
	return NewThemeServiceFromJSON(string(data))
}

func (s *ThemeService) GetColors() map[string]ColorScheme {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]ColorScheme, len(s.colors))
	for k, v := range s.colors {
		result[k] = v
	}
	return result
}

func (s *ThemeService) GetBaseColor(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.colors[name]; ok {
		return c.Base
	}
	return ""
}

func (s *ThemeService) GetHoverColor(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.colors[name]; ok {
		return c.Hover
	}
	return ""
}

func (s *ThemeService) GetPressedColor(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.colors[name]; ok {
		return c.Pressed
	}
	return ""
}

func (s *ThemeService) ComputeShading(baseColor string) ColorScheme {
	return ColorScheme{
		Base:    baseColor,
		Hover:   Hover(baseColor),
		Pressed: Pressed(baseColor),
	}
}

func (s *ThemeService) AddColor(name string, scheme ColorScheme) *ThemeService {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.colors[name] = scheme
	s.shadeCache = make(map[string]ColorScheme)
	return s
}

func (s *ThemeService) AddColorFromBase(name, baseColor string) *ThemeService {
	scheme := s.ComputeShading(baseColor)
	return s.AddColor(name, scheme)
}

func (s *ThemeService) GetHexMap() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string)
	for name, scheme := range s.colors {
		result[name+"-base"] = scheme.Base
		result[name+"-hover"] = scheme.Hover
		result[name+"-pressed"] = scheme.Pressed
	}
	return result
}

func (s *ThemeService) ToCSSVariables() string {
	return s.ToCSSForSelector(":root")
}

func (s *ThemeService) ToCSSForSelector(selector string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var names []string
	for name := range s.colors {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString(selector)
	b.WriteString(" {\n")
	for _, name := range names {
		scheme := s.colors[name]
		fmt.Fprintf(&b, "  --color-%s-base: %s;\n", name, scheme.Base)
		fmt.Fprintf(&b, "  --color-%s-hover: %s;\n", name, scheme.Hover)
		fmt.Fprintf(&b, "  --color-%s-pressed: %s;\n", name, scheme.Pressed)
	}
	b.WriteString("}")
	return b.String()
}

type Builder struct {
	colors map[string]ColorScheme
	errors []error
}

func NewBuilder() *Builder {
	return &Builder{
		colors: make(map[string]ColorScheme),
	}
}

func (b *Builder) FromJSON(jsonStr string) *Builder {
	svc, err := NewThemeServiceFromJSON(jsonStr)
	if err != nil {
		b.errors = append(b.errors, err)
		return b
	}
	for name, scheme := range svc.colors {
		b.colors[name] = scheme
	}
	return b
}

func (b *Builder) FromFS(fsys fs.FS, path string) *Builder {
	svc, err := NewThemeServiceFromFS(fsys, path)
	if err != nil {
		b.errors = append(b.errors, err)
		return b
	}
	for name, scheme := range svc.colors {
		b.colors[name] = scheme
	}
	return b
}

func (b *Builder) AddColor(name, baseColor string) *Builder {
	scheme := ColorScheme{
		Base:    baseColor,
		Hover:   Hover(baseColor),
		Pressed: Pressed(baseColor),
	}
	b.colors[name] = scheme
	return b
}

func (b *Builder) Build() (*ThemeService, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}
	svc := NewThemeService()
	for name, scheme := range b.colors {
		svc.colors[name] = scheme
	}
	return svc, nil
}
