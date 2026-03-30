package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/lars/kaleidoscope/browser"
	"github.com/lars/kaleidoscope/output"
)

// Catalog represents a full component library catalog.
type Catalog struct {
	Name       string              `json:"name"`
	URL        string              `json:"url"`
	CrawledAt  time.Time           `json:"crawledAt"`
	Components []CatalogComponent  `json:"components"`
	Categories []string            `json:"categories"`
}

// CatalogComponent represents a single component in the catalog.
type CatalogComponent struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Variants    []ComponentVariant `json:"variants,omitempty"`
	Props       []ComponentProp   `json:"props,omitempty"`
	UsageSnippets []string        `json:"usageSnippets,omitempty"`
	Screenshot  string            `json:"screenshot,omitempty"`
	Relations   []string          `json:"relations,omitempty"`
	Tokens      ComponentTokens   `json:"tokens,omitempty"`
}

// ComponentVariant represents a variant/example of a component.
type ComponentVariant struct {
	Name       string `json:"name"`
	Screenshot string `json:"screenshot,omitempty"`
	HTML       string `json:"html,omitempty"`
}

// ComponentProp represents a component prop/attribute.
type ComponentProp struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

// ComponentTokens captures the design tokens used by a component.
type ComponentTokens struct {
	Colors     []string `json:"colors,omitempty"`
	FontSizes  []string `json:"fontSizes,omitempty"`
	Spacing    []string `json:"spacing,omitempty"`
	Radii      []string `json:"radii,omitempty"`
}

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

		// Discover component links from navigation
		linksResult, err := page.Eval(`() => {
			const links = [];
			const seen = new Set();
			// Look for nav links that point to component pages
			const allLinks = document.querySelectorAll('a[href]');
			for (const a of allLinks) {
				const href = a.href;
				const text = a.textContent.trim();
				// Filter for likely component pages
				if (!text || text.length > 60) continue;
				if (seen.has(href)) continue;
				seen.add(href);
				// Heuristic: component pages often have paths like /docs/components/X or /components/X
				const path = new URL(href, window.location.origin).pathname;
				if (path.match(/\/(components|docs|elements|forms|typography|utilities|extend|plugins)\//i) ||
				    path.match(/\/(button|card|modal|table|input|select|nav|tab|alert|badge|avatar|accordion|drawer|dropdown|footer|sidebar|toast|tooltip|pagination|progress|spinner|breadcrumb|carousel|gallery|timeline|rating|stepper|banner|bottom-nav|device-mockups|drawer|indicator|list-group|mega-menu|popover|speed-dial|video)/i)) {
					// Try to determine category from path
					const parts = path.split('/').filter(Boolean);
					let category = '';
					for (const p of parts) {
						if (['docs', 'components', 'elements', 'forms', 'typography', 'utilities', 'extend', 'plugins'].includes(p.toLowerCase())) {
							category = p;
						}
					}
					links.push({
						text: text,
						href: href,
						path: path,
						category: category,
					});
				}
			}
			return links;
		}`)
		if err != nil {
			return fmt.Errorf("discovering component links: %w", err)
		}

		rawLinks := linksResult.Value.Val()
		linkList, ok := rawLinks.([]interface{})
		if !ok || len(linkList) == 0 {
			return fmt.Errorf("no component links found at %s", url)
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
			CrawledAt: time.Now(),
		}

		categories := make(map[string]bool)

		// Visit each component page
		for i, item := range linkList {
			link, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			href, _ := link["href"].(string)
			text, _ := link["text"].(string)
			category, _ := link["category"].(string)

			if href == "" || text == "" {
				continue
			}

			categories[category] = true

			// Navigate to component page
			if err := page.Navigate(href); err != nil {
				continue
			}
			if err := page.WaitLoad(); err != nil {
				continue
			}
			page.MustWaitStable()

			comp := CatalogComponent{
				Name:     text,
				URL:      href,
				Category: category,
			}

			// Extract description (first paragraph or meta description)
			descResult, _ := page.Eval(`() => {
				const meta = document.querySelector('meta[name="description"]');
				if (meta) return meta.content;
				const firstP = document.querySelector('main p, article p, .content p');
				if (firstP) return firstP.textContent.trim().substring(0, 200);
				return '';
			}`)
			if descResult != nil {
				comp.Description = descResult.Value.Str()
			}

			// Extract variant/example sections
			variantsResult, _ := page.Eval(`() => {
				const variants = [];
				// Look for example sections (common patterns in component docs)
				const headings = document.querySelectorAll('h2, h3');
				for (const h of headings) {
					const name = h.textContent.trim();
					if (!name) continue;
					// Skip non-example headings
					if (/^(component data|props|events|slots|references|see also|best practices|accessibility)/i.test(name)) continue;
					variants.push({ name: name });
				}
				return variants.slice(0, 20); // Cap at 20 variants
			}`)
			if variantsResult != nil {
				if vList, ok := variantsResult.Value.Val().([]interface{}); ok {
					for _, v := range vList {
						if vm, ok := v.(map[string]interface{}); ok {
							name, _ := vm["name"].(string)
							comp.Variants = append(comp.Variants, ComponentVariant{Name: name})
						}
					}
				}
			}

			// Extract props from tables or definition lists
			propsResult, _ := page.Eval(`() => {
				const props = [];
				// Look for props tables
				const tables = document.querySelectorAll('table');
				for (const table of tables) {
					const headers = [...table.querySelectorAll('th')].map(th => th.textContent.trim().toLowerCase());
					const nameIdx = headers.findIndex(h => h === 'name' || h === 'prop' || h === 'property' || h === 'attribute');
					const typeIdx = headers.findIndex(h => h === 'type');
					const defaultIdx = headers.findIndex(h => h === 'default');
					const descIdx = headers.findIndex(h => h === 'description' || h === 'desc');

					if (nameIdx === -1) continue;

					const rows = table.querySelectorAll('tbody tr');
					for (const row of rows) {
						const cells = row.querySelectorAll('td');
						const prop = {
							name: nameIdx >= 0 && cells[nameIdx] ? cells[nameIdx].textContent.trim() : '',
							type: typeIdx >= 0 && cells[typeIdx] ? cells[typeIdx].textContent.trim() : '',
							default: defaultIdx >= 0 && cells[defaultIdx] ? cells[defaultIdx].textContent.trim() : '',
							description: descIdx >= 0 && cells[descIdx] ? cells[descIdx].textContent.trim() : '',
						};
						if (prop.name) props.push(prop);
					}
				}
				return props.slice(0, 30);
			}`)
			if propsResult != nil {
				if pList, ok := propsResult.Value.Val().([]interface{}); ok {
					for _, p := range pList {
						if pm, ok := p.(map[string]interface{}); ok {
							comp.Props = append(comp.Props, ComponentProp{
								Name:        strVal(pm, "name"),
								Type:        strVal(pm, "type"),
								Default:     strVal(pm, "default"),
								Description: strVal(pm, "description"),
							})
						}
					}
				}
			}

			// Extract code snippets
			snippetsResult, _ := page.Eval(`() => {
				const snippets = [];
				const codeBlocks = document.querySelectorAll('pre code, .highlight code, [class*="language-"]');
				for (const block of codeBlocks) {
					const code = block.textContent.trim();
					if (code.length > 20 && code.length < 2000) {
						snippets.push(code);
					}
				}
				return snippets.slice(0, 5); // First 5 snippets
			}`)
			if snippetsResult != nil {
				if sList, ok := snippetsResult.Value.Val().([]interface{}); ok {
					for _, s := range sList {
						if str, ok := s.(string); ok {
							comp.UsageSnippets = append(comp.UsageSnippets, str)
						}
					}
				}
			}

			// Extract design tokens used by examples on this page
			tokensResult, _ := page.Eval(`() => {
				const examples = document.querySelectorAll('[class*="example"], [class*="preview"], [class*="demo"], main > div, .prose > div');
				const colors = new Set();
				const fontSizes = new Set();
				const spacing = new Set();
				const radii = new Set();

				for (const ex of examples) {
					const els = ex.querySelectorAll('*');
					for (const el of els) {
						const cs = getComputedStyle(el);
						if (cs.color && cs.color !== 'rgba(0, 0, 0, 0)') colors.add(cs.color);
						if (cs.backgroundColor && cs.backgroundColor !== 'rgba(0, 0, 0, 0)') colors.add(cs.backgroundColor);
						fontSizes.add(cs.fontSize);
						if (cs.padding && cs.padding !== '0px') spacing.add(cs.padding);
						if (cs.margin && cs.margin !== '0px') spacing.add(cs.margin);
						if (cs.borderRadius && cs.borderRadius !== '0px') radii.add(cs.borderRadius);
					}
				}

				return {
					colors: [...colors].slice(0, 20),
					fontSizes: [...fontSizes].slice(0, 10),
					spacing: [...spacing].slice(0, 10),
					radii: [...radii].slice(0, 10),
				};
			}`)
			if tokensResult != nil {
				if tm, ok := tokensResult.Value.Val().(map[string]interface{}); ok {
					comp.Tokens = ComponentTokens{
						Colors:    toStringSlice(tm["colors"]),
						FontSizes: toStringSlice(tm["fontSizes"]),
						Spacing:   toStringSlice(tm["spacing"]),
						Radii:     toStringSlice(tm["radii"]),
					}
				}
			}

			// Extract related components (links within the page to other components)
			relationsResult, _ := page.Eval(`(currentHref) => {
				const relations = [];
				const links = document.querySelectorAll('a[href]');
				for (const a of links) {
					if (a.href !== currentHref && a.href.match(/\/(components|docs)\//)) {
						const text = a.textContent.trim();
						if (text.length > 0 && text.length < 40) {
							relations.push(text);
						}
					}
				}
				return [...new Set(relations)].slice(0, 10);
			}`, href)
			if relationsResult != nil {
				if rList, ok := relationsResult.Value.Val().([]interface{}); ok {
					for _, r := range rList {
						if s, ok := r.(string); ok {
							comp.Relations = append(comp.Relations, s)
						}
					}
				}
			}

			// Screenshot the page
			ssPath := filepath.Join(ssDir, fmt.Sprintf("%s-%d.png", sanitizeName(text), i))
			data, err := page.Screenshot(false, nil)
			if err == nil {
				if writeErr := os.WriteFile(ssPath, data, 0644); writeErr == nil {
					comp.Screenshot = ssPath
				}
			}

			catalog.Components = append(catalog.Components, comp)

			// Print progress to stderr (JSON goes to stdout)
			fmt.Fprintf(os.Stderr, "  cataloged: %s (%d variants, %d props)\n", text, len(comp.Variants), len(comp.Props))
		}

		// Collect categories
		for cat := range categories {
			if cat != "" {
				catalog.Categories = append(catalog.Categories, cat)
			}
		}

		// Save catalog
		catalogPath := filepath.Join(catalogDir, "catalog.json")
		catalogData, err := json.MarshalIndent(catalog, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling catalog: %w", err)
		}
		if err := os.WriteFile(catalogPath, catalogData, 0644); err != nil {
			return fmt.Errorf("writing catalog: %w", err)
		}

		output.Success("catalog", map[string]any{
			"name":           catalog.Name,
			"url":            catalog.URL,
			"componentCount": len(catalog.Components),
			"categories":     catalog.Categories,
			"catalogPath":    catalogPath,
			"screenshotDir":  ssDir,
		})
		return nil
	})

	if err != nil {
		output.Fail("catalog", err, "Is the browser running? Run: ks start")
		os.Exit(2)
	}
}

