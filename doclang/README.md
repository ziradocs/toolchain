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

External themes are JSON files in `~/.doclang/themes`, `/usr/local/share/doclang/themes` or
`./themes`. HTML and PDF read their `--doclang-*` CSS variables directly.

#### Surfaces in HTML and PDF

Tables, grid columns, diagram and map panels, keys (`[x]{.kbd}`), blockquotes, special blocks,
links, the PDF page margin and the viewer's sidebar take their colors from the variables below.
Each one falls back to the value doclang has always used, so a theme that defines none of them
renders as before, and the embedded themes render exactly as before.

For an **external** theme, doclang fills in the ones the theme does not define, from the theme's own
variables, and checks the contrast (WCAG) so that no text ends up less readable than with the old
fixed colors. A variable the theme defines itself always wins.

- **What the theme asks for explicitly is applied.** `--doclang-quote-bg` and
  `--doclang-alert-<kind>-bg` color the blockquote and the special blocks; `--doclang-quote-border`
  and `--doclang-alert-<kind>-border` their accent; `--doclang-page-bg` the PDF margin and, if the
  theme has no `--doclang-page-surface-bg`, the page-view sheet. A background is taken only together
  with a text color that reads on it at 4.5:1: the theme's own color for that block
  (`--doclang-quote-color`, `--doclang-alert-<kind>-color`), the block's usual color, or
  `--doclang-text-color`, the first that does. If none does, or the background is not a plain color
  that can be measured (a gradient, for instance), the block keeps its usual colors.
- **Everything else changes only when the usual color would not read with the theme's text.**
  Tables, table stripes and hover, panels, keys, quiz and poll, the page-view header and footer, the
  viewer's sidebar and table of contents, chart/diagram/map titles, captions and `h2` to `h4` keep
  their usual colors in a light theme. In a dark theme they take the page color (or the table
  stripe, for panels and keys) and the theme's text color (`--doclang-h1-color` for `h2` to `h4`).
  A new background is taken only if every text the theme defines for that surface reads on it: the
  viewer's sidebar and table of contents keep their light background when the theme's own
  `--doclang-toc-*-color` values are dark.
- **Links** take `--doclang-link-color` only where it reads at 4.5:1 on every background it sits on
  (the page and panels where the theme's text reads; each blockquote and special block
  separately, and only when that block takes its background from the theme). Otherwise they keep
  the browser's blue, and if that does not read either (a dark page), a light blue (`#8ab4f8`). If
  none of them reads on the page and the panels, the first one that reads on the page is taken, and
  only if none does, the page's text color (the link stays underlined). Tables do not lower the
  link of the whole document: a link in a table cell takes `--doclang-table-link-color` when the
  body link does not read on the table, and `--doclang-table-stripe-link-color` in a stripe row
  when the table's link does not read on the stripe (the theme's link color, the browser's blue,
  the light blue, the table's text color, or black or white, the first that reads on that
  background). A pressed link keeps the
  browser's red.
