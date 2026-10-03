# Typed grid columns

Status: design for issue #373. Opt-in per column, no change to the JSON
schema (stays 2.20.0). Without the opt-in, every existing source and AST keeps
its current meaning byte for byte.

## Problem

A `<<grid>>` column body is stored as raw text (`ColumnElement.content`).
Because the body is not parsed into elements, nothing inside a column can carry
`<!-- node-id: Name -->`: the directive has no AST node to bind to and the
build stops with an orphan node-id error. The reverse also holds. The AST
already has `ColumnElement.elements` (an optional, polymorphic list that every
renderer and `ast.Walk` already traverse), but no parser fills it, so a
column with typed elements has no source form and `fmt` refuses to write it.
This matches how the formatter documents the limit today.

Two facts about the current code shape the decision:

- **Flex does not produce typed columns either.** `::: column` and
  `<<column>>` both store `content`. Only a hand-built AST carries `elements`.
  The goal "flex and strict give the same AST for a typed column" therefore
  needs a typed path in flex as well, not only in strict.
- **`elements` is already part of the schema.** `ColumnElement.elements` is in
  `schema/ast.schema.json` and `DecodeAST` accepts it at 2.14.0. The change
  adds a way to write it, not a field.

## Decision

A column opts in by its own opening marker, not by a document switch.

| Dialect | Raw column (unchanged) | Typed column (new) |
| --- | --- | --- |
| strict | `<<column>>` | `<<column typed>>` |
| flex | `::: column` | `::: column typed` |

The two forms can be mixed in one grid. A raw column keeps filling `content`
and leaves `elements` empty. A typed column leaves `content` empty and fills
`elements`.

Why a per-column marker instead of the alternatives:

- **Auto-detecting element keywords in the existing body** would change the AST
  of any document whose column text happens to start with `TEXT` or `POINTS`.
  That is not additive.
- **A document-level opt-in** (frontmatter `ast_capabilities`, like
  `nested-list-types-v1`) switches every column of the document at once. It
  forbids mixing raw and typed columns, it needs frontmatter handling in `fmt`
  and a schema capability, and strict `.slidelang` files that rely on a raw
  column would need a rewrite the moment someone adds one typed column. Those
  precedents exist because the older opt-ins reinterpret syntax that already
  carries meaning at document scope. A column marker is new syntax, the same
  situation as `poster=` or `results:`, which are inferred from the authored
  key and need no declaration.
- **The only existing source that changes meaning** is a literal line
  `<<column typed>>` inside a raw column (strict), or the suffix `typed` after
  `::: column` (flex, whose `CanParse` is a prefix match and ignores any suffix
  today). Both are treated as the new marker. A marker with any other suffix
  stays what it is now.

## Strict grammar

The body of a typed column is the run of lines indented deeper than the
`<<column typed>>` marker, in the same way the body of a `SLIDE` is indented
under `SLIDE`. It uses the `SLIDE` body grammar: `TEXT`, `POINTS`, `CODE`,
`IMAGE`, `TABLE`, `QUOTE`, `CHECKLIST`, `<<chart>>`, `<<map>>`, `<<quiz>>`,
`<<poll>>`, `<<metric>>`, `<<math>>`, `<<mermaid>>`, `<<plantuml>>`,
`:::code-group`, special blocks, media and directives.

```slidelang
SLIDE content
  title: "Grid"
  <<grid>>
  <<column typed>>
    <!-- node-id: ColTextA -->
    TEXT
      Left side.
  <<column>>
  Right side.
  <<end>>
```

Rules:

- The body ends at the first nonblank line indented no deeper than the marker
  (the next `<<column>>`, `<<column typed>>` or `<<end>>`). Because the body is
  delimited by indentation, an `<<end>>` that closes an element inside the
  body (a chart, a map, a quiz) does not close the grid. In a raw column the
  first `<<end>>` still closes the grid, exactly as today.
- Block properties (`key: value`) are an error inside a column, because a
  column has no properties. `SECTION "Title"` is an error too (see Headings).
- A `<<grid>>` inside a typed column is an error in this version. The grid
  markers are recognized on trimmed lines, and flex has no unambiguous nested
  closer, so both dialects reject the case instead of guessing.
- An unrecognized line is an error in both SlideLang and DocLang. A `SLIDE`
  body downgrades it to a `STRICT003` warning only because existing decks
  depend on that; typed columns are new syntax with no such legacy, so nothing
  is dropped silently.
- A body line indented only one space deeper than the marker, instead of two,
  is an error rather than a silent end of the body. A line at the grid's own
  indentation after a typed column is an error too (it would otherwise read as
  loose prose).

## Flex grammar

`::: column typed` opens a column whose body is parsed with the flex element
registry (`TextParser` is the fallback), the same loop a flex slide uses,
with no `#`/`##`/`---` cuts. The body ends at `:::`, at the next
`::: column`, or at a strict slide boundary, so a raw `:::`
inside an element that owns it (a fenced code block, a special block) is
consumed by that element and never read as the column closer. Loose prose
becomes a `TextElement`, bullets a `PointsElement`, `![alt](src)` an
`ImageElement`, pipes a `TableElement`, and so on.

```slidelang
---
mode: flex
---
# Grid

::: grid
::: column typed
<!-- node-id: ColTextA -->
Left side.
:::
::: column
Right side.
:::
:::
```

## Same AST in both dialects

