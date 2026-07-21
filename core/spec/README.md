# SlideLang / DocLang spec — v0.1

**Spec v0.1, tracks `ast.SchemaVersion` 2.0.0.**

This directory is the entry point for the formal specification of the SlideLang/DocLang
language family. It lives with the core library (`core`) rather than in a
separate repo because the AST contract — the spec's most load-bearing piece — is
generated directly from the Go structs here; splitting it out would create drift
between the spec and the implementation it describes. **This directory is the source of
truth.** docs.ziradocs.com will host a rendered, browsable version of this spec, but the
Markdown here is what gets edited — the site consumes it, it doesn't fork it.

The spec documents two levels, per the product positioning: the **AST is the open
intermediate representation**; the **DSL syntax is an authoring interface** on top of it.
Both are documented, but the AST/JSON contract below is the one with an explicit
semver compatibility policy — the DSL syntax docs are living documentation, not a
versioned wire format.

**The Go package API of `core` is explicitly out of scope of this
stability promise.** `ast.SchemaVersion` versions the *serialized JSON* shape,
not the Go types/functions that produce it — the Go API itself is an internal
implementation detail with no compatibility guarantee (see
[`doc.go`](../doc.go)). Don't infer that a stable `SchemaVersion` implies a
stable `import "go.ziradocs.com/core/ast"`.

## Contents

- **[Language Specification](language-specification.md)** — the formal grammar and
  semantics of `.slidelang` syntax (Strict Mode and Flex Mode), directives, special
  blocks, and validation rules. `.doclang` is a sibling dialect: it always parses in
  the flex family (`DocumentFlexParser`) and treats `#`/`##`/`###` headings as a nested
  **section** hierarchy rather than separate slides — see
  [`docs/user/language-reference/`](../../docs/user/language-reference/) for the
  user-facing (non-formal) walkthrough of both.
- **[JSON/AST contract](../../docs/architecture/json-ast-contract.md)** — the
  authoritative, versioned description of what `--format json` emits: which fields
  each element serializes, the pre-rendered `*HTML` fields, and the semver
  compatibility policy for `SchemaVersion` (MAJOR/MINOR/PATCH rules).
- **[`schema/ast.schema.json`](../../schema/ast.schema.json)** — the JSON Schema
  generated from the AST Go structs (`core/cmd/gen-schema`), the
  machine-readable counterpart to the JSON/AST contract doc. CI
  (`.github/workflows/schema-drift.yml`) fails any PR that changes
  `core/ast/` without regenerating this file (and `ast-types/`) to match —
  so the schema in this repo is always current, never a stale copy.

## Where to look for what

| I want to know...                                    | Read...                                                        |
|--------------------------------------------------------|-----------------------------------------------------------------|
| How do I write `.slidelang`/`.doclang` files?           | [`docs/user/language-reference/`](../../docs/user/language-reference/) (practical, example-driven) |
| What is the formal grammar of the DSL?                  | [`language-specification.md`](language-specification.md) (this directory) |
| What does `--format json` actually emit, field by field? | [`docs/architecture/json-ast-contract.md`](../../docs/architecture/json-ast-contract.md) |
| What's the machine-readable JSON Schema for the AST?     | [`schema/ast.schema.json`](../../schema/ast.schema.json) |
| Is a change to the AST breaking?                         | The MAJOR/MINOR/PATCH compatibility policy in [`ast-types/README.md`](../../ast-types/README.md#compatibility-policy) |

## Versioning

`ast.SchemaVersion` (`core/ast/ast.go`) is the single source of truth for the
AST contract's semver version, currently **2.0.0**. The spec's own version (**v0.1**)
tracks the maturity of this documentation set, not the AST — a spec version bump means
"the spec now documents things that were previously true but undocumented, or restructures
how it's presented," while an `ast.SchemaVersion` bump means "the JSON shape itself
changed." The two are expected to evolve at different rates; the spec always documents
whatever the current `ast.SchemaVersion` is, and will call out which SchemaVersion a given
spec revision tracks (see the footer of [`language-specification.md`](language-specification.md)).

Spec v0.1 is a first formalization pass, not a finished document: `language-specification.md`
predates this reorganization and still has minor drift from the current implementation
(flagged inline where known). Treat the JSON/AST contract and `schema/ast.schema.json`
as authoritative wherever they and the language specification disagree.
