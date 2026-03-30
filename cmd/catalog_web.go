package cmd

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod"
)

// discoveredLink represents a link found during crawling with its inferred kind.
type discoveredLink struct {
	text     string
	href     string
	category string
	kind     CatalogEntryKind
}

// discoverLinks finds all design system pages from navigation and classifies them by kind.
func discoverLinks(page *rod.Page) ([]discoveredLink, error) {
	linksResult, err := page.Eval(`() => {
		const links = [];
		const seen = new Set();
		const allLinks = document.querySelectorAll('a[href]');
		for (const a of allLinks) {
			const href = a.href;
			const text = a.textContent.trim();
			if (!text || text.length > 60) continue;
			if (seen.has(href)) continue;
			seen.add(href);

			const path = new URL(href, window.location.origin).pathname.toLowerCase();
			let kind = '';
			let category = '';

			// Classify by path
			if (path.match(/\/(foundations?|tokens?|colors?|palette|typography|type-scale|spacing|elevation|shadows?|layout|grid|breakpoints?|motion|animation)\//i)) {
				kind = 'foundation';
			} else if (path.match(/\/(patterns?|recipes?|templates?|empty-state|error-handling|loading|skeleton|composition|page-layout)\//i)) {
				kind = 'pattern';
			} else if (path.match(/\/(content|voice|tone|writing|copy|guidelines|terminology|word-list|grammar|capitalization)\//i)) {
				kind = 'content';
			} else if (path.match(/\/(icons?|iconography|glyphs?|symbols?)\//i)) {
				kind = 'icon';
			} else if (path.match(/\/(components?|elements?|forms?|utilities)\//i) ||
			           path.match(/\/(button|card|modal|table|input|select|nav|tab|alert|badge|avatar|accordion|drawer|dropdown|footer|sidebar|toast|tooltip|pagination|progress|spinner|breadcrumb|carousel|gallery|timeline|rating|stepper|banner|bottom-nav|device-mockups|indicator|list-group|mega-menu|popover|speed-dial|video)/i)) {
				kind = 'component';
			} else if (path.match(/\/(docs|extend|plugins)\//i)) {
				kind = 'component';
			}

			if (!kind) continue;

			// Determine category from path
			const parts = path.split('/').filter(Boolean);
			for (const p of parts) {
				if (['docs', 'components', 'elements', 'forms', 'typography', 'utilities',
				     'extend', 'plugins', 'foundations', 'patterns', 'content', 'icons',
				     'tokens', 'guidelines'].includes(p.toLowerCase())) {
					category = p;
				}
			}

			links.push({ text, href, path, category, kind });
		}
		return links;
	}`)
	if err != nil {
		return nil, fmt.Errorf("discovering links: %w", err)
	}

	rawLinks := linksResult.Value.Val()
	linkList, ok := rawLinks.([]interface{})
	if !ok {
		return nil, nil
	}

	var result []discoveredLink
	for _, item := range linkList {
		link, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		result = append(result, discoveredLink{
			text:     strVal(link, "text"),
			href:     strVal(link, "href"),
			category: strVal(link, "category"),
			kind:     CatalogEntryKind(strVal(link, "kind")),
		})
	}
	return result, nil
}

// extractDescription extracts a description from meta tag or first paragraph.
func extractDescription(page *rod.Page) string {
	descResult, _ := page.Eval(`() => {
		const meta = document.querySelector('meta[name="description"]');
		if (meta) return meta.content;
		const firstP = document.querySelector('main p, article p, .content p');
		if (firstP) return firstP.textContent.trim().substring(0, 200);
		return '';
	}`)
	if descResult != nil {
		return descResult.Value.Str()
	}
	return ""
}

// extractRelations extracts links to other design system pages.
func extractRelations(page *rod.Page, currentHref string) []string {
	relationsResult, _ := page.Eval(`(currentHref) => {
		const relations = [];
		const links = document.querySelectorAll('a[href]');
		for (const a of links) {
			if (a.href !== currentHref && a.href.match(/\/(components|docs|patterns|foundations)\//)) {
				const text = a.textContent.trim();
				if (text.length > 0 && text.length < 40) {
					relations.push(text);
				}
			}
		}
		return [...new Set(relations)].slice(0, 10);
	}`, currentHref)
	if relationsResult != nil {
		if rList, ok := relationsResult.Value.Val().([]interface{}); ok {
			var relations []string
			for _, r := range rList {
				if s, ok := r.(string); ok {
					relations = append(relations, s)
				}
			}
			return relations
		}
	}
	return nil
}