- **Content inside a surface.** A surface with its own background can hold text that has its own
  color, and that color is checked against that background, not only against the page:
  - **Status colors** (`[x]{.danger}`, `.info`, `.success`, `.warning`, `.accent`). Each surface
    can set its own: `--doclang-<surface>-ink-<color>`, for example
    `--doclang-callout-warning-ink-danger`. doclang fills one in only where the document's color
    does not read at 4.5:1 on that surface: a light theme color inside a special block that keeps its
    light background, or inside a table header. It takes the usual color, a darker or a lighter
    shade of the same color, the surface's text color or, as a last resort, black or white
    (whichever contrasts more). The innermost surface wins (a table inside a special block uses the
    table's). The document's own color (`--doclang-danger-color`, ...) is filled in only if the
    theme does not define it and the usual one does not read on the page.
  - **Text in a table.** When the table or its stripes take their background from the theme
    (its own or filled in), `--doclang-table-color` is set to the theme's text color instead of
    inheriting: inherited, a table inside a special block that keeps its light background took the
    block's dark text on the dark table.
  - **Links in a table header** take `--doclang-table-header-link-color`, filled in only when the
    body link does not read on the header (a header whose background is the link color, for
    instance): the theme's link color, the browser's blue, the light blue, the header text, or black
    or white, the first that reads.
  - **Keys** inherit the text of wherever they are, but always sit on `--doclang-kbd-bg`. If any of
    those texts (the page, a table, a table header, a blockquote, a special block, a panel, the
    quiz) would not read on it, the key gets `--doclang-kbd-color`: the theme's text color, or
    black or white. This includes a table header with the default gradient: the gradient cannot
    be measured, but the key paints its own background and would inherit the header's white text.
    A theme's own `--doclang-kbd-color` is used when deciding whether the key's background has to
    change.
  - **Inline code** paints its own background, so only its pair is checked: if
    `--doclang-code-inline-color` does not read on `--doclang-code-inline-bg`, the half the theme
    did not set is filled in. A missing background takes the table stripe, the page, the code
    block's background, `#1e1e1e` or the theme's text color, the first the color reads on. A missing
    color takes the first of a darker pink (`#c2185b`), the theme's text color or the code block's
    color that reads on the background and does not read worse than before in the viewer's dark
    mode (see the limitation below); the theme's text color goes in as `var(--doclang-text-color)`,
    so that dark mode repaints it. With neither half, the pink `#e83e8c` (3.59:1 on its gray, 3.61:1
    in dark mode) becomes the theme's text color; in a dark theme, whose text does not read on the
    gray, the background becomes the page (or the code block's background) too. The code block,
    which also paints its own background, follows the same rule with `--doclang-code-bg` and
    `--doclang-code-color`. A pair the theme set completely is left as is.

  A background that cannot be measured (the default gradient of the table header, when the theme
  does not set `--doclang-table-header-bg`) is left alone.
- **Charts** keep their white card: Chart.js and the chart image draw on their own light background.
- **PDF header and footer.** Chromium prints them in the page margin, in black. When the margin is
  painted, their text takes `--doclang-text-light` or `--doclang-text-color` if one reads on it; if
  none does, the margin is left white, unless the theme set `--doclang-print-margin-bg` itself.

| Variable | Fallback |
|----------|----------|
| `--doclang-body-link-color` | the browser's link colors |
| `--doclang-body-link-active-color` | the browser's pressed-link color (not derived) |
| `--doclang-table-bg`, `--doclang-table-color` | `white`, inherited |
| `--doclang-panel-bg`, `--doclang-panel-border`, `--doclang-panel-color` (grid columns and PlantUML in every output; Mermaid and maps only in browser-mode HTML) | `#f8fafc`, `#e2e8f0`, inherited |
| `--doclang-kbd-bg`, `--doclang-kbd-border`, `--doclang-kbd-color` | `#f7f7f7`, `#ccc`, inherited |
| `--doclang-chart-bg` (browser-mode HTML only) | `white` |
| `--doclang-blockquote-bg`, `-color`, `-border`, `-footer-color`, `-link-color` | `#f7fafc`, `#4a5568`, `#3498db`, `#718096`, the browser's link colors |
| `--doclang-callout-<kind>-bg`, `-color`, `-border`, `-link-color` | each block's usual colors, the browser's link colors |
| `--doclang-element-title-color` (chart, diagram and map titles) | `#2d3748` |
| `--doclang-caption-color` (table and figure captions, unavailable-content notes) | `#718096` |
| `--doclang-code-tab-active-bg`, `-active-color`, `-accent` | `#1e1e1e`, `#fff`, `#3498db` |
| `--doclang-print-margin-bg` (PDF page margin) | not painted |
| `--doclang-table-link-color`, `--doclang-table-header-link-color` | the body link color |
| `--doclang-table-stripe-link-color` | the table's link color |
| `--doclang-<surface>-ink-<color>` | the document's status color (`--doclang-<color>-color`) |

`<kind>` is `info`, `warning`, `danger`, `success` or `tip`; a theme describes each special block with
`--doclang-alert-<kind>-bg`, `-border` and `-color`, `tip` included. `<surface>` is `table`,
`table-stripe`, `table-header`, `panel`, `blockquote`, `callout-<kind>` or `quiz`, and `<color>` is
`danger`, `info`, `success`, `warning` or `accent`. The CSS copies each of these into an internal
`--_doclang-ink-<color>` (and the table links into `--_doclang-table-link`), which is not part of
the theme contract: a theme variable whose name starts with `--_doclang-` is dropped with a
warning. doclang may also fill in these
contract variables for an external theme, under the same rules: `--doclang-danger-color`,
`--doclang-info-color`, `--doclang-success-color`, `--doclang-warning-color`,
`--doclang-accent-color`, `--doclang-kbd-color`, `--doclang-table-color`, `--doclang-code-inline-bg`,
`--doclang-code-inline-color`, `--doclang-code-bg`, `--doclang-code-color`,
`--doclang-page-surface-bg`,
`--doclang-table-stripe-bg`, `--doclang-table-hover-bg`, `--doclang-header-footer-bg`,
`--doclang-quiz-bg`, `--doclang-quiz-correct-bg`, `--doclang-quiz-correct-color`,
`--doclang-h2-color` to `--doclang-h4-color`, `--doclang-sidebar-bg`, `--doclang-toc-bg` and the
`--doclang-toc-*-color` text colors.

The new names are separate from `--doclang-link-color`, `--doclang-quote-*`, `--doclang-alert-*` and
`--doclang-page-bg` on purpose: the embedded themes define those with values that were never applied
to links, blockquotes, special blocks or the PDF margin, and reading them there would change how the
embedded themes look. The filled-in values are copies, not `var()` references, so what is checked
is exactly what is painted; the viewer's dark-mode toggle does not repaint them. The one exception is
the code color, which may be `var(--doclang-text-color)`.

**Limitation: the viewer's dark mode** (the toggle, or `theme_mode: dark`) redefines some contract
variables on top of the theme's (page, text, headings, `--doclang-link-color`, the code
backgrounds, the status colors) but not the filled-in ones above, which were measured for the light
page. With an external theme, a filled-in link color (`--doclang-body-link-color`,
`--doclang-table-link-color`, `--doclang-table-stripe-link-color`) can fall under 4.5:1 on the
dark page, as it already could before.
The code colors are the exception: they are only filled in when they do not read worse than before
on the dark mode's code background (dark mode also replaces the code backgrounds, so a filled-in
code background does not matter there).

