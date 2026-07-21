# Inline Formatting

ZiraDocs supports a rich set of inline formatting options to style your text and add various elements directly within your content. This works in both [Strict Mode](strict-mode.md) and [Flex Mode](flex-mode.md), though the syntax may vary slightly.

## Basic Text Formatting

### Standard Markdown
All standard Markdown formatting is supported:

```markdown
**bold text** and *italic text* and ***bold italic***
~~strikethrough text~~
`inline code`
```

**Result:**
- **bold text** and *italic text* and ***bold italic***
- ~~strikethrough text~~
- `inline code`

### Extended Formatting
ZiraDocs extends standard Markdown with a highlight syntax:

```markdown
==highlighted text==
```

**Result:**
- ==highlighted text== → rendered as `<mark>highlighted text</mark>`

> **Accuracy note.** The renderer's inline pipeline currently implements a
> **fixed** set of formats: `**bold**`, `*italic*`, `***bold italic***`,
> `~~strikethrough~~`, `==highlight==`, `` `code` ``, `[text](url)` links, and
> the class-based spans documented below. Superscript (`^x^`), subscript
> (`~x~`), keyboard keys (`++x++`), footnotes, definition lists, abbreviations,
> emoji shortcodes, and reference-style links are **not** transformed today —
> the characters are HTML-escaped and rendered literally. Sections further down
> that show those syntaxes describe aspirational/planned behavior, not current
> output; they are kept for roadmap context and will be reconciled separately.
> For colored, highlighted, underlined, or resized inline text, use the
> **class-based spans** below.

## Class-Based Text Spans

ZiraDocs supports **pandoc-style bracketed spans** to apply a small, fixed
palette of semantic styles to inline text:

```markdown
[content]{.token}
```

The `content` is still fully inline-processed (so `[**bold** text]{.danger}`
keeps its bold) and still HTML-escaped. The `.token` is validated against a
**fixed allowlist**; the renderer emits a hard-coded tag for each known token
and **never interpolates the token into a class attribute**. An unknown or
malformed token (anything not in the table below, or a token containing spaces,
quotes, or other non-`[a-zA-Z0-9-]` characters) is left **literal and
inert** — no markup is injected. This preserves ZiraDocs's core sanitization
invariant: user content is always escaped, and only renderer-authored tags from
a closed set are ever added.

The `[...]{.token}` delimiter (bracket + **brace**) does not collide with the
link syntax `[text](url)` (bracket + **parenthesis**).

### Token palette

| Token | Rendered as | Purpose |
|-------|-------------|---------|
| `danger`  | `<span class="slidelang-text-danger">…</span>`   | Red / error-toned text |
| `info`    | `<span class="slidelang-text-info">…</span>`     | Blue / informational text |
| `success` | `<span class="slidelang-text-success">…</span>`  | Green / positive text |
| `warning` | `<span class="slidelang-text-warning">…</span>`  | Amber / caution text |
| `accent`  | `<span class="slidelang-text-accent">…</span>`   | Violet / accent text |
| `highlight-warning` | `<mark class="slidelang-highlight-warning">…</mark>` | Amber-tinted highlight |
| `highlight-info`    | `<mark class="slidelang-highlight-info">…</mark>`    | Blue-tinted highlight |
| `highlight-success` | `<mark class="slidelang-highlight-success">…</mark>` | Green-tinted highlight |
| `underline` | `<u>…</u>` | Underlined text |
| `small`   | `<small class="slidelang-text-small">…</small>` | Smaller text (0.875em) |
| `large`   | `<span class="slidelang-text-large">…</span>`   | Larger, semi-bold text (1.25em) |

### Examples

```markdown
This is [critical]{.danger} and this is [a note]{.highlight-warning}.
You can [**combine**]{.danger} spans with other inline formatting.
Fine print goes [here]{.small}, and [KEY POINTS]{.large} can stand out.
An [unknown]{.mystery} token renders literally: [unknown]{.mystery}.
```

**Styling.** The CLI ships reference CSS for every class (see
`slidelang` CSS assets). Every color is overridable per theme via CSS
custom properties with accessible neutral fallbacks, e.g.:

```css
.slidelang-text-danger { color: var(--slidelang-danger-color, #dc2626); }
```

## Links and References

### Basic Links
```markdown
[Link text](https://example.com)
[Link with title](https://example.com "Hover title")
```

### Images
```markdown
![Alt text](./images/photo.jpg)
![Alt text](./images/photo.jpg "Image title")
```

### Image Links
Combine images and links:
```markdown
[![Image alt text](./images/thumbnail.jpg)](https://example.com)
```

### Footnotes
Add footnotes for additional context:
```markdown
This is text with a footnote[^1] and another one[^2].

[^1]: This is the first footnote content
[^2]: Second footnote with a [link](https://example.com)
```

### Reference Links
Use reference-style links for cleaner text:
```markdown
Check out [ZiraDocs][slidelang] and [GitHub][gh].

[slidelang]: https://ziradocs.com
[gh]: https://github.com
```

## Lists and Tasks

### Task Lists
Create interactive checklists:
```markdown
- [x] Completed task
- [ ] Pending task  
- [~] In progress task
```

### Definition Lists
Define terms clearly:
```markdown
ZiraDocs
:   A presentation language optimized for AI generation

Markdown
:   A lightweight markup language for formatting text
```

## Mathematical Expressions

