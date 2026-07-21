# JSON/AST contract (`--format json`)

`--format json` serializes the full AST (`core/ast.AST`) via `json.MarshalIndent` (`slidelang/internal/serializer/json.go`). It's a faithful dump of the parse tree: each `ContentBlock` (a slide in slidelang, a section in doclang) includes its `elements[]` array with the complete prose body of every element (text, bullets, tables, code, images, quotes, callouts, grids, etc.), not just metadata.

**This is the stable contract for external consumers** (e.g. the `slidelang-app` web viewer). The `<script id="slidelang-metadata">` block embedded in the generated HTML is a **derived, metadata-only** artifact (slide id/type/title/duration/transition/interactiveElements/notes + charts/diagrams/maps for in-browser JS rendering) — it does not repeat prose bodies and must not be used as the source of truth for a presentation's or document's content.

## What each element includes

Element structs (`core/ast/nodes.go`) already serialize their full bodies via `json` tags:

- `text` → `content`
- `points` → `items[].content` (+ nested `subPoints`)
- `quote` → `content`, `author`, `source`
- `table` → `headers`, `rows`, `caption` (`caption:` is only parsed in strict/YAML mode; Markdown tables have no caption syntax)
- `code` / `code_group` → `content` (per block)
- `image` → `source`, `alt`, `caption`, `context`
- `grid` → `columns[].content` (+ `columns[].elements` if structured), and `content` for loose prose inside the grid but outside any column
- `special_block` (info/warning/success/danger/tip callouts) → `blockType`, `title`, `content`, `icon`
- `checklist` → `items[].content`, `checked` (+ nested `subItems`)
- `mermaid` / `plantuml` → `content` (plain string, escaped once by the JSON marshaller)
- `chart` → `data`/`series`/`labels`/`options` (structured mode) or `rawJSON` as a nested JSON object (raw-JSON mode, `json.RawMessage` — never a re-escaped string)
- `map` → `markers[]`, `options`

## Guarantees

