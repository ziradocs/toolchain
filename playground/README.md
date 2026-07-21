# SlideLang / DocLang Playground (WASM)

A minimal, no-backend playground: edit SlideLang or DocLang source and see
a live preview, entirely client-side. This is the `.wasm` + JS-bindings
harness for issue #134 — a runnable proof that the pure parse→lint→render
path compiles to WebAssembly and works in a real browser. It is **not**
the polished ziradocs.com site; that consumes these same artifacts
separately.

## Build

```bash
./build.sh
```

This produces `slidelang.wasm` (built from `slidelang/cmd/wasm`) and
refreshes `wasm_exec.js` to match your installed Go toolchain — the two
must come from the same Go version, since `wasm_exec.js` is the JS-side
runtime shim for whatever `GOOS=js GOARCH=wasm` build produced the
`.wasm`.

## Run

Serve this directory with any static file server that sets the
`application/wasm` MIME type for `.wasm` (most do by default):

```bash
python3 -m http.server 8080
# then open http://localhost:8080/
```

Opening `index.html` directly via `file://` will not work — `fetch()`ing
the `.wasm` file requires an HTTP origin.

## What's exposed

`playground.js` calls five plain JS-global functions exported by the wasm
module (`slidelang/cmd/wasm/*.go`), each taking/returning strings (JSON
for structured results):

| JS function | Wraps |
|---|---|
| `slidelangLint(source)` | Same parser+linter pipeline as `slidelang build --lint-only` / MCP `lint` |
| `slidelangGetAST(source)` | Same as `slidelang build --format json` / MCP `get_ast` |
| `slidelangRenderSlides(source, theme)` | Self-contained slide-deck HTML (CSS/JS inlined) |
| `doclangRenderHTML(source, theme)` | Self-contained document HTML |
| `slidelangListThemes()` | Built-in + marquee named theme names |

## Known scoping limits (read before filing a bug)

- **No chromium/offline rendering.** `--format pdf`, `--render-mode
  offline-assets`, and `--render-mode offline-inline` all need headless
  Chrome, which doesn't exist in a browser. The playground always renders
  in `browser` mode: mermaid/chart/map render **client-side** via CDN
  scripts (mermaid.js, Chart.js, Leaflet) — the same as an ordinary
  `slidelang build` without `--render-mode`.
- **DocLang theming is minimal.** doclang's named theme presets
  (`professional`, `academic`, `technical`, `page-view`) live in the
  `doclang` Go module, which this wasm build (living in `slidelang`)
  doesn't import — keeping the two CLIs' code separate, per this repo's
  module boundary. The DocLang preview renders with plain, functional
  default styling regardless of the `theme` argument; the theme selector
  in the UI is disabled for DocLang for this reason.
- **SlideLang's marquee named themes** (`modern-blue`, `cyberpunk-neon`,
  `elegant-minimal`, `startup-tech`, `aurora-holographic`,
  `neomorphism-glass`) normally live on disk
  (`slidelang/themes/<name>/`) and are looked up via `os.ReadFile` —
  there's no filesystem to read from in a browser. The wasm build embeds
  their `theme.json`/`styles.css` at compile time
  (`slidelang/themes/embed.go`) and layers the resulting CSS on top of
  the normal render as an override (`cmd/wasm/theme.go`) — they render
  correctly, just via a different code path than the CLI's disk-based
  external-theme system.
- **No AST-schema-version negotiation, no Share button.** Lead-capture and
  any commercial-platform integration are explicitly out of scope for this
  OSS harness.
