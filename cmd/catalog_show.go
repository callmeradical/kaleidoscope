package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/callmeradical/kaleidoscope/output"
)

func RunCatalogShow(args []string) {
	name := strings.Join(getNonFlagArgs(args), " ")
	if name == "" {
		output.Fail("catalog-show", fmt.Errorf("missing entry name"), "Usage: ks catalog-show <name> [--kind <type>]")
		os.Exit(2)
	}

	kindFilter := CatalogEntryKind(getFlagValue(args, "--kind"))

	catalog, err := loadCatalog()
	if err != nil {
		output.Fail("catalog-show", err, "Run 'ks catalog <url>' first to build a catalog")
		os.Exit(2)
	}

	nameLower := strings.ToLower(name)
	for _, entry := range catalog.Entries {
		if kindFilter != "" && entry.Kind != kindFilter {
			continue
		}
		if strings.ToLower(entry.Name) == nameLower || strings.Contains(strings.ToLower(entry.Name), nameLower) {
			result := map[string]any{
				"kind":        entry.Kind,
				"name":        entry.Name,
				"url":         entry.URL,
				"category":    entry.Category,
				"description": entry.Description,
				"screenshot":  entry.Screenshot,
				"relations":   entry.Relations,
			}

			// Add kind-specific data
			switch entry.Kind {
			case KindComponent:
				if entry.Component != nil {
					result["variants"] = entry.Component.Variants
					result["props"] = entry.Component.Props
					result["usageSnippets"] = entry.Component.UsageSnippets
					result["tokens"] = entry.Component.Tokens
				}
			case KindFoundation:
				if entry.Foundation != nil {
					result["tokenCategory"] = entry.Foundation.TokenCategory
					result["tokens"] = entry.Foundation.Tokens
					result["usageGuidance"] = entry.Foundation.UsageGuidance
					result["cssSnippets"] = entry.Foundation.CSSSnippets
				}
			case KindPattern:
				if entry.Pattern != nil {
					result["problemSolved"] = entry.Pattern.ProblemSolved
					result["whenToUse"] = entry.Pattern.WhenToUse
					result["whenNotToUse"] = entry.Pattern.WhenNotToUse
					result["composedOf"] = entry.Pattern.ComposedOf
					result["usageSnippets"] = entry.Pattern.UsageSnippets
					result["variants"] = entry.Pattern.Variants
					result["bestPractices"] = entry.Pattern.BestPractices
				}
			case KindContent:
				if entry.Content != nil {
					result["contentType"] = entry.Content.ContentType
					result["guidelines"] = entry.Content.Guidelines
					result["doExamples"] = entry.Content.DoExamples
					result["dontExamples"] = entry.Content.DontExamples
					result["wordList"] = entry.Content.WordList
				}
			case KindIcon:
				if entry.Icon != nil {
					result["iconName"] = entry.Icon.IconName
					result["sizes"] = entry.Icon.Sizes
					result["svg"] = entry.Icon.SVG
					result["usageNote"] = entry.Icon.UsageNote
					result["tags"] = entry.Icon.Tags
				}
			}

			output.Success("catalog-show", result)
			return
		}
	}

	output.Fail("catalog-show", fmt.Errorf("entry not found: %s", name), "Run 'ks catalog-search' to find entries")
	os.Exit(1)
}
