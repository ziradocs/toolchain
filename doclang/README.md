# doclang

The documents CLI: `.doclang` source in, HTML / PDF / DOCX / Markdown out.

Parsing, linting and rendering all live in [`core`](../core/); this module owns the DOCX and
Markdown generators, the document themes, and the CLI itself. For the language itself see the
[DocLang documentation](https://ziradocs.com/docs/doclang/overview/) and the [formal spec](../core/spec/).

## Install

```bash
go install go.ziradocs.com/doclang/cmd/doclang@latest
```

Or build from a checkout of this repo:

```bash
cd doclang
go build -o doclang ./cmd/doclang
```

Requires Go 1.26.10+. `--format pdf` and the offline rendering modes additionally need
Chrome/Chromium — see [Interactive elements](#interactive-elements).

## How `.doclang` differs from `.slidelang`

Same element vocabulary, different document model — worth knowing before you write:

- **Sections, not slides.** `#`/`##`/`###` form a nested section hierarchy; sub-levels nest
  inside their parent rather than starting a new top-level block.
- **Two modes, picked by `mode:` in the frontmatter.** `flex` (also `flex-full` and `auto`,
  which are synonyms for documents, and the default with no frontmatter) is Markdown.
  `strict` is the declarative dialect: `SECTION "Title"` blocks with indented elements,
  never rewritten by the normalizer. `doclang fmt --strict` transpiles flex to strict.
- **Frontmatter is optional.** A plain Markdown file with no `---` block builds as is:
  `FRONT003` and the slide layout rules don't run on documents. The output title comes
  from the first `#` heading, or from the file name when there is none. Without
  frontmatter, TOC and numbering stay off unless you pass `--toc`/`--numbering`.
- **Nothing before the first `#` is dropped.** Text before the first `#` (or the whole file,
  if it only uses `##`/`###`) becomes a leading section with no heading, and `##`/`###`
  keep their level. An empty document is still an error (`CORE001`).

## Commands

| Command | What it does |
|---|---|
| `doclang build <file>` | Parse, lint and generate output |
| `doclang init [name]` | Scaffold a new document |
| `doclang fmt [file]` | Rewrite a file as canonical source |
| `doclang mcp` | Run the MCP server over stdio, for editor/agent integration (`--root` confines `export`) |

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
| `--toc` | `true`¹ | Generate a table of contents. |
| `--numbering` | `true`¹ | Number sections. |
| `--page-breaks` | `false` | Break pages between sections. |

¹ The flag's own zero value is `false`, but any document that has a frontmatter block gets
`--toc`/`--numbering` defaulted to `true` unless the flag itself is passed explicitly (in
either direction) — pass `--toc=false`/`--numbering=false` to opt out. `numbering:` in
frontmatter can also opt out per-document; `toc:` in frontmatter is not yet parsed (see
[Known-ignored keys](../llm-kit/reference/frontmatter.md#known-ignored-keys-do-not-rely-on-these)),
so today the CLI flag is the only way to control TOC generation per build.

Format caveats worth knowing up front: **DOCX** now honours `--toc`, `--numbering` and
`--page-breaks` (previously it emitted a TOC unconditionally and ignored the other two), but
does not yet emit clickable hyperlinks, bookmarks or cell shading. **Markdown** is partial —
it skips PlantUML, maps and code groups, and emits charts as a placeholder fence.

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
| `--filter` | — | External binary that transforms the AST between parse and lint. Repeatable; communicates over JSON on stdin/stdout — see the [JSON/AST contract](https://ziradocs.com/docs/architecture/json-ast-contract/). |
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

`doclang mcp` speaks MCP over stdio and exposes five tools. Four work in memory and never
touch the disk: `lint`, `get_ast`, `list_themes` and `preview`. Its `lint` applies the
resolved lint policy, so it reports the same diagnostics `--lint-only` would.

The fifth, `export`, writes DOCX, PDF or HTML for a file on disk with the same pipeline as
`doclang build`: `@include` expansion, parsing, the document lint and the generator.

```json
{ "command": "doclang", "args": ["mcp", "--root", "/path/to/docs"] }
```

| Input | |
|---|---|
| `path` | `.md`, `.markdown` or `.doclang` file, relative to the root or absolute inside it |
| `format` | `docx`, `pdf` or `html` |
| `theme` | optional, a name from `list_themes` |
| `output` | optional; defaults to the input path with the format's extension. Its directory must exist and its extension must match `format` |
| `overwrite` | optional, default `false`: an existing file is never replaced unless it is `true` |
| `toc` | optional; without it the front matter decides, as in `build` |

The result is `{outputPath, format, bytes, diagnostics}`, with `outputPath` relative to the
root. Lint errors come back as an error result carrying the diagnostics, as in `lint`.

- **Root.** `--root` (default: the current directory) is resolved once at startup. The
  input, the output, `@include` files and local images all have to stay inside it, also
  after resolving symlinks.
- **Chromium.** PDF always needs Chromium, and so does DOCX with mermaid diagrams, math or
  charts that can't be drawn natively. If it's needed and missing, `export` fails with an
  error that names what needs it. It never downloads anything: install Chrome, Chromium or
  Edge, or run `doclang build --install-chromium` once from a terminal. `--chromium-path`
  points the server at a specific binary.
- **Network.** HTML uses `build`'s default `browser` mode, so diagrams, charts and math
  load their scripts from a CDN when the page is opened. While exporting, the same hosts
  as `build` are contacted: PlantUML diagrams go to the public `plantuml.com` server, and
  map tiles come from OpenStreetMap or ArcGIS.
- **Limits.** Two exports at a time; a third concurrent call gets a "server busy" error
  instead of waiting. Three minutes per export, and the input size cap of `build`
  (10 MB, or `DOCLANG_MAX_SIZE`). The document is written to a temporary file and moved
  into place at the end, so a failed, cancelled or timed-out export never leaves a
  partial file at the output path. The temporary file is removed when the export
  finishes, or when the server shuts down, but not if the process is killed.

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
upstream are documented in the fork itself: [github.com/mmonterroca/docxgo](https://github.com/mmonterroca/docxgo).

## License

Apache-2.0 — see [LICENSE](../LICENSE) at the repository root.