In PDF and `offline-*` HTML, Mermaid diagrams, maps and charts are inserted as images without a
panel around them: Mermaid follows the diagram colors of the theme, and charts and maps keep their
own light background. Their titles still follow `--doclang-element-title-color`.

**External themes that already exist change in HTML and PDF:**

- A **dark** theme gets dark tables, stripes, panels, keys, quiz, viewer sidebar and table of
  contents, readable headings and captions, its link color (or the light blue), its blockquote and
  special-block colors where defined, and a PDF margin in its page color. Before, several of these
  were white or light gray under light text.
- A **light** theme mostly keeps its look: tables, panels, keys, headings, captions and the
  sidebar do not change. What changes is what the theme asked for: blockquote and special-block
  backgrounds, text and borders where it defines `--doclang-quote-*` or `--doclang-alert-*`, the
  link color if it reads at 4.5:1 (a link color under 4.5:1 is ignored, so the links stay the
  browser's blue), and the PDF margin, painted with `--doclang-page-bg`. Besides that, a status
  color that falls under 4.5:1 on the page or inside a surface takes a darker shade of the same
  color; a key takes the theme's text color when a text it would inherit does not read on it (a
  table header with light text over a key); a table with its own stripes takes the theme's text
  color instead of inheriting it; a link in a table whose stripes do not let it read gets its own
  color; and the inline-code pink becomes the theme's text color when the theme sets no inline-code
  color.

Every theme variable goes into the page's CSS after a check, and a variable that fails it is dropped
with a warning. The check rejects a value that contains `{`, `}`, `<`, `>`, `;` or a line break, or
`/*` or `*/`; that leaves a string open (a quote with no closing quote of the same kind, so
`"O'Reilly Sans"` is fine); that leaves a parenthesis or bracket unclosed (`url(x`) or closes one
that was never opened; or that has a backslash before a quote or at the very end. Each of those
would break out of the declaration or swallow the variables after it. The check does not require a
color: any other CSS value passes as written, although only a value that parses as a color can
count as readable. doclang also refuses to copy a value with any backslash into the variables it
fills in.

#### DOCX

DOCX reads these:

