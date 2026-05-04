package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHexToRGB(t *testing.T) {
	r, g, b, err := HexToRGB("#FF0000")
	require.NoError(t, err)
	assert.Equal(t, 255, r)
	assert.Equal(t, 0, g)
	assert.Equal(t, 0, b)

	r, g, b, err = HexToRGB("#00FF00")
	require.NoError(t, err)
	assert.Equal(t, 0, r)
	assert.Equal(t, 255, g)
	assert.Equal(t, 0, b)

	r, g, b, err = HexToRGB("#F00")
	require.NoError(t, err)
	assert.Equal(t, 255, r)
	assert.Equal(t, 0, g)
	assert.Equal(t, 0, b)

	_, _, _, err = HexToRGB("invalid")
	assert.Error(t, err)
}

func TestRGBToHex(t *testing.T) {
	assert.Equal(t, "#FF0000", RGBToHex(255, 0, 0))
	assert.Equal(t, "#00FF00", RGBToHex(0, 255, 0))
	assert.Equal(t, "#0000FF", RGBToHex(0, 0, 255))
	assert.Equal(t, "#000000", RGBToHex(0, 0, 0))
	assert.Equal(t, "#FFFFFF", RGBToHex(255, 255, 255))
}

func TestLighten(t *testing.T) {
	result := Lighten("#808080")
	assert.Equal(t, "#999999", result)

	result = Lighten("#000000")
	assert.Equal(t, "#333333", result)
}

func TestDarken(t *testing.T) {
	result := Darken("#808080")
	assert.Equal(t, "#666666", result)

	result = Darken("#FFFFFF")
	assert.Equal(t, "#CCCCCC", result)
}

func TestHoverPressed(t *testing.T) {
	base := "#3B82F6"
	hover := Hover(base)
	pressed := Pressed(base)

	r1, g1, b1, _ := HexToRGB(hover)
	r2, g2, b2, _ := HexToRGB(base)
	assert.True(t, r1+g1+b1 > r2+g2+b2, "hover should be lighter than base")

	r3, g3, b3, _ := HexToRGB(pressed)
	assert.True(t, r3+g3+b3 < r2+g2+b2, "pressed should be darker than base")
}

func TestAdjustBrightnessInvalid(t *testing.T) {
	assert.Equal(t, "notacolor", AdjustBrightness("notacolor", 0.2))
}

func TestNewThemeServiceFromJSON(t *testing.T) {
	json := `{"colors":{"primary":{"base":"#3B82F6","hover":"#60A5FA","pressed":"#2563EB"}}}`
	svc, err := NewThemeServiceFromJSON(json)
	require.NoError(t, err)

	assert.Equal(t, "#3B82F6", svc.GetBaseColor("primary"))
	assert.Equal(t, "#60A5FA", svc.GetHoverColor("primary"))
	assert.Equal(t, "#2563EB", svc.GetPressedColor("primary"))
}

func TestNewThemeServiceFromJSONInvalid(t *testing.T) {
	_, err := NewThemeServiceFromJSON("not json")
	assert.Error(t, err)
}

func TestAddColor(t *testing.T) {
	svc := NewThemeService()
	svc.AddColor("test", ColorScheme{Base: "#FF0000", Hover: "#FF3333", Pressed: "#CC0000"})

	assert.Equal(t, "#FF0000", svc.GetBaseColor("test"))
	assert.Equal(t, "", svc.GetBaseColor("nonexistent"))
}

func TestAddColorFromBase(t *testing.T) {
	svc := NewThemeService()
	svc.AddColorFromBase("brand", "#3B82F6")

	base := svc.GetBaseColor("brand")
	hover := svc.GetHoverColor("brand")
	pressed := svc.GetPressedColor("brand")

	assert.Equal(t, "#3B82F6", base)
	assert.NotEqual(t, base, hover)
	assert.NotEqual(t, base, pressed)
}

func TestGetHexMap(t *testing.T) {
	svc := NewThemeService()
	svc.AddColor("primary", ColorScheme{Base: "#A", Hover: "#B", Pressed: "#C"})

	m := svc.GetHexMap()
	assert.Equal(t, "#A", m["primary-base"])
	assert.Equal(t, "#B", m["primary-hover"])
	assert.Equal(t, "#C", m["primary-pressed"])
}

func TestToCSSVariables(t *testing.T) {
	svc := NewThemeService()
	svc.AddColor("primary", ColorScheme{Base: "#3B82F6", Hover: "#60A5FA", Pressed: "#2563EB"})

	css := svc.ToCSSVariables()
	assert.Contains(t, css, ":root {")
	assert.Contains(t, css, "--color-primary-base: #3B82F6;")
	assert.Contains(t, css, "--color-primary-hover: #60A5FA;")
	assert.Contains(t, css, "--color-primary-pressed: #2563EB;")
	assert.Contains(t, css, "}")
}

func TestToCSSForSelector(t *testing.T) {
	svc := NewThemeService()
	svc.AddColor("accent", ColorScheme{Base: "#A", Hover: "#B", Pressed: "#C"})

	css := svc.ToCSSForSelector(".dark")
	assert.Contains(t, css, ".dark {")
}

func TestBuilder(t *testing.T) {
	svc, err := NewBuilder().
		AddColor("primary", "#3B82F6").
		AddColor("danger", "#EF4444").
		Build()

	require.NoError(t, err)
	assert.Equal(t, "#3B82F6", svc.GetBaseColor("primary"))
	assert.Equal(t, "#EF4444", svc.GetBaseColor("danger"))
}

func TestBuilderFromJSON(t *testing.T) {
	json := `{"colors":{"primary":{"base":"#3B82F6","hover":"#60A5FA","pressed":"#2563EB"}}}`
	svc, err := NewBuilder().FromJSON(json).Build()
	require.NoError(t, err)
	assert.Equal(t, "#3B82F6", svc.GetBaseColor("primary"))
}

func TestComputeShading(t *testing.T) {
	svc := NewThemeService()
	scheme := svc.ComputeShading("#808080")
	assert.Equal(t, "#808080", scheme.Base)
	assert.Equal(t, Lighten("#808080"), scheme.Hover)
	assert.Equal(t, Darken("#808080"), scheme.Pressed)
}
