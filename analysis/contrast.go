package analysis

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RGBA represents a color with red, green, blue, alpha channels (0-255).
type RGBA struct {
	R, G, B, A float64
}

// ContrastResult holds the result of a contrast check.
type ContrastResult struct {
	Foreground   string  `json:"foreground"`
	Background   string  `json:"background"`
	Ratio        float64 `json:"ratio"`
	AANormal     bool    `json:"aaNormal"`
	AALarge      bool    `json:"aaLarge"`
	AAANormal    bool    `json:"aaaNormal"`
	AAALarge     bool    `json:"aaaLarge"`
	FontSize     float64 `json:"fontSize,omitempty"`
	FontWeight   string  `json:"fontWeight,omitempty"`
	IsLargeText  bool    `json:"isLargeText"`
	MeetsMinimum bool    `json:"meetsMinimum"` // Meets AA for its text size
}

// ParseColor parses CSS color strings: rgb(), rgba(), hex.
func ParseColor(s string) (RGBA, error) {
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "rgba(") {
		return parseRGBA(s)
	}
	if strings.HasPrefix(s, "rgb(") {
		return parseRGB(s)
	}
	if strings.HasPrefix(s, "#") {
		return parseHex(s)
	}

	// Named colors (common ones)
	if c, ok := namedColors[strings.ToLower(s)]; ok {
		return c, nil
	}

	return RGBA{}, fmt.Errorf("unsupported color format: %s", s)
}

func parseRGB(s string) (RGBA, error) {
	s = strings.TrimPrefix(s, "rgb(")
	s = strings.TrimSuffix(s, ")")
	parts := strings.Split(s, ",")
	if len(parts) != 3 {
		return RGBA{}, fmt.Errorf("invalid rgb: %s", s)
	}
	r, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	g, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	b, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	return RGBA{R: r, G: g, B: b, A: 255}, nil
}

func parseRGBA(s string) (RGBA, error) {
	s = strings.TrimPrefix(s, "rgba(")
	s = strings.TrimSuffix(s, ")")
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return RGBA{}, fmt.Errorf("invalid rgba: %s", s)
	}
	r, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	g, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	b, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	a, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
	return RGBA{R: r, G: g, B: b, A: a * 255}, nil
}

func parseHex(s string) (RGBA, error) {
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3:
		r, _ := strconv.ParseInt(string(s[0])+string(s[0]), 16, 64)
		g, _ := strconv.ParseInt(string(s[1])+string(s[1]), 16, 64)
		b, _ := strconv.ParseInt(string(s[2])+string(s[2]), 16, 64)
		return RGBA{R: float64(r), G: float64(g), B: float64(b), A: 255}, nil
	case 6:
		r, _ := strconv.ParseInt(s[0:2], 16, 64)
		g, _ := strconv.ParseInt(s[2:4], 16, 64)
		b, _ := strconv.ParseInt(s[4:6], 16, 64)
		return RGBA{R: float64(r), G: float64(g), B: float64(b), A: 255}, nil
	case 8:
		r, _ := strconv.ParseInt(s[0:2], 16, 64)
		g, _ := strconv.ParseInt(s[2:4], 16, 64)
		b, _ := strconv.ParseInt(s[4:6], 16, 64)
		a, _ := strconv.ParseInt(s[6:8], 16, 64)
		return RGBA{R: float64(r), G: float64(g), B: float64(b), A: float64(a)}, nil
	}
	return RGBA{}, fmt.Errorf("invalid hex color: #%s", s)
}

// RelativeLuminance calculates the WCAG 2.1 relative luminance.
func RelativeLuminance(c RGBA) float64 {
	r := linearize(c.R / 255.0)
	g := linearize(c.G / 255.0)
	b := linearize(c.B / 255.0)
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func linearize(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// ContrastRatio computes the WCAG contrast ratio between two colors.
func ContrastRatio(fg, bg RGBA) float64 {
	l1 := RelativeLuminance(fg)
	l2 := RelativeLuminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// IsLargeText returns true if the text qualifies as "large" per WCAG.
// Large text is >= 18pt (24px) or >= 14pt (18.66px) bold.
func IsLargeText(fontSize float64, fontWeight string) bool {
	if fontSize >= 24 {
		return true
	}
	w, _ := strconv.Atoi(fontWeight)
	if fontWeight == "bold" || w >= 700 {
		return fontSize >= 18.66
	}
	return false
}

// CheckContrast performs a full WCAG contrast check.
func CheckContrast(fgStr, bgStr string, fontSize float64, fontWeight string) (*ContrastResult, error) {
	fg, err := ParseColor(fgStr)
	if err != nil {
		return nil, fmt.Errorf("foreground color: %w", err)
	}
	bg, err := ParseColor(bgStr)
	if err != nil {
		return nil, fmt.Errorf("background color: %w", err)
	}

	ratio := ContrastRatio(fg, bg)
	large := IsLargeText(fontSize, fontWeight)

	result := &ContrastResult{
		Foreground:  fgStr,
		Background:  bgStr,
		Ratio:       math.Round(ratio*100) / 100,
		AANormal:    ratio >= 4.5,
		AALarge:     ratio >= 3.0,
		AAANormal:   ratio >= 7.0,
		AAALarge:    ratio >= 4.5,
		FontSize:    fontSize,
		FontWeight:  fontWeight,
		IsLargeText: large,
	}

	if large {
		result.MeetsMinimum = ratio >= 3.0
	} else {
		result.MeetsMinimum = ratio >= 4.5
	}

	return result, nil
}

var namedColors = map[string]RGBA{
	"black":   {0, 0, 0, 255},
	"white":   {255, 255, 255, 255},
	"red":     {255, 0, 0, 255},
	"green":   {0, 128, 0, 255},
	"blue":    {0, 0, 255, 255},
	"yellow":  {255, 255, 0, 255},
	"cyan":    {0, 255, 255, 255},
	"magenta": {255, 0, 255, 255},
	"gray":    {128, 128, 128, 255},
	"grey":    {128, 128, 128, 255},
	"orange":  {255, 165, 0, 255},
	"purple":  {128, 0, 128, 255},
	"transparent": {0, 0, 0, 0},
}
