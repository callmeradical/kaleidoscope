package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// findGitRoot walks up from the current directory to find the git repository root.
func findGitRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a git repository")
		}
		dir = parent
	}
}

// hasFlag checks if a flag is present in the args slice.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// getArg returns the first non-flag argument, or empty string.
func getArg(args []string) string {
	nonFlags := getNonFlagArgs(args)
	if len(nonFlags) > 0 {
		return nonFlags[0]
	}
	return ""
}

// getFlagValue returns the value following a flag, or empty string.
func getFlagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// getNonFlagArgs returns all arguments that are not flags or flag values.
func getNonFlagArgs(args []string) []string {
	var result []string
	skip := false
	for _, a := range args {
		if skip {
			skip = false
			continue
		}
		if len(a) > 0 && a[0] == '-' {
			// If it's a flag that takes a value, skip next arg too
			if a == "--selector" || a == "--output" || a == "--depth" ||
				a == "--width" || a == "--height" || a == "--format" ||
				a == "--quality" || a == "--wait-until" || a == "--min-size" ||
				a == "--kind" || a == "--ref" {
				skip = true
			}
			continue
		}
		result = append(result, a)
	}
	return result
}
