# @ziradocs/ast-types

TypeScript types for the JSON/AST contract emitted by `slidelang build --format json` (see [`docs/architecture/json-ast-contract.md`](../docs/architecture/json-ast-contract.md) and [`schema/ast.schema.json`](../schema/ast.schema.json)).

The types are generated from the Go structs in `core/ast/` — never hand-written — so the TypeScript contract can never silently drift from the real AST produced by the CLI.

## Installation

> **Status:** the package is publish-ready (scoped, `publishConfig.access: public`, CI-guarded) but not yet published to the npm registry. Until the first release lands, consume it via a relative path or a local `npm pack` — `npm install` below will 404 until then.

```sh
npm install --save-dev @ziradocs/ast-types
```

## Usage

```ts
import type { AST, ContentBlock, Element } from "@ziradocs/ast-types";

const doc: AST = JSON.parse(rawJson);
for (const block of doc.contentBlocks) {
  for (const el of block.elements) {
    // `el` is the discriminated union `Element`; use `el.type` to narrow it
    if (el.type === "quote") {
      console.log(el.content, el.author);
    }
  }
}
```

`Element` is a union discriminated by the `type` field (`"text" | "points" | "code" | "image" | "table" | "special_block" | "code_group" | "mermaid" | "plantuml" | "chart" | "map" | "quote" | "checklist" | "grid" | "column" | "directive" | "math" | "media"`), reflecting the 18 Go structs that implement `ast.Element`.

## Regenerating the types

Requires Go and [`tygo`](https://github.com/gzuidhof/tygo) installed:

```sh
go install github.com/gzuidhof/tygo@latest
npm run generate   # runs tygo generate + scripts/postprocess.cjs
npm run build      # typecheck + emits dist/
```

`tygo` parses the Go structs directly (no reflection), so it needs two manual adjustments applied by `scripts/postprocess.cjs` after generation:

1. **`Element` is a Go interface** — tygo can't enumerate its implementers via static parsing, so it emits `Element = Node` (`any`). The script replaces it with the real discriminated union. The type list must stay in sync with the `func (X) element() {}` methods in `core/ast/nodes.go`/`directives.go` and with the `elementTypes` list in `core/cmd/gen-schema/main.go`.
2. **`diagnostics.Position`** is generated into a separate file (`generated/diagnostics.ts`); the script adds the `import type { Position } from "./diagnostics"` that tygo doesn't automatically add across packages.

CI (`.github/workflows/schema-drift.yml`) runs this regeneration on every PR that touches `core/ast/` and fails if the result diverges from what's committed — so an AST change that doesn't update `ast-types` (or `schema/ast.schema.json`) can't be merged unnoticed.

## Compatibility policy

`schemaVersion` (a field on the root document, see `ast.SchemaVersion` in Go) is semver:

- **MAJOR**: a breaking change to the serialized shape — a field removed or renamed, a field's type changed, a discriminator (`type`) value changed or reordered incompatibly.
- **MINOR**: a new optional field added, a new element type added to the `Element` union.
- **PATCH**: fixes that don't change the JSON shape (e.g. fixing a serialization bug so it matches what the schema already documented).

The package's `MAJOR.MINOR` tracks `schemaVersion`'s `MAJOR.MINOR` 1:1 (CI fails if they drift — see `schema-drift.yml`). `PATCH` is free to diverge for packaging-only releases (e.g. fixing `package.json` metadata) that don't touch the generated types.

### 2.5.0 (issue #100)

- **Additive**: `frontMatter.numbering` (`boolean | undefined`) — a tri-state override for DocLang's section auto-numbering: `true`/`false` for an explicit opinion in the document itself, or the field absent for "no opinion" (distinct from `false`), leaving the `--numbering`/`--numbering=false` CLI flag or its own default to decide. Has no effect on SlideLang output — section numbering is a DocLang-only concept. See `llm-kit/reference/frontmatter.md` for the frontmatter YAML shapes this maps from (including a legacy map form kept for backward compatibility).

### 2.4.0 (issue #63 code review, finding #9)

- **Additive**: the `langRuns` field (`LangRun[]`) added on `SpecialBlockElement`, `GridElement`, and `ColumnElement` — each also carries its own loose `content` prose that the 2.3.0 population pass was skipping. Same derivation and posture as 2.3.0's `langRuns` fields.

### 2.3.0 (issue #63)

- **Additive**: new `LangRun` type (`text`, `lang`) plus a `langRuns` field (`LangRun[]`) on `TextElement`, `PointItem`, `ChecklistItem`, and `QuoteElement` — exposes `[texto]{lang=xx}` inline spans as structured runs, so a rulepack can flag a passage marked in a different language than the document's `frontMatter.lang` without re-parsing rendered HTML. Derived fresh from `content` on every build; unlike the `*HTML` fields, not cleared when an external `--filter` runs (there's nothing pre-rendered here to distrust — it's always re-derived, never carried over from the filter's output).

### 2.2.0 (issues #62/#63 prerequisite)

- **Additive**: `frontMatter.lang` — the document's declared language as a first-class BCP 47 field, so a renderer can emit a real `<html lang>` and a rulepack (e.g. `A11Y005`) can read it without depending on the author having written it into the free-form `variables` map. Deliberately not folded into `variables` — see the field's own doc comment in `core/ast/nodes.go`.

### 2.1.0 (issues #22, #20, #21 — A11Y AST seams)

- **Additive**: `TextElement.level` — the heading level (1-6) for a `##`-`######` heading, so an A11Y rulepack can check heading order/nesting without re-parsing the rendered `<hN>` in `content`.
- **Additive**: `TableElement.cells` — real cell structure (`content`, `isHeader`, `scope`, `colSpan`, `rowSpan`) alongside the existing `headers`/`rows`, which are unchanged for compatibility. Populated for every table, including simple ones (derived from `headers`/`rows`), so a rulepack always has cell-level structure to walk.
- **Additive**: new `MediaElement` (`type: "media"`) for embedded audio/video (`mediaType`, `source`, `autoplay`, `controls`, `loop`, `muted`), exposing autoplay/controls as first-class fields so a rule can flag autoplay content with no controls exposed to the user.

### 2.0.0 (issues #60, #64)

- **Breaking**: `ChecklistItem.type` changed from `"point_item"` (shared, ambiguous, with `PointItem`) to its own `"checklist_item"`.
- **Additive**: prose fields (`content`, `title`, `author`, `source`, `headers`, `rows`, `caption`, `alt`) gained an optional sibling `*HTML` field with the same prose already rendered to inline HTML (Markdown applied, `{{variables}}` substituted and escaped) — see [`docs/architecture/json-ast-contract.md`](../docs/architecture/json-ast-contract.md#pre-rendered-html-fields-issue-64-since-schemaversion-200).
