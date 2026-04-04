package snapshot

import (
	"strings"
	"testing"
)

func TestURLToKey(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		check func(t *testing.T, got string)
	}{
		{
			name:  "root URL",
			input: "https://example.com/",
			want:  "example.com",
		},
		{
			name:  "deep path",
			input: "https://example.com/about/team",
			want:  "example.com-about-team",
		},
		{
			name:  "query string stripped",
			input: "https://example.com/about?foo=bar",
			want:  "example.com-about",
		},
		{
			name:  "fragment stripped",
			input: "https://example.com/page#section",
			want:  "example.com-page",
		},
		{
			name:  "path traversal blocked",
			input: "https://example.com/../secret",
			check: func(t *testing.T, got string) {
				if strings.Contains(got, "..") {
					t.Errorf("URLToKey(%q) = %q; must not contain '..'", "https://example.com/../secret", got)
				}
			},
		},
		{
			name:  "consecutive slashes collapsed",
			input: "https://example.com//about//team",
			want:  "example.com-about-team",
		},
		{
			name:  "no leading or trailing dashes",
			input: "https://example.com/",
			check: func(t *testing.T, got string) {
				if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
					t.Errorf("URLToKey returned %q; must not have leading/trailing dashes", got)
				}
			},
		},
		{
			name:  "no path separators in result",
			input: "https://example.com/a/b/c",
			check: func(t *testing.T, got string) {
				if strings.Contains(got, "/") || strings.Contains(got, "\\") {
					t.Errorf("URLToKey(%q) = %q; must not contain path separators", "https://example.com/a/b/c", got)
				}
			},
		},
		{
			name: "long URL truncated",
			input: "https://example.com/" + strings.Repeat("a", 200),
			check: func(t *testing.T, got string) {
				if len(got) > 128 {
					t.Errorf("URLToKey returned %d chars; must be <= 128", len(got))
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := URLToKey(tc.input)
			if tc.check != nil {
				tc.check(t, got)
			} else if got != tc.want {
				t.Errorf("URLToKey(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}
