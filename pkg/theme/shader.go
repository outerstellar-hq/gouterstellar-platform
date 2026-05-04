package theme

import (
	"fmt"
	"math"
	"strings"
)

func HexToRGB(hex string) (r, g, b int, err error) {
	hex = strings.TrimPrefix(hex, "#")
	switch len(hex) {
	case 3:
		r = expandShort(hex[0])
		g = expandShort(hex[1])
		b = expandShort(hex[2])
		if r < 0 || g < 0 || b < 0 {
			return 0, 0, 0, fmt.Errorf("invalid hex color: #%s", hex)
		}
		return r, g, b, nil
	case 6:
		r = hexPairToInt(hex[0:2])
		g = hexPairToInt(hex[2:4])
		b = hexPairToInt(hex[4:6])
		if r < 0 || g < 0 || b < 0 {
			return 0, 0, 0, fmt.Errorf("invalid hex color: #%s", hex)
		}
		return r, g, b, nil
	default:
		return 0, 0, 0, fmt.Errorf("invalid hex color length: #%s", hex)
	}
}

func expandShort(c byte) int {
	v := parseHexDigit(c)
	if v < 0 {
		return -1
	}
	return v*17 + v*0
}

func hexPairToInt(s string) int {
	hi := parseHexDigit(s[0])
	lo := parseHexDigit(s[1])
	if hi < 0 || lo < 0 {
		return -1
	}
	return hi<<4 | lo
}

func parseHexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	default:
		return -1
	}
}

func RGBToHex(r, g, b int) string {
	return fmt.Sprintf("#%02X%02X%02X", clamp(r), clamp(g), clamp(b))
}

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func Lighten(hexColor string) string {
	return AdjustBrightness(hexColor, 0.2)
}

func Darken(hexColor string) string {
	return AdjustBrightness(hexColor, -0.2)
}

func Hover(hexColor string) string {
	return Lighten(hexColor)
}

func Pressed(hexColor string) string {
	return Darken(hexColor)
}

func AdjustBrightness(hexColor string, factor float64) string {
	r, g, b, err := HexToRGB(hexColor)
	if err != nil {
		return hexColor
	}
	if factor > 0 {
		r = int(math.Round(float64(r) + (255.0-float64(r))*factor))
		g = int(math.Round(float64(g) + (255.0-float64(g))*factor))
		b = int(math.Round(float64(b) + (255.0-float64(b))*factor))
	} else {
		r = int(math.Round(float64(r) * (1.0 + factor)))
		g = int(math.Round(float64(g) * (1.0 + factor)))
		b = int(math.Round(float64(b) * (1.0 + factor)))
	}
	return RGBToHex(r, g, b)
}
