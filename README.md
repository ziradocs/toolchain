# ZiraDocs / DocLang

[![GitHub Release](https://img.shields.io/github/v/release/ziradocs/toolchain?style=flat-square&color=blue)](https://github.com/ziradocs/toolchain/releases)
[![Go Reference](https://pkg.go.dev/badge/go.ziradocs.com/core.svg)](https://pkg.go.dev/go.ziradocs.com/core)

Two sibling DSLs for generating **presentations** and **documents** from plain-text
files, built on a shared Go library.

- **`core`** — shared library (parser, AST, elements, renderer, linter, normalizer).
- **`slidelang`** — presentations CLI (`.slidelang` → HTML/PDF).
- **`doclang`** — documents CLI (`.doclang` → HTML / PDF / DOCX / Markdown).

> **Status:** Functional. There is known, prioritized *correctness* debt tracked in the
> [issue tracker](https://github.com/ziradocs/toolchain/issues) — see especially the open items
> labeled `security`/`audit-2026-07`.

## Quick Start

Requirements: **Go 1.26.5+** (version floor for stdlib CVEs patched up to that patch — see
`govulncheck.yml`). PDF generation and *offline* rendering modes require
Chrome/Chromium (or use `--install-chromium`).

```bash
# ZiraDocs — presentations
cd slidelang
go build -o slidelang ./cmd/slidelang
./slidelang build ../examples/02_diagrams_and_charts/02_diagrams_and_charts_flex.slidelang
./slidelang build slides.slidelang --theme modern-blue --format html

# DocLang — documents
cd doclang
go build -o doclang ./cmd/doclang
./doclang build ../examples/advanced_elements_test.doclang --output output
./doclang build report.doclang --format docx --toc --numbering
```

## Output formats

| CLI       | Format     | Status                                                          |
|-----------|------------|-------------------------------------------------------------------|
| slidelang | `html`     | Supported (effective default)                                    |
| slidelang | `json`     | Supported (AST dump)                                             |
| slidelang | `pdf`      | Supported (via Chromium)                                         |
| slidelang | `pptx`     | MVP (text, bullets, tables and images only; other elements skipped) |
| doclang   | `html`     | Supported (default)                                              |
| doclang   | `pdf`      | Supported (via Chromium)                                         |
| doclang   | `docx`     | Supported with caveats (no clickable hyperlinks/bookmarks/shading yet) |
| doclang   | `markdown` | Partial (skips PlantUML/Map/CodeGroup/Math; charts as a placeholder) |

## Try it in the browser (Playground)

A no-install, no-backend way to try both DSLs: a `.wasm` build of the
parse → lint → render pipeline with a live-preview editor. Run it locally:

```bash
./playground/build.sh
cd playground && python3 -m http.server 8080   # then open http://localhost:8080/
```

See the **[Playground guide](docs/user/guides/playground.md)** for what it
does and its known limits, or **[`playground/README.md`](playground/README.md)**
for full build/export details.

## Syntax modes

**slidelang** picks its mode from the frontmatter `mode:` field (or `--mode`):

- `strict` — structured keyword-driven syntax (`TEXT`, `POINTS`, `CODE`, …).
- `flex` — extended Markdown.
- `flex-full` / `auto` — Markdown + heuristic normalization (reorders/repairs syntax). `flex-ai` still works as a deprecated alias for `flex-full`.

> The content normalizer **does not use LLMs**: it's heuristic rules
> (`core/internal/normalize/normalizer/`) that detect loose Markdown-like content and
> convert it into valid elements. (Historical flag/mode names like `--enable-ai`/`flex-ai` predate
> this naming and are kept as deprecated aliases.)

**doclang** always uses `DocumentFlexParser`: it interprets `#` / `##` / `###` as a
**section** hierarchy (sub-levels nest, they are not separate sections) and works with or without frontmatter.

## Generating with LLMs / AI agents

Any LLM/agent (bring your own model) can emit valid ZiraDocs/DocLang using
the calibrated prompts and syntax reference in **[`llm-kit/`](llm-kit/)** —
system prompts, per-target grammar, use-case skeletons, and a
linter-rule-mapped validation checklist. See the
**[Generating with LLMs guide](docs/user/guides/generating-with-llms.md)**
for a quick start, or **[`llms.txt`](llms.txt)** for the machine-readable
entry point.

## Themes

- **slidelang** (`slidelang/themes/`): `modern-blue`, `cyberpunk-neon`, `elegant-minimal`,
  `aurora-holographic`, `neomorphism-glass`, `startup-tech`, `startup-tech-solid`.
- **doclang**: `professional` (default), `academic`, `technical`, `page-view`.
- Resolution priority in both: **`--theme` flag > frontmatter `theme:` > default**.

## Rendering interactive elements (doclang)

Diagrams (Mermaid, PlantUML), charts (Chart.js) and maps (Leaflet) are controlled with `--render-mode`:

- `browser` (default) — links to CDNs and renders in the browser. Small files; requires internet.
- `offline-assets` — renders at build time into image files under an `assets/` folder. Portable.
- `offline-inline` — same, but embedded in the HTML. Single file, heavier.

Related flags: `--install-chromium`, `--chromium-path`, `--image-format png|webp`,
`--webp-quality`, `--plantuml-server`, `--plantuml-format`.

## Evidence Pipeline & Linting (v2.0+)

Both `slidelang` and `doclang` include a built-in strict linter and an **Evidence Pipeline** to integrate with CI/CD and security tools.

- **Machine-Readable Reports**: Use `--report sarif` or `--report json` to generate standards-compliant diagnostic reports. By default, it outputs to stdout, which is ideal for piping into uploaders: `slidelang build ... --report sarif | uploader`
- **Waivers**: Suppress specific diagnostic warnings via a `lint_policy` block in the document frontmatter or via an external `--lint-config` file.
- **External Rulepacks**: Inject third-party or proprietary checks by passing a CLI tool (e.g. `--rulepack path/to/binary`). The toolchain pipes the document AST and merges the returned findings into the final report with full provenance.

Example:
```bash
slidelang build slides.slidelang --report sarif --report-out evidence.sarif
```

## Security

The renderer escapes **element content** (HTML sanitization and blocking of dangerous URL
protocols — `javascript:`/`data:`/`vbscript:`/`file:` — see `core/renderer/sanitizer.go`,
[SECURITY.md](SECURITY.md)). A [2026-07 security audit](docs/SECURITY_AUDIT_2026-07.md) found a
broad set of XSS/SSRF/DoS issues across both CLIs' default output; nearly all were fixed in the
following days (see the audit doc's own status note, and closed issues labeled `security` +
`audit-2026-07` in the [issue tracker](https://github.com/ziradocs/toolchain/issues?q=label%3Aaudit-2026-07)
for what was found and fixed). A handful of lower-severity hardening items (dependency
governance, signed releases/SAST in CI) remain open — see the same label set.

## Development

### Structure

```
core/               # Shared library
├── parser/         # Parsing of .slidelang / .doclang (+ mode selection)
├── ast/            # AST nodes (FrontMatter + []ContentBlock)
├── renderer/       # HTML/Markdown; Chromium fetchers; sanitizer
├── linter/ · diagnostics/ · config/ · util/
├── spec/           # Formal language spec + AST/JSON contract
└── internal/       # Not importable outside this module (see doc.go)
    ├── elements/       # Element types + parser registry (priority order)
    └── normalize/normalizer/  # Heuristic content normalization rules (no LLM)

slidelang/          # Presentations CLI (slide generator + modular CSS/JS + themes)
doclang/            # Documents CLI (uses renderer.GenerateDocumentHTML + DOCX/MD generators)
```

### Build and test

> **There is no `go.work`.** Each CLI uses `replace … => ../core`, so
> **`go build ./...` from the repo root fails**: you need to `cd` into each module. This
> will change once `core` has its first tagged release — see `CONTRIBUTING.md`.

```bash
# Library
cd core && go build ./... && go test ./...

# CLIs
cd slidelang  && go build -o slidelang ./cmd/slidelang && go test ./...
cd doclang    && go build -o doclang   ./cmd/doclang   && go test ./...

# A single test / a single package
go test -run TestName ./parser/
go test -v -cover ./...
```

### Where changes go

- Parsing → `core/parser/`
- New element type → `core/internal/elements/` (+ register it in `common.go` + renderer)
- Rendering → `core/renderer/`
- Normalization rules → `core/internal/normalize/normalizer/rules/`
- CLI-specific logic → that CLI's `internal/` (not in `core`)

## Documentation

- **[Language spec](core/spec/)** — formal DSL specification (v0.1) and the JSON/AST contract.
- **[DocLang Overview](docs/doclang/DOCLANG_OVERVIEW.md)** — introduction to DocLang.
- **[User Guide](docs/user/)** · **[Architecture](docs/architecture/)** · **[Contributing](CONTRIBUTING.md)**
- **[Backlog / open issues](https://github.com/ziradocs/toolchain/issues)** — bugs, security, features, refactors.

## License

See [LICENSE](LICENSE).
 
