package main

import (
	"fmt"
	"os"

	"github.com/callmeradical/kaleidoscope/cmd"
)

var version = "0.1.0"

var usage = `kaleidoscope (ks) — AI agent front-end design toolkit

Usage: ks <command> [options]

Browser Lifecycle:
  start [--local]         Launch headless Chrome
  stop                    Shut down Chrome
  status                  Show browser state

Navigation:
  open <url>              Navigate to URL
  viewport <preset|WxH>  Set viewport size

Capture:
  screenshot [options]    Take a screenshot
  breakpoints [options]   Screenshot at multiple viewports

Inspection:
  inspect <selector>      Element position, size, styles
  layout [selector]       DOM layout tree with bounding boxes
  html [selector]         Get HTML content
  text [selector]         Get text content
  js <expression>         Evaluate JavaScript
  ax-tree [options]       Accessibility tree dump

UX Evaluation:
  audit [options]         Full UX/a11y audit
  contrast [selector]     WCAG color contrast check
  spacing [selector]      Spacing consistency analysis
  report [options]        Generate HTML report with screenshots and findings
  diff-report [snapshot-id]  Side-by-side HTML diff vs baseline

Design System Catalog:
  catalog <url>              Crawl a design system site and build searchable index
  catalog-repo <repo-url>    Catalog from a git repository (tokens, icons, docs)
  catalog-search <query>     Search the catalog (--kind to filter by type)
  catalog-show <name>        Show full details of a cataloged entry

Skills:
  install-skills          Install Claude Code skills for front-end design

Options:
  --human                 Human-readable output (default: JSON)
  --local                 Use project-local state (.kaleidoscope/)
  --version               Show version
  --help                  Show this help
`

func main() {
	args := os.Args[1:]

	if len(args) == 0 || hasFlag(args, "--help") || hasFlag(args, "-h") {
		fmt.Print(usage)
		os.Exit(0)
	}

	if hasFlag(args, "--version") {
		fmt.Println(version)
		os.Exit(0)
	}

	command := args[0]
	cmdArgs := args[1:]

	// Check for --usage flag on any command
	if cmd.PrintUsage(command, cmdArgs) {
		os.Exit(0)
	}

	switch command {
	case "start":
		cmd.RunStart(cmdArgs)
	case "stop":
		cmd.RunStop(cmdArgs)
	case "status":
		cmd.RunStatus(cmdArgs)
	case "open":
		cmd.RunOpen(cmdArgs)
	case "screenshot":
		cmd.RunScreenshot(cmdArgs)
	case "viewport":
		cmd.RunViewport(cmdArgs)
	case "js":
		cmd.RunJS(cmdArgs)
	case "html":
		cmd.RunHTML(cmdArgs)
	case "text":
		cmd.RunText(cmdArgs)
	case "inspect":
		cmd.RunInspect(cmdArgs)
	case "layout":
		cmd.RunLayout(cmdArgs)
	case "ax-tree":
		cmd.RunAxTree(cmdArgs)
	case "audit":
		cmd.RunAudit(cmdArgs)
	case "contrast":
		cmd.RunContrast(cmdArgs)
	case "spacing":
		cmd.RunSpacing(cmdArgs)
	case "breakpoints":
		cmd.RunBreakpoints(cmdArgs)
	case "report":
		cmd.RunReport(cmdArgs)
	case "diff-report":
		cmd.RunDiffReport(cmdArgs)
	case "catalog":
		cmd.RunCatalog(cmdArgs)
	case "catalog-search":
		cmd.RunCatalogSearch(cmdArgs)
	case "catalog-show":
		cmd.RunCatalogShow(cmdArgs)
	case "catalog-repo":
		cmd.RunCatalogRepo(cmdArgs)
	case "install-skills":
		cmd.RunInstallSkills(cmdArgs)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\nRun 'ks --help' for usage.\n", command)
		os.Exit(2)
	}
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
