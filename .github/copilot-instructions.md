# GitHub Copilot Instructions for SlideLang/DocLang

This repository contains the SlideLang and DocLang domain-specific languages (DSLs) for creating
professional presentations and documents. See `CLAUDE.md` at the repo root for the authoritative,
actively-maintained version of this guidance — this file mirrors it in Copilot's expected format.

## 🏗️ Project Architecture

A Go monorepo, **three independent Go modules, no `go.work`**:

1. **`core/`** (module `go.ziradocs.com/core`) — shared library
   - Parser for `.slidelang` and `.doclang` files (`parser/`)
   - AST definitions (`ast/`)
   - Renderer: HTML/Markdown generation (`renderer/`)
   - Element definitions — `internal/elements/` (not importable outside this module)
   - Content normalizer — `internal/normalize/normalizer/` (heuristic rules, **not** LLM-based,
     despite the historical `--enable-ai`/`flex-ai` flag names)

2. **`slidelang/`** (module `go.ziradocs.com/slidelang`) — presentation CLI
   - Entry point: `cmd/slidelang/`
   - Slide-specific generator: `internal/generator/`
   - Themes: `themes/`

3. **`doclang/`** (module `go.ziradocs.com/doclang`) — document CLI
   - Entry point: `cmd/doclang/`
   - Document-specific generator: `internal/generator/`

### Key Architectural Principles

- **Code Reuse**: parsing, AST, rendering, and linting are shared through `core`
- **Consistency**: both CLIs consume the same parser/renderer contract
- **Separation**: CLI-specific logic stays in each CLI's own `internal/`
- Each CLI's `go.mod` has `replace go.ziradocs.com/core => ../core`, so editing `core` is picked
  up immediately by both CLIs — no extra step needed (this changes once `core` has a tagged
  release; see `CONTRIBUTING.md`).

## 🔧 Development Setup

There is no root-level module — `go build ./...` at the repo root fails. Always `cd` into a
specific module first.

### Building the Project

```bash
cd core && go build ./...

cd slidelang && go build -o slidelang ./cmd/slidelang

cd doclang && go build -o doclang ./cmd/doclang
```

### Running Tests

```bash
cd core && go test ./...
cd slidelang && go test ./...
cd doclang && go test ./...
```

### Running the CLIs

```bash
cd slidelang
./slidelang build ../examples/02_diagrams_and_charts/02_diagrams_and_charts_flex.slidelang
./slidelang build slides.slidelang --theme modern-blue

cd doclang
./doclang build ../examples/advanced_elements_test.doclang --output output
./doclang build doc.doclang --toc --numbering
```

## 📝 Coding Standards

- **Formatting**: always `gofmt` before committing
- **Linting**: `go vet ./...` per module; CI also runs `golangci-lint` (see `.golangci.yml`)
- **Naming**: idiomatic Go — `PascalCase` exported, `camelCase` unexported, interfaces as noun or
  `-er` suffix (`Parser`, `Renderer`)
- **Comments/log messages**: much of this codebase is in **Spanish** — match the surrounding
  language when editing existing files; prefer English for brand-new files (see `CONTRIBUTING.md`
  § Conventions)

### Project-Specific Conventions

- Keep CLI-specific code out of `core` — it's the shared library both CLIs depend on
- Renderer changes → `core/renderer/`
- Parser changes → `core/parser/`
- New element types → `core/internal/elements/` (register in `GetDefaultRegistry()`, add an
  `ast.NodeType`, add renderer support)
- CLI-specific features → that CLI's own `internal/`

### File Organization

```
core/
├── parser/                          # Parsing .slidelang/.doclang files
├── ast/                             # AST node definitions
├── renderer/                        # HTML/Markdown generation; sanitizer
├── linter/ · diagnostics/ · config/ · util/
├── spec/                            # Formal language spec + JSON/AST contract
└── internal/
    ├── elements/                    # Element type definitions + parser registry
    └── normalize/normalizer/        # Heuristic content normalization (no LLM)

slidelang/
├── cmd/slidelang/                   # Main entry point
├── internal/generator/              # Slide generation logic
└── themes/                          # Presentation themes

doclang/
├── cmd/doclang/                     # Main entry point
├── internal/generator/              # Document generation logic
└── themes/document/                 # Document themes
```

## 🧪 Testing Guidelines

- **Test files**: `*_test.go` in the same directory as the code under test
- **Table-driven tests** for multiple cases
- Most tests live in `core` (parser, elements, renderer, normalizer). If you touch `core`, also
  run the test suites of both CLIs.

```bash
go test -run TestFunctionName ./parser/
go test -v -cover ./...
```

## 🚀 Key Features to Understand

Both SlideLang and DocLang support: titles/text/lists/code blocks, inline Markdown formatting,
Mermaid diagrams, Chart.js charts, Leaflet maps, WebP images with fallbacks, and grid layouts.

### Rendering Pipeline

1. **Parse**: source file → AST (`parser/`)
2. **Normalize** (slidelang only, `flex`/`flex-full`/`auto` modes): heuristic rules repair loose
   Markdown-like input into valid elements (`internal/normalize/normalizer/`)
3. **Render**: AST → HTML/Markdown (`renderer/`)
4. **Generate**: CLI-specific output (HTML/PDF/DOCX/Markdown)

### Theme System

- Themes live in `slidelang/themes/` and `doclang/themes/document/`
- Resolution priority: CLI `--theme` flag > frontmatter `theme:` > default

## 🐛 Common Issues and Solutions

### Import Errors

```bash
cd core && go mod tidy
cd slidelang && go mod tidy
cd doclang && go mod tidy
```

### Build Failures

- Go 1.26.5+ required (see each module's `go.mod`)
- Run `go mod tidy` in the affected module
- Check for syntax errors with `go vet`

### Test Failures

- Some pre-existing tests may fail unrelated to your change — focus on tests touching your area

## 📚 Documentation

- **User docs**: `docs/user/`, `docs/doclang/` (migrating to `docs.ziradocs.com` — see
  `ziradocs/website`)
- **Developer/architecture docs**: `docs/developer/`, `docs/architecture/`
- **Contributing**: `CONTRIBUTING.md` at the repo root

## ⚠️ Important Notes

- Prefer creating new commits over force-pushing or rebasing published branches
- Discuss breaking changes before implementing
- Aim for high test coverage on new code

## 🔗 Notable Dependencies

- `gopkg.in/yaml.v3` — YAML parsing (frontmatter); kept over alternatives per issue #35 (a
  migration attempt caused real regressions)
- `github.com/chromedp/chromedp` — headless Chrome, for PDF/offline diagram-chart-map rendering
- `github.com/mmonterroca/docxgo` (doclang), `github.com/mmonterroca/pptxgo` (slidelang) — DOCX/PPTX
  output backends

## 📞 Getting Help

- **Issues / Discussions**: use the repository's GitHub Issues and Discussions
- **Contributing Guide**: see `CONTRIBUTING.md`

---

**Remember**: this is a multi-module monorepo with shared core logic. Changes to parsing,
rendering, or element definitions go in `core/` to benefit both CLIs.
