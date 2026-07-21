# Element Reference (ZiraDocs strict / ZiraDocs flex / DocLang flex)

This is the per-element syntax table. It is derived directly from the parser
source (`core/parser/strict.go`, `core/internal/elements/*.go`)
and the linter (`core/linter/rules.go`), not from prose docs, so it
stays accurate as the ground truth for "will this parse."

Both `.slidelang` and `.doclang` share the same element parsers. The
difference between the two CLIs is only in how the top-level document is
split into blocks (see `slidelang-strict.md`, `slidelang-flex.md`,
`doclang-flex.md`) — element syntax inside a block/section is identical.

## Quick table

`StrictParser` (`parser/strict.go`) dispatches each element by an explicit,
hand-written prefix switch — it does **not** use the same element-parser
registry as flex mode. Elements not in its switch are either misrouted or
silently dropped, even though the same parser type exists and works in flex.
The "Strict marker" column below reflects the switch, not the element's
theoretical existence.

| Element | Strict marker | Flex marker (slidelang + doclang) |
|---|---|---|
| text | `TEXT` | any unmatched line (fallback) |
| points (list) | `POINTS` | `- item`, `* item`, `+ item`, `1. item` |
| code | `CODE` | fenced ` ```lang ` |
| code-group | `:::code-group` | `:::code-group` or `::::code-group` |
| image | `IMAGE <src>` | `![alt](src)` |
| table | `TABLE` | pipe rows: `\| a \| b \|` |
| quote | `QUOTE` | `> quoted text` |
| checklist | `CHECKLIST` | `- [ ] todo` / `- [x] done` |
| mermaid | `<<mermaid>>` | `<<mermaid>>` or fenced ` ```mermaid ` |
| plantuml | `<<plantuml>>` | same |
| chart | `<<chart:type>>` | same |
| map | `<<map>>` | same |
| math | `<<math>>` … `<<end>>` | `<<math>>` … `<<end>>` or `$$ … $$` |
| grid | `<<grid>>` / `<<column>>` / `<<end>>` | `::: grid` / `::: column` |
| special-block | `::: info\|warning\|danger\|success\|tip\|details` | same |
| directive | `@name` | `@name` |

**Grid uses a distinct spelling in each mode.** In strict, a grid is the
delimited block `<<grid>>` … `<<end>>`, with each column introduced by
`<<column>>` (column body = raw Markdown content; lines before the first
`<<column>>` are loose prose spanning the grid). The flex `::: grid` /
`::: column` form is **not** recognized in strict — there it would fall through
to the generic `:::` branch (`SpecialBlockParser`), producing a malformed
special block that trips `SPECIAL001` and never renders as a grid. Both
spellings produce the same typed `GridElement`.

In flex mode, element detection goes through a priority-ordered parser
registry (most specific first, plain text last) — you never need to worry
about ordering when writing content, just use the markers above. This is
also why flex supports strictly more element types than strict: everything
registered in the registry works, not just what strict's switch special-cases.

## Charts — canonical row-based schema

```
<<chart: bar>>
  data: [
    ["Q1", 45, 32],
    ["Q2", 52, 38]
  ]
  series: ["Product A", "Product B"]
  options:
    responsive: true
```

- `data` rows: first cell is the label, remaining cells are numeric values.
- `series` length must equal the number of numeric columns per row — a
  mismatch doesn't hard-fail the parser, but keep it consistent for a
  correct render.
- Types: `bar`, `line`, `combo`, `doughnut`, `radar`, `scatter`. No closing
  tag — do **not** write `<</chart>>`.
- The linter (`CHART001`) only warns if a chart has **no** data at all
  (neither `data`/`series` YAML nor a JSON payload) — it does not validate
  column/series consistency, so get this right yourself.

## Mermaid

```
<<mermaid>>
  graph TD
      A[Start] --> B{Decision}
      B -->|Yes| C[Path 1]
      B -->|No| D[Path 2]
```

- No closing tag. Any valid Mermaid diagram type works: `graph`,
  `flowchart`, `sequenceDiagram`, `gantt`, `classDiagram`, `stateDiagram-v2`,
  `erDiagram`, `mindmap`.
- Renders **client-side** via mermaid.js in the browser — no build-time
  dependency.

## Map

```
<<map>>
  type: world
  markers:
    - lat: 40.7128
      lng: -74.0060
      label: "New York"
      value: 45
  heatmap: true
  zoom: 2
```

- Each marker needs `lat`, `lng`; `label` and `value` are optional but
  recommended.
- Renders client-side via Leaflet in browser mode.

## Math (LaTeX)

Two spellings. The delimited block works in both modes:

```
<<math>>
label: "eq:euler"
e^{i\pi} + 1 = 0
<<end>>
```

The `$$` form is flex-only (ZiraDocs flex and DocLang):

```
$$
E = mc^2
$$
```

- **Block/display math only.** There is no inline math — `$x$` in the
  middle of a sentence is not parsed as math, it stays literal text. `$$`
  must open the line; `$$formula$$` on a single line also works.
- `<<end>>` closes the delimited form (a `---` slide separator or EOF also
  ends it, but write `<<end>>`).
