# SlideLang Language Specification

This document provides the formal technical specification for the SlideLang Domain-Specific Language (DSL) syntax. It is part of [Spec v0.1](README.md); for the exact, versioned shape of the AST that this syntax parses into, see the [JSON/AST contract](https://ziradocs.com/docs/architecture/json-ast-contract/) and [`schema/ast.schema.json`](../../schema/ast.schema.json) — the TypeScript interfaces below are illustrative of the AST's general shape, not the authoritative reference.

## Language Overview

SlideLang is a presentation markup language that supports two syntax modes:
- **Strict Mode**: Keyword-driven, structured syntax
- **Flex Mode**: Markdown-extended syntax with embedded elements

## 📋 **Formal Grammar**

### Common Elements

All SlideLang documents begin with optional YAML frontmatter:

```ebnf
presentation ::= frontmatter? slide+
frontmatter  ::= "---" yaml_content "---"
```

#### Frontmatter Schema
```yaml
mode: "strict" | "flex" | "flex-full" | "auto"  # "flex-ai" is a deprecated alias for "flex-full"
title: string
author: string  
date: string
theme: string
lang: string  # BCP 47 language tag (e.g. "es", "en-US", "zh-Hans-CN"); no top-level default
numbering: boolean  # DocLang only; see below — no top-level default
variables: object
```

`lang` declares the document's *default* language. A passage in a different language can be
marked inline, in flex-mode prose, with a pandoc-style span: `[texto]{lang=xx}` (e.g.
`[bonjour]{lang=fr}` renders `<span lang="fr">bonjour</span>`). `xx` must be a well-formed
BCP 47 tag (`core/a11y.IsValidLangTag`); a malformed tag degrades to literal text rather than
emitting an invalid attribute. See `core/renderer.InlineLangSpanPattern` for the implementation
and `core/ast.LangRun` for how a marked passage is exposed on the AST — as of SchemaVersion 2.4.0,
`langRuns` is populated on `TextElement`, `PointItem`, `ChecklistItem`, `QuoteElement.Content`,
`SpecialBlockElement.Content`, `GridElement.Content`, and `ColumnElement.Content`.

`numbering` (as of SchemaVersion 2.5.0, `ast.FrontMatterNode.Numbering`) is a tri-state
override for DocLang's section auto-numbering: `true`/`false` for an explicit opinion, or
the key omitted entirely for "no opinion," which is distinct from `false` — `doclang build`'s
`--numbering`/`--numbering=false` CLI flag wins over either. It has no effect on `.slidelang`
output; section numbering is a DocLang-only concept. The frontmatter parser also accepts a
legacy map form (`numbering:\n  enabled: true`) for backward compatibility with older
`doclang init` templates — see `llm-kit/reference/frontmatter.md` for the full resolution
order and both accepted shapes.

### Strict Mode Grammar

```ebnf
presentation ::= frontmatter? slide+
slide        ::= slide_type property* element*
slide_type   ::= "SLIDE" identifier
property     ::= identifier ":" value
element      ::= text_element | points_element | image_element | 
                 code_element | table_element | directive_element |
                 special_block | embedded_element

text_element   ::= "TEXT" INDENT content_lines DEDENT
points_element ::= "POINTS" INDENT point_item+ DEDENT  
point_item     ::= "-" text_content NEWLINE
image_element  ::= "IMAGE" INDENT property+ DEDENT
code_element   ::= "CODE" INDENT code_content DEDENT
table_element  ::= "TABLE" INDENT table_data DEDENT

directive_element ::= "@" directive_name ":" directive_value
special_block     ::= ":::" block_type NEWLINE block_content ":::"
embedded_element  ::= "<<" element_type (":" element_subtype)? ">>"
                      NEWLINE element_data element_terminator
element_terminator ::= "<<end>>" | block_boundary | EOF

identifier ::= LETTER (LETTER | DIGIT | "_")*
value      ::= STRING | NUMBER | BOOLEAN
```

`element_data` for an `embedded_element` is **not** delimited line-by-line —
it runs until whichever `element_terminator` comes first: an explicit
`<<end>>`, the next top-level `block_boundary` (a `SLIDE`/`SECTION` keyword
at column 0, closing the element without consuming it), or end of file. This
is what lets `<<mermaid>>`, `<<plantuml>>`, `<<chart:type>>`, and `<<map>>`
omit a closing tag by convention (see "Embedded Elements" below) without an
unclosed block silently absorbing the rest of the document — each reaches
this guarantee through its own parser (indentation tracking, an explicit
allowlist, or a direct boundary check), not a single shared mechanism, but
the outcome the grammar promises is the same for all four.

### Document Strict Mode Grammar

The grammar above describes *presentations*, whose unit is the slide. A **document**
(`.doclang`) in strict mode has the same element vocabulary and the same indentation rule,
but its unit is the **section**:

```ebnf
document     ::= frontmatter? section+
section      ::= "SECTION" quoted_string NEWLINE INDENT section_property* element* DEDENT
section_property ::= ("level" ":" level_value | "id" ":" identifier)
level_value  ::= "1" | "2" | "3" | "4" | "5" | "6"

quoted_string ::= '"' character* '"'
```

Differences from the presentation grammar, all deliberate:

- **The title is part of the opening line** and must be quoted (`SECTION "Introduction"`),
  rather than being a `title:`/`heading:` property. A section has exactly one title, so
  giving it two possible homes would create two sources of truth. The `title`, `heading`,
  `subtitle` and `logo` properties are SLIDE-only and are rejected inside a `SECTION`.
- **Properties are never inline.** `SECTION "Intro" level: 2` is an error; properties go on
  indented lines below the opening line, exactly as in a `SLIDE`.
- **`level:` declares the hierarchy, indentation does not.** Sections are never nested
  syntactically; a `SECTION` indented under another one is an error, not a subsection.
- **`id:` is only accepted on levels 2-6.** It overrides the anchor that would otherwise be
  derived from the section's title, so a reference survives a title change. A level-1 section
  maps to a `ContentBlock`, which has no id field in the AST, so accepting one there would
  be accepting-and-ignoring. The value is **normalized to anchor form** — lowercased, spaces
  to hyphens, then narrowed to `[a-z0-9_-]` — and that normalized form is canonical: it is
  the only form stored in the AST, so it is also what a formatter emits. The normalization
  is idempotent, which is what makes that round-trip stable. An `id:` with no surviving
  characters (say, only emoji) is an error rather than an empty anchor.
- `numbered:` and `pagebreak:`, which appeared in early design sketches of this dialect, are
  **not** part of the grammar: no renderer implements per-section numbering or page breaks,
  and a property that parses but does nothing is worse than one that errors.

**AST shape.** A level-1 `SECTION` opens a `ContentBlock` — the first one is the document's
`title` block (its text lands in `Heading`), the rest are `content` blocks (text in `Title`),
the same positional rule the flex dialect uses. Levels 2-6 are **not** blocks: they become
`<hN id="…">` heading elements inside the currently open block, carrying their depth in
`TextElement.Level`. This mirrors what `#`/`##` produce in flex, which is what lets the
document renderer, the TOC generator and the transform stages (`xref`, numbering) consume
either dialect without a single per-dialect branch.

Unlike flex — where a `#` with no content is treated as stray Markdown and dropped — a
declared `SECTION` is always kept, empty or not. The author wrote it on purpose.

**Normalization never runs on a strict document**, in either dialect. That is the property
the mode exists to provide: what you read is what gets parsed.

### Flex Mode Grammar

Flex mode extends CommonMark Markdown with SlideLang-specific elements:

```ebnf
presentation   ::= frontmatter? slide+
slide         ::= slide_content ("---" | EOF)
slide_content ::= (markdown_element | slidelang_extension)*

slidelang_extension ::= directive | special_block | embedded_element
directive          ::= "@" directive_name ":" directive_value
special_block      ::= ":::" block_type NEWLINE block_content NEWLINE ":::"  
embedded_element   ::= "<<" element_type (":" element_subtype)? ">>"
                       NEWLINE element_data

markdown_element ::= heading | paragraph | list | code_block | 
                     image | table | blockquote
```

## 🔧 **Data Type Specifications**

### AST Node Types

#### Base Node
```typescript
interface BaseNode {
  type: NodeType
  position: Position
  endPosition: Position
  comments?: string[]
}

interface Position {
  line: number
  column: number
}
```

#### Presentation Node
```typescript
interface PresentationNode extends BaseNode {
  type: "presentation"
  frontMatter?: FrontMatterNode
  slides: SlideNode[]
  filePath?: string
}
```

#### Slide Node  
```typescript
interface SlideNode extends BaseNode {
  type: "slide"
  slideType: string
  title?: string
  elements: ElementNode[]
  notes: string[]
  properties: Record<string, any>
}
```

#### Element Nodes
```typescript
interface ElementNode extends BaseNode {
  content: string | object
  properties: Record<string, any>
}

interface TextElement extends ElementNode {
  type: "text"
  content: string
}

interface PointsElement extends ElementNode {
  type: "points"
  items: PointItem[]
}

interface PointItem extends BaseNode {
  content: string
  nestedItems?: PointItem[]
}

interface CodeElement extends ElementNode {
  type: "code"
  content: string
  language?: string
}

interface ImageElement extends ElementNode {
  type: "image"
  source: string
  caption?: string
  alt?: string
}

interface ChartElement extends ElementNode {
  type: "chart"
  chartType: "bar" | "line" | "pie" | "combo" | "scatter" | "radar"
  data: ChartData
  configuration: ChartConfig
}

interface MermaidElement extends ElementNode {
  type: "mermaid"
  diagramType: string
  content: string
}

interface TableElement extends ElementNode {
  type: "table"
  headers: string[]
  rows: string[][]
  caption?: string
}

interface SpecialBlockNode extends ElementNode {
  type: "special_block"
  blockType: string
  content: string | ElementNode[]
}

interface GridElement extends ElementNode {
  type: "grid"
  columns: ColumnElement[]
}

interface ColumnElement extends ElementNode {
  type: "column"
  content: ElementNode[]
}
```

#### Directive Node
```typescript
interface DirectiveNode extends BaseNode {
  type: "directive"
  name: string
  parameters: Record<string, any>
}
```

## 📝 **Syntax Elements**

### Variables and Expressions

Both modes support variable substitution:
```
{{ variable_name }}
${expression}
{{ variable | filter:argument }}
```

**Supported Filters:**
- `currency:code` - Format as currency
- `date:format` - Date formatting
- `upper` - Uppercase
- `lower` - Lowercase
- `title` - Title case

### Comments

```slidelang
// Single-line comment (both modes)
```

### Directives

Directives control slide and element behavior:

| Directive | Syntax | Description |
|-----------|--------|-------------|
| `@notes` | `@notes: content` | Presenter notes |
| `@background` | `@background: color\|image` | Slide background |
| `@transition` | `@transition: type` | Slide transition |
| `@layout` | `@layout: layout_name` | Slide layout |
| `@timer` | `@timer: seconds` | Slide timing |
| `@reveal` | `@reveal: animation` | Element reveal animation |

### Special Blocks

Special blocks provide structured content:

| Block Type | Usage | Description |
|------------|-------|-------------|
| `info` | `::: info` | Information callout |
| `warning` | `::: warning` | Warning callout |
| `success` | `::: success` | Success callout |
| `danger` | `::: danger` | Danger callout |
| `tip` | `::: tip` | Tip callout |
| `left` | `::: left` | Left column |
| `right` | `::: right` | Right column |
| `highlight` | `::: highlight` | Highlighted content |
| `code-group` | `::: code-group` | Grouped code blocks |

Grid and column are **not** special blocks — despite sharing the `:::` sigil
in flex mode, they parse into their own typed `GridElement`/`ColumnElement`,
not a generic special block, and have a different strict-mode spelling. See
"Grid and Column Layouts" below.

#### Grid and Column Layouts

Grid layouts provide flexible content organization with automatic responsive
behavior. **The spelling is mode-specific — this is the one place strict and
flex genuinely disagree on syntax for the same element:**

**Flex** uses the `:::`-delimited form, sharing its sigil with special blocks:
```slidelang
::: grid
::: column
Content for first column
:::
::: column
Content for second column
:::
:::
```

**Strict** uses the `<<...>>` delimited-block form, consistent with the
other strict embedded elements (`<<mermaid>>`, `<<chart>>`, `<<map>>`), with
`<<column>>` introducing each column and `<<end>>` closing the block:
```slidelang
<<grid>>
<<column>>
Content for first column
<<column>>
Content for second column
<<end>>
```

The flex `::: grid` / `::: column` form is a syntax error in strict mode —
it is not recognized, translated, or accepted there. Both forms produce the
same typed `GridElement`/`ColumnElement` pair.

**Features:**
- Automatic equal-width columns
- Responsive breakpoints (collapses to single column on mobile)
- Support for nested content (lists, text, images, etc.)
- CSS Grid implementation with `.slidelang-grid` and `.slidelang-grid-cols-*` classes

**Use Cases:**
- Before/after comparisons
- Feature comparisons
- Multi-step processes
- Organized information display

### Embedded Elements

Embedded elements add rich content:

| Element | Syntax | Description |
|---------|--------|-------------|
| Charts | `<<chart: type>>` | Data visualizations |
| Diagrams | `<<mermaid>>` | Mermaid diagrams |
| Maps | `<<map>>` | Geographic maps |
| Grid | `<<grid>>` / `<<column>>` / `<<end>>` | Column layouts — see "Grid and Column Layouts" above |

## 🔍 **Validation Rules**

### Structural Validation

1. **Slide Requirements:**
   - Each slide must have at least one element or a title
   - Slide types must be valid identifiers
   - Column layouts require balanced left/right blocks
   - Grid containers must contain at least one column element

2. **Element Validation:**
   - Chart elements must have valid data structure
   - Image elements must have valid source paths
   - Code elements should specify language for highlighting
   - Grid blocks must contain only column elements as direct children
   - Column elements should only be used within grid containers

3. **Directive Validation:**
   - Required parameters must be present
   - Parameter values must match expected types
   - Directive placement must be contextually appropriate

### Semantic Validation

1. **Variable Resolution:**
   - All variable references must be defined
   - Variable types must match usage context
   - Filter parameters must be valid

2. **Reference Validation:**
   - Theme references must exist
   - Layout references must be valid
   - Asset paths must be accessible

## 🎨 **Layout Specifications**

### Specialized Layouts

SlideLang provides 17+ predefined layouts:

**Impact Layouts:**
- `hero` - Full-screen impact slide
- `testimonial` - Customer testimonial
- `call_to_action` - Action-driving slide

**Business Layouts:**
- `stats` - Statistics presentation
- `dashboard` - Data dashboard
- `pricing` - Pricing table
- `comparison` - Feature comparison

**Technical Layouts:**
- `code_example` - Code demonstration
- `feature_showcase` - Feature highlights
- `process` - Process flow

**Corporate Layouts:**
- `team` - Team introductions
- `timeline` - Timeline visualization
- `before_after` - Transformation stories

### Layout Syntax

**Strict Mode:**
```slidelang
SLIDE content
  layout: "hero"
  
  TEXT
    Main content here
```

**Flex Mode:**
```markdown
---
layout: hero
---
# Slide Title

Main content here
```

## 📊 **Output Specifications**

### HTML Structure

Generated presentations follow this structure:
```html
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>{{presentation.title}}</title>
  <link rel="stylesheet" href="theme.css">
</head>
<body>
  <div class="presentation">
    <section class="slide" data-slide-type="{{type}}">
      <!-- Slide content -->
    </section>
  </div>
  
  <script type="application/json" id="slidelang-metadata">
    {
      "title": "{{title}}",
      "slides": [...]
    }
  </script>
  
  <script src="presentation.js"></script>
</body>
</html>
```

### CSS Classes

**Core Classes:**
- `.presentation` - Main container
- `.slide` - Individual slide
- `.slide-title` - Slide title
- `.slide-content` - Slide content area
- `.element` - Generic element
- `.text-element` - Text content
- `.points-element` - Bullet points
- `.code-element` - Code blocks
- `.image-element` - Images
- `.chart-container` - Chart wrapper
- `.special-block` - Special block wrapper

**Layout Classes:**
- `.layout-{name}` - Applied to slides with specific layouts
- `.two-column` - Two-column layout
- `.full-width` - Full-width content

## 🔧 **Extension Points**

### Custom Elements

Parsers can be extended with custom element types:
```typescript
interface CustomElementParser {
  canParse(element: ElementNode): boolean
  parse(element: ElementNode): ParsedElement
  validate(element: ParsedElement): ValidationResult
  render(element: ParsedElement): string
}
```

### Custom Directives

New directives can be registered:
```typescript
interface DirectiveHandler {
  name: string
  parameters: ParameterSchema[]
  apply(context: SlideContext, parameters: any): void
}
```

### Theme Extensions

Themes can extend base functionality:
```json
{
  "name": "custom-theme",
  "extends": "default",
  "customElements": [...],
  "customDirectives": [...],
  "assets": {...}
}
```

## 📝 **Compliance and Standards**

### Web Standards
- HTML5 semantic markup
- WCAG 2.1 accessibility guidelines
- Progressive Web App capabilities
- Mobile-responsive design

### Markdown Compatibility
- CommonMark specification compliance (Flex mode)
- GitHub Flavored Markdown extensions
- Standard image and link syntax

### Data Format Standards
- YAML 1.2 for frontmatter
- JSON for embedded data
- CSS3 for styling
- JavaScript ES2020 for interactivity

---

**Spec version:** v0.1
**Tracks:** `ast.SchemaVersion` 2.5.0
**Status:** Living document — see [Spec v0.1 index](README.md) for scope and versioning policy
