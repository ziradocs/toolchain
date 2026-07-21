# SlideLang Core

**SlideLang Core** is the shared Go engine (parser, AST, renderer, linter) behind both the `slidelang` and `doclang` CLIs.

## 🎯 Consumption model: invoke the CLI, not the library

SlideLang/DocLang are designed to be used as executables, not as an embedded Go library:

```bash
slidelang build presentation.slidelang --format html
doclang build document.doclang --format docx
```

The Go packages in this module (`parser`, `renderer`, `ast`, `config`, …) exist to be consumed by `slidelang` and `doclang` — the two sibling CLIs in this monorepo — not by third-party Go programs. See [`doc.go`](doc.go) for the full API stability policy.

## ✅ Stable public contracts

What this project commits to maintaining and versioning:

1. **The CLI interface** — subcommands, flags, input formats (`.slidelang`, `.doclang`), output formats (`html`, `json`, `pdf`, `pptx`, `docx`, `markdown`; which ones apply depends on the CLI — see the root README's format table).
2. **The AST serialized via `--format json`**, versioned semver by `ast.SchemaVersion` (see [`../schema/ast.schema.json`](../schema/ast.schema.json) and the `@ziradocs/ast-types` npm package). This is the recommended integration point for third parties — AI agents generating SlideLang, the web viewer, or any external consumer of the content tree. See [`docs/architecture/json-ast-contract.md`](../docs/architecture/json-ast-contract.md).
3. **A future WASM entrypoint** (issue #134) to run the parser/renderer in-browser, as a wrapper over this same module.

Generated HTML structure and CSS classes are **not** part of this contract and may change release to release without notice.

## ⚠️ The Go API is an internal implementation detail

No package/type/function exported from this module carries a semver stability guarantee — signatures can change in any minor version. The module is tagged `v0.x` deliberately (Go convention for "no API stability promised"). If a real need to embed this engine from another Go program emerges later, a stable subset can be curated and versioned at that point — promoting a symbol from unstable to stable doesn't break anyone; the reverse does.

`normalize/` and `elements/` live under `internal/` specifically because neither CLI imports them directly (only `parser` uses them internally) — Go's compiler enforces that no external module can import them.

## 🏗️ Package layout

```
core/
├── ast/             # AST node/element definitions — owns the JSON contract (SchemaVersion)
├── parser/          # Strict + Flex parsing, frontmatter
├── renderer/        # AST → HTML rendering, sanitizers, CSP, native chart/map rasterization
│   └── chromium/    # Headless-Chrome backend: PDF export, diagram fetchers
├── config/          # Theme/layout config model
├── linter/          # Lint rule engine
├── diagnostics/     # Position/Severity/Diagnostic primitives
├── util/            # Logger, path confinement, guards, bounded download
├── cmd/gen-schema/  # Regenerates ../schema/ast.schema.json (repo root) from the ast package
└── internal/        # Implementation detail, not importable outside this module
    ├── elements/    # Per-element parsers (chart, code, image, table, …)
    └── normalize/   # Heuristic content normalizer (no LLM, despite the historical flag names)
```

## 🧪 Testing

```bash
go test ./...
```

## 📚 Documentation

- [`doc.go`](doc.go) — API stability policy (start here)
- [`spec/`](spec/) — SlideLang/DocLang language specification and AST contract
- [`docs/architecture/json-ast-contract.md`](../docs/architecture/json-ast-contract.md) — the `--format json` contract in detail

## 📄 License

Apache-2.0 — see the repository root for the license and DCO sign-off requirements.

## 🔗 Related Projects

- [slidelang](https://github.com/ziradocs/toolchain/tree/main/slidelang) - Presentation CLI
- [doclang](https://github.com/ziradocs/toolchain/tree/main/doclang) - Document CLI