### Inline Math
Use single dollar signs for inline mathematical expressions:
```markdown
The formula is $E = mc^2$ where E is energy.
```

### Block Math
Use double dollar signs for centered mathematical blocks:
```markdown
$$
\sum_{i=1}^{n} x_i = \frac{a + b}{c}
$$
```

**Math Examples:**
- **Inline:** The quadratic formula is $x = \frac{-b \pm \sqrt{b^2-4ac}}{2a}$
- **Block equations:** For complex formulas, use block notation

## Tables with Alignment

Create tables with custom alignment:

```markdown
| Left Aligned | Center Aligned | Right Aligned |
|:-------------|:--------------:|--------------:|
| Text         | Text           | Text          |
| More data    | Centered       | Right-aligned |
```

**Result:**

| Left Aligned | Center Aligned | Right Aligned |
|:-------------|:--------------:|--------------:|
| Text         | Text           | Text          |
| More data    | Centered       | Right-aligned |

## Special Elements

### Keyboard Shortcuts
Display keyboard combinations:
```markdown
Press ++ctrl+alt+del++ to restart
Use ++cmd+c++ to copy on Mac
```

### Abbreviations
Define abbreviations with hover tooltips:
```markdown
HTML is widely used on the web.
CSS makes it look good.

*[HTML]: HyperText Markup Language
*[CSS]: Cascading Style Sheets
```

### Emojis
Use emoji codes or direct Unicode:
```markdown
:smile: :heart: :rocket: :thumbsup:
Or directly: 😊 ❤️ 🚀 👍
```

## HTML Integration

> **Accuracy note — raw HTML is NOT passed through.** ZiraDocs's renderer
> escapes **all** user-supplied HTML as a security invariant (see
> [Sanitization architecture](../architecture/sanitization.md)). Writing
> `<mark>…</mark>`, `<span style="color: red;">…</span>`, `<kbd>…</kbd>`, or any
> other raw tag produces **escaped, literal text** (`&lt;mark&gt;…`), not
> rendered markup. The examples below describe an aspirational "safe HTML"
> capability that the current implementation does **not** provide.
>
> To get the same results **safely**, use the renderer-authored syntax instead:
>
> | Instead of raw HTML | Use |
> |---------------------|-----|
> | `<mark>text</mark>` | `==text==` or `[text]{.highlight-warning}` |
> | `<span style="color: red;">text</span>` | `[text]{.danger}` |
> | `<u>text</u>` | `[text]{.underline}` |
> | small / large text via inline styles | `[text]{.small}` / `[text]{.large}` |
>
> See [Class-Based Text Spans](#class-based-text-spans) for the full token
> palette. Raw HTML passthrough is intentionally not supported and is not on the
> roadmap.

## Complete Example

Here's a comprehensive example showing multiple formatting options:

```markdown
# Advanced Text Formatting Demo

## Basic Formatting
This presentation covers **essential concepts** in *modern web development*, 
including ~~outdated practices~~ and ==key highlights==.

## Mathematical Content  
The algorithm complexity is O(n^2^) where n represents the input size.
For detailed calculations: $complexity = \frac{n!}{(n-k)!}$

## Reference Material
Check our documentation[^1] and community guidelines[^2].

### Tasks for Today
- [x] Review formatting options
- [ ] Create sample presentation
- [~] Test with live audience

### Technical Terms
API
:   Application Programming Interface

REST
:   Representational State Transfer

## Quick Actions
- Copy: ++ctrl+c++
- Paste: ++ctrl+v++ 
- Save: ++ctrl+s++

**Remember:** Practice makes perfect! 🚀

[^1]: Available at [docs.ziradocs.com](https://docs.ziradocs.com)
[^2]: See our [GitHub community](https://github.com/ziradocs)
```

## Mode-Specific Considerations

### Strict Mode
In [Strict Mode](strict-mode.md), inline formatting is used within `TEXT` blocks:

```yaml
SLIDE: content
TEXT: "This is **bold** and *italic* text with `code`"
```

### Flex Mode  
In [Flex Mode](flex-mode.md), use inline formatting directly in Markdown:

```markdown
# My Slide

This is **bold** and *italic* text with `code`.
```

## Best Practices

1. **Be Consistent** - Use the same formatting patterns throughout your presentation
2. **Don't Overformat** - Too much formatting can be distracting
3. **Test Math Rendering** - Complex mathematical expressions should be tested
4. **Use Semantic HTML** - Prefer semantic HTML elements over styling
5. **Check Accessibility** - Ensure formatted text remains readable

## Performance Notes

- **Math Rendering** - Mathematical expressions require additional processing time
- **Image Links** - External images may slow down presentation loading
- **Custom HTML** - Excessive HTML styling can impact performance

## Related Documentation

- [Strict Mode Syntax](strict-mode.md) - How formatting works in strict mode
- [Flex Mode Syntax](flex-mode.md) - Markdown-style formatting  
- [Special Blocks](special-blocks.md) - Block-level formatting elements
- [Advanced Elements](advanced-elements.md) - Charts, diagrams, and interactive content
- [Variables & Templates](../features/variables-templates.md) - Dynamic text formatting

## Migration Notes

If migrating from other tools:
- **PowerPoint/Keynote:** Most text formatting translates directly to Markdown
- **LaTeX:** Mathematical expressions use standard LaTeX syntax
- **HTML:** Most HTML formatting is preserved, but may need adjustment for slide output