For the same logical column the two dialects produce the same elements (types
and fields), ignoring `position` and `nodeId`. The covered set is `TEXT`,
`POINTS`, `CODE`, `IMAGE`, `TABLE`, `QUOTE`, `CHECKLIST` and `<<chart>>`, and
the test suite asserts exactly that set. Where the dialects cannot be equal the
design says so instead of hiding it:

- **Image paths.** Flex and strict already resolve `source` and infer
  `context` differently for an image at slide level (flex prefixes
  `assets/images/`). A typed column inherits that difference; it does not add
  one, so the image case compares everything except those two fields.
- **Where a flex body ends.** A typed flex column ends only at `:::`, the
  next `::: column` or a strict slide boundary, the same cuts a raw column has.
  A `# `/`## ` heading line or a `---` inside it stays in the column, each as
  its own `TextElement` (the same as a strict `TEXT` holding that line), so
  the body is never split or dropped silently; an unclosed column therefore
  runs to the end of the grid, as a raw one does.
- **Pipe tables in flex.** A flex pipe table directly after `::: column` or
  `::: column typed`, with no blank line between them, is read with the
  `::: grid` line as a table row. This already happens in raw columns and is
  not changed here; leave a blank line before the table (the tests do).

- **Headings.** `###` and `SECTION` are not typed inside a column. A flex
  `### Title` becomes a `TextElement` (the registry has no heading parser) and a
  strict `SECTION` is an error. Writing the heading as `TEXT` with a `###`
  line gives the same AST in both. Typed headings in columns can follow later
  without changing this design.
- **Normalization.** `flex-full` and `auto` run the content normalizer, which
  can rewrite an annotated source; identity binding is rejected in that case,
  as it is today everywhere else. Equivalence is asserted with the normalizer
  off or with sources it leaves untouched.

## Node identities

`<!-- node-id: Name -->` before an element in a typed column binds to that
element. Before the `<<column typed>>` / `::: column typed` line it binds to
the `ColumnElement`. Positions of nested elements are the real authored lines
(the sub-parse is offset into the file), because binding is by line. A raw
column body is still not an identifiable node. This replaces the sentence in
`portable-node-identities.md` that states the limit without the way out.

## AST and JSON

No new field, no new capability, no version bump (stays 2.20.0, regenerated
artifacts are unchanged). `ColumnElement.elements` was already in the 2.14.0
schema and `DecodeAST` already accepts it without a capability. Adding an
extension row whose test is "some column has `elements`" would make
`validateUsedContract` reject 2.14.0 JSON that is valid today, which is the
hand-built case the issue starts from, so it is deliberately not done.

Consumers: a JSON reader that follows the published contract already handles
`elements` on a column. A reader that ignored it, because no core parser ever
filled it, sees an empty `content` for a typed column. That is the one
behavioral difference and it is documented here and in the spec. Documents that
do not use `<<column typed>>` / `::: column typed` are unaffected.

## Formatter

- **SlideLang strict.** `formatStrictGrid` writes `<<column typed>>` and each
  element through `formatStrictElement`, indented under the marker. A column
  with `elements` is typed; a column with `content` is raw. A column with both
  is rejected explicitly (not representable without losing one of them). A
  nested `GridElement`, a typed `HeadingElement` and any other element strict
  cannot write are rejected explicitly, as they are in a `SLIDE`.
- **Identities.** `formatNodeIDs` already reparses the formatted source and
  places each directive by traversal order and position; typed columns join
  that traversal, so nested identities round trip and the formatter keeps
  failing, never moving an ID, when a node cannot be represented.
- **DocLang.** `doclang fmt` has no text syntax for grids today ("DocLang no
  tiene sintaxis de texto para GRID"), and this change does not add one.
- **Round trip.** parse, `fmt`, parse yields the same AST including `nodeId`,
  and `fmt` run twice yields the same bytes.

## Renderers

The core HTML renderer already draws `ColumnElement.elements` after `content`,
which covers DocLang HTML and PDF. The consumers that read only `content` are
changed so a typed column does not render empty:

- SlideLang HTML: the column data carries rendered nested elements, the
  template renders them, and the used-element scan descends into columns so a
  chart, diagram or map inside a column loads its own CSS and JS.
- DocLang DOCX and Markdown: the column renders each nested element through the
  existing element renderers, in order, after the raw `content` (empty for a
  typed column).

The linter and the cross-reference/numbering transforms already walk
`ColumnElement.elements`.

## Compatibility and release order

Source compatibility: raw columns parse exactly as before. The one change to
existing source is that a line exactly equal to `<<column typed>>` (strict) or
`::: column typed` (flex) now opens a typed column instead of being text or an
ignored suffix, and `fmt` refuses a raw column whose content contains the
literal line `<<column typed>>`, because it would not parse back as text. Release
notes should carry that sentence; the repository has no changelog file.
There is no schema change, so no regenerated schema or TypeScript types. The
parser, formatter and spec live in `core`, so the release follows the
repository sequence: `core/vX.Y.Z` first, then the pin bump in
`slidelang/go.mod` and `doclang/go.mod`, then the product tags. The renderer
changes in `slidelang` and `doclang` use only symbols `core` already exports
and build against the pinned core; their parse-level tests need the new core
and run in the CI job that builds consumers against the tree's core.

## Out of scope

Typed headings inside a column, a grid nested in a column, and a DocLang text
syntax for grids. Each can be added later without changing the marker.
