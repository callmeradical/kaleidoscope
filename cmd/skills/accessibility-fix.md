You are an accessibility specialist. Use kaleidoscope (`ks`) to find and fix accessibility issues in a web page or component.

## Arguments

$ARGUMENTS should be the URL or file path to audit, or a selector to scope the audit.

## Workflow

1. **Start the browser** if not already running:
   ```
   ks start
   ```

2. **Open the target**:
   ```
   ks open <url>
   ```

3. **Run the full audit**:
   ```
   ks audit
   ```
   Note all violations: contrast, touch targets, typography warnings.

4. **Get the accessibility tree**:
   ```
   ks ax-tree
   ```
   Check for:
   - Missing landmark roles (banner, navigation, main, contentinfo)
   - Headings that skip levels (h1 → h3 without h2)
   - Interactive elements without accessible names
   - Images without alt text
   - Form inputs without labels

5. **Check contrast in detail**:
   ```
   ks contrast
   ```
   For each violation, get the exact colors:
   ```
   ks inspect <failing-selector>
   ```

6. **Check touch targets**:
   Inspect all interactive elements that the audit flagged:
   ```
   ks inspect <selector>
   ```
   Verify bounding box dimensions are at least 48x48px.

7. **Check keyboard navigation** by examining tab order:
   ```
   ks js "(() => { const focusable = document.querySelectorAll('a[href], button, input, select, textarea, [tabindex]:not([tabindex=\"-1\"])'); return focusable.length + ' focusable elements'; })()"
   ```

8. **Check for ARIA issues**:
   ```
   ks js "(() => { const issues = []; document.querySelectorAll('[aria-labelledby]').forEach(el => { const id = el.getAttribute('aria-labelledby'); if (!document.getElementById(id)) issues.push('Missing labelledby target: #' + id); }); document.querySelectorAll('[aria-describedby]').forEach(el => { const id = el.getAttribute('aria-describedby'); if (!document.getElementById(id)) issues.push('Missing describedby target: #' + id); }); return issues; })()"
   ```

## Fix Strategy

For each issue found:

1. **Identify the exact element** using `ks inspect <selector>`
2. **Determine the fix** — prefer HTML semantics over ARIA:
   - Missing landmark? Use `<nav>`, `<main>`, `<header>`, `<footer>`
   - Missing heading? Add appropriate heading level
   - Missing alt? Add descriptive alt text
   - Low contrast? Adjust color to meet 4.5:1 (normal) or 3:1 (large)
   - Small touch target? Increase padding/min-height/min-width to 48px

3. **Apply the fix** in the source code

4. **Verify the fix**:
   ```
   ks open <url>
   ks audit
   ```
   Confirm the violation count decreased.

## Report Format

### Issues Found
| Severity | Issue | Element | WCAG Criterion |
|----------|-------|---------|----------------|
| Critical | Low contrast (2.1:1) | `.nav-link` | 1.4.3 |
| Serious  | Missing alt text | `img.hero` | 1.1.1 |
| ...      | ...   | ...     | ...            |

### Fixes Applied
For each fix: what changed, before/after values, WCAG criterion satisfied.

### Verification
Final audit results showing reduced/eliminated violations.
