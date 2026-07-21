# Rendered-output sanitization

This document describes the security measures `core`'s renderer applies to
user-provided content when generating HTML output, and how to use ZiraDocs/DocLang safely. For
how to report a security vulnerability, see [SECURITY.md](../../SECURITY.md) at the repo root.

## Security features

### 1. HTML escaping

All user-provided content is HTML-escaped before being rendered to prevent script injection:

```
& → &amp;
< → &lt;
> → &gt;
" → &quot;
' → &#39;
```

### 2. URL validation

URLs in links and images are validated to block dangerous protocols.

**Blocked protocols:** `javascript:`, `data:`, `vbscript:`, `file:`

**Allowed protocols:** `http:`, `https:`, `mailto:`, `tel:`, `ftp:`, and relative URLs (e.g.
`/path/to/resource`).

### 3. Attribute sanitization

HTML attributes (alt text, captions, data attributes) are sanitized to prevent attribute
injection: newlines/carriage returns are removed, tabs are replaced with spaces, and HTML special
characters are escaped.

### 4. Code block protection

Code blocks are HTML-escaped to prevent execution of embedded scripts while preserving syntax
highlighting — e.g. a fenced ```` ```javascript ```` block containing `<script>alert('x')</script>`
renders the text safely rather than executing it.

### 5. Renderer-authored inline formatting (fixed allowlist)

Inline formatting never passes user HTML through. The pipeline escapes the whole
string first (section 1), then injects a **fixed, renderer-authored** set of
tags via regex: `<strong>`, `<em>`, `<mark>`, `<del>`, `<code>`, and `<a>` (with
URL validation per section 2). Class-based spans (`[content]{.token}`) extend
this set with a few more renderer-authored tags — `<span class="slidelang-text-*">`,
`<mark class="slidelang-highlight-*">`, `<u>`, `<small>` — but only via a **fixed
token→output allowlist** (`inlineSpanTokens` in `renderer/sanitizer.go`). The
captured token is used solely as a map key and is **never interpolated** into a
class attribute; a token outside the allowlist (or one containing characters
other than `[a-zA-Z0-9-]`) leaves the text literal and escaped, injecting
nothing. This mirrors the `SanitizeColor`/`cssNamedColors` validate-against-a-
fixed-map model and keeps the escape-everything-then-inject-known-tags invariant
intact: the only markup the renderer can emit is from this closed set.

## What gets sanitized

Automatically protected: text elements, tables (headers and cells), lists (items and sub-items),
quotes (content/author/source), checklists, images (URL validation + attribute escaping), code
blocks, special blocks (title/content/icon), code groups (labels/content), maps (titles/marker
labels/attributes), charts (titles/labels), diagram titles, and template variables.

### Special cases

**Mermaid and PlantUML diagrams.** Diagram *content* is not HTML-escaped because these renderers
require raw source syntax. Diagram *titles* are still sanitized, and diagrams render in isolated
contexts (SVG or rasterized images via chromedp).

**Raw JSON in charts.** When using `json:` mode for charts, the JSON is validated as JSON but not
HTML-escaped, since it's consumed directly by Chart.js on the client. Only use trusted data
sources for JSON-mode charts.

## Safe usage

```slidelang
# Safe Heading

**Bold text** and *italic text* work safely.

This <script>alert('xss')</script> is escaped.
```

```slidelang
[Safe link](https://example.com)
[Blocked link](javascript:alert('xss'))  <!-- blocked -->
```

```slidelang
![Safe image](https://example.com/image.png)
![Blocked](javascript:alert('xss'))  <!-- blocked -->
```

```yaml
---
title: My Presentation
user_input: "<script>alert('xss')</script>"
---

Text: {{user_input}}  # rendered safely escaped
```

## Best practices

- **Validate untrusted input** if you're generating presentations/documents from user-submitted
  content, beyond what the renderer's escaping already covers.
- **Be mindful of variable sources.** Frontmatter/template variables are escaped automatically,
  but prefer trusted sources over raw unvalidated user input.
- **Keep dependencies up to date** — pull the latest `slidelang`/`doclang` release to get
  security fixes.
- **Review generated HTML** for sensitive presentations/documents before publishing it externally.

## Implementation

The sanitizer lives in `core/renderer/sanitizer.go` and is used by both the shared
document renderer (`renderer/document_html.go`, used by `doclang`) and slidelang's own slide
generator (`slidelang/internal/generator/`). New rendered output should always go through it.

## Further reading

- [OWASP XSS Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross_Site_Scripting_Prevention_Cheat_Sheet.html)
- [Content Security Policy Reference](https://content-security-policy.com/)
- [Go HTML Template Security](https://pkg.go.dev/html/template)