func RunCatalogSearch(args []string) {
	query := strings.Join(getNonFlagArgs(args), " ")
	if query == "" {
		output.Fail("catalog-search", fmt.Errorf("missing search query"), "Usage: ks catalog-search <query>")
		os.Exit(2)
	}

	catalog, err := loadCatalog()
	if err != nil {
		output.Fail("catalog-search", err, "Run 'ks catalog <url>' first to build a catalog")
		os.Exit(2)
	}

	queryLower := strings.ToLower(query)
	var matches []map[string]any

	for _, comp := range catalog.Components {
		score := 0

		// Name match (highest weight)
		if strings.Contains(strings.ToLower(comp.Name), queryLower) {
			score += 10
		}

		// Category match
		if strings.Contains(strings.ToLower(comp.Category), queryLower) {
			score += 5
		}

		// Description match
		if strings.Contains(strings.ToLower(comp.Description), queryLower) {
			score += 3
		}

		// Variant name match
		for _, v := range comp.Variants {
			if strings.Contains(strings.ToLower(v.Name), queryLower) {
				score += 2
				break
			}
		}

		// Prop name match
		for _, p := range comp.Props {
			if strings.Contains(strings.ToLower(p.Name), queryLower) {
				score += 1
				break
			}
		}

		if score > 0 {
			matches = append(matches, map[string]any{
				"name":        comp.Name,
				"category":    comp.Category,
				"description": comp.Description,
				"url":         comp.URL,
				"screenshot":  comp.Screenshot,
				"score":       score,
				"variantCount": len(comp.Variants),
				"propCount":   len(comp.Props),
			})
		}
	}

	// Sort by score (simple bubble sort for small lists)
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			si, _ := matches[i]["score"].(int)
			sj, _ := matches[j]["score"].(int)
			if sj > si {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	output.Success("catalog-search", map[string]any{
		"query":   query,
		"results": matches,
		"total":   len(matches),
	})
}

func RunCatalogShow(args []string) {
	name := strings.Join(getNonFlagArgs(args), " ")
	if name == "" {
		output.Fail("catalog-show", fmt.Errorf("missing component name"), "Usage: ks catalog-show <component-name>")
		os.Exit(2)
	}

	catalog, err := loadCatalog()
	if err != nil {
		output.Fail("catalog-show", err, "Run 'ks catalog <url>' first to build a catalog")
		os.Exit(2)
	}

	nameLower := strings.ToLower(name)
	for _, comp := range catalog.Components {
		if strings.ToLower(comp.Name) == nameLower || strings.Contains(strings.ToLower(comp.Name), nameLower) {
			output.Success("catalog-show", map[string]any{
				"name":          comp.Name,
				"url":           comp.URL,
				"category":      comp.Category,
				"description":   comp.Description,
				"variants":      comp.Variants,
				"props":         comp.Props,
				"usageSnippets": comp.UsageSnippets,
				"screenshot":    comp.Screenshot,
				"relations":     comp.Relations,
				"tokens":        comp.Tokens,
			})
			return
		}
	}

	output.Fail("catalog-show", fmt.Errorf("component not found: %s", name), "Run 'ks catalog-search' to find components")
	os.Exit(1)
}

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

func loadCatalog() (*Catalog, error) {
	dir, err := catalogDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if err != nil {
		return nil, fmt.Errorf("no catalog found: %w", err)
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	return &catalog, nil
}

func sanitizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	// Remove non-alphanumeric chars except hyphens
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
