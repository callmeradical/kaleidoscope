package analysis

// TouchTargetResult holds the result of a touch target size check.
type TouchTargetResult struct {
	Selector   string  `json:"selector"`
	TagName    string  `json:"tagName"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	MinSize    float64 `json:"minSize"`
	Passes     bool    `json:"passes"`
	Violation  string  `json:"violation,omitempty"`
}

const MinTouchTarget = 48.0 // WCAG 2.5.8 minimum

// CheckTouchTarget checks if an element meets the minimum touch target size.
func CheckTouchTarget(tagName string, width, height float64) TouchTargetResult {
	result := TouchTargetResult{
		TagName: tagName,
		Width:   width,
		Height:  height,
		MinSize: MinTouchTarget,
		Passes:  width >= MinTouchTarget && height >= MinTouchTarget,
	}

	if !result.Passes {
		if width < MinTouchTarget && height < MinTouchTarget {
			result.Violation = "both width and height below 48px minimum"
		} else if width < MinTouchTarget {
			result.Violation = "width below 48px minimum"
		} else {
			result.Violation = "height below 48px minimum"
		}
	}

	return result
}
