package snapshot

import (
	"strings"
	"testing"
)

func TestURLToDir(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		notWant []string // substrings that must NOT appear
	}{
		{
			input: "http://localhost:3000",
			want:  "localhost-3000",
		},
		{
			input: "http://localhost:3000/about",
			want:  "localhost-3000-about",
		},
		{
			input: "https://example.com/foo/bar",
			want:  "example-com-foo-bar",
		},
		{
			input:   "http://localhost:3000/../etc",
			notWant: []string{"..", "/"},
		},
		{
			input:   "http://evil.com/%2F%2F",
			notWant: []string{"/", "\\"},
		},
		{
			input: "", // empty string — must not panic
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := URLToDir(tc.input)

			if tc.want != "" && got != tc.want {
				t.Errorf("URLToDir(%q) = %q, want %q", tc.input, got, tc.want)
			}

			for _, bad := range tc.notWant {
				if strings.Contains(got, bad) {
					t.Errorf("URLToDir(%q) = %q contains forbidden substring %q", tc.input, got, bad)
				}
			}
		})
	}
}
