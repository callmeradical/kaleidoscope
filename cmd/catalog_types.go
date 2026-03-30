package cmd

import "time"

// CatalogEntryKind identifies what type of design system entry this is.
type CatalogEntryKind string

const (
	KindComponent  CatalogEntryKind = "component"
	KindFoundation CatalogEntryKind = "foundation"
	KindPattern    CatalogEntryKind = "pattern"
	KindContent    CatalogEntryKind = "content"
	KindIcon       CatalogEntryKind = "icon"
)

// Catalog represents a full design system catalog.
type Catalog struct {
	Name       string           `json:"name"`
	URL        string           `json:"url"`
	Source     string           `json:"source"` // "web" or "repo"
	CrawledAt  time.Time        `json:"crawledAt"`
	Entries    []CatalogEntry   `json:"entries"`
	Categories []string         `json:"categories"`
	Kinds      []string         `json:"kinds"`
}

// CatalogEntry represents a single item in the design system catalog.
type CatalogEntry struct {
	Kind        CatalogEntryKind `json:"kind"`
	Name        string           `json:"name"`
	URL         string           `json:"url,omitempty"`
	Category    string           `json:"category"`
	Description string           `json:"description"`
	Screenshot  string           `json:"screenshot,omitempty"`
	Relations   []string         `json:"relations,omitempty"`

	// Kind-specific data — only one will be populated.
	Component  *ComponentData  `json:"component,omitempty"`
	Foundation *FoundationData `json:"foundation,omitempty"`
	Pattern    *PatternData    `json:"pattern,omitempty"`
	Content    *ContentData    `json:"content,omitempty"`
	Icon       *IconData       `json:"icon,omitempty"`
}

// ComponentData holds data specific to UI components.
type ComponentData struct {
	Variants      []ComponentVariant `json:"variants,omitempty"`
	Props         []ComponentProp    `json:"props,omitempty"`
	UsageSnippets []string           `json:"usageSnippets,omitempty"`
	Tokens        ComponentTokens    `json:"tokens,omitempty"`
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

// ComponentTokens captures design tokens used by a component.
type ComponentTokens struct {
	Colors    []string `json:"colors,omitempty"`
	FontSizes []string `json:"fontSizes,omitempty"`
	Spacing   []string `json:"spacing,omitempty"`
	Radii     []string `json:"radii,omitempty"`
}

// FoundationData holds data for design foundations (color, typography, spacing, etc.).
type FoundationData struct {
	TokenCategory string        `json:"tokenCategory"` // "color", "typography", "spacing", "elevation", "layout"
	Tokens        []DesignToken `json:"tokens"`
	UsageGuidance string        `json:"usageGuidance,omitempty"`
	CSSSnippets   []string      `json:"cssSnippets,omitempty"`
}

// DesignToken represents a single design token.
type DesignToken struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Alias       string `json:"alias,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

// PatternData holds data for UI patterns (form layouts, empty states, error handling, etc.).
type PatternData struct {
	ProblemSolved string             `json:"problemSolved"`
	WhenToUse     string             `json:"whenToUse,omitempty"`
	WhenNotToUse  string             `json:"whenNotToUse,omitempty"`
	ComposedOf    []string           `json:"composedOf,omitempty"`
	UsageSnippets []string           `json:"usageSnippets,omitempty"`
	Variants      []ComponentVariant `json:"variants,omitempty"`
	BestPractices []string           `json:"bestPractices,omitempty"`
}

// ContentData holds voice/tone, writing patterns, and terminology guidelines.
type ContentData struct {
	ContentType  string        `json:"contentType"` // "voice-tone", "writing-pattern", "terminology"
	Guidelines   []string      `json:"guidelines"`
	DoExamples   []string      `json:"doExamples,omitempty"`
	DontExamples []string      `json:"dontExamples,omitempty"`
	WordList     []ContentTerm `json:"wordList,omitempty"`
}

// ContentTerm represents a terminology guideline entry.
type ContentTerm struct {
	Term    string `json:"term"`
	Use     string `json:"use"`
	DontUse string `json:"dontUse,omitempty"`
	Context string `json:"context,omitempty"`
}

// IconData holds data for individual icons.
type IconData struct {
	IconName  string   `json:"iconName"`
	Sizes     []string `json:"sizes,omitempty"`
	SVG       string   `json:"svg,omitempty"`
	UsageNote string   `json:"usageNote,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// legacyCatalog is used to detect and migrate old-format catalog files.
type legacyCatalog struct {
	Name       string                 `json:"name"`
	URL        string                 `json:"url"`
	CrawledAt  time.Time              `json:"crawledAt"`
	Components []legacyCatalogComponent `json:"components"`
	Categories []string               `json:"categories"`
}

type legacyCatalogComponent struct {
	Name          string           `json:"name"`
	URL           string           `json:"url"`
	Category      string           `json:"category"`
	Description   string           `json:"description"`
	Variants      []ComponentVariant `json:"variants,omitempty"`
	Props         []ComponentProp  `json:"props,omitempty"`
	UsageSnippets []string         `json:"usageSnippets,omitempty"`
	Screenshot    string           `json:"screenshot,omitempty"`
	Relations     []string         `json:"relations,omitempty"`
	Tokens        ComponentTokens  `json:"tokens,omitempty"`
}

// migrateFromLegacy converts an old Components-based catalog to the new Entries format.
func migrateFromLegacy(lc *legacyCatalog) *Catalog {
	catalog := &Catalog{
		Name:       lc.Name,
		URL:        lc.URL,
		Source:     "web",
		CrawledAt:  lc.CrawledAt,
		Categories: lc.Categories,
		Kinds:      []string{"component"},
	}
	for _, comp := range lc.Components {
		entry := CatalogEntry{
			Kind:        KindComponent,
			Name:        comp.Name,
			URL:         comp.URL,
			Category:    comp.Category,
			Description: comp.Description,
			Screenshot:  comp.Screenshot,
			Relations:   comp.Relations,
			Component: &ComponentData{
				Variants:      comp.Variants,
				Props:         comp.Props,
				UsageSnippets: comp.UsageSnippets,
				Tokens:        comp.Tokens,
			},
		}
		catalog.Entries = append(catalog.Entries, entry)
	}
	return catalog
}
