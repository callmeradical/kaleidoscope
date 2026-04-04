package snapshot

import (
	"net/url"
	"strings"
)

// URLToKey converts a raw URL to a filesystem-safe directory name.
// The result is always a flat name with no path separators, no "..", and at most 128 chars.
func URLToKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// Fall back to a simple sanitization
		return sanitize(rawURL)
	}

	// Strip scheme, query, and fragment — keep host + path only
	base := u.Host + u.Path

	// Replace path separators with dashes
	base = strings.ReplaceAll(base, "/", "-")

	// Sanitize remaining special characters
	base = sanitize(base)

	// Collapse consecutive dashes
	for strings.Contains(base, "--") {
		base = strings.ReplaceAll(base, "--", "-")
	}

	// Strip leading/trailing dashes
	base = strings.Trim(base, "-")

	// Remove any ".." components
	base = strings.ReplaceAll(base, "..", "")

	// Collapse again after ".." removal
	for strings.Contains(base, "--") {
		base = strings.ReplaceAll(base, "--", "-")
	}
	base = strings.Trim(base, "-")

	// Truncate to 128 characters
	if len(base) > 128 {
		base = base[:128]
	}

	return base
}

// sanitize replaces characters that are unsafe in filesystem names with dashes.
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if isAllowed(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}

// isAllowed returns true for characters safe in directory names.
func isAllowed(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '.' || r == '-' || r == '_'
}