- No prose body is lost to a missing `json` tag.
- Grid content outside columns is no longer silently dropped (previously: issue #9).
- `rawJSON` for charts in raw-JSON mode serializes as a nested object, never as a double-escaped string or the literal `"[object Object]"` (previously: issue #11).
- Mermaid content is serialized exactly once (no wrapping quotes, no literal `\n` sequences).

See `slidelang/internal/serializer/json_prose_coverage_test.go` for the test that freezes this contract element by element.

## Pre-rendered `*HTML` fields (issue #64, since `SchemaVersion 2.0.0`)

Alongside the raw `content` (Markdown + unsubstituted `{{variables}}`), every prose field exposes a sibling `*HTML` field with the same content **already rendered to inline HTML** — Markdown applied and `{{variables}}` substituted and escaped — identical to the fragment produced by `--format html` for that same element:

- `text` → `contentHTML`
- `points` → `items[].contentHTML` (+ nested `subPoints[].contentHTML`)
- `quote` → `contentHTML`, `authorHTML`, `sourceHTML`
- `table` → `headersHTML`, `rowsHTML`, `captionHTML`
- `code` → `contentHTML` (variables substituted + HTML-escaped, **no** Markdown)
- `code_group` → `codeBlocks[].contentHTML`, `codeBlocks[].labelHTML` (variables substituted + HTML-escaped for both, **no** Markdown, same as `code`)
- `image` → `altHTML`, `captionHTML` (variables only, no Markdown)
- `grid` → `contentHTML`, `columns[].contentHTML`
- `special_block` → `titleHTML` (**does** apply Markdown, unlike the other `title*HTML` fields in this list — see the note below), `contentHTML`
- `checklist` → `items[].contentHTML` (+ nested `subItems[].contentHTML`)
- `mermaid` / `plantuml` / `chart` / `map` → `titleHTML` (variables only, no Markdown; their `content`/`data`/`series`/`labels`/`rawJSON`/`markers` is diagram source or config — see the no-goals section below)
- Each `ContentBlock` (slide/section) → `titleHTML`, `headingHTML`, `subtitleHTML`

Populated by `renderer.PopulateInlineHTML` (`core/renderer/populate_inline_html.go`) right before serialization (`generateJSON`), reusing the same sanitizer functions the real HTML uses. **`slidelang` is the only CLI that emits `--format json`** (doclang has no such path), so the parity "ground truth" is the HTML produced by `slidelang build --format html` (`internal/generator/template/base.go`), not `renderer.RenderElementToHTML` (used only by doclang). Both pipelines match field by field except for two deliberate exceptions, where `PopulateInlineHTML` follows slidelang's real pipeline:

- `special_block.titleHTML` **does** apply Markdown (slidelang's template uses `{{.Title | markdown}}`), unlike `RenderElementToHTML` (used only by doclang), which doesn't.
- `code_group.codeBlocks[].labelHTML` substitutes `{{variables}}` (matching slidelang's real pipeline), unlike `RenderElementToHTML`, which doesn't substitute variables in the tab label.

This is additive: raw content is never modified or removed.

**Non-goals** (remain raw scalars, no `*HTML` variant): image `source`/`src` (the viewer decides escaping based on the attribute context it inserts it into), colors, numeric fields, and diagram content/config (`mermaid.content`, `plantuml.content`, `chart.data`/`series`/`labels`/`rawJSON`, `map.markers`) — this isn't text meant for DOM insertion. See `core/renderer/populate_inline_html_test.go` for the anti-drift test that compares every `*HTML` field against the equivalent fragment from `RenderElementToHTML` (except for the two documented exceptions above, which are covered by dedicated tests).

## `checklist_item` discriminator (issue #60, since `SchemaVersion 2.0.0`)

Before 2.0.0, a `checklist` item serialized with `"type": "point_item"`, sharing a discriminator with `PointItem` (an item of a `points` list) despite having a different shape (`checked` is required). Since 2.0.0, `ChecklistItem` has its own `"type": "checklist_item"` discriminator.

## Classes and HTML structure are NOT part of the contract (issue #16)

Only `--format json` (this document) is the stable contract. The `slidelang-*` CSS classes (and any structure of the generated HTML — `<div>` hierarchy, wrappers, `id` names) are **internal implementation detail**, not a versioned API, and may change between releases without notice or a `SchemaVersion` bump (which only covers the JSON/AST tree).

**Why:** slidelang's HTML generator (`slidelang/internal/generator/`) uses the `slidelang-` prefix for defensive namespacing (avoiding collisions with third-party CSS when the presentation is embedded), not as a public-API convention. The code itself is inconsistent about what it namespaces: for example, before this change, the `checklist` element's classes (`checklist-item`, `checklist-checkbox`, `checklist-content`, etc.) were emitted **without** the `slidelang-` prefix, while others (`slidelang-image-caption`, `slidelang-quote-author`) did carry it — evidence that the class set was never designed as a coherent contract and must not be treated as one. (A now-removed historical planning doc once claimed namespacing was "100% completed" — that claim was made stale by this very finding, which is exactly why session-summary-style documents get retired rather than kept as reference.)

**What this means for consumers** (e.g. the `slidelang-app` web viewer, which historically parsed the HTML): any CSS/DOM selector on the generated HTML can break in any release, including a patch release. The migration to `--format json` (this document) exists precisely to remove that dependency — use the `content`/`items`/`markers`/etc. fields (and their pre-rendered `*HTML` variants, see above) instead of scraping the DOM.

If the HTML ever needs to be exposed as a real contract (option B of issue #16, not adopted), the full class set would need to be documented with an explicit versioning policy — given that the current generator has ~300 distinct `slidelang-*` classes, including Tailwind-style utilities (`slidelang-flex`, `slidelang-mt-4`, etc.) never meant to be public API, that option would mean freezing a much larger surface than any consumer actually needs.
