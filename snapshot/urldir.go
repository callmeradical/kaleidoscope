package snapshot

import (
	"net/url"
	"regexp"
	"strings"
)

var multiDash = regexp.MustCompile(`-{2,}`)
var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// URLToDir converts a URL into a safe directory name with no path separators
// or traversal sequences.
func URLToDir(rawURL string) string {
	var hostPath string
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Host != "" {
		hostPath = parsed.Host + parsed.Path
	} else {
		// Treat the whole string as a path component
		hostPath = rawURL
	}

	// Replace all non-alphanumeric chars with dashes
	safe := nonAlphanumeric.ReplaceAllString(hostPath, "-")

	// Collapse consecutive dashes
	safe = multiDash.ReplaceAllString(safe, "-")

	// Trim leading/trailing dashes
	safe = strings.Trim(safe, "-")

	if safe == "" {
		return "root"
	}

	// Safety check: reject any remaining path traversal or separators
	if strings.Contains(safe, "/") || strings.Contains(safe, "\\") || strings.Contains(safe, "..") {
		return "invalid"
	}

	return safe
}
