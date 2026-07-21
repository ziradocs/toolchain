# SlideLang Language Specification

This document provides the formal technical specification for the SlideLang Domain-Specific Language (DSL) syntax. It is part of [Spec v0.1](README.md); for the exact, versioned shape of the AST that this syntax parses into, see the [JSON/AST contract](../../docs/architecture/json-ast-contract.md) and [`schema/ast.schema.json`](../../schema/ast.schema.json) — the TypeScript interfaces below are illustrative of the AST's general shape, not the authoritative reference.

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
variables: object
```

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
                      NEWLINE element_data

identifier ::= LETTER (LETTER | DIGIT | "_")*
value      ::= STRING | NUMBER | BOOLEAN
```

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
| `grid` | `::: grid` | Grid container for column layouts |
| `column` | `::: column` | Individual column within grid |
| `left` | `::: left` | Left column |
| `right` | `::: right` | Right column |
| `highlight` | `::: highlight` | Highlighted content |
| `code-group` | `::: code-group` | Grouped code blocks |

#### Grid and Column Layouts

Grid layouts provide flexible content organization with automatic responsive behavior:

**Basic Grid Syntax:**
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
**Tracks:** `ast.SchemaVersion` 2.0.0
**Status:** Living document — see [Spec v0.1 index](README.md) for scope and versioning policy
