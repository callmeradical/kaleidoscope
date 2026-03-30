package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/callmeradical/kaleidoscope/browser"
	"github.com/callmeradical/kaleidoscope/output"
	"github.com/go-rod/rod"
)

func RunCatalog(args []string) {
	url := getArg(args)
	if url == "" {
		output.Fail("catalog", fmt.Errorf("missing URL"), "Usage: ks catalog <url>")
		os.Exit(2)
	}

	err := browser.WithPage(func(page *rod.Page) error {
		// Navigate to the library root
		if err := page.Navigate(url); err != nil {
			return err
		}
		if err := page.WaitLoad(); err != nil {
			return err
		}
		page.MustWaitStable()

		// Get the site title
		title := ""
		if el, err := page.Element("title"); err == nil {
			title, _ = el.Text()
		}

		// Discover all design system links with kind classification
		links, err := discoverLinks(page)
		if err != nil {
			return err
		}
		if len(links) == 0 {
			return fmt.Errorf("no design system pages found at %s", url)
		}

		// Ensure catalog directory
		catalogDir, err := catalogDir()
		if err != nil {
			return err
		}
		ssDir := filepath.Join(catalogDir, "screenshots")
		if err := os.MkdirAll(ssDir, 0755); err != nil {
			return err
		}

		catalog := Catalog{
			Name:      title,
			URL:       url,
			Source:    "web",
			CrawledAt: time.Now(),
		}

		categories := make(map[string]bool)
		kinds := make(map[string]bool)

		// Visit each discovered page
		for i, link := range links {
			if link.href == "" || link.text == "" {
				continue
			}

			categories[link.category] = true
			kinds[string(link.kind)] = true

			// Navigate to the page
			if err := page.Navigate(link.href); err != nil {
				continue
			}
			if err := page.WaitLoad(); err != nil {
				continue
			}
			page.MustWaitStable()

			// Build the base entry
			entry := CatalogEntry{
				Kind:     link.kind,
				Name:     link.text,
				URL:      link.href,
				Category: link.category,
			}

			// Extract description
			entry.Description = extractDescription(page)

			// Extract relations
			entry.Relations = extractRelations(page, link.href)

			// Take screenshot
			ssPath := filepath.Join(ssDir, fmt.Sprintf("%s-%d.png", sanitizeName(link.text), i))
			data, err := page.Screenshot(false, nil)
			if err == nil {
				if writeErr := os.WriteFile(ssPath, data, 0644); writeErr == nil {
					entry.Screenshot = ssPath
				}
			}

			// Extract kind-specific data
			switch link.kind {
			case KindComponent:
				entry.Component = extractComponent(page)
			case KindFoundation:
				entry.Foundation = extractFoundation(page, link.category)
			case KindPattern:
				entry.Pattern = extractPattern(page)
			case KindContent:
				entry.Content = extractContent(page)
			case KindIcon:
				iconEntries := extractIcons(page, link.href, link.category)
				if len(iconEntries) > 0 {
					// Icon pages may produce multiple entries (one per icon)
					for _, ie := range iconEntries {
						ie.URL = link.href
						ie.Category = link.category
						ie.Screenshot = entry.Screenshot
						catalog.Entries = append(catalog.Entries, ie)
					}
					fmt.Fprintf(os.Stderr, "  cataloged: %s (%d icons)\n", link.text, len(iconEntries))
					continue
				}
				// If no individual icons extracted, store as a single entry
				entry.Icon = &IconData{IconName: link.text}
			}

			catalog.Entries = append(catalog.Entries, entry)

			// Print progress
			kindLabel := string(link.kind)
			fmt.Fprintf(os.Stderr, "  cataloged: %s [%s]\n", link.text, kindLabel)
		}

		// Collect categories and kinds
		for cat := range categories {
			if cat != "" {
				catalog.Categories = append(catalog.Categories, cat)
			}
		}
		for k := range kinds {
			catalog.Kinds = append(catalog.Kinds, k)
		}

		// Save catalog
		if err := saveCatalog(&catalog, catalogDir); err != nil {
			return err
		}

		output.Success("catalog", map[string]any{
			"name":       catalog.Name,
			"url":        catalog.URL,
			"source":     catalog.Source,
			"entryCount": len(catalog.Entries),
			"kinds":      catalog.Kinds,
			"categories": catalog.Categories,
			"catalogPath": filepath.Join(catalogDir, "catalog.json"),
		})
		return nil
	})

	if err != nil {
		output.Fail("catalog", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}

// catalogDir returns the catalog storage directory.
func catalogDir() (string, error) {
	dir, err := browser.StateDir()
	if err != nil {
		return "", err
	}
	catDir := filepath.Join(dir, "catalog")
	if err := os.MkdirAll(catDir, 0755); err != nil {
		return "", err
	}
	return catDir, nil
}

// saveCatalog writes the catalog to disk.
func saveCatalog(catalog *Catalog, dir string) error {
	catalogPath := filepath.Join(dir, "catalog.json")
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling catalog: %w", err)
	}
	if err := os.WriteFile(catalogPath, data, 0644); err != nil {
		return fmt.Errorf("writing catalog: %w", err)
	}
	return nil
}

// loadCatalog reads the catalog from disk, migrating from legacy format if needed.
func loadCatalog() (*Catalog, error) {
	dir, err := catalogDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if err != nil {
		return nil, fmt.Errorf("no catalog found: %w", err)
	}

	// Try new format first
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}

	// Detect legacy format: has no entries but raw JSON has "components" key
	if len(catalog.Entries) == 0 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err == nil {
			if _, hasComponents := raw["components"]; hasComponents {
				var lc legacyCatalog
				if err := json.Unmarshal(data, &lc); err == nil && len(lc.Components) > 0 {
					return migrateFromLegacy(&lc), nil
				}
			}
		}
	}

	return &catalog, nil
}

func sanitizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result = append(result, c)
		}
	}
	return string(result)
}

func strVal(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func toStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
