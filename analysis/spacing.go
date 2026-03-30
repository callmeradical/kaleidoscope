package analysis

import (
	"math"
	"sort"
)

// SpacingResult holds the result of a spacing analysis.
type SpacingResult struct {
	Gaps         []float64 `json:"gaps"`
	DetectedScale float64  `json:"detectedScale,omitempty"`
	Inconsistencies []SpacingInconsistency `json:"inconsistencies,omitempty"`
}

// SpacingInconsistency flags a gap that deviates from the detected scale.
type SpacingInconsistency struct {
	Index    int     `json:"index"`
	Gap      float64 `json:"gap"`
	Expected float64 `json:"expected"`
}

// AnalyzeSpacing takes a slice of gaps (distances between sibling elements)
// and checks for consistency.
func AnalyzeSpacing(gaps []float64) SpacingResult {
	result := SpacingResult{
		Gaps: gaps,
	}

	if len(gaps) < 2 {
		return result
	}

	// Detect the most common gap (mode)
	counts := make(map[float64]int)
	for _, g := range gaps {
		rounded := math.Round(g)
		counts[rounded]++
	}

	var mode float64
	maxCount := 0
	for g, c := range counts {
		if c > maxCount {
			maxCount = c
			mode = g
		}
	}
	result.DetectedScale = mode

	// Try to detect if gaps follow a scale (multiples of 4 or 8)
	scale := detectScale(gaps)
	if scale > 0 {
		result.DetectedScale = scale
	}

	// Flag inconsistencies
	for i, g := range gaps {
		rounded := math.Round(g)
		if scale > 0 {
			remainder := math.Mod(rounded, scale)
			if remainder > 1 && remainder < scale-1 {
				nearest := math.Round(rounded/scale) * scale
				result.Inconsistencies = append(result.Inconsistencies, SpacingInconsistency{
					Index:    i,
					Gap:      g,
					Expected: nearest,
				})
			}
		} else if math.Abs(rounded-mode) > 2 {
			result.Inconsistencies = append(result.Inconsistencies, SpacingInconsistency{
				Index:    i,
				Gap:      g,
				Expected: mode,
			})
		}
	}

	return result
}

func detectScale(gaps []float64) float64 {
	// Check common scales: 4px, 8px
	for _, scale := range []float64{4, 8} {
		matches := 0
		for _, g := range gaps {
			remainder := math.Mod(math.Round(g), scale)
			if remainder < 1 || remainder > scale-1 {
				matches++
			}
		}
		if float64(matches)/float64(len(gaps)) >= 0.8 {
			return scale
		}
	}

	// Try GCD of gaps
	rounded := make([]int, 0, len(gaps))
	for _, g := range gaps {
		r := int(math.Round(g))
		if r > 0 {
			rounded = append(rounded, r)
		}
	}
	if len(rounded) >= 2 {
		sort.Ints(rounded)
		g := rounded[0]
		for _, v := range rounded[1:] {
			g = gcd(g, v)
		}
		if g >= 4 {
			return float64(g)
		}
	}

	return 0
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
