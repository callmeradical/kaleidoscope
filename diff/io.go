package diff

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoadPNG opens and decodes a PNG file from disk.
func LoadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// SavePNG encodes img as PNG and writes it to path.
func SavePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// DiffImagePath builds a unique diff-image filename in snapshotDir based on
// the current screenshot's base name and a Unix timestamp.
func DiffImagePath(snapshotDir, _, currentFile string) string {
	base := filepath.Base(currentFile)
	label := strings.TrimSuffix(base, filepath.Ext(base))
	label = strings.TrimPrefix(label, "current-")
	timestamp := time.Now().Unix()
	return filepath.Join(snapshotDir, fmt.Sprintf("diff-%s-%d.png", label, timestamp))
}
