package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/callmeradical/kaleidoscope/output"
)

func RunCatalogRepo(args []string) {
	repoURL := getArg(args)
	if repoURL == "" {
		output.Fail("catalog-repo", fmt.Errorf("missing repository URL"), "Usage: ks catalog-repo <repo-url> [--ref <branch/tag>]")
		os.Exit(2)
	}

	ref := getFlagValue(args, "--ref")

	// Clone to temp dir
	tmpDir, err := os.MkdirTemp("", "ks-catalog-*")
	if err != nil {
		output.Fail("catalog-repo", err, "Failed to create temp directory")
		os.Exit(2)
	}
	defer os.RemoveAll(tmpDir)

	cloneArgs := []string{"clone", "--depth", "1"}
	if ref != "" {
		cloneArgs = append(cloneArgs, "--branch", ref)
	}
	cloneArgs = append(cloneArgs, repoURL, tmpDir)

	fmt.Fprintf(os.Stderr, "  cloning %s...\n", repoURL)
	cloneCmd := exec.Command("git", cloneArgs...)
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		output.Fail("catalog-repo", fmt.Errorf("git clone failed: %w", err), "Check the repository URL and ensure git is installed")
		os.Exit(2)
	}

	// Derive catalog name from repo URL
	name := filepath.Base(repoURL)
	name = strings.TrimSuffix(name, ".git")

	catalog := Catalog{
		Name:      name,
		URL:       repoURL,
		Source:    "repo",
		CrawledAt: time.Now(),
	}

	kinds := make(map[string]bool)
	categories := make(map[string]bool)

	// Walk the repo for design system artifacts
	fmt.Fprintf(os.Stderr, "  scanning repository...\n")

	// 1. Find and parse token files
	tokenEntries := findTokenFiles(tmpDir)
	for _, e := range tokenEntries {
		catalog.Entries = append(catalog.Entries, e)
		kinds["foundation"] = true
		categories[e.Category] = true
		fmt.Fprintf(os.Stderr, "  found tokens: %s (%d tokens)\n", e.Name, len(e.Foundation.Tokens))
	}

	// 2. Find SVG icons
	iconEntries := findIconFiles(tmpDir)
	for _, e := range iconEntries {
		catalog.Entries = append(catalog.Entries, e)
		kinds["icon"] = true
		categories["icons"] = true
	}
	if len(iconEntries) > 0 {
		fmt.Fprintf(os.Stderr, "  found icons: %d\n", len(iconEntries))
	}

	// 3. Find component READMEs
	compEntries := findComponentDocs(tmpDir)
	for _, e := range compEntries {
		catalog.Entries = append(catalog.Entries, e)
		kinds["component"] = true
		categories["components"] = true
		fmt.Fprintf(os.Stderr, "  found component: %s\n", e.Name)
	}

	// 4. Find content/pattern markdown
	docEntries := findDocMarkdown(tmpDir)
	for _, e := range docEntries {
		catalog.Entries = append(catalog.Entries, e)
		kinds[string(e.Kind)] = true
		categories[e.Category] = true
		fmt.Fprintf(os.Stderr, "  found %s: %s\n", e.Kind, e.Name)
	}

	// Collect kinds and categories
	for k := range kinds {
		catalog.Kinds = append(catalog.Kinds, k)
	}
	for c := range categories {
		if c != "" {
			catalog.Categories = append(catalog.Categories, c)
		}
	}

	// Save
	catalogDir, err := catalogDir()
	if err != nil {
		output.Fail("catalog-repo", err, "")
		os.Exit(2)
	}
	if err := saveCatalog(&catalog, catalogDir); err != nil {
		output.Fail("catalog-repo", err, "")
		os.Exit(2)
	}

	output.Success("catalog-repo", map[string]any{
		"name":        catalog.Name,
		"url":         catalog.URL,
		"source":      catalog.Source,
		"entryCount":  len(catalog.Entries),
		"kinds":       catalog.Kinds,
		"categories":  catalog.Categories,
		"catalogPath": filepath.Join(catalogDir, "catalog.json"),
	})
}

// findTokenFiles looks for design token files (JSON Style Dictionary, CSS custom properties).
func findTokenFiles(root string) []CatalogEntry {
	var entries []CatalogEntry

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			// Skip common non-relevant directories
			if info != nil && info.IsDir() {
				base := info.Name()
				if base == "node_modules" || base == ".git" || base == "dist" || base == "build" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		relPath, _ := filepath.Rel(root, path)
		name := info.Name()

		// JSON token files
		if (strings.HasSuffix(name, ".tokens.json") || strings.Contains(relPath, "tokens/")) && strings.HasSuffix(name, ".json") {
			tokens := parseStyleDictionaryJSON(path)
			if len(tokens) > 0 {
				tokenName := strings.TrimSuffix(name, ".json")
				tokenName = strings.TrimSuffix(tokenName, ".tokens")
				category := inferTokenCategory(tokenName, relPath)
				entries = append(entries, CatalogEntry{
					Kind:     KindFoundation,
					Name:     tokenName,
					URL:      relPath,
					Category: "tokens",
					Foundation: &FoundationData{
						TokenCategory: category,
						Tokens:        tokens,
					},
				})
			}
		}

		// CSS files with custom properties
		if strings.HasSuffix(name, ".css") && (strings.Contains(relPath, "tokens") || strings.Contains(relPath, "variables") || strings.Contains(relPath, "foundations")) {
			tokens := parseCSSTokens(path)
			if len(tokens) > 0 {
				tokenName := strings.TrimSuffix(name, ".css")
				entries = append(entries, CatalogEntry{
					Kind:     KindFoundation,
					Name:     tokenName,
					URL:      relPath,
					Category: "tokens",
					Foundation: &FoundationData{
						TokenCategory: inferTokenCategory(tokenName, relPath),
						Tokens:        tokens,
					},
				})
			}
		}

		return nil
	})

	return entries
}

