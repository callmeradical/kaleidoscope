package cmd

import "fmt"

// CommandUsage maps command names to their detailed usage info.
var CommandUsage = map[string]string{
	"start": `ks start [--local]

Launch a persistent headless Chrome instance.

Options:
  --local    Store state in .kaleidoscope/ (project-local) instead of ~/.kaleidoscope/

Output:
  { "ok": true, "result": { "pid": 1234, "wsEndpoint": "ws://...", "viewport": "1280x720" } }

Examples:
  ks start             # Start with global state
  ks start --local     # Start with project-local state

Notes:
  Chrome persists after the CLI exits. Run 'ks stop' to shut it down.
  If Chrome is already running, reports the existing instance.`,

	"stop": `ks stop

Shut down the persistent Chrome instance and remove state.

Output:
  { "ok": true, "result": { "message": "browser stopped" } }

Notes:
  Safe to call even if Chrome isn't running (will report an error but won't crash).`,

	"status": `ks status

Show the current state of the browser instance.

Output:
  { "ok": true, "result": { "running": true, "pid": 1234, "currentUrl": "...", "viewport": {...}, "uptime": "5m30s" } }

Notes:
  If the browser process has died but state remains, reports stale=true.`,

	"open": `ks open <url>

Navigate the browser to a URL and wait for it to load.

Arguments:
  url    The URL to navigate to (required)

Output:
  { "ok": true, "result": { "url": "https://...", "title": "Page Title" } }

Examples:
  ks open https://example.com
  ks open file:///path/to/local.html

Notes:
  Updates the currentUrl in state. Waits for the load event before returning.`,

	"screenshot": `ks screenshot [--selector <sel>] [--full-page] [--output <path>]

Take a screenshot of the current page or a specific element.

Options:
  --selector <sel>    CSS selector to screenshot a specific element
  --full-page         Capture the entire scrollable page, not just the viewport
  --output <path>     Save to a specific path (default: auto-generated in .kaleidoscope/screenshots/)

Output:
  { "ok": true, "result": { "path": "/path/to/screenshot.png", "viewport": {...}, "url": "..." } }

Examples:
  ks screenshot                             # Viewport screenshot
  ks screenshot --full-page                 # Full scrollable page
  ks screenshot --selector ".hero"          # Just the .hero element
  ks screenshot --output /tmp/shot.png      # Custom output path

Notes:
  Returns the file path so the agent can view the image.
  Screenshots are PNG format.`,

	"viewport": `ks viewport <preset|WxH>

Set the browser viewport size.

Arguments:
  preset    One of: mobile (375x812), tablet (768x1024), desktop (1280x720), wide (1920x1080)
  WxH       Custom size, e.g., 1440x900

Output:
  { "ok": true, "result": { "width": 375, "height": 812 } }

Examples:
  ks viewport mobile        # iPhone 14 Pro
  ks viewport tablet        # iPad
  ks viewport desktop       # Standard desktop
  ks viewport wide          # Full HD
  ks viewport 1440x900      # Custom

Notes:
  With no argument, lists available presets.
  Updates the viewport in state for subsequent commands.`,

	"inspect": `ks inspect <selector>

Get comprehensive information about a DOM element: bounding box, computed styles, visibility, tag name.

Arguments:
  selector    CSS selector for the element to inspect (required)

Output:
  { "ok": true, "result": {
    "selector": "h1",
    "tagName": "h1",
    "visible": true,
    "boundingBox": { "x": 20, "y": 100, "width": 600, "height": 40 },
    "styles": { "fontSize": "32px", "color": "rgb(0,0,0)", "padding": "0px", ... }
  }}

Computed styles returned:
  color, backgroundColor, fontSize, fontFamily, fontWeight, lineHeight,
  padding, margin, display, position, zIndex, opacity, visibility,
  overflow, width, height, borderRadius

Examples:
  ks inspect h1
  ks inspect ".nav-link"
  ks inspect "#main-content"

Notes:
  Use this to verify exact measurements, colors, and positioning.
  The bounding box coordinates are relative to the viewport.`,

	"layout": `ks layout [selector] [--depth N]

Dump the DOM layout tree with bounding boxes for each element.

Arguments:
  selector    CSS selector for the root element (default: body)

Options:
  --depth N   Maximum tree depth to traverse (default: 4)

Output:
  { "ok": true, "result": { "tree": {
    "tag": "body", "box": { "x": 0, "y": 0, "width": 1280, "height": 720 },
    "children": [ { "tag": "header", "box": {...}, "children": [...] } ]
  }}}

Examples:
  ks layout                    # Full page layout from body
  ks layout main --depth 2     # Just the main content, 2 levels deep
  ks layout ".container"       # Layout of a specific container

Notes:
  Invisible and zero-size elements are pruned.
  Each node includes: tag, id, classes, bounding box, display type, children.`,

	"html": `ks html [selector] [--outer]

Get the HTML content of the page or a specific element.

Arguments:
  selector    CSS selector (default: entire page)

Options:
  --outer     Return outerHTML instead of innerHTML

Output:
  { "ok": true, "result": { "html": "<div>...</div>", "selector": "..." } }

Examples:
  ks html                  # Full page HTML
  ks html nav              # Navigation inner HTML
  ks html nav --outer      # Navigation outer HTML`,

	"text": `ks text [selector]

Get the text content of the page or a specific element.

Arguments:
  selector    CSS selector (default: body)

Output:
  { "ok": true, "result": { "text": "...", "selector": "..." } }

Examples:
  ks text           # All page text
  ks text h1        # Just the h1 text
  ks text ".error"  # Error message text`,

	"js": `ks js <expression>

Evaluate a JavaScript expression in the page context and return the result.

Arguments:
  expression    JavaScript expression to evaluate (required)

Output:
  { "ok": true, "result": { "value": <result> } }

Examples:
  ks js "document.title"
  ks js "document.querySelectorAll('a').length"
  ks js "window.innerWidth"
  ks js "JSON.parse(document.querySelector('#data').textContent)"
  ks js "getComputedStyle(document.body).backgroundColor"

Notes:
  The expression is wrapped in an arrow function: () => <expression>
  Return values are JSON-serialized.`,

	"ax-tree": `ks ax-tree

Dump the full accessibility tree of the current page using Chrome's Accessibility CDP domain.

Output:
  { "ok": true, "result": { "nodeCount": 42, "nodes": [
    { "nodeId": "1", "role": "RootWebArea", "name": "Page Title", "children": [...] },
    { "nodeId": "2", "role": "heading", "name": "Welcome", "properties": { "level": 1 } },
    ...
  ]}}

Examples:
  ks ax-tree

Notes:
  Ignored/hidden nodes are filtered out.
  Each node includes: role, name, properties, children.
  Use to check landmark roles, heading hierarchy, and ARIA attributes.`,

	"audit": `ks audit [selector]

Run a comprehensive UX and accessibility audit on the current page.

Arguments:
  selector    CSS selector to scope the audit (default: entire page)

Output:
  { "ok": true, "result": {
    "summary": { "totalIssues": 5, "contrastViolations": 2, "touchViolations": 1, "typographyWarnings": 2 },
    "accessibility": { "totalNodes": 50, "activeNodes": 45 },
    "contrast": { "violations": 2 },
    "touchTargets": { "total": 10, "violations": 1 },
    "typography": { "warnings": 2 }
  }}

Checks performed:
  - Accessibility tree analysis (CDP Accessibility domain)
  - WCAG color contrast (4.5:1 normal, 3:1 large text)
  - Touch target sizes (48x48px minimum per WCAG 2.5.8)
  - Typography (font size, line height, font-family fallbacks)

Examples:
  ks audit                  # Full page audit
  ks audit ".main-content"  # Scope to main content

Notes:
  Exit code 0 = audit ran successfully (even if issues found).
  Exit code 1 = audit found violations.
  Exit code 2 = error running the audit.`,

	"contrast": `ks contrast [selector]

Check WCAG color contrast ratios for all text elements on the page.

Arguments:
  selector    CSS selector to scope the check (default: all text elements)

Output:
  { "ok": true, "result": {
    "elements": [
      { "selector": "h1", "text": "Hello", "ratio": 18.1, "meetsMinimum": true, "aa": true, "aaa": true },
      { "selector": "p.muted", "text": "Some text", "ratio": 2.5, "meetsMinimum": false, "aa": false }
    ],
    "summary": { "total": 10, "passes": 8, "violations": 2 }
  }}

WCAG thresholds:
  AA normal text:  4.5:1
  AA large text:   3.0:1 (≥18pt or ≥14pt bold)
  AAA normal text: 7.0:1
  AAA large text:  4.5:1

Examples:
  ks contrast               # Check all text
  ks contrast ".hero"       # Check just the hero section

Notes:
  Walks up the DOM to find effective background colors for transparent elements.
  Defaults to white (#fff) if no background is found.`,

	"spacing": `ks spacing [selector]

Analyze spacing consistency between sibling elements.

Arguments:
  selector    CSS selector to scope the analysis (default: entire page)

Output:
  { "ok": true, "result": {
    "groups": [
      { "container": "div#main", "childCount": 5, "gaps": [16, 16, 24, 16],
        "detectedScale": 8, "inconsistencies": [{ "index": 2, "gap": 24, "expected": 16 }] }
    ],
    "summary": { "groupsAnalyzed": 3, "totalInconsistencies": 1 }
  }}

Examples:
  ks spacing                # Analyze entire page
  ks spacing ".card-list"   # Analyze a specific container

Notes:
  Detects the dominant spacing scale (e.g., 4px or 8px grid).
  Flags gaps that deviate from the detected scale.
  Measures vertical gaps between consecutive sibling elements.`,

	"breakpoints": `ks breakpoints [--full-page]

Take screenshots at all standard breakpoints in one command.

Options:
  --full-page    Capture full scrollable page at each breakpoint

Breakpoints:
  mobile:   375x812
  tablet:   768x1024
  desktop:  1280x720
  wide:     1920x1080

Output:
  { "ok": true, "result": {
    "url": "https://...",
    "screenshots": [
      { "breakpoint": "mobile", "width": 375, "height": 812, "path": "/path/to/mobile.png" },
      { "breakpoint": "tablet", ... },
      { "breakpoint": "desktop", ... },
      { "breakpoint": "wide", ... }
    ]
  }}

Examples:
  ks breakpoints                # Viewport screenshots at all breakpoints
  ks breakpoints --full-page    # Full page at each breakpoint

Notes:
  Waits for layout to stabilize at each breakpoint before capturing.
  Restores the original viewport after completing.
  Screenshots are saved to .kaleidoscope/screenshots/ with breakpoint labels.`,

	"catalog": `ks catalog <url>

Crawl a design system website and build a searchable catalog.
Discovers components, foundations (tokens), patterns, content guidelines, and icons.

Arguments:
  url    The root URL of the design system (required)

Output:
  { "ok": true, "result": {
    "name": "Library Name",
    "source": "web",
    "entryCount": 42,
    "kinds": ["component", "foundation", "pattern", "content", "icon"],
    "categories": ["components", "foundations", ...],
    "catalogPath": "/path/to/catalog.json"
  }}

Entry kinds and what gets extracted:
  component   - variants, props, code snippets, design tokens, screenshot
  foundation  - token name/value pairs, usage guidance, CSS snippets
  pattern     - problem solved, when to use, composed-of components, best practices
  content     - guidelines, do/don't examples, terminology word lists
  icon        - icon name, sizes, SVG content, tags

Examples:
  ks catalog https://helios.hashicorp.design
  ks catalog https://flowbite-svelte.com
  ks catalog https://ui.shadcn.com

Notes:
  Discovers pages by scanning navigation links and classifying by URL path.
  Progress is printed to stderr during crawling.
  Catalog is saved to .kaleidoscope/catalog/catalog.json`,

	"catalog-repo": `ks catalog-repo <repo-url> [--ref <branch/tag>]

Catalog a design system from its git repository. Extracts tokens, icons,
component docs, and content guidelines from the source code.

Arguments:
  repo-url    Git repository URL (required)

Options:
  --ref <branch/tag>    Checkout a specific branch or tag (default: default branch)

Output:
  { "ok": true, "result": {
    "name": "design-system",
    "source": "repo",
    "entryCount": 150,
    "kinds": ["foundation", "icon", "component"],
    "catalogPath": "/path/to/catalog.json"
  }}

What gets extracted:
  - Token files: *.tokens.json (Style Dictionary), CSS custom properties
  - Icons: SVG files in icon directories (name, sizes, SVG content)
  - Components: README.md in component directories (name, description)
  - Content: Markdown files in content/patterns/guidelines directories

Examples:
  ks catalog-repo https://github.com/hashicorp/design-system
  ks catalog-repo https://github.com/primer/primitives --ref main

Notes:
  Does NOT require the browser (no ks start needed).
  Performs a shallow clone to a temp directory, which is cleaned up after.
  Useful for JS-heavy design system sites that don't crawl well.`,

	"catalog-search": `ks catalog-search <query> [--kind <type>]

Search the catalog by name, category, description, and kind-specific fields.

Arguments:
  query    Search terms (required)

Options:
  --kind <type>    Filter by entry kind: component, foundation, pattern, content, icon

Output:
  { "ok": true, "result": {
    "query": "button",
    "kind": "",
    "total": 3,
    "results": [
      { "name": "Buttons", "kind": "component", "category": "components", "score": 15, ... },
      ...
    ]
  }}

Scoring (universal):
  Name match:        +10
  Category match:    +5
  Description match: +3

Scoring (kind-specific):
  Component:   variant name +2, prop name +1
  Foundation:  token name +2, token value +1
  Icon:        icon name +3, tag +2
  Pattern:     problem solved +3, composed-of +2
  Content:     term +3, guideline text +2

Examples:
  ks catalog-search button
  ks catalog-search --kind foundation color
  ks catalog-search --kind icon arrow
  ks catalog-search --kind pattern "empty state"
  ks catalog-search --kind content "error message"`,

	"catalog-show": `ks catalog-show <name> [--kind <type>]

Show full details of a cataloged entry.

Arguments:
  name    Name of the entry (case-insensitive, partial match)

Options:
  --kind <type>    Filter by kind to disambiguate (e.g., "Typography" may be both foundation and component)

Output varies by kind:

  component:
    variants, props, usageSnippets, tokens

  foundation:
    tokenCategory, tokens (name/value pairs), usageGuidance, cssSnippets

  pattern:
    problemSolved, whenToUse, whenNotToUse, composedOf, bestPractices, usageSnippets

  content:
    contentType, guidelines, doExamples, dontExamples, wordList

  icon:
    iconName, sizes, svg, usageNote, tags

Examples:
  ks catalog-show Buttons
  ks catalog-show --kind foundation "Color"
  ks catalog-show --kind pattern "Empty State"
  ks catalog-show --kind icon "arrow-right"`,

	"report": `ks report [--output <path>] [--full-page] [--selector <sel>]

Generate a self-contained HTML report with screenshots and UX findings.

Options:
  --output <path>     Save report to a specific path (default: auto-generated in .kaleidoscope/)
  --full-page         Capture full scrollable page at each breakpoint
  --selector <sel>    Scope analysis to a specific element

What the report includes:
  - Screenshots at 4 breakpoints (mobile, tablet, desktop, wide)
  - Contrast violations (WCAG AA/AAA)
  - Touch target violations (48x48px minimum)
  - Typography warnings (font size, line height, fallbacks)
  - Spacing inconsistencies
  - Accessibility tree summary

Output:
  { "ok": true, "result": {
    "path": "/path/to/report.html",
    "url": "https://...",
    "totalIssues": 5,
    "summary": { "contrastViolations": 2, "touchViolations": 1, ... }
  }}

Examples:
  ks report                           # Generate report for current page
  ks report --output /tmp/review.html # Save to specific path
  ks report --full-page               # Full-page screenshots
  ks report --selector ".main"        # Scope analysis to .main

Notes:
  The HTML report is self-contained with base64-embedded screenshots.
  Open it in any browser to view.`,

	"install-hook": `ks install-hook [--force]

Install a git pre-commit hook that runs snapshot and diff on every commit.

Flags:
  --force    Overwrite existing hook without warning

Examples:
  ks install-hook
  ks install-hook --force

Output:
  {"ok":true,"command":"install-hook","result":{"hookPath":".git/hooks/pre-commit","overwrite":false}}

Notes:
  The hook is advisory and always exits 0, so it never blocks commits.
  Requires .ks-project.json in the current directory.`,

	"install-skills": `ks install-skills

Install Claude Code skills for front-end design to ~/.claude/commands/.

Skills installed:
  /ks-design-review       Full page design review with structured report
  /ks-build-component     Iteratively build and verify UI components visually
  /ks-visual-audit        Compare page against design tokens, flag deviations
  /ks-responsive-check    Verify responsive design across breakpoints
  /ks-design-tokens       Extract implicit design system from a page
  /ks-accessibility-fix   Find and fix WCAG accessibility issues
  /ks-use-catalog         Build using components from a cataloged library

Output:
  { "ok": true, "result": { "installed": [...], "directory": "~/.claude/commands", "count": 7 } }

Notes:
  Skills are prefixed with ks- to namespace them.
  Invoke in Claude Code with /ks-design-review, /ks-build-component, etc.`,
}

// PrintUsage prints detailed usage for a command and returns true if --usage was found.
func PrintUsage(command string, args []string) bool {
	if !hasFlag(args, "--usage") {
		return false
	}
	usage, ok := CommandUsage[command]
	if !ok {
		fmt.Printf("No usage information for command: %s\n", command)
		return true
	}
	fmt.Println(usage)
	return true
}
