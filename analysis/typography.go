package analysis

import (
	"strconv"
	"strings"
)

// TypographyResult holds the result of a typography check.
type TypographyResult struct {
	Selector       string  `json:"selector,omitempty"`
	FontSize       float64 `json:"fontSize"`
	LineHeight     float64 `json:"lineHeight"`
	LineHeightRatio float64 `json:"lineHeightRatio"`
	FontFamily     string  `json:"fontFamily"`
	HasFallback    bool    `json:"hasFallback"`
	Warnings       []string `json:"warnings,omitempty"`
}

// CheckTypography validates typography properties against best practices.
func CheckTypography(fontSize, lineHeight float64, fontFamily string, isHeading bool) TypographyResult {
	result := TypographyResult{
		FontSize:   fontSize,
		LineHeight: lineHeight,
		FontFamily: fontFamily,
	}

	// Line height ratio
	if fontSize > 0 {
		result.LineHeightRatio = lineHeight / fontSize
	}

	// Check font size minimum
	if !isHeading && fontSize < 16 {
		result.Warnings = append(result.Warnings, "body text font-size below 16px recommended minimum")
	}

	// Check line height
	minRatio := 1.5
	if isHeading {
		minRatio = 1.2
	}
	if result.LineHeightRatio > 0 && result.LineHeightRatio < minRatio {
		result.Warnings = append(result.Warnings, "line-height ratio below recommended minimum")
	}

	// Check for fallback font
	families := strings.Split(fontFamily, ",")
	result.HasFallback = len(families) > 1
	if !result.HasFallback {
		result.Warnings = append(result.Warnings, "no fallback font-family specified")
	}

	// Check for generic family
	generics := []string{"serif", "sans-serif", "monospace", "cursive", "fantasy", "system-ui", "ui-serif", "ui-sans-serif", "ui-monospace"}
	hasGeneric := false
	for _, f := range families {
		trimmed := strings.TrimSpace(strings.Trim(f, "\"'"))
		for _, g := range generics {
			if strings.EqualFold(trimmed, g) {
				hasGeneric = true
				break
			}
		}
	}
	if !hasGeneric {
		result.Warnings = append(result.Warnings, "no generic font-family fallback (e.g., sans-serif)")
	}

	return result
}

// ParseFontSize converts a CSS font-size string (e.g., "16px") to a float64.
func ParseFontSize(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "px")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// ParseLineHeight converts a CSS line-height value to pixels given the font size.
func ParseLineHeight(s string, fontSize float64) float64 {
	s = strings.TrimSpace(s)
	if s == "normal" {
		return fontSize * 1.2 // Browser default
	}
	if strings.HasSuffix(s, "px") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 64)
		return v
	}
	// Unitless multiplier
	v, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return v * fontSize
	}
	return fontSize * 1.2
}