// extractComponent extracts component-specific data from a page.
func extractComponent(page *rod.Page) *ComponentData {
	comp := &ComponentData{}

	// Extract variants from section headings
	variantsResult, _ := page.Eval(`() => {
		const variants = [];
		const headings = document.querySelectorAll('h2, h3');
		for (const h of headings) {
			const name = h.textContent.trim();
			if (!name) continue;
			if (/^(component data|props|events|slots|references|see also|best practices|accessibility)/i.test(name)) continue;
			variants.push({ name: name });
		}
		return variants.slice(0, 20);
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

	// Extract props from tables
	propsResult, _ := page.Eval(`() => {
		const props = [];
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
		return snippets.slice(0, 5);
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

	// Extract design tokens
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

	return comp
}

// extractFoundation extracts foundation/token data from a page.
func extractFoundation(page *rod.Page, category string) *FoundationData {
	fd := &FoundationData{}

	// Infer token category from the page path/category
	catLower := strings.ToLower(category)
	switch {
	case strings.Contains(catLower, "color") || strings.Contains(catLower, "palette"):
		fd.TokenCategory = "color"
	case strings.Contains(catLower, "typo") || strings.Contains(catLower, "font") || strings.Contains(catLower, "type"):
		fd.TokenCategory = "typography"
	case strings.Contains(catLower, "spac") || strings.Contains(catLower, "grid") || strings.Contains(catLower, "layout"):
		fd.TokenCategory = "spacing"
	case strings.Contains(catLower, "elev") || strings.Contains(catLower, "shadow"):
		fd.TokenCategory = "elevation"
	case strings.Contains(catLower, "motion") || strings.Contains(catLower, "anim"):
		fd.TokenCategory = "motion"
	default:
		fd.TokenCategory = "general"
	}

	// Also try to infer from the page URL
	urlResult, _ := page.Eval(`() => window.location.pathname.toLowerCase()`)
	if urlResult != nil {
		path := urlResult.Value.Str()
		if fd.TokenCategory == "general" {
			switch {
			case strings.Contains(path, "color"):
				fd.TokenCategory = "color"
			case strings.Contains(path, "typo") || strings.Contains(path, "font"):
				fd.TokenCategory = "typography"
			case strings.Contains(path, "spac"):
				fd.TokenCategory = "spacing"
			case strings.Contains(path, "elev") || strings.Contains(path, "shadow"):
				fd.TokenCategory = "elevation"
			}
		}
	}

	// Extract tokens from tables (name/value pattern)
	tokensResult, _ := page.Eval(`() => {
		const tokens = [];
		const tables = document.querySelectorAll('table');
		for (const table of tables) {
			const headers = [...table.querySelectorAll('th')].map(th => th.textContent.trim().toLowerCase());
			const nameIdx = headers.findIndex(h => h === 'token' || h === 'name' || h === 'variable' || h === 'property' || h === 'css variable');
			const valueIdx = headers.findIndex(h => h === 'value' || h === 'hex' || h === 'rgb' || h === 'size' || h === 'px');
			const descIdx = headers.findIndex(h => h === 'description' || h === 'usage' || h === 'use');
			if (nameIdx === -1) continue;
			const rows = table.querySelectorAll('tbody tr');
			for (const row of rows) {
				const cells = row.querySelectorAll('td');
				const token = {
					name: cells[nameIdx] ? cells[nameIdx].textContent.trim() : '',
					value: valueIdx >= 0 && cells[valueIdx] ? cells[valueIdx].textContent.trim() : '',
					description: descIdx >= 0 && cells[descIdx] ? cells[descIdx].textContent.trim() : '',
				};
				if (token.name) tokens.push(token);
			}
		}
		return tokens.slice(0, 100);
	}`)
	if tokensResult != nil {
		if tList, ok := tokensResult.Value.Val().([]interface{}); ok {
			for _, t := range tList {
				if tm, ok := t.(map[string]interface{}); ok {
					fd.Tokens = append(fd.Tokens, DesignToken{
						Name:        strVal(tm, "name"),
						Value:       strVal(tm, "value"),
						Description: strVal(tm, "description"),
						Category:    fd.TokenCategory,
					})
				}
			}
		}
	}

	// Extract color swatches if this is a color page
	if fd.TokenCategory == "color" && len(fd.Tokens) == 0 {
		swatchResult, _ := page.Eval(`() => {
			const tokens = [];
			const swatches = document.querySelectorAll('[class*="swatch"], [class*="color"], [style*="background"]');
			for (const el of swatches) {
				const cs = getComputedStyle(el);
				const bg = cs.backgroundColor;
				if (!bg || bg === 'rgba(0, 0, 0, 0)' || bg === 'transparent') continue;
				const label = el.textContent.trim().substring(0, 60) || el.getAttribute('title') || el.getAttribute('aria-label') || '';
				if (label) tokens.push({ name: label, value: bg });
			}
			return tokens.slice(0, 50);
		}`)
		if swatchResult != nil {
			if sList, ok := swatchResult.Value.Val().([]interface{}); ok {
				for _, s := range sList {
					if sm, ok := s.(map[string]interface{}); ok {
						fd.Tokens = append(fd.Tokens, DesignToken{
							Name:     strVal(sm, "name"),
							Value:    strVal(sm, "value"),
							Category: "color",
						})
					}
				}
			}
		}
	}

	// Extract CSS custom properties from the page
	cssResult, _ := page.Eval(`() => {
		const cs = getComputedStyle(document.documentElement);
		const props = [];
		for (const p of cs) {
			if (p.startsWith('--')) {
				props.push(p + ': ' + cs.getPropertyValue(p).trim() + ';');
			}
		}
		return props.slice(0, 20);
	}`)
	if cssResult != nil {
		if cList, ok := cssResult.Value.Val().([]interface{}); ok {
			for _, c := range cList {
				if s, ok := c.(string); ok {
					fd.CSSSnippets = append(fd.CSSSnippets, s)
				}
			}
		}
	}

	// Extract usage guidance from paragraphs
	guidanceResult, _ := page.Eval(`() => {
		const ps = document.querySelectorAll('main p, article p, .content p');
		const texts = [];
		for (const p of ps) {
			const t = p.textContent.trim();
			if (t.length > 30 && t.length < 500) texts.push(t);
		}
		return texts.slice(0, 3).join(' ');
	}`)
	if guidanceResult != nil {
		fd.UsageGuidance = guidanceResult.Value.Str()
	}

	return fd
}

// extractPattern extracts pattern data from a page.
func extractPattern(page *rod.Page) *PatternData {
	pd := &PatternData{}

	// Extract structured sections: problem, when to use, when not to use, best practices
	sectionsResult, _ := page.Eval(`() => {
		const result = { problemSolved: '', whenToUse: '', whenNotToUse: '', bestPractices: [] };
		const headings = document.querySelectorAll('h2, h3');
		for (const h of headings) {
			const text = h.textContent.trim().toLowerCase();
			let content = '';
			let el = h.nextElementSibling;
			while (el && !['H2', 'H3'].includes(el.tagName)) {
				content += el.textContent.trim() + '\n';
				el = el.nextElementSibling;
			}
			content = content.trim().substring(0, 500);

			if (text.match(/when to use/i)) {
				result.whenToUse = content;
			} else if (text.match(/when not to use|don.?t use/i)) {
				result.whenNotToUse = content;
			} else if (text.match(/overview|about|description|problem/i)) {
				result.problemSolved = content;
			} else if (text.match(/best practice|guideline|recommendation/i)) {
				const items = [];
				let li = h.nextElementSibling;
				while (li && !['H2', 'H3'].includes(li.tagName)) {
					if (li.tagName === 'UL' || li.tagName === 'OL') {
						li.querySelectorAll('li').forEach(l => items.push(l.textContent.trim()));
					}
					li = li.nextElementSibling;
				}
				result.bestPractices = items.slice(0, 10);
			}
		}
		return result;
	}`)
	if sectionsResult != nil {
		if sm, ok := sectionsResult.Value.Val().(map[string]interface{}); ok {
			pd.ProblemSolved = strVal(sm, "problemSolved")
			pd.WhenToUse = strVal(sm, "whenToUse")
			pd.WhenNotToUse = strVal(sm, "whenNotToUse")
			pd.BestPractices = toStringSlice(sm["bestPractices"])
		}
	}

	// Extract component references (links to component pages)
	composedResult, _ := page.Eval(`() => {
		const comps = [];
		const links = document.querySelectorAll('a[href]');
		for (const a of links) {
			if (a.href.match(/\/components?\//)) {
				const t = a.textContent.trim();
				if (t.length > 0 && t.length < 40) comps.push(t);
			}
		}
		return [...new Set(comps)].slice(0, 10);
	}`)
	if composedResult != nil {
		pd.ComposedOf = toStringSlice(composedResult.Value.Val())
	}

	// Extract code snippets
	snippetsResult, _ := page.Eval(`() => {
		const snippets = [];
		const codeBlocks = document.querySelectorAll('pre code, .highlight code, [class*="language-"]');
		for (const block of codeBlocks) {
			const code = block.textContent.trim();
			if (code.length > 20 && code.length < 2000) snippets.push(code);
		}
		return snippets.slice(0, 5);
	}`)
	if snippetsResult != nil {
		pd.UsageSnippets = toStringSlice(snippetsResult.Value.Val())
	}

	// Extract variants from headings
	variantsResult, _ := page.Eval(`() => {
		const variants = [];
		const headings = document.querySelectorAll('h2, h3');
		for (const h of headings) {
			const name = h.textContent.trim();
			if (!name) continue;
			if (/^(when to use|when not|best practice|guideline|overview|about|props|api|references)/i.test(name)) continue;
			variants.push({ name });
		}
		return variants.slice(0, 15);
	}`)
	if variantsResult != nil {
		if vList, ok := variantsResult.Value.Val().([]interface{}); ok {
			for _, v := range vList {
				if vm, ok := v.(map[string]interface{}); ok {
					pd.Variants = append(pd.Variants, ComponentVariant{Name: strVal(vm, "name")})
				}
			}
		}
	}

	return pd
}

// extractContent extracts content/writing guideline data from a page.
func extractContent(page *rod.Page) *ContentData {
	cd := &ContentData{}

	// Classify content type from URL
	urlResult, _ := page.Eval(`() => window.location.pathname.toLowerCase()`)
	if urlResult != nil {
		path := urlResult.Value.Str()
		switch {
		case strings.Contains(path, "voice") || strings.Contains(path, "tone"):
			cd.ContentType = "voice-tone"
		case strings.Contains(path, "termin") || strings.Contains(path, "word"):
			cd.ContentType = "terminology"
		default:
			cd.ContentType = "writing-pattern"
		}
	}

	// Extract do/don't examples
	doDontResult, _ := page.Eval(`() => {
		const doExamples = [];
		const dontExamples = [];

		// Look for do/don't sections by heading
		const headings = document.querySelectorAll('h2, h3, h4');
		for (const h of headings) {
			const text = h.textContent.trim().toLowerCase();
			const isDo = text.match(/^do$|^do\b|✓|✅|correct/i);
			const isDont = text.match(/don.?t|✗|✘|❌|incorrect|avoid/i);
			if (!isDo && !isDont) continue;

			let el = h.nextElementSibling;
			while (el && !['H2', 'H3', 'H4'].includes(el.tagName)) {
				const t = el.textContent.trim();
				if (t) {
					if (isDo) doExamples.push(t.substring(0, 200));
					if (isDont) dontExamples.push(t.substring(0, 200));
				}
				el = el.nextElementSibling;
			}
		}

		// Also look for styled do/don't blocks (green/red)
		const doBlocks = document.querySelectorAll('[class*="do-"], [class*="correct"], [class*="positive"]');
		for (const b of doBlocks) doExamples.push(b.textContent.trim().substring(0, 200));
		const dontBlocks = document.querySelectorAll('[class*="dont"], [class*="incorrect"], [class*="negative"]');
		for (const b of dontBlocks) dontExamples.push(b.textContent.trim().substring(0, 200));

		return { doExamples: doExamples.slice(0, 10), dontExamples: dontExamples.slice(0, 10) };
	}`)
	if doDontResult != nil {
		if dm, ok := doDontResult.Value.Val().(map[string]interface{}); ok {
			cd.DoExamples = toStringSlice(dm["doExamples"])
			cd.DontExamples = toStringSlice(dm["dontExamples"])
		}
	}

	// Extract guidelines from bullet lists
	guidelinesResult, _ := page.Eval(`() => {
		const guidelines = [];
		const lists = document.querySelectorAll('main ul li, article ul li, .content ul li');
		for (const li of lists) {
			const t = li.textContent.trim();
			if (t.length > 10 && t.length < 300) guidelines.push(t);
		}
		return guidelines.slice(0, 20);
	}`)
	if guidelinesResult != nil {
		cd.Guidelines = toStringSlice(guidelinesResult.Value.Val())
	}

	// Extract terminology table if present
	if cd.ContentType == "terminology" {
		termsResult, _ := page.Eval(`() => {
			const terms = [];
			const tables = document.querySelectorAll('table');
			for (const table of tables) {
				const headers = [...table.querySelectorAll('th')].map(th => th.textContent.trim().toLowerCase());
				const termIdx = headers.findIndex(h => h === 'term' || h === 'word' || h === 'phrase');
				const useIdx = headers.findIndex(h => h === 'use' || h === 'preferred' || h === 'correct' || h === 'instead');
				const dontIdx = headers.findIndex(h => h === 'avoid' || h === 'don\'t use' || h === 'incorrect' || h === 'instead of');
				const ctxIdx = headers.findIndex(h => h === 'context' || h === 'notes' || h === 'description');
				if (termIdx === -1) continue;
				const rows = table.querySelectorAll('tbody tr');
				for (const row of rows) {
					const cells = row.querySelectorAll('td');
					terms.push({
						term: cells[termIdx] ? cells[termIdx].textContent.trim() : '',
						use: useIdx >= 0 && cells[useIdx] ? cells[useIdx].textContent.trim() : '',
						dontUse: dontIdx >= 0 && cells[dontIdx] ? cells[dontIdx].textContent.trim() : '',
						context: ctxIdx >= 0 && cells[ctxIdx] ? cells[ctxIdx].textContent.trim() : '',
					});
				}
			}
			return terms.slice(0, 50);
		}`)
		if termsResult != nil {
			if tList, ok := termsResult.Value.Val().([]interface{}); ok {
				for _, t := range tList {
					if tm, ok := t.(map[string]interface{}); ok {
						cd.WordList = append(cd.WordList, ContentTerm{
							Term:    strVal(tm, "term"),
							Use:     strVal(tm, "use"),
							DontUse: strVal(tm, "dontUse"),
							Context: strVal(tm, "context"),
						})
					}
				}
			}
		}
	}

	return cd
}

// extractIcons extracts icon data from an icon listing page.
// Returns multiple entries if the page contains an icon grid.
func extractIcons(page *rod.Page, href string, category string) []CatalogEntry {
	iconsResult, _ := page.Eval(`() => {
		const icons = [];

		// Strategy 1: Look for SVG elements with labels
		const svgEls = document.querySelectorAll('svg');
		for (const svg of svgEls) {
			const parent = svg.closest('[class*="icon"], [data-icon], li, [role="listitem"]');
			if (!parent) continue;
			const label = parent.textContent.replace(svg.textContent, '').trim() ||
			              svg.getAttribute('aria-label') ||
			              svg.getAttribute('title') ||
			              parent.getAttribute('title') || '';
			if (!label || label.length > 40) continue;
			const svgStr = svg.outerHTML;
			const viewBox = svg.getAttribute('viewBox') || '';
			const sizes = [];
			if (viewBox) {
				const parts = viewBox.split(' ');
				if (parts.length === 4) sizes.push(parts[2]);
			}
			icons.push({ name: label, svg: svgStr.substring(0, 2000), sizes });
		}

		// Strategy 2: Look for icon grid items with img tags or class-based icons
		if (icons.length === 0) {
			const items = document.querySelectorAll('[class*="icon-grid"] > *, [class*="icon-list"] > *, [class*="icons"] li');
			for (const item of items) {
				const label = item.textContent.trim();
				if (!label || label.length > 40) continue;
				const img = item.querySelector('img');
				const svg = item.querySelector('svg');
				icons.push({
					name: label,
					svg: svg ? svg.outerHTML.substring(0, 2000) : '',
					sizes: [],
				});
			}
		}

		return icons.slice(0, 200);
	}`)

	if iconsResult == nil {
		return nil
	}

	iList, ok := iconsResult.Value.Val().([]interface{})
	if !ok || len(iList) == 0 {
		return nil
	}

	var entries []CatalogEntry
	for _, item := range iList {
		im, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name := strVal(im, "name")
		if name == "" {
			continue
		}
		entries = append(entries, CatalogEntry{
			Kind: KindIcon,
			Name: name,
			Icon: &IconData{
				IconName: name,
				SVG:      strVal(im, "svg"),
				Sizes:    toStringSlice(im["sizes"]),
			},
		})
	}
	return entries
}