// parseStyleDictionaryJSON parses a Style Dictionary format JSON file into flat tokens.
func parseStyleDictionaryJSON(path string) []DesignToken {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	var tokens []DesignToken
	flattenTokens("", raw, &tokens)
	return tokens
}

// flattenTokens recursively flattens a nested token structure.
func flattenTokens(prefix string, obj map[string]interface{}, tokens *[]DesignToken) {
	for key, val := range obj {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "-" + key
		}

		switch v := val.(type) {
		case map[string]interface{}:
			// Check if this is a leaf token (has "value" key)
			if value, hasValue := v["value"]; hasValue {
				token := DesignToken{Name: fullKey}
				switch vv := value.(type) {
				case string:
					token.Value = vv
				case float64:
					token.Value = fmt.Sprintf("%g", vv)
				default:
					token.Value = fmt.Sprintf("%v", vv)
				}
				if desc, ok := v["description"].(string); ok {
					token.Description = desc
				}
				if cat, ok := v["type"].(string); ok {
					token.Category = cat
				}
				*tokens = append(*tokens, token)
			} else {
				// Recurse into nested object
				flattenTokens(fullKey, v, tokens)
			}
		}
	}
}

var cssVarRe = regexp.MustCompile(`(--[\w-]+)\s*:\s*([^;]+);`)

// parseCSSTokens extracts custom properties from a CSS file.
func parseCSSTokens(path string) []DesignToken {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	matches := cssVarRe.FindAllStringSubmatch(string(data), -1)
	var tokens []DesignToken
	for _, m := range matches {
		tokens = append(tokens, DesignToken{
			Name:  m[1],
			Value: strings.TrimSpace(m[2]),
		})
	}
	return tokens
}

// findIconFiles walks the repo for SVG icons.
func findIconFiles(root string) []CatalogEntry {
	var entries []CatalogEntry

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				base := info.Name()
				if base == "node_modules" || base == ".git" || base == "dist" || base == "build" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".svg") {
			return nil
		}

		relPath, _ := filepath.Rel(root, path)

		// Only include SVGs that are likely icons (in icon-related directories)
		relLower := strings.ToLower(relPath)
		if !strings.Contains(relLower, "icon") && !strings.Contains(relLower, "glyph") && !strings.Contains(relLower, "symbol") {
			return nil
		}

		iconName := strings.TrimSuffix(info.Name(), ".svg")

		// Read SVG content
		svgData, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		svg := string(svgData)

		// Extract size from viewBox or parent directory
		var sizes []string
		dir := filepath.Base(filepath.Dir(path))
		if dir == "16" || dir == "24" || dir == "32" || dir == "48" {
			sizes = append(sizes, dir)
		}
		// Also try viewBox
		if idx := strings.Index(svg, "viewBox=\""); idx >= 0 {
			end := strings.Index(svg[idx+9:], "\"")
			if end > 0 {
				vb := svg[idx+9 : idx+9+end]
				parts := strings.Fields(vb)
				if len(parts) == 4 {
					sizes = append(sizes, parts[2])
				}
			}
		}

		// Truncate large SVGs
		if len(svg) > 2000 {
			svg = svg[:2000]
		}

		entries = append(entries, CatalogEntry{
			Kind:     KindIcon,
			Name:     iconName,
			URL:      relPath,
			Category: "icons",
			Icon: &IconData{
				IconName: iconName,
				SVG:      svg,
				Sizes:    sizes,
			},
		})

		return nil
	})

	return entries
}

// findComponentDocs finds component README files.
func findComponentDocs(root string) []CatalogEntry {
	var entries []CatalogEntry

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				base := info.Name()
				if base == "node_modules" || base == ".git" || base == "dist" || base == "build" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		relPath, _ := filepath.Rel(root, path)
		relLower := strings.ToLower(relPath)

		// Look for README.md in component directories
		if strings.ToLower(info.Name()) != "readme.md" {
			return nil
		}
		if !strings.Contains(relLower, "component") {
			return nil
		}

		// Component name from parent directory
		compName := filepath.Base(filepath.Dir(path))
		if compName == "." || compName == "components" || compName == "src" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)

		// Extract first paragraph as description
		desc := extractMarkdownDescription(content)

		entries = append(entries, CatalogEntry{
			Kind:        KindComponent,
			Name:        compName,
			URL:         relPath,
			Category:    "components",
			Description: desc,
			Component:   &ComponentData{},
		})

		return nil
	})

	return entries
}

