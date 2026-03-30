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

Crawl a component library website and build a searchable catalog.

Arguments:
  url    The root URL of the component library (required)

Output:
  { "ok": true, "result": {
    "name": "Library Name",
    "componentCount": 42,
    "categories": ["components", "forms", ...],
    "catalogPath": "/path/to/catalog.json",
    "screenshotDir": "/path/to/screenshots/"
  }}

What gets cataloged for each component:
  - Name, URL, category, description
  - Variants/examples (section headings)
  - Props (from tables on the page)
  - Code snippets (first 5 code blocks)
  - Design tokens used (colors, font sizes, spacing, border radii)
  - Related components (cross-links)
  - Screenshot

Examples:
  ks catalog https://flowbite-svelte.com
  ks catalog https://ui.shadcn.com

Notes:
  Discovers component pages by scanning navigation links.
  Progress is printed to stderr during crawling.
  Catalog is saved to .kaleidoscope/catalog/catalog.json`,

	"catalog-search": `ks catalog-search <query>

Search the component catalog by name, category, description, variants, or props.

Arguments:
  query    Search terms (required)

Output:
  { "ok": true, "result": {
    "query": "button",
    "total": 3,
    "results": [
      { "name": "Buttons", "category": "components", "description": "...", "score": 15, ... },
      ...
    ]
  }}

Scoring:
  Name match:        +10
  Category match:    +5
  Description match: +3
  Variant match:     +2
  Prop match:        +1

Examples:
  ks catalog-search button
  ks catalog-search "date picker"
  ks catalog-search form
  ks catalog-search navigation`,

	"catalog-show": `ks catalog-show <component-name>

Show full details of a cataloged component.

Arguments:
  component-name    Name of the component (case-insensitive, partial match)

Output:
  { "ok": true, "result": {
    "name": "Buttons",
    "url": "https://...",
    "category": "components",
    "description": "...",
    "variants": [{ "name": "Default" }, { "name": "Pill" }, ...],
    "props": [{ "name": "color", "type": "string", "default": "blue" }],
    "usageSnippets": ["<Button color='blue'>Click</Button>"],
    "screenshot": "/path/to/screenshot.png",
    "relations": ["Button group", "Icon"],
    "tokens": { "colors": [...], "fontSizes": [...], "spacing": [...] }
  }}

Examples:
  ks catalog-show Buttons
  ks catalog-show "date picker"
  ks catalog-show card`,

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