- The optional `label:` line is recognized **only inside the `<<math>>`
  form** — it numbers the equation and makes it addressable by `\ref{...}`.
  The `$$` form carries no metadata.
- Rendered with MathJax (SVG output), loaded from CDN in the default
  browser render mode.

## Special blocks (info/callouts)

```
::: info
Key contextual note.
:::

::: warning
Important caution.
:::
```

**The only valid `BlockType` values are `info`, `warning`, `danger`,
`success`, `tip`, `details`** (linter rule `SPECIAL001`). Anything else —
`note`, `error`, `example`, `poll`, `qa_session`, `reveal`, `notes` — is
**not** a real element type and will only warn, not render as intended.
Represent unsupported interactive ideas (polls, quizzes, progressive
reveal) as plain prose instead of inventing a block type. Presenter notes
are the exception — they exist, as the `@notes` directive, not as a block.

## Code groups

```
:::code-group
```bash [npm]
npm install package-name
```

```bash [yarn]
yarn add package-name
```
:::
```

- Must contain at least one fenced code block (`CODEGROUP001` — error if
  empty).
- Do not leave a `:::code-item{title="..."}` tab outside a recognized
  `:::code-group`/`::::code-group` wrapper — that produces an orphaned tab
  (`CODEGROUP002` — error) instead of a working code group.

## Table

Markdown pipe syntax works in both modes:

```
| Metric | Q4 | Q3 | Change |
|--------|----|----|--------|
| Revenue | $387K | $312K | +24.0% |
```

- Needs headers (`TABLE001` — warning if missing) and at least one row
  (`TABLE002` — warning if empty).
- **Every row must have the same number of columns as the header row**
  (`TABLE003` — error, not warning, if a row's column count doesn't match).

## Checklist

Flex only — see the quick table note above.

```
- [x] Done item
- [ ] Todo item
```

## Grid layout

Different spelling per mode (both produce the same typed `GridElement`).

Flex:

```
::: grid
::: column
<!-- column content -->
:::
::: column
<!-- column content -->
:::
:::
```

Strict:

```
<<grid>>
<<column>>
<!-- column content -->
<<column>>
<!-- column content -->
<<end>>
```

## Directives

A directive is a single line starting with `@`, in both strict and flex.
`@name args` and `@name: args` are equivalent — the trailing colon is
stripped. The parser accepts any name, but only these seven have
argument semantics of their own:

| Directive | Forms | Parsed as |
|---|---|---|
| `@notes` | `@notes "text"`, or `@notes:` / `@notes` followed by lines of text | Presenter notes. The multi-line form collects following lines until a blank line, another `@`, a `#` heading, or `---` |
| `@timer` | `@timer 300` or `@timer duration=300 warning=60` | Countdown in seconds (bare number → `duration`) |
| `@transition` | `@transition type="fade" duration="1000ms"` | key=value only |
| `@highlight` | `@highlight yellow` or `@highlight color="yellow"` | Bare value → `color` |
| `@delay` | `@delay 2000` | Milliseconds (always a bare value) |
| `@auto-play` | `@auto-play 5000` or `@auto-play interval=5000` | Bare value → `interval` |
| `@include` | `@include path/to/file.doclang` | Build-time file transclusion — see below |

Any other name still parses: `key=value` pairs become parameters, a single
bare argument becomes `value`.

**Where they actually do something.** ZiraDocs renders directives:
`@notes` is lifted out of the slide into presenter notes (toggled with `N`
in the generated deck), `@timer` renders a live countdown, `@transition`
and `@auto-play` drive the deck's JS, and the generator also recognizes a
set of styling names that map to CSS classes — `@center`, `@large`,
`@small`, `@fade-in`, `@slide-up`, `@bounce`, `@float-left`,
`@float-right`, `@spacing-wide`, `@margin-large`, `@no-transition`,
`@full-screen`. **DocLang parses directives but renders nothing for
them** — they silently disappear from document output. Don't use them to
carry content a `.doclang` reader needs to see.

### `@include`

`@include` is not an AST directive at all: it is textual expansion that
runs at **build time, before parsing**, in both CLIs. The line is replaced
by the referenced file's content (its frontmatter, if any, is stripped),
recursively, up to 32 levels; include cycles are detected and abort the
build.

Paths are **confined** to a root directory — by default the input file's
own directory, overridable with `--include-root`. Absolute paths and any
attempt to escape the root (including via symlink) are rejected. Two
consequences worth knowing: `fmt` does not expand includes (the formatted
file keeps declaring `@include`), and neither does the MCP `lint`/`get_ast`
path, which works on in-memory source with no base directory — so an
`@include` line surfaces there unexpanded.

## No unsupported closing tags

Legacy/invalid syntax that must never appear in output: `<</chart>>`,
`<</mermaid>>`, `<</map>>`, `<<poll>>`, `<<quiz>>`, `:::poll`,
`:::qa_session`, `:::reveal`, `:::notes` (as a block type — the `@notes`
directive is the real presenter-notes mechanism). None of these are
implemented by the parser.