| DOCX | Variables |
|------|-----------|
| Body font | `--doclang-font-main` |
| Code font | `--doclang-code-font`, else `--doclang-font-code` |
| Headings h1 to h4 | `--doclang-h1-color` to `--doclang-h4-color` |
| Text, light text, links | `--doclang-text-color`, `--doclang-text-light`, `--doclang-link-color` |
| Code block | `--doclang-code-bg`, `--doclang-code-color` |
| Inline code | `--doclang-code-inline-color` |
| Table | `--doclang-table-header-bg`, `--doclang-table-header-color`, `--doclang-table-border`, `--doclang-table-stripe-bg` |
| Special blocks | `--doclang-alert-info-color`, `--doclang-alert-warning-border`, `--doclang-alert-danger-border`, `--doclang-alert-success-border`, `--doclang-alert-tip-border` (optional) |

Rules, because these values end up in Word's XML:

- **Colors** must be `#rgb` or `#rrggbb`; anything else (`rgb()`, `var()`, names, gradients) is ignored.
- **Fonts**: the first usable family of the CSS stack. Generic families (`sans-serif`, `system-ui`,
  `ui-sans-serif`, `ui-serif`, `ui-monospace`, `ui-rounded`, `-apple-system`, ...) and emoji or symbol
  families (`Apple Color Emoji`, `Segoe UI Emoji`, `Segoe UI Symbol`, `Noto Color Emoji`, `Emoji`) are
  skipped, and so is any name that is not ASCII letters, digits, spaces and hyphens (1 to 64
  characters, so accented or CJK names are skipped too). `system-ui, 'Inter', sans-serif` gives
  `Inter`, `'游ゴシック体', YuGothic` gives `YuGothic`, and the default Tailwind stack gives
  `Segoe UI`. If no family qualifies, the font comes from `professional`.
- **DOCX is for white paper.** It does not paint the page background (Word does not print it by
  default), so text colors that would not read on white are ignored: body text, links and inline
  code need a WCAG contrast of at least 4.5:1 against `#ffffff`; light text and h1 to h4, at least
  3:1. A dark theme (light text, dark page) therefore comes out with the `professional` colors.
- **Pairs** resolve together: code block background and text, and table header background and text,
  must both be valid, defined and at least 4.5:1 against each other; otherwise both come from
  `professional`. A table stripe that leaves the body text under 4.5:1 is ignored too, and so is one that leaves the links under 4.5:1 when the theme defines an accepted `--doclang-link-color` (the fallback link color is not checked).
- Special blocks (`:::info`, `warning`, `danger`, `success`, `tip`) take the title color from the
  variables above in external themes (embedded themes keep their own colors). The title is drawn on
  white, so an alert color under 3:1 is ignored; a dark theme's pastel alert colors come out with
  the `professional` ones.

Anything the theme does not define or that is ignored comes from `professional`, as do sizes,
spacing and bold (the theme does not control those in DOCX). Page background, gradients, heading
borders and the heading font are not applied in DOCX. Embedded themes are unchanged.

### Parsing and linting

doclang runs the same linter and the same rule set as `slidelang` — `TABLE003`,
`CODEGROUP001`, `IMG001` and the rest all apply to `.doclang` source.

Two warnings are doclang's own. `INCLUDE001` is an `@include` line inside a code
block (a fence or a strict `CODE` body). That line is code: it stays as
written in every output and is not expanded, also when the path does not
exist. To include a file, write the `@include` outside the code block.
(`slidelang build` still expands an `@include` inside a code block.)
`INCLUDE002` is an included file that leaves a fence, a `:::code-group` or a
`$$` block open: an included file is a container, so the block ends where
that file ends and does not take in the lines after the `@include`. The
including document can no longer close it. The warning is on the `@include`
line and names the file and the line that opens the block; like any rule, the
lint policy can turn it off.

