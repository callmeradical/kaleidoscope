package analysis

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// sanitizeFilename replaces characters outside [a-zA-Z0-9._-] with hyphens.
func sanitizeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '-'
	}, s)
}

// WriteDiffImage writes a diff image as PNG to dir with the given baseName.
// Stub: returns empty path — tests will fail until implemented.
func WriteDiffImage(dir, baseName string, diffImg image.Image) (string, error) {
	sanitized := sanitizeFilename(baseName)
	outPath := filepath.Join(dir, fmt.Sprintf("diff-%s.png", sanitized))
	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, diffImg); err != nil {
		return "", err
	}
	return outPath, nil
}
