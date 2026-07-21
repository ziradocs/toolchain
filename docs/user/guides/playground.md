# Try It In Your Browser (Playground)

The playground is a minimal, no-backend way to try ZiraDocs and DocLang
without installing anything: paste source, get a live preview. It's a
`.wasm` build of the same parse → lint → render pipeline the CLIs use,
running entirely in your browser.

There's no hosted URL yet — you run it locally. (The polished
ziradocs.com site will consume the same build artifacts separately.)

## Run it (about a minute)

```bash
./playground/build.sh
cd playground && python3 -m http.server 8080
# open http://localhost:8080/
```

`build.sh` compiles `slidelang.wasm` from `slidelang/cmd/wasm` and
copies the matching `wasm_exec.js` runtime shim from your Go toolchain.
Any static file server works, as long as it serves `.wasm` with the
`application/wasm` MIME type (most do by default) — opening `index.html`
directly via `file://` will **not** work, since fetching the `.wasm` file
needs an HTTP origin.

## What it does

- Edit ZiraDocs (strict or flex) or DocLang source in the left pane; the
  right pane live-previews the rendered output.
- A **Lint** button runs the real linter against ZiraDocs source and
  shows every diagnostic. This is ZiraDocs only — DocLang has no linter
  at all, so its only validity signal is "did it parse without an error."
- Diagrams (Mermaid), charts (Chart.js), and maps (Leaflet) all render
  **client-side** via CDN — the same as an ordinary `slidelang build`/
  `doclang build` with no `--render-mode` flag.

## Known limits

- **No PDF or offline rendering.** `--format pdf` and `--render-mode
  offline-assets`/`offline-inline` all need headless Chrome, which doesn't
  exist in a browser — the playground always renders in `browser` mode.
- **DocLang theming is minimal.** doclang's named theme presets
  (`professional`, `academic`, `technical`, `page-view`) live in a
  separate Go module the wasm build doesn't import; DocLang previews use
  plain default styling regardless of the theme selector.
- **ZiraDocs's marquee named themes** (`modern-blue`, `cyberpunk-neon`,
  etc.) are embedded at compile time and layered on as a CSS override —
  they render correctly, just via a different code path than the CLI's
  disk-based theme system.

See [`playground/README.md`](../../../playground/README.md) for the full
build/export details and exactly which JS functions the wasm module
exposes.
