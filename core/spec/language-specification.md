# SlideLang Language Specification

This document provides the formal technical specification for the SlideLang Domain-Specific Language (DSL) syntax. It is part of [Spec v0.1](README.md); for the exact, versioned shape of the AST that this syntax parses into, see the [JSON/AST contract](https://ziradocs.com/docs/architecture/json-ast-contract/) and [`schema/ast.schema.json`](../../schema/ast.schema.json) — the TypeScript interfaces below are illustrative of the AST's general shape, not the authoritative reference.

## Language Overview

SlideLang is a presentation markup language that supports two syntax modes:
- **Strict Mode**: Keyword-driven, structured syntax
- **Flex Mode**: Markdown-extended syntax with embedded elements

### Explicit node identities (AST schema 2.14.0)

In either mode, a standalone `<!-- node-id: Name -->` line attaches `Name` to
the AST node beginning on the next nonblank line. The annotation may precede
a slide, section, heading, or typed element, including a typed nested element.
It does not attach to frontmatter, delimiters, or text inside code/diagrams.
The name is case sensitive, document-wide unique, 1–128 ASCII characters,
starts with a letter, and then uses letters, digits, `.`, `_`, or `-`.

`nodeId` is an optional editorial identity and does not replace the existing
`id`/`label` fields used for cross references or HTML anchors. A duplicate
node needs a fresh explicit ID. Invalid, duplicate, orphan, or ambiguous
annotations are errors. No ID is generated from content or source position.
See [the identity contract](../../docs/portable-node-identities.md).

### AST capabilities

Extensions to the JSON AST are versioned by capability. A document declares
exactly the capabilities it uses, and its `schemaVersion` is the version of
the newest one; a document that uses none stays at 2.14.0. Whether the author
has to opt in follows one rule:

- **New syntax infers its capability.** Syntax that did not exist before
  (`tableRows:`, `poster=`/`caption=` on media, a filename on `CODE`,
  `results:`/`responses:` on quiz and poll) cannot change the meaning of an
  existing document, so using it is the opt-in: `table-rows-v1`,
  `media-figure-v1`, `code-filename-v1` and `quiz-poll-results-v1` are never
  written in frontmatter, and declaring them there is an error.
- **Changing the meaning of existing syntax needs an explicit opt-in.** When the
  same source would produce a different AST (typed child list types, typed
  headings), the author declares it with `ast_capabilities` in frontmatter:
  `nested-list-types-v1` and `typed-headings-v1`. Without the declaration the
  source keeps its previous meaning.

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
watermark: string | object  # repeating overlay, both CLIs — see llm-kit/reference/frontmatter.md
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
element      ::= text_element | points_element | checklist_element |
                 quote_element | image_element | code_element | table_element |
                 heading_element | directive_element | special_block |
                 embedded_element

text_element      ::= "TEXT" INDENT content_lines DEDENT
heading_element   ::= "SECTION" quoted_string NEWLINE
                      (INDENT heading_property+ DEDENT)?
heading_property  ::= "level" ":" ("3" | "4" | "5" | "6") | "id" ":" identifier
points_element    ::= "POINTS" INDENT point_item+ DEDENT
point_item        ::= ("-" | "*" | "+" | DIGIT+ ".") text_content NEWLINE
checklist_element ::= "CHECKLIST" INDENT checklist_item+ DEDENT
checklist_item     ::= ("-" | "*" | "+")? "[" ("x" | "X" | " ") "]" text_content NEWLINE
image_element     ::= "IMAGE" INDENT property+ DEDENT
quote_element     ::= "QUOTE" INDENT quote_line (BLANK* quote_line)*
                      ("AUTHOR:" text)? ("SOURCE:" text)? DEDENT
code_element      ::= "CODE" "{verbatim}"? (language (filename | info)?)? INDENT code_content DEDENT
table_element     ::= "TABLE" INDENT table_data DEDENT

directive_element ::= "@" directive_name ":" directive_value
special_block     ::= ":::" block_type ("{" attributes "}")? title? NEWLINE block_content ":::"
embedded_element  ::= "<<" element_type (":" element_subtype)? ">>"
                      NEWLINE element_data element_terminator
element_terminator ::= "<<end>>" | block_boundary | EOF

identifier ::= LETTER (LETTER | DIGIT | "_")*
value      ::= STRING | NUMBER | BOOLEAN
```

With frontmatter `ast_capabilities: [nested-list-types-v1]`, a `point_item`
may own an indented list of child `point_item`s at any depth. Each child
list has one marker type, ordered or unordered. The AST records that type on
its parent as `subListType`; mixed ordered/unordered siblings in one list are
diagnosed. See [portable nested list types](../../docs/portable-nested-list-types.md).

Without that declaration the parser does not keep the depth of a nested list.
Every item indented deeper than the first item of the list is a `subPoints`
entry of the last item at the base level, so a third level ends up next to the
second one, as a sibling. The kind of list drawn for those `subPoints` (bullets
or numbers) is the marker of the first of them, even when later ones were
written with the other marker. None of this is serialized differently: the JSON
AST has no marker per item. `slidelang fmt` and `doclang fmt` write each
sub-point with the marker the author used (the parser keeps it in memory only),
so a mixed sublist keeps its markers, but a third level comes back indented as
a sibling of the second, which builds to the same AST.

`CODE typescript renewals.ts` records `renewals.ts` as the block's filename
(`CodeElement.filename`, schema 2.19.0, inferred capability
`code-filename-v1`); a flex fence takes it after the language or as
`title="…"`. See [code block filenames](../../docs/portable-code-filenames.md).

The body of a `CODE` block is every line indented deeper than its `CODE` line,
blank lines included. The indentation that structures the body is the whitespace
that all of its non-blank lines have in common (compared as a string, so a tab is
one character), and it is removed; anything a line has beyond it is code. That
common whitespace cannot be told apart from indentation that belongs to the code,
so a body whose lines are all indented, or whose first line is the most indented,
needs `CODE{verbatim}` to be written exactly. With the attribute the structural
indentation is fixed as that of the `CODE` line plus two spaces, and everything
beyond it is code; a line with less than that loses only the indentation it has.
The attribute follows the keyword with no space (`CODE{verbatim} typescript
renewals.ts`), changes nothing in the AST or the schema, and is written by
`slidelang fmt` and `doclang fmt` only when a body needs it. See
[code body indentation](../../docs/portable-code-indentation.md).

If the second word of a `CODE` line starts with `{` or `[`, it is not a filename:
the whole text after `CODE` is the language (`CodeElement.language` is
`python {1,3-5}`), with its spacing exactly as written and no filename. This rule
exists only to reproduce what a flex fence already stores, because the flex parser
keeps the whole info string as the language when what follows the first word starts
with `{` or `[` (highlighted lines, or a code-group label); it lets
`slidelang fmt` write such a fence as strict. The highlighted-lines text has no
meaning of its own in strict, and no renderer reads it from there. Before this rule
such a header was an error, because the code-filename contract rejects a filename
that starts with `{` or `[`. Code groups are unaffected: in strict they are
`:::code-group` blocks with fenced code and `[label]`.

A `QUOTE` body is the lines that follow it, up to a blank line, a `---`, or the
start of another element. A blank line does not end the quote when the next
line with text is indented deeper than the `QUOTE` keyword: it stays inside as
an empty line, so a quote can have several paragraphs, the way a `CODE` body
keeps its blank lines (every blank line counts, not only the first):

```
SLIDE content
  QUOTE
    First paragraph.

    Second paragraph.
    AUTHOR: Someone
```

reads as the content `First paragraph.\n\nSecond paragraph.` with author
`Someone`. A body that starts with a blank line, or a blank line followed by
text that is not indented deeper than `QUOTE`, ends the element as before. A
quote cannot start or end with an empty line. `slidelang fmt` writes a flex
quote with an empty `>` line this way.

An `IMAGE` may carry `context:` with one of `title`, `hero`, `gallery`,
`content` or `standalone` (`ImageElement.context`). Without it the parser
infers the value from where the image sits, and the two dialects read that
position differently: a cover image under `# Title` in flex is `title`, while
the same `IMAGE` inside a `SLIDE` is not. `slidelang fmt` writes `context:` only
on the images whose inferred value would otherwise change, so a formatted flex
deck builds to the same AST. Any other value is the `IMG003` warning and the
inferred context is used. The field itself remains historical metadata, not a
way to frame the image (use `fit`, `focus` and `bleed`).

A `heading_element` is a subsection heading inside a slide, the strict form of
flex `###`–`######`. `level:` defaults to 3; `id:` overrides the anchor, which
is otherwise `heading-` plus the anchor derived from the title, suffixed when
already used earlier in the deck (flex headings inside `:::` blocks also take
a place in that sequence).

The keyword is shared with the Document Strict Mode Grammar below, but the
construct is different. In a document, `SECTION` is a top-level container: it
starts at column 0, its body elements are indented under it, and level 1 opens
a new section. Inside a slide, `SECTION` is a leaf element: it sits at the
slide's element indentation, only `level:` (3–6) and `id:` may be indented under
it, and the elements after it stay at the slide's element indentation, never
under the heading. Indenting an element under a slide heading is an error. Flex and strict produce the same AST for headings
that are direct children of a slide. With frontmatter `ast_capabilities: [typed-headings-v1]`,
headings in either dialect become typed `heading` nodes (schema 2.17.0)
instead of raw-HTML text; without it they keep the legacy form. See
[portable typed headings](../../docs/portable-typed-headings.md).

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
`TextElement.Level` (or, with `ast_capabilities: [typed-headings-v1]`, typed
`heading` nodes with `level`, authored `text` and `anchor`). This mirrors what
`#`/`##` produce in flex, which is what lets the
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

`line`/`column` are 1-based and count from the first line of the source
file, front matter included (issue #245) — never from the body a given
parser happened to receive.

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
  chartType: "bar" | "line" | "pie" | "doughnut" | "radar" | "polarArea"
           | "scatter" | "bubble" | "combo" | "treemap"
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
  content: string          // raw body; empty in a typed column
  elements?: ElementNode[] // typed body (see "Typed columns")
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

### Links and images

Prose fields (paragraphs, list and checklist items, table cells, quotes,
special blocks, columns, metrics, quiz and poll text, headings) accept inline
links and images:

```
[link text](destination)
![alt text](destination)
```

The destination runs to the `)` that balances the parentheses opened inside
it, as in CommonMark, so `[Foo](https://es.wikipedia.org/wiki/Foo_(bar))` keeps
its full URL. It must close on the same line and cannot be empty; otherwise the
text is not a link and is kept literally. Complete character references in
the destination, ending in `;` (`&#106;`, `&colon;`, `&amp;`), are decoded
before it is checked; a bare `&name` without `;`, as in a query string
(`?a=1&section=2`), is left as written.

Only `http:`, `https:`, `mailto:`, `tel:`, `ftp:` and scheme-less (relative
or `#anchor`) destinations are emitted. Any other scheme (`javascript:`,
`data:`, `vbscript:`, `file:`, or an unknown one), or a destination that is
not a valid URL, is dropped: a link renders as its text alone, an inline image
as its alt text, and a block image is not drawn. The linter reports each such
destination as `LINK001` (warning). Destinations built from `{{variables}}` are
resolved at render time and are not checked by the linter.

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
| `@timer` | `@timer: seconds` | Slide timing |

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

#### Typed special blocks

A flex block fills two views of its body: `content` (the trimmed raw lines) and,
when a line is recognized as a heading, fenced code, a pipe table, an image, a
chart or a nested block, `elements` (with the loose prose between them as text
elements). The recognizers are flex's. A strict block reads its body with strict's
own, which do not include headings or fences, so the same lines give an empty
`elements`.

A `typed` attribute on the opening line of a **strict** block (`:::card{typed}`,
`:::details{typed} Advanced settings`, `:::card{type="success" typed}`) reads the
body with the flex recognizers, so that `content` and `elements` equal what the
flex block with that body produces. There is no general attribute reader: the text
inside the braces stays in the block type, as it always did, and the flag is
detected as a word inside the braces. Words are separated by spaces or commas
outside quotes, so `{ typed }`, `{typed}` and `{type="x",typed}` are all the flag;
`typed` has to be the whole word (`untyped`, `typed-x`, `typed=false` and `TYPED`
are not) and not inside quotes. The block type is what is left, with the other
attributes separated by one space: `:::card{type="success" typed}` has the block
type `card{type="success"}`, the same as the flex block. `slidelang fmt` writes it
where strict would otherwise lose the nested elements, and only if the text then
reads back with the same elements.

The typed reading removes from the lines that follow only the indentation of the
opening line. A body indented deeper than its opening line keeps the extra
indentation inside nested code, which a flex block (whose body is flush with its
opening) never has, so `slidelang fmt` writes the body flush with the opening line.
`fmt` refuses a flex block whose type already contains the word `typed` inside
braces, since writing it would read back as the flag.

It is an attribute and not a word of the title so that it never meets one: a title
such as "Dynamically typed" is a title with or without nested elements, and no
existing source reads differently.

This is deliberately not the same form as a typed grid column (`::: column typed`,
`<<column typed>>`). A column has no title, so a trailing word is unambiguous there,
and only `elements` is filled while `content` is empty: a flex column never had
`elements`, so its typed form is a new shape. A flex block has always filled both
views, so the strict form has to reproduce both to build the same AST, and it
carries the marker in the block's attribute list, which is where block options
already live (`:::card{type="success"}`).

The attribute applies to any strict `:::` block, because they share one parser, and
matters for the ones whose body holds nested elements: `card` (and its `type`
variants), `columns`, `tabs`, `accordion`, `details`, `reveal`, and the callouts
`info`, `warning`, `danger`, `success`, `tip`, `note`, `example`, `left`, `right`
and `highlight`. Flex needs no marker: there `{typed}` stays part of the block type
like any other attribute.

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

##### Typed columns

By default a column body is raw text, stored in `ColumnElement.content`, so an
element inside it cannot carry a `<!-- node-id: Name -->` identity. A column
opts into a typed body with its own opening marker: `<<column typed>>` in
strict and `::: column typed` in flex. The marker must be exactly that text
(after trimming); any other suffix leaves the column raw. Raw and typed columns
can be mixed in one grid.

```slidelang
<<grid>>
<<column typed>>
  <!-- node-id: ColTextA -->
  TEXT
    Left side.
<<column>>
Right side.
<<end>>
```

In strict, the body of a typed column is the run of lines indented deeper than
the marker, parsed with the grammar of a `SLIDE` body; because it is delimited by
indentation, an element inside it (a chart, a quiz) closes with its own
`<<end>>` without closing the grid. In flex the body is ordinary flex content,
ending at `:::`, the next `::: column` or a strict slide boundary (a `## Heading` or `---` inside it stays in the column as text).
The parser fills `ColumnElement.elements` and leaves `content` empty. A
property line, a `SECTION` heading or a nested grid inside a typed column is an
error, and a heading line is plain text (`TextElement`), never a typed heading.
`slidelang fmt` writes a column with `elements` as `<<column typed>>` and
round-trips it, `nodeId` included. This adds no schema field or version:
`ColumnElement.elements` is part of the 2.14.0 contract. See
`docs/portable-typed-columns.md` for the full design.

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
| Charts | `<<chart: type>>` or `<<chart` … `<<end>>`, or a fenced ` ```chart ` block (flex only, JSON body) | Data visualizations |
| Diagrams | `<<mermaid>>` or `<<mermaid title="…">>`, or a fenced ` ```mermaid ` block (flex only) | Mermaid diagrams; `title` is the figure caption |
| Maps | `<<map>>` or `<<map attr="…">>`, or a fenced ` ```map ` block (flex only, JSON body) | Geographic maps; the body ends at `<</map>>` or at the first line the map parser does not recognize (a `<<end>>` is not read by the map parser) |
| Grid | `<<grid>>` / `<<column>>` / `<<end>>` | Column layouts — see "Grid and Column Layouts" above |
| Quiz | `<<quiz>>` … `<<end>>` | Multiple-choice question with a correct answer — see "Quiz and Poll" below |
| Poll | `<<poll>>` … `<<end>>` | Question without a correct answer — see "Quiz and Poll" below |

`<<plantuml>>` accepts the same `title="…"` caption; a chart's caption is its
existing `title:` property. The only attribute either
diagram tag accepts is `title`; anything else is an error. `<<video …>>` and
`<<audio …>>` accept `src`, `poster` (video), `caption`, `controls`,
`autoplay`, `loop` and `muted` (`src` and `poster` are references that the
toolchain never downloads); any other attribute is reported as warning
`MEDIA001`, and `poster`/`caption` move the document to schema 2.18.0 with the
inferred capability `media-figure-v1`. In chart `data`, `null` is a missing
value kept in its column. A map marker without usable coordinates is an error.
See [figures and media](../../docs/portable-figures-media.md).

The fenced form (`` ```lang `` … `` ``` ``, four backticks also accepted for
the opener/closer) exists so a markdown-savvy author or an LLM writing loose
markdown can drop in a code fence instead of learning the native tag — it is
recognized in **flex mode only** (strict is keyword-driven and has no fence
syntax at all). For charts and maps, the fenced body must be a single JSON
object — it is the only body shape the fence supports, unlike the native tags,
which additionally accept a YAML-ish `key: value` property block (charts) or
`key: value`/`marker:` lines (maps). A chart's fenced JSON becomes its raw
Chart.js config, same as `<<chart>>` followed directly by a `{`-prefixed JSON
block; a map's fenced JSON is a plain data serialization (`center`, `zoom`,
`markers: [{position: [lat, lng], popup: "…"}, …]`, `type`, `heatmap`, `title`,
`width`, `height`) translated into the same fields the native `<<map>>`
properties populate — a map has no raw-passthrough mode, since it carries no
third-party config to preserve verbatim. Invalid JSON in either fenced form is
reported (`CHART002`/`MAP002`) and the element renders with no data, same
severity as its native-tag equivalent.

**Tag boundaries.** A tag that takes no attributes — `<<quiz>>`, `<<poll>>`,
`<<grid>>` — must be written **alone on its line**. `<<quiz>>anything` is not a
quiz in either dialect: it is ordinary content, and in `strict` the line is
reported as unrecognized (a warning in presentations, an error in documents,
which is the same severity split every unrecognized line gets).

A tag that does take attributes — `<<map …>>`, `<<chart: …>>`, `<<video …>>` —
matches only on a **whole-word** tag name. `<<mapa>>`, `<<charts>>` and
`<<videofoo …>>` are not mistyped tags that get parsed anyway; they are ordinary
content. This matters more than it looks: a tag matched on a loose prefix
consumes the lines under it, so a mistyped `<<mapa>>` above a `title:` used to
absorb the slide's title and leave the slide untitled, with nothing reported.

The same tags must also **close exactly once, on the line they open**. All
three of these are ordinary content, not charts: `<<chart: bar` (no `>>`),
`<<chart: bar>>trailing` (text after the close), and `<<chart: bar>>junk>>`
(a second `>>`). The one exception is the multi-line opener `<<chart`, written
alone on its line and closed by `<<end>>`: it carries no `>>` by definition.

A tag that takes **no** attributes closes by being the whole line:
`<<mermaid>>`, `<<plantuml>>` and `<<math>>` match only when nothing follows
them, the same rule `<<quiz>>`, `<<poll>>` and `<<grid>>` already had in the
strict dialect.

### Math blocks

Block equations use raw LaTeX. The portable, canonical form is an explicitly
closed `<<math>>` block. It may include a prose `caption:` and a cross-reference
`label:` after the LaTeX body, using the same quoted metadata syntax as images
and tables:

```
<<math>>
E = mc^2
caption: "Mass-energy equivalence"
label: "eq:einstein"
<<end>>
```

`caption:` is not part of the LaTeX content. A labeled equation receives its
equation number during the cross-reference pass; an unlabeled equation remains
unnumbered. In the flex dialect, `$$ ... $$` remains supported for backwards
compatibility, but cannot carry `caption:` or `label:` metadata. Formatters
canonicalize block equations to the `<<math>>` form so that metadata is retained.

This section is about the `<<…>>` family only. The `:::` blocks match their
name on a loose prefix and have no boundary of their own — `::: gridJUNK` is
still parsed, and consumes the lines under it exactly as a mistyped `<<…>>` tag
used to. That is tracked separately in
[toolchain#307](https://github.com/ziradocs/toolchain/issues/307).

The three halves of the rule exist for the same reason and are worth naming
separately, because closing one and calling the boundary done is how the other
two survived: the opening boundary decides whether a *mistyped* tag is reparsed
as something else, the terminator decides whether an *unfinished* one is, and
requiring the terminator to be unique decides whether a *well-formed tag with
text glued after it* is. A tag that gets reparsed anyway consumes the lines
below it, which in a slide are its properties.

### Layout options

A slide's layout can carry **options** that change how its content is arranged.
They are declared per layout — an option written on a layout that does not
accept it is reported, not silently ignored.

| Option | Layouts | Values |
|---|---|---|
| `columns` | `comparison`, `stats`, `dashboard`, `call_to_action` | an integer from 1 to 4 |
| `align` | `hero`, `testimonial`, `call_to_action` | `left` or `center` |

**Flex** puts them in the same block as `layout:`; the order inside the block
does not matter.

```slidelang
---
layout: comparison
columns: 2
---
## Two side by side
```

Consecutive metadata blocks before the same slide are one declaration. A block
that does not write `layout:` inherits the layout the previous one set, and its
options are **added** to those already declared:

```slidelang
---
layout: call_to_action
columns: 2
---
---
align: left
---
## Both options apply
```

A block that **does** write `layout:` starts over: the options carried so far
belonged to the previous layout, and applying them to the new one would invent a
declaration nobody wrote. Inheritance never crosses a slide — the layout and its
options are consumed by the block that follows them.

**Strict** puts them as properties under the `SLIDE` line, like `title:`:

```slidelang
SLIDE comparison
  title: "Two side by side"
  columns: 2
```

A value outside its range is reported (`FLEX004` in flex, a parse error in
strict) and the option is dropped — never applied halfway. They travel in the
AST as `layout_config`, and the HTML emits them as `data-layout-*` attributes,
so a theme can react to them in pure CSS.

### Quiz and Poll

`<<quiz>>` and `<<poll>>` carry a **YAML body**, unlike the other embedded
elements. They are valid in both dialects and in both modes.

```slidelang
<<quiz>>
question: "Which type of ML would you use for email spam detection?"
options:
  - "Unsupervised learning"
  - "Supervised learning"
  - "Reinforcement learning"
answer: 1
explanation: "We have labeled examples of spam and legitimate email."
<<end>>

<<poll>>
question: "What's your programming experience level?"
options: ["Beginner", "Intermediate", "Advanced", "Expert"]
multiple: false
<<end>>
```

| Key | Element | Meaning |
|---|---|---|
| `question` | both | The prompt. |
| `options` | both | A YAML list or an inline array — both forms are accepted. |
| `answer` | quiz | **0-based** index of the correct option: `answer: 1` selects the *second* option. |
| `explanation` | quiz | Shown with the answer. Optional. |
| `multiple` | poll | Allows selecting more than one option. Optional, defaults to `false`. |

A key outside that set is reported and ignored rather than dropped silently
(`QUIZ005` / `POLL004`), and a body that is not valid YAML is reported as
`QUIZ004` / `POLL003` — the block is still consumed, so nothing after it is
reprocessed as prose.

**`question`, each `option` and `explanation` are prose**, and carry the same
inline formatting as any paragraph: `**bold**`, `*italic*`, `` `code` ``, links
and the `[x]{.class}` spans. They are rendered, not printed verbatim — so a `*`
that is meant literally has to be escaped, exactly as in a paragraph. Which
formats honour which token is a property of the output, not of the element; see
the per-format table in the inline-formatting reference.

`<<end>>` is the canonical closer. `<</quiz>>` and `<</poll>>` are also
accepted, the same way `<</chart>>` is.

**Both are static.** There is no backend and no collection of answers: a quiz
renders with the correct option marked and its explanation visible, and a poll
renders as a list. Any interaction is local to whoever opens the HTML.

Both blocks accept `results` (one percentage from 0 to 100 per option, in
order; the values need not add up to 100) and `responses` (a non-negative
integer). They are new syntax, so the capability `quiz-poll-results-v1` is
inferred from them (see "AST capabilities"). Documents that use them emit
schema 2.20.0. See
[quiz and poll results](../../docs/portable-quiz-poll-results.md).

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
   - Link and image destinations must use an allowed scheme (`LINK001`, see
     [Links and images](#links-and-images))
   - Code elements should specify language for highlighting
   - Grid blocks must contain only column elements as direct children
   - Column elements should only be used within grid containers
   - A typed column (`<<column typed>>` / `::: column typed`) accepts elements only; a
     nested grid, a property line or a heading is an error

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
