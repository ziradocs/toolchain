# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go monorepo for two sibling DSLs that share one core library:

- **`.slidelang`** files → presentations (HTML/PDF), built by `slidelang`
- **`.doclang`** files → documents (HTML/PDF/DOCX/Markdown), built by `doclang`
- Both parse/render through **`core`** (parser, AST, elements, renderer, linter, content normalizer)

Note on the codebase language: comments, log messages, and many docs are in **Spanish**. Match the surrounding language when editing.

## Module layout & the go.mod gotcha

Three independent Go modules, each `/v2` (Go 1.26.5):

- `core/` — module `go.ziradocs.com/core/v2`
- `slidelang/`  — module `go.ziradocs.com/slidelang/v2`
- `doclang/`    — module `go.ziradocs.com/doclang/v2`

`slidelang`/`doclang` depend on a **published** `go.ziradocs.com/core/v2` (no `replace` directive) — CI, goreleaser, and `go install` for external consumers all fetch it over the network like any other module, no special `GOPROXY`/`GOPRIVATE` needed (it's a real public domain with a working vanity redirect). For local multi-module editing (change `core` and immediately build/test against it from a checkout with both directories side by side), a gitignored root `go.work` (`use ./core ./doclang ./slidelang`) takes over automatically — no extra command needed, same as `replace` used to provide.

**Consequence of no `replace`:** if you bump `core` and need slidelang/doclang to pick it up as a *dependency* (not just for local dev via `go.work`), you must cut a new `core/vX.Y.Z` release first, then bump the `require go.ziradocs.com/core/v2` line in `slidelang/go.mod`/`doclang/go.mod` to that version. Local `go.work` builds always use the working-tree `core`, tag or not — only `GOWORK=off` builds (CI, external installs) actually resolve the pinned `require` version over the network.

**Consequence of no root module:** you cannot build or test the whole repo from the root. `go build ./...` at the root fails. Always `cd` into the specific module first.

**The vanity import (`go.ziradocs.com`, separate `ziradocs/website` repo) required a fix for any of this to work at all.** The site's `go-import.astro` serves a 4-field meta tag per module (`prefix vcs reporoot subdir`, e.g. `go.ziradocs.com/core/v2 git https://github.com/ziradocs/toolchain core`), a form supported since Go 1.25 that declares the module's physical subdirectory explicitly instead of making `go` derive it by stripping the major-version suffix — the derivation is ambiguous for a `/v2`+ module living in a same-named subdirectory (`core/go.mod` declaring `.../core/v2`) and was breaking `go install` outright before this fix. Confirmed working end-to-end (clean module cache, no `GOWORK`, no `GOPRIVATE`) against real CI-shaped builds and `go install`. One thing worth knowing: requires the installing machine to have **Go ≥1.25** already (the initial go-import handshake can't self-upgrade); not a new burden here since this repo already requires 1.26.5.

**The `core/v2.1.0`, `slidelang/v2.1.0`, `doclang/v2.1.0` tags are permanently broken and must never be used or reused.** They were cut on 2026-07-21, before the `/v2` module-path migration (`5820531`, 2026-07-22), so the `go.mod` at those revisions still declares the unversioned path. Since v2.1.0 > v2.0.6 in semver, `@latest` (and even an explicit `@v2.0.6` request, since `go` enumerates all matching tags) resolved to the broken tag and failed — this is why the first valid post-migration release is `v2.1.1`.

## Common commands

Run each from inside its module directory:

```bash
# core (library) — no binary
cd core && go build ./... && go test ./...

# slidelang
cd slidelang && go build -o slidelang ./cmd/slidelang
./slidelang build ../examples/02_diagrams_and_charts/02_diagrams_and_charts_flex.slidelang
./slidelang build slides.slidelang --theme modern-blue --format html
./slidelang build slides.slidelang --lint-only        # parse+lint, no output
./slidelang themes                                     # list presentation themes

# doclang
cd doclang && go build -o doclang ./cmd/doclang
./doclang build ../examples/advanced_elements_test.doclang --output output
./doclang build doc.doclang --format docx --toc --numbering
./doclang build doc.doclang --render-mode offline-inline   # embed all assets, no CDN
```

Testing (per module):

```bash
go test ./...                        # all tests in the module
go test -run TestName ./parser/      # single test / single package
go test -v -cover ./...              # verbose + coverage
go vet ./...                         # static checks; gofmt for formatting
```

Most tests live in `core` (parser, elements, renderer, content normalizer). Some pre-existing tests may fail unrelated to your change — focus on tests touching your area.

## Pipeline & architecture

The end-to-end flow (`internal/cli/build.go` in each CLI drives it):

**read file → detect mode → parse (+ AI normalization) → AST → lint → generate output**

### Parsing differs between the two CLIs — this is the key design point

- **slidelang** uses `parser.New(logger).Parse(content, path)`. The parser switches on the frontmatter `mode:` field — `strict`, `flex`, `flex-full`, or `auto` (`flex-ai` is a permanently-supported deprecated alias for `flex-full`); `flex`/`flex-full`/`auto` trigger the content normalizer. Each `ContentBlock` is a **slide**. Note: a `.slidelang` file **must** start with a `---` frontmatter block — `parser.FrontMatterParser` returns a fatal "Missing FrontMatter delimiter" error otherwise (`parser/frontmatter.go:97`, `parser/parser.go:45`), even though `slidelang/internal/cli/build.go` assumes a bare file means `flex-full`. (doclang's `DocumentFlexParser` does tolerate no frontmatter.)
- **doclang** ignores mode switching and *always* uses `parser.NewDocumentFlexParserWithNormalization(content, logger)`. It treats `#`/`##`/`###` as a **section hierarchy** (subsections are nested, not separate blocks). Each top-level `ContentBlock` is a **section**.

Both produce the same `ast.AST` (`core/ast/ast.go`): a `FrontMatter` plus `[]ContentBlock`. `ContentBlock` is deliberately named generically because it is a slide in one CLI and a section in the other.

### Element registry (the extension point for content types)

`core/internal/elements/common.go` defines the `ElementParser` interface and `GetDefaultRegistry()`, which registers all element parsers **in priority order** (most specific first — e.g. `CodeGroupParser` before `CodeParser`, `GridParser` before `SpecialBlockParser`, `TextParser` last as fallback). Element types: text, points, code, code-group, image, table, quote, checklist, mermaid, plantuml, chart, map, grid, special-block, directive. Adding a new element = new file in `internal/elements/` + register it in `GetDefaultRegistry()` + add an `ast.NodeType` + renderer support. Note: `elements/` and `ai/` were moved under `internal/` (and `ai/` renamed `internal/normalize/`) since neither CLI imports them directly — see `core/doc.go` for the Go API stability policy this enforces.

### Rendering

- Shared document renderer: `core/renderer/document_html.go` exposes `GenerateDocumentHTML(doc, opts)`, used by **doclang** for HTML/PDF/DOCX.
- **slidelang** has its own slide generator in `slidelang/internal/generator/` (`html_modular.go`; the sibling `html.go`/`generateHTML` is dead code, no callers) with modular CSS/JS assets and theme support; it consumes `ast`/`config`/`util` from core but not the document renderer.
- `renderer/sanitizer.go` escapes all user HTML and blocks dangerous URL protocols (`javascript:`, `data:`, `vbscript:`, `file:`). Keep new rendered output going through it.
- Diagrams/charts/maps (`*_fetcher.go`) and PDF/offline rendering use **chromedp** (headless Chrome). doclang's `--render-mode`: `browser` (CDN links, needs internet), `offline-assets` (render at build time into an assets folder), `offline-inline` (embed everything in one file). `--install-chromium` auto-downloads Chromium.

### Content normalizer (`core/internal/normalize/`)

`internal/normalize/normalizer/` detects "AI-generated"-looking content (`detector.go`) and applies transformation rules organized by category: `rules/content/`, `rules/enhancement/`, `rules/frontmatter/`, `rules/structure/`. The `Parser` enables this by default; it's what makes loose Markdown-ish input parse into valid elements. `slidelang build --enable-ai` and the `flex-full` mode (formerly `flex-ai`, still accepted as a deprecated alias) force it on. Despite the historical `--enable-ai`/`flex-ai` flag names, this is **not** LLM-based — it's deterministic heuristic rules, which is why the package itself is named `normalize`, not `ai` (the `ai`/`elements` top-level dirs were moved under `internal/` — not importable outside this module — and `ai` renamed to `normalize` to stop implying an LLM dependency it never had).

## Where changes go

- Parsing logic → `core/parser/`
- New/changed element types → `core/internal/elements/` (+ registry + renderer)
- HTML/Markdown/document rendering → `core/renderer/`
- Normalization rules → `core/internal/normalize/normalizer/rules/`
- Presentation-only features → `slidelang/internal/`
- Document-only features (DOCX/PDF/markdown output) → `doclang/internal/generator/`

Keep CLI-specific code out of `core`.

## Output formats & themes

- slidelang: `html` (default), `json`, `pdf` (issue #59 — this was previously misdocumented as `json`-default, which the code never actually did). Themes live in `slidelang/themes/` (e.g. `modern-blue`, `cyberpunk-neon`, `elegant-minimal`, `startup-tech`).
- doclang: `html` (default), `pdf`, `docx`, `markdown`. Themes: `professional` (default), `academic`, `technical`, `page-view`.
- Theme resolution priority in both: **CLI `--theme` flag > frontmatter `theme:` > default**.

## Docs

Documentation lives under `docs/` (`docs/user/`, `docs/doclang/`, `docs/architecture/`,
`docs/developer/`). User-facing docs (`docs/user/`, `docs/doclang/`) are migrating to
`docs.ziradocs.com` (Starlight, in the separate `ziradocs/website` repo) — this repo keeps
developer/architecture docs and the formal spec (`core/spec/`) as source of truth.
