# doclang

The documents CLI: `.doclang` source in, HTML / PDF / DOCX / Markdown out.

Parsing, linting and rendering all live in [`core`](../core/); this module owns the DOCX and
Markdown generators, the document themes, and the CLI itself. For the language itself see the
[DocLang documentation](../docs/doclang/) and the [formal spec](../core/spec/).

## Install

```bash
go install go.ziradocs.com/doclang/cmd/doclang@latest
```

Or build from a checkout of this repo:

```bash
cd doclang
go build -o doclang ./cmd/doclang
```

Requires Go 1.26.5+. `--format pdf` and the offline rendering modes additionally need
Chrome/Chromium — see [Interactive elements](#interactive-elements).

## How `.doclang` differs from `.slidelang`

Same element vocabulary, different document model — worth knowing before you write:

- **Sections, not slides.** `#`/`##`/`###` form a nested section hierarchy; sub-levels nest
  inside their parent rather than starting a new top-level block.
- **No mode switching.** doclang always parses with `DocumentFlexParser`. A `mode:` key in
  the frontmatter is ignored — there is no strict mode here.
- **Write a frontmatter block.** The parser itself tolerates a file without one, but the
  linter's `FRONT003` rule reports a missing frontmatter as an *error*, so `doclang build`
  exits non-zero on a bare file. Until that is reconciled, treat frontmatter as required —
  or disable `FRONT003` via `--lint-config`.

## Commands

| Command | What it does |
|---|---|
| `doclang build <file>` | Parse, lint and generate output |
| `doclang init [name]` | Scaffold a new document |
| `doclang fmt [file]` | Rewrite a file as canonical source |
| `doclang mcp` | Run the MCP server over stdio, for editor/agent integration |

## build

```bash
doclang build report.doclang
doclang build report.doclang --format docx --toc --numbering
doclang build report.doclang --render-mode offline-inline
doclang build report.doclang --lint-only          # diagnostics only, no output
```

### Output

| Flag | Default | Notes |
|---|---|---|
| `--format`, `-f` | `html` | `html`, `pdf`, `docx`, `markdown`. |
| `--output`, `-o` | `./output` | Output directory. |
| `--toc` | `false` | Generate a table of contents. |
| `--numbering` | `false` | Number sections. |
| `--page-breaks` | `false` | Break pages between sections. |

Format caveats worth knowing up front: **DOCX** does not yet emit clickable hyperlinks,
bookmarks or cell shading. **Markdown** is partial — it skips PlantUML, maps and code
groups, and emits charts as a placeholder fence.

### Themes

`professional` (default), `academic`, `technical`, `page-view`, selected with `--theme` or a
frontmatter `theme:` key; the flag wins.

### Parsing and linting

doclang runs the same linter and the same rule set as `slidelang` — `TABLE003`,
`CODEGROUP001`, `IMG001` and the rest all apply to `.doclang` source.

| Flag | Default | Notes |
|---|---|---|
| `--lint-only` | `false` | Run the linter and stop; generate nothing. |
| `--lint-config` | — | YAML lint policy (enable/disable rules, override severity by ID). Falls back to a `lint_policy:` block in the document's own frontmatter. |
| `--filter` | — | External binary that transforms the AST between parse and lint. Repeatable; communicates over JSON on stdin/stdout — see the [JSON/AST contract](../docs/architecture/json-ast-contract.md). |
| `--include-root` | *(input file's dir)* | Directory `@include` paths are confined to. Absolute paths and `..` escapes are rejected. |
| `--asset-root` | *(input file's dir)* | Same confinement for local image sources. |
| `--max-size` | 10 MB | Input size cap. Also settable via `DOCLANG_MAX_SIZE`. |

### Interactive elements

Diagrams (Mermaid, PlantUML), charts (Chart.js) and maps (Leaflet) are controlled by
`--render-mode`:

- `browser` (default) — reference CDNs and render client-side. Small files, needs internet.
- `offline-assets` — rasterize at build time into an assets folder. Portable.
- `offline-inline` — same, embedded in the HTML. One self-contained file, larger.

Related flags: `--chromium-path`, `--install-chromium`, `--image-format png|webp`,
`--webp-quality`, `--plantuml-server`, `--plantuml-format svg|png`.

Note that a single binary is enough for HTML and Markdown, but PDF, DOCX and the offline
modes reach for headless Chromium — `--install-chromium` fetches a pinned build if you
don't have one.

## MCP server

`doclang mcp` speaks MCP over stdio and exposes four tools — `lint`, `get_ast`,
`list_themes` and `preview`. Its `lint` applies the resolved lint policy, so it reports the
same diagnostics `--lint-only` would.

## Development

`doclang` depends on `core` through a `replace` directive, so a checkout with both
directories side by side picks up local `core` changes with no extra step. There is no
`go.work`, and `go build ./...` at the repo root fails by design — `cd` into the module
first. See [`CONTRIBUTING.md`](../CONTRIBUTING.md).

```bash
cd doclang
go build ./... && go vet ./... && go test ./...
```

DOCX output depends on a personal fork of `docxgo`; the rationale and what diverges from
upstream are documented in [`docs/developer/docxgo-fork.md`](../docs/developer/docxgo-fork.md).

## License

Apache-2.0 — see [LICENSE](../LICENSE) at the repository root.
