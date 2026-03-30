package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/callmeradical/kaleidoscope/output"
)

func RunCatalogSearch(args []string) {
	query := strings.Join(getNonFlagArgs(args), " ")
	if query == "" {
		output.Fail("catalog-search", fmt.Errorf("missing search query"), "Usage: ks catalog-search <query> [--kind <type>]")
		os.Exit(2)
	}

	kindFilter := CatalogEntryKind(getFlagValue(args, "--kind"))

	catalog, err := loadCatalog()
	if err != nil {
		output.Fail("catalog-search", err, "Run 'ks catalog <url>' first to build a catalog")
		os.Exit(2)
	}

	queryLower := strings.ToLower(query)
	var matches []map[string]any

	for _, entry := range catalog.Entries {
		// Apply kind filter
		if kindFilter != "" && entry.Kind != kindFilter {
			continue
		}

		score := 0

		// Universal scoring
		if strings.Contains(strings.ToLower(entry.Name), queryLower) {
			score += 10
		}
		if strings.Contains(strings.ToLower(entry.Category), queryLower) {
			score += 5
		}
		if strings.Contains(strings.ToLower(entry.Description), queryLower) {
			score += 3
		}

		// Kind-specific scoring
		switch entry.Kind {
		case KindComponent:
			if entry.Component != nil {
				for _, v := range entry.Component.Variants {
					if strings.Contains(strings.ToLower(v.Name), queryLower) {
						score += 2
						break
					}
				}
				for _, p := range entry.Component.Props {
					if strings.Contains(strings.ToLower(p.Name), queryLower) {
						score += 1
						break
					}
				}
			}

		case KindFoundation:
			if entry.Foundation != nil {
				for _, t := range entry.Foundation.Tokens {
					if strings.Contains(strings.ToLower(t.Name), queryLower) {
						score += 2
						break
					}
				}
				for _, t := range entry.Foundation.Tokens {
					if strings.Contains(strings.ToLower(t.Value), queryLower) {
						score += 1
						break
					}
				}
			}

		case KindIcon:
			if entry.Icon != nil {
				if strings.Contains(strings.ToLower(entry.Icon.IconName), queryLower) {
					score += 3
				}
				for _, tag := range entry.Icon.Tags {
					if strings.Contains(strings.ToLower(tag), queryLower) {
						score += 2
						break
					}
				}
			}

		case KindPattern:
			if entry.Pattern != nil {
				if strings.Contains(strings.ToLower(entry.Pattern.ProblemSolved), queryLower) {
					score += 3
				}
				for _, c := range entry.Pattern.ComposedOf {
					if strings.Contains(strings.ToLower(c), queryLower) {
						score += 2
						break
					}
				}
			}

		case KindContent:
			if entry.Content != nil {
				for _, g := range entry.Content.Guidelines {
					if strings.Contains(strings.ToLower(g), queryLower) {
						score += 2
						break
					}
				}
				for _, t := range entry.Content.WordList {
					if strings.Contains(strings.ToLower(t.Term), queryLower) {
						score += 3
						break
					}
				}
			}
		}

		if score > 0 {
			result := map[string]any{
				"name":        entry.Name,
				"kind":        entry.Kind,
				"category":    entry.Category,
				"description": entry.Description,
				"url":         entry.URL,
				"screenshot":  entry.Screenshot,
				"score":       score,
			}

			// Add kind-specific counts
			switch entry.Kind {
			case KindComponent:
				if entry.Component != nil {
					result["variantCount"] = len(entry.Component.Variants)
					result["propCount"] = len(entry.Component.Props)
				}
			case KindFoundation:
				if entry.Foundation != nil {
					result["tokenCount"] = len(entry.Foundation.Tokens)
					result["tokenCategory"] = entry.Foundation.TokenCategory
				}
			case KindIcon:
				if entry.Icon != nil {
					result["iconName"] = entry.Icon.IconName
					result["sizeCount"] = len(entry.Icon.Sizes)
				}
			case KindPattern:
				if entry.Pattern != nil {
					result["componentCount"] = len(entry.Pattern.ComposedOf)
				}
			case KindContent:
				if entry.Content != nil {
					result["contentType"] = entry.Content.ContentType
					result["guidelineCount"] = len(entry.Content.Guidelines)
				}
			}

			matches = append(matches, result)
		}
	}

	// Sort by score descending
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
		"kind":    kindFilter,
		"results": matches,
		"total":   len(matches),
	})
}