// findDocMarkdown finds content/pattern markdown files.
func findDocMarkdown(root string) []CatalogEntry {
	var entries []CatalogEntry

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				base := info.Name()
				if base == "node_modules" || base == ".git" || base == "dist" || base == "build" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".md") || strings.ToLower(info.Name()) == "readme.md" {
			return nil
		}

		relPath, _ := filepath.Rel(root, path)
		relLower := strings.ToLower(relPath)

		var kind CatalogEntryKind
		var category string

		switch {
		case strings.Contains(relLower, "pattern"):
			kind = KindPattern
			category = "patterns"
		case strings.Contains(relLower, "content") || strings.Contains(relLower, "writing") || strings.Contains(relLower, "voice") || strings.Contains(relLower, "tone"):
			kind = KindContent
			category = "content"
		case strings.Contains(relLower, "guideline"):
			kind = KindContent
			category = "guidelines"
		default:
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)

		docName := strings.TrimSuffix(info.Name(), ".md")
		desc := extractMarkdownDescription(content)

		entry := CatalogEntry{
			Kind:        kind,
			Name:        docName,
			URL:         relPath,
			Category:    category,
			Description: desc,
		}

		if kind == KindPattern {
			entry.Pattern = parsePatternMarkdown(content)
		} else {
			entry.Content = parseContentMarkdown(content, relLower)
		}

		entries = append(entries, entry)
		return nil
	})

	return entries
}

// extractMarkdownDescription extracts the first paragraph from markdown content.
func extractMarkdownDescription(content string) string {
	lines := strings.Split(content, "\n")
	var para []string
	inParagraph := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inParagraph {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if inParagraph {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "```") {
			continue
		}
		inParagraph = true
		para = append(para, trimmed)
	}

	desc := strings.Join(para, " ")
	if len(desc) > 200 {
		desc = desc[:200]
	}
	return desc
}

// parsePatternMarkdown extracts pattern data from markdown content.
func parsePatternMarkdown(content string) *PatternData {
	pd := &PatternData{}
	sections := splitMarkdownSections(content)

	for heading, body := range sections {
		hLower := strings.ToLower(heading)
		switch {
		case strings.Contains(hLower, "when to use"):
			pd.WhenToUse = body
		case strings.Contains(hLower, "when not") || strings.Contains(hLower, "don't"):
			pd.WhenNotToUse = body
		case strings.Contains(hLower, "overview") || strings.Contains(hLower, "about"):
			pd.ProblemSolved = body
		case strings.Contains(hLower, "best practice") || strings.Contains(hLower, "guideline"):
			pd.BestPractices = extractBulletList(body)
		}
	}

	return pd
}

// parseContentMarkdown extracts content guideline data from markdown.
func parseContentMarkdown(content string, path string) *ContentData {
	cd := &ContentData{ContentType: "writing-pattern"}

	if strings.Contains(path, "voice") || strings.Contains(path, "tone") {
		cd.ContentType = "voice-tone"
	} else if strings.Contains(path, "termin") || strings.Contains(path, "word") {
		cd.ContentType = "terminology"
	}

	// Extract bullet points as guidelines
	cd.Guidelines = extractBulletList(content)
	if len(cd.Guidelines) > 20 {
		cd.Guidelines = cd.Guidelines[:20]
	}

	return cd
}

// splitMarkdownSections splits markdown into heading -> body pairs.
func splitMarkdownSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")
	currentHeading := ""
	var body []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			if currentHeading != "" {
				sections[currentHeading] = strings.TrimSpace(strings.Join(body, "\n"))
			}
			currentHeading = strings.TrimLeft(line, "# ")
			body = nil
		} else if currentHeading != "" {
			body = append(body, line)
		}
	}
	if currentHeading != "" {
		sections[currentHeading] = strings.TrimSpace(strings.Join(body, "\n"))
	}

	return sections
}

// extractBulletList pulls bullet points from markdown text.
func extractBulletList(text string) []string {
	var items []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			item := strings.TrimPrefix(trimmed, "- ")
			item = strings.TrimPrefix(item, "* ")
			if len(item) > 0 {
				items = append(items, item)
			}
		}
	}
	return items
}

// inferTokenCategory guesses the token category from filename and path.
func inferTokenCategory(name string, path string) string {
	combined := strings.ToLower(name + " " + path)
	switch {
	case strings.Contains(combined, "color") || strings.Contains(combined, "palette"):
		return "color"
	case strings.Contains(combined, "typo") || strings.Contains(combined, "font"):
		return "typography"
	case strings.Contains(combined, "spac"):
		return "spacing"
	case strings.Contains(combined, "elev") || strings.Contains(combined, "shadow"):
		return "elevation"
	case strings.Contains(combined, "motion") || strings.Contains(combined, "anim"):
		return "motion"
	case strings.Contains(combined, "border") || strings.Contains(combined, "radius"):
		return "border"
	default:
		return "general"
	}
}