Findings in an `@include`d file carry that file (`file` in the `--report json`
findings, relative to the document's directory, and its own path in SARIF)
and the line in it, not the line of the expanded content.
`file` is meant to be joined to the directory of `document.path` lexically,
the way `path.Join` and SARIF URI resolution do. When the document was
reached through a symlinked directory it can start with `..` that crosses
that symlink: `document.path` `lnk/main.doclang` with `file`
`../inc/x.doclang` gives `lnk/../inc/x.doclang`, which a normalizing join
turns into `inc/x.doclang` but which fails if opened as written.

| Flag | Default | Notes |
|---|---|---|
| `--lint-only` | `false` | Run the linter and stop; generate nothing. |
| `--lint-config` | — | YAML lint policy (enable/disable rules, override severity by ID). Falls back to a `lint_policy:` block in the document's own frontmatter. |
| `--filter` | — | External binary that transforms the AST between parse and lint. Repeatable; communicates over JSON on stdin/stdout — see the [JSON/AST contract](https://ziradocs.com/docs/architecture/json-ast-contract/). |
| `--base-dir` | *(input file's dir)* | Directory relative image sources and `@include` paths resolve from. For building a temporary copy of a file (an editor buffer with unsaved changes, a generated file that `@include`s real ones): the input file is then only a carrier, so `--base-dir` also becomes the default of `--asset-root` and `--include-root`. It must exist, be a directory and sit inside `--asset-root`. A relative path resolves from the current directory, like `--asset-root` and `--include-root`. An explicit `--include-root` doesn't have to contain it: `@include` paths then resolve from the include root. Images are only read at build time for DOCX, PDF and `--render-mode offline-*` HTML; in the default `browser` mode HTML keeps each relative `src` as written, except images inside an `@include`d file, which are rewritten relative to `--base-dir`. |
| `--include-root` | *(`--base-dir`, else input file's dir)* | Directory `@include` paths resolve from and are confined to, at every nesting level (a nested `@include` resolves from this root, not from the file that contains it). Absolute paths and `..` escapes are rejected. Relative images *inside* an included file (block and inline `![alt](src)`) do resolve from that file's own directory, still confined to `--asset-root`; where the output keeps a relative `src` (browser and `offline-assets` HTML), it is rewritten relative to `--base-dir`, like an image written in the top-level file. |
| `--asset-root` | *(`--base-dir`, else input file's dir)* | Same confinement for local image sources. Relative sources resolve from `--base-dir` (by default, the input file's directory); the root only bounds what can be read (e.g. `--asset-root ..` allows `../shared/b.png`). |
| `--allow-remote-images` | off | Lets PDF and `--render-mode offline-*` HTML show `http(s)` images from the document. Off by default: those outputs are rendered on the machine that builds them, so a remote image in an untrusted document would make it fetch that URL. Without the flag such images are left as a marker with a warning, and the page's CSP only allows `data:` and same-origin images. Browser-mode HTML is unaffected (the reader's browser loads the image), and DOCX never downloads images. |
| `--fail-on-unavailable` | off | Exit with an error, and write no output, if anything would be left as a "not available" marker: an image that is missing, outside `--asset-root` or remote and not fetched, or a diagram, chart, map, equation or media item that could not be drawn (no Chromium, no network, a server error). The message lists each item with its line and type. A previous output with the same name is removed; with `--render-mode offline-assets` files already written under `assets/` stay. Browser-mode HTML resolves almost nothing at build time, so almost nothing can be missing there; a missing header logo counts only where the logo is drawn (HTML page-view with the header enabled). A map whose tiles mostly failed to download counts as not available. Not valid with `--format markdown`. |
| `--max-size` | 10 MB | Input size cap. Also settable via `DOCLANG_MAX_SIZE`. |

### Interactive elements

Diagrams (Mermaid, PlantUML), charts (Chart.js) and maps (Leaflet) are controlled by
`--render-mode`:

- `browser` (default) — reference CDNs and render client-side. Small files, needs internet.
- `offline-assets` — rasterize at build time into an assets folder. Portable.
- `offline-inline` — same, embedded in the HTML. One self-contained file, larger.

Related flags: `--chromium-path`, `--install-chromium`, `--image-format png|webp`,
`--webp-quality`, `--plantuml-server`, `--plantuml-format svg|png`.

The offline modes render at build time, so the build itself still reaches the diagram
and map tile servers: PlantUML goes to `--plantuml-server` (default `www.plantuml.com`) or,
with `--diagram-backend kroki`, to `--kroki-server`, in HTML, PDF and DOCX alike; maps
download tiles from OpenStreetMap (ArcGIS World Imagery for satellite maps).

| Flag | Default | Notes |
|---|---|---|
| `--tile-server` | — | Tile URL template for maps drawn at build time (DOCX, PDF, `offline-*` HTML), e.g. `https://tiles.example.com/{z}/{x}/{y}.png`; `{s}` expands to the `a`, `b`, `c` subdomains. Replaces OpenStreetMap and the satellite provider. Browser-mode HTML maps are unaffected. |
| `--tile-user-agent` | — | `User-Agent` for the requests to `--tile-server`. |
| `--tile-attribution` | — | Credit drawn in the corner of maps whose tiles come from `--tile-server` (plain text, up to 200 bytes). Without it those maps carry no credit; the default providers keep their own. |
| `--no-network` | off | Contact no diagram or tile server while building: PlantUML, Mermaid through Kroki and maps are left as a "not available" marker instead of being fetched. Charts, equations and Mermaid through Chromium are drawn locally, and Chromium, when needed, starts with no network access (no name resolution, no outbound connection, no component updates). Cannot be combined with `--allow-remote-images`, `--install-chromium` or `--tile-server`. |

Note that a single binary is enough for HTML and Markdown, but PDF, DOCX and the offline
modes reach for headless Chromium — `--install-chromium` fetches a pinned build if you
don't have one.

PDF, DOCX and the offline modes don't need network access for Mermaid, charts or math:
the binary carries its own copies of mermaid 10.9.6, Chart.js 4.5.1 (plus the treemap
plugin) and MathJax 4.1.3, the same builds browser mode loads from the CDN. Math also
accepts a fenced ` ```math ` (or `~~~math`) block, besides `<<math>>` and `$$`;
`doclang fmt` rewrites it as `<<math>>`. The TeX extensions that
need a font of their own besides mhchem (`\require{bbm}`, `bboldx`, `dsfont`) are not
embedded and still need network; without it they fail with a warning. As with the CDN,
`numcases` needs `\require{cases}` and `\rowcolor` needs `\require{colortbl}` in the
formula; the rest load on their own. `doclang licenses`
prints the license and third-party notices of everything the binary embeds. Every Chromium capture
has a deadline (30 s per capture of a chart, diagram or equation, 5 min to print the
PDF), so a resource that can't load ends in a warning instead of a hung build.

Maps still need the tile servers (and Leaflet from unpkg.com), and PlantUML its server.
Without network a map is left out with a warning: in under a second when connections are
refused, and at most after a 5 s probe plus the map's deadline (15 s, 20 s from zoom 7,
60 s from zoom 11) when they hang.

When a map, chart, diagram or equation can't be rendered, or a local image is missing or
outside `--asset-root`, HTML and PDF show a box with a short note in the document's
language ("Mapa no disponible", "Image not available", with the image's alt text) instead
of a broken-image icon or the internal error; the detail goes to the warning. SVG images
reach DOCX rasterized to PNG with Chromium, at the size the SVG declares
(`width`/`height`, else its `viewBox`); without Chromium they are left as a marker with a
warning.

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
| `baseDir` | optional, like `build --base-dir`: the directory relative images and `@include` paths resolve from, inside the root. Defaults to the input file's directory. HTML export uses `browser` mode: it reads no image and keeps each relative `src` as written, except images inside an `@include`d file, which are rewritten relative to `baseDir` |
| `includeRoot` | optional, like `build --include-root`. Defaults to `baseDir` |
| `allowRemoteImages` | optional, like `build --allow-remote-images`; default `false`, so a PDF export leaves the document's `http(s)` images as a marker with a warning instead of fetching them. The client sets it, so it only takes effect when the server was started with `doclang mcp --allow-remote-images`; otherwise it is ignored with a warning in the diagnostics |

The result is `{outputPath, format, bytes, diagnostics}`, with `outputPath` relative to the
root. Lint errors come back as an error result carrying the diagnostics, as in `lint`.

- **Root.** `--root` (default: the current directory) is resolved once at startup. The
  input, the output, `@include` files and local images all have to stay inside it, also
  after resolving symlinks.
- **Chromium.** PDF always needs Chromium, and so does DOCX with mermaid diagrams, math,
  local SVG images or charts that can't be drawn natively. If it's needed and missing, `export` fails with an
  error that names what needs it. It never downloads anything: install Chrome, Chromium or
  Edge, or run `doclang build --install-chromium` once from a terminal. `--chromium-path`
  points the server at a specific binary.
- **Network.** HTML uses `build`'s default `browser` mode, so diagrams, charts and math
  load their scripts from a CDN when the page is opened. DOCX and PDF render Mermaid,
  charts and math without network. While exporting, the same hosts as `build` are
  contacted: PlantUML diagrams go to the public `plantuml.com` server, and map tiles come
  from OpenStreetMap or ArcGIS.
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
