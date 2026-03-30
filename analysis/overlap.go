package analysis

// BoundingBox represents an element's position and size.
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// OverlapResult describes an overlap between two elements.
type OverlapResult struct {
	ElementA string  `json:"elementA"`
	ElementB string  `json:"elementB"`
	OverlapArea float64 `json:"overlapArea"`
	OverlapBox  BoundingBox `json:"overlapBox"`
}

// DetectOverlaps finds overlapping pairs from a set of labeled bounding boxes.
func DetectOverlaps(elements map[string]BoundingBox) []OverlapResult {
	var results []OverlapResult

	keys := make([]string, 0, len(elements))
	for k := range elements {
		keys = append(keys, k)
	}

	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			a := elements[keys[i]]
			b := elements[keys[j]]

			ox, oy, ow, oh := intersect(a, b)
			if ow > 0 && oh > 0 {
				results = append(results, OverlapResult{
					ElementA:    keys[i],
					ElementB:    keys[j],
					OverlapArea: ow * oh,
					OverlapBox:  BoundingBox{X: ox, Y: oy, Width: ow, Height: oh},
				})
			}
		}
	}

	return results
}

func intersect(a, b BoundingBox) (x, y, w, h float64) {
	x1 := max(a.X, b.X)
	y1 := max(a.Y, b.Y)
	x2 := min(a.X+a.Width, b.X+b.Width)
	y2 := min(a.Y+a.Height, b.Y+b.Height)

	if x2 > x1 && y2 > y1 {
		return x1, y1, x2 - x1, y2 - y1
	}
	return 0, 0, 0, 0
}
