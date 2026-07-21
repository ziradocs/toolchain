# slidelang

The presentations CLI: `.slidelang` source in, HTML / PDF / PPTX / JSON out.

Parsing, linting and rendering all live in [`core`](../core/); this module owns the slide
generator (`internal/generator/`), the bundled themes, and the CLI itself. For the language
itself see the [user documentation](../docs/user/) and the [formal spec](../core/spec/).

## Install

```bash
go install go.ziradocs.com/slidelang/cmd/slidelang@latest
```

Or build from a checkout of this repo:

```bash
cd slidelang
go build -o slidelang ./cmd/slidelang
```

Requires Go 1.26.5+. `--format pdf` and the offline rendering modes additionally need
Chrome/Chromium — see [Interactive elements](#interactive-elements).

## Commands

| Command | What it does |
|---|---|
| `slidelang build <file>` | Parse, lint and generate output |
| `slidelang fmt [file]` | Rewrite a file as canonical strict-mode source |
| `slidelang themes` | List, install, validate, create and preview themes |
| `slidelang mcp` | Run the MCP server over stdio, for editor/agent integration |

## build

```bash
slidelang build slides.slidelang
slidelang build slides.slidelang --theme modern-blue --format html
slidelang build slides.slidelang --format html,json     # several formats in one pass
slidelang build slides.slidelang --lint-only            # diagnostics only, no output
```

### Output

| Flag | Default | Notes |
|---|---|---|
| `--format`, `-f` | *(html)* | `json`, `html`, `pdf`, `pptx`. Accepts a comma-separated list. |
| `--output`, `-o` | `./dist` | Output directory. |
| `--theme`, `-t` | *(built-in default)* | See [Themes](#themes). |
| `--embed-assets` | `false` | Inline CSS/JS into the HTML instead of writing separate files. |
| `--no-navigation` | `false` | Drop the arrow/keyboard slide navigation. |
| `--no-utilities` | `false` | Drop utility scripts (code tabs, collapsibles). |

### Parsing and linting

| Flag | Default | Notes |
|---|---|---|
| `--mode`, `-m` | `auto` | Force `strict`, `flex` or `auto` instead of the frontmatter `mode:`. |
| `--lint-only` | `false` | Run the linter and stop; generate nothing. |
| `--lint-config` | — | YAML lint policy (enable/disable rules, override severity by ID, e.g. `IMG001`). Falls back to a `lint_policy:` block in the document's own frontmatter. |
| `--enable-normalization` | `false` | Apply the heuristic content normalizer. Deterministic rules, no network calls and no LLM, despite the deprecated `--enable-ai` alias. |
| `--filter` | — | External binary that transforms the AST between parse and lint. Repeatable; communicates over JSON on stdin/stdout — see the [JSON/AST contract](../docs/architecture/json-ast-contract.md). |
| `--include-root` | *(input file's dir)* | Directory `@include` paths are confined to. Absolute paths and `..` escapes are rejected. |
| `--asset-root` | *(input file's dir)* | Same confinement for local images embedded by `--format pptx`. |
| `--max-size` | 10 MB | Input size cap. Also settable via `SLIDELANG_MAX_SIZE`. |

### Interactive elements

Mermaid diagrams, Chart.js charts and Leaflet maps are controlled by `--render-mode`:

- `browser` (default) — reference CDNs and render client-side. Small files, needs internet.
- `offline-assets` — rasterize at build time into an assets folder. Portable.
- `offline-inline` — same, embedded in the HTML. One self-contained file, larger.

Charts that map cleanly onto the native renderer are rasterized without a browser; anything
else (and `--format pdf`) falls back to headless Chromium. Related flags: `--chromium-path`,
`--install-chromium`, `--image-format png|webp`, `--webp-quality`.

### Diagnostics

`--log-level`/`-l` (`error`, `warn`, `info`, `debug`) and `--no-colors`.

## Themes

Bundled: `modern-blue`, `cyberpunk-neon`, `elegant-minimal`, `aurora-holographic`,
`neomorphism-glass`, `startup-tech`, `startup-tech-solid`.

Resolution order is `--theme` > frontmatter `theme:` > the built-in default.

```bash
slidelang themes list
slidelang themes info modern-blue
slidelang themes create my-theme        # scaffolds ./themes/my-theme
slidelang themes preview ./themes/my-theme   # live reload while editing
slidelang themes validate ./themes/my-theme
slidelang themes install ./themes/my-theme   # into ~/.slidelang/themes
```

`themes create` scaffolds `theme.json` plus `styles.css`, `presentation.css` and
`navigation.css`. `themes paths` manages where external themes are searched for.

## MCP server

`slidelang mcp` speaks MCP over stdio and exposes four tools — `lint`, `get_ast`,
`list_themes` and `preview` — so an editor or agent can validate and inspect
`.slidelang` source without shelling out to `build`.

## Development

`slidelang` depends on `core` through a `replace` directive, so a checkout with both
directories side by side picks up local `core` changes with no extra step. There is no
`go.work`, and `go build ./...` at the repo root fails by design — `cd` into the module
first. See [`CONTRIBUTING.md`](../CONTRIBUTING.md).

```bash
cd slidelang
go build ./... && go vet ./... && go test ./...
```

## License

Apache-2.0 — see [LICENSE](../LICENSE) at the repository root.
