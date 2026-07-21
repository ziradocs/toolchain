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

`Element` is a union discriminated by the `type` field (`"text" | "points" | "code" | "image" | "table" | "special_block" | "code_group" | "mermaid" | "plantuml" | "chart" | "map" | "quote" | "checklist" | "grid" | "column" | "directive" | "math"`), reflecting the 17 Go structs that implement `ast.Element`.

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

### 2.0.0 (issues #60, #64)

- **Breaking**: `ChecklistItem.type` changed from `"point_item"` (shared, ambiguous, with `PointItem`) to its own `"checklist_item"`.
- **Additive**: prose fields (`content`, `title`, `author`, `source`, `headers`, `rows`, `caption`, `alt`) gained an optional sibling `*HTML` field with the same prose already rendered to inline HTML (Markdown applied, `{{variables}}` substituted and escaped) — see [`docs/architecture/json-ast-contract.md`](../docs/architecture/json-ast-contract.md#pre-rendered-html-fields-issue-64-since-schemaversion-200).
