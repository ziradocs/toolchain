# Frontmatter Reference

ZiraDocs and DocLang share **the exact same frontmatter parser**
(`core/parser/frontmatter.go`). Whatever is true here applies to
both formats, with the differences noted.

## Hard requirement: the `---` delimiter

**Every `.slidelang` file must start with a `---` line**, or the parser
fails immediately with a fatal "Missing FrontMatter delimiter" error —
before anything else is even attempted. A missing closing `---` is
likewise fatal.

**DocLang's parser is the one exception**: it tolerates a file with no
frontmatter at all. The linter does not — `doclang build` reports a
missing frontmatter block as an error (`FRONT003`) and stops, so write one
regardless. It must be well-formed (opening and closing `---`).

## Recognized fields (the only ones that do anything)

```yaml
---
mode: flex            # strict | flex | flex-full | auto (ZiraDocs only — ignored by doclang; flex-ai is a deprecated alias for flex-full)
title: "..."
author: "..."
date: "..."
theme: "modern-blue"   # CLI --theme flag overrides this if both are given
numbering: true        # true | false (DocLang only, see below) — opt out of section auto-numbering
toc: true              # true | false | {enabled: bool, depth: int} (DocLang only, see below)
page:                  # {size: "A4", margins: "2cm"} (DocLang only, PDF format only, see below)
variables:             # arbitrary key/value map, used for {{variable}} substitution
  company: "Acme Inc"
header:                # optional rich header config (see below)
footer:                # optional rich footer config (see below)
layout_defaults:       # per-layout header/footer overrides (see below)
lint_policy:           # per-document linter policy (see below)
watermark:             # optional repeating overlay, both CLIs (see below)
---
```

Anything else you put in the YAML block is **silently ignored** — the YAML
parser (`yaml.Unmarshal`, not strict mode) does not error on unknown keys,
it just drops them. This is a common trap: a key that "looks like" it
should configure something (because you saw it in an example or an
`init` template) may in fact do nothing.

### `mode` (ZiraDocs only)

- Valid values: `strict`, `flex`, `flex-full`, `auto` (`flex-ai` still works as a
  permanently-supported deprecated alias for `flex-full`).
- If omitted, the **parser** backfills `mode: auto` and emits a WARNING
  (`FRONT001`) before the linter ever runs — the linter has its own
  `FRONT001` check for a missing `mode`, but it can never fire, since `mode`
  is never actually empty by the time the linter sees the AST. So a missing
  `mode` is a warning, not a build-blocking error — but always set it
  explicitly anyway, since silently defaulting to `auto` changes which
  grammar your content is parsed as.
- DocLang has **two** dialects and `mode` picks between them: `strict` selects
  the declared `SECTION`-based grammar, which is never normalized (see
  `doclang-strict.md`), and everything else — `flex`, `flex-full`, `flex-ai`,
  `auto`, or omitting the key — selects the inferred Markdown grammar. In
  documents those four are synonyms: there is one flex grammar and nothing to
  auto-detect. Note this differs from SlideLang, where `strict` means the
  `SLIDE`-based grammar; `SLIDE` blocks never parse in a `.doclang` file.

### Known-ignored keys (do NOT rely on these)

`doctype` — appears in some `doclang init` templates but is **not** part of
the parsed schema; nothing reads it. (`toc`, `page`, and `numbering` used to
be in this list too — see below, all three are now recognized.)

### `numbering` (DocLang only)

- Valid values: `true` or `false`. Omitting the key leaves it unset — not
  the same as `false`.
- Controls whether DocLang's document sections get an auto-numbered prefix
  (`1. `, `2. `, ...) on level-1 headings, the TOC, and the sidebar. A
  document whose section titles already carry their own numbering typed
  directly into the heading text (e.g. `# 1. Objetivo del Proyecto`) should
  set `numbering: false` to avoid doubling the number.
- Resolution order: **`--numbering`/`--numbering=false` CLI flag >
  frontmatter `numbering:` > default (`true`)** — the same three-level
  pattern as `theme`. If the flag is passed explicitly (in either
  direction), it wins regardless of what frontmatter says.
- Parsed by the same `core/parser/frontmatter.go` shared by both CLIs, but
  only `doclang build` reads it — section numbering is a DocLang-only
  concept, so it's a silently-inert key for `.slidelang` files.
- **Legacy map form still accepted**: `numbering:\n  enabled: true\n
  style: 1.1.1`, the shape older `doclang init` templates emitted before
  this key was recognized, still parses — `enabled` maps to the same
  true/false as the plain-bool form above, and `style` is accepted but has
  no effect (no code reads it). Prefer the plain `numbering: true` /
  `numbering: false` form in new documents; the map form exists only for
  backward compatibility with documents/templates written before it.
  `doclang fmt` canonicalizes it to the plain-bool form, dropping `style`
  along the way — the AST only models the bool, and `style` has no consumer.

### `toc` (DocLang only)

- Valid values: `true`/`false` (shorthand for `{enabled: <bool>}`), or a map
  with `enabled`/`depth`.
- Controls whether DocLang's HTML/PDF/DOCX output gets a table of contents
  (and, in HTML, the interactive sidebar viewer that comes with it). A
  bare-Markdown document's TOC only ever lists top-level sections — there's
  no notion of depth outside HTML/PDF.
- Resolution order: **`--toc`/`--toc=false` CLI flag > frontmatter
  `toc.enabled` > default (`true` when frontmatter is present)** — same
  three-level pattern as `numbering`/`theme`.
- `toc.depth` (int, 1-6, default `3`) has **no CLI flag equivalent** —
  `--toc` only talks about enabled/disabled — so it always resolves from
  frontmatter, independent of whether `--toc`/`--numbering` were passed on
  the command line.
- `depth: N` is the highest heading level shown, counting the section title
  itself as level 1 — `depth: 2` shows only `##` headings, `depth: 3` adds
  `###`, and so on up to `depth: 6`. (Fixed in `core/v2.8.1`, issue #123:
  before that release `extractSubsections` had an off-by-one and `depth: 2`
  also included `###` headings.)
- A bad shape or value (e.g. `toc: [a, b]`, `depth: "3"`, an out-of-range
  `depth`) degrades to a `FRONT005` warning instead of failing the build —
  see `core/parser/frontmatter.go`.

### `page` (DocLang only, PDF format only)

- Valid values: a bare paper size string (`page: A4`, shorthand for
  `{size: A4}`), or a map with `size`/`margins`.
- `size`: a recognized paper size name (`A4`, `Letter`, `Legal`, `A3`,
  `A5`, `Tabloid`, case-insensitive). Only affects `doclang build --format
  pdf` — HTML, Markdown, and DOCX output ignore this key entirely (DOCX
  doesn't set page geometry at all; HTML's page-view width comes from the
  theme's own CSS, not from `page:`).
- `margins`: a single length (fills all four sides) or a map with
  `top`/`right`/`bottom`/`left`. Recognized units: `cm`, `mm`, `in`, `pt`,
  `px`. An undeclared side falls back to the PDF default (0.4in).
- An unrecognized `size` or margin value degrades to a `FRONT006` warning
  at parse time and is conserved verbatim in the AST; at PDF-generation
  time DocLang falls back to the default (A4, 0.4in margins) with another
  warning, rather than failing the build.

### `lint_policy` (both CLIs)

A linter policy embedded in the document itself, with the same YAML shape
as a `--lint-config` file: `rules` keyed by **diagnostic ID** (not rule
name) to disable or re-severify a diagnostic, and `layouts` keyed by layout
type to override element-count/forbidden-element limits.

```yaml
lint_policy:
  rules:
    IMG001:
      severity: warning   # error | warning | info
    SPECIAL001:
      enabled: false
  layouts:
    team:
      max_elements: 12
```

Resolution is **`--lint-config` flag > frontmatter `lint_policy:` >
default** (all rules on, original severities) — the same three-level
pattern as theme resolution. If `--lint-config` is passed, the frontmatter
block is not consulted at all. Unknown diagnostic IDs or layout types are
silently inert, not errors.

Both `slidelang build` and `doclang build` honour this, as does `doclang
mcp`'s `lint` tool; `slidelang mcp`'s `lint` tool does not (it lints with
no policy).

## `header` / `footer` / `layout_defaults` (rich configuration)

These are optional and mostly relevant to presentation/document chrome,
not content validity — full shape:

```yaml
header:
  enabled: true
  height: "60px"
  background: "#ffffff"
  text:
    left: "Left text"
    center: "Center text"
    right: "Right text"
  logo:
    source: "logo.png"
    alt: "Company logo"
    height: "40px"
    position: "left"
  border:
    enabled: true
    color: "#e0e0e0"
    width: "1px"
    style: "solid"
    position: "bottom"

footer:
  enabled: true
  height: "40px"
  text:
    left: "..."
    center: "..."
    right: "..."
  page_numbers:
    enabled: true
    format: "{{current}} / {{total}}"
    position: "right"
    exclude_title_slides: true
    exclude_closing_slides: true
    start_from: 1
    style: "default"
  border: { ... }

layout_defaults:
  title:
    header: { ... }
    footer: { ... }
```

**Shorthand forms.** Both `text:` and `page_numbers:` also accept a simpler
value instead of the full map:

```yaml
header:
  enabled: true
  text: "Centered title"   # same as text: { center: "Centered title" }

footer:
  enabled: true
  page_numbers: true       # same as page_numbers: { enabled: true }
```

A bare string under `text:` fills `center` only (`left`/`right` stay empty);
a bare `true`/`false` under `page_numbers:` fills `enabled` only, same as
`numbering:`'s bool-or-map tolerance above. This applies at all four sites
that accept `text:`/`page_numbers:` — `header`, `footer`, and both under
`layout_defaults`.

Only include these if you actually need custom header/footer chrome —
most decks and documents don't need them at all.

**doclang-specific limitations (issue #117).** slidelang renders this
config identically across every slide; doclang's four output formats each
hit a real limit of their own backend:

- **`layout_defaults`** only has two keys worth setting for a document:
  `title` (the document's opening section) and `content` (every other
  section) — a document's `ContentBlock.BlockType` is never anything else,
  unlike a presentation's richer set of slide types. A key other than
  those two is parsed but never matched against any section.
- **HTML (`page-view` chrome).** `header.enabled`/`footer.enabled` draw
  regardless of `theme:` — front matter always wins over the theme gate.
  `{{total}}` in `page_numbers.format` resolves to the number of visual
  `.document-page` containers HTML paginates into, which is not
  necessarily the same as how many sheets a browser's print dialog would
  actually produce for that content.
- **PDF.** Chromium's print header/footer is one template for the *entire*
  PDF, decided once before any page exists — `layout_defaults` has no
  effect (there's no per-page config to select from at that point), only
  the top-level `header:`/`footer:` apply. `{{current}}`/`{{total}}` are
  real: they resolve to Chromium's own page-number counters, not a static
  count. `height`/`background`/`logo`/`border` have no effect: Chromium's
  print API reserves header/footer space from the page's own top/bottom
  margins (`page.margins`), not from anything declared under `header`/
  `footer`, and doesn't expose a logo-image or border primitive for its
  print header/footer at all — bump `page.margins.top`/`bottom` if you
  need more room, not `header.height`.
- **DOCX.** Same PDF-style limitation on `layout_defaults` (Word sections
  are global, not per-page-block). `{{current}}`/`{{total}}` are also
  real — Word's native page-number fields, recalculated on open/print, not
  a static count. `logo`/`border`/`height`/`background` have no DOCX
  equivalent and are silently ignored — `docxgo` (the underlying library)
  exposes no page-border or page-chrome-image API to map them onto. Each
  non-empty `text:` zone (`left`/`center`/`right`) becomes its own
  aligned paragraph rather than one line, since Word's default header/
  footer style has no tab stops to place three zones side by side.
- **Markdown.** No page concept exists, so `header:`/`footer:`/
  `layout_defaults:` are round-tripped back into the front matter verbatim
  (so a build → re-parse cycle doesn't lose them) but nothing is rendered
  from them.
- **Section-level overrides.** `ContentBlock.HeaderFooterOverride` (a
  per-section override, distinct from `layout_defaults`) exists in the
  shared AST but has no frontmatter or DSL syntax that produces it in
  either slidelang or doclang — it's reachable only by constructing/
  decoding an AST directly (e.g. from JSON). Nothing you write in a
  `.slidelang`/`.doclang` file can set it today.

## `watermark` (repeating overlay, both CLIs)

A repeating, semi-transparent text overlay drawn on top of content, on
every slide (slidelang) or page (doclang) — issue #179. Full shape:

```yaml
watermark:
  enabled: true          # implicit true when the block is declared at all, even with no `enabled:` key
  text: "BORRADOR"       # goes through the same {{variable}} substitution as header/footer text
  color: "#000000"       # any CSS color slidelang/core/a11y.ParseColor accepts (hex, named, rgb(), ...)
  opacity: 0.08           # 0.0-1.0; out-of-range values clamp with a warning
  rotation: -45            # degrees, clockwise; normalized into (-360, 360) via modulo
  font_size: "72pt"       # cm | mm | in | pt | px — NOT rem/em/%, see below
  repeat: true            # tile diagonally vs. a single centered instance
```

**Shorthand form.** A bare string is shorthand for `{enabled: true, text:
"..."}`, the same "the interesting value is what a scalar means" pattern
`page: A4` uses:

```yaml
watermark: "BORRADOR"
```

**Declaring the block is itself "on."** Unlike `header:`/`footer:` (whose
`enabled` is a plain bool defaulting to `false` — a block declared without
`enabled: true` draws nothing), `watermark:` defaults `enabled` to `true`
the moment the block exists in any shape, scalar or map. Only an explicit
`enabled: false` turns it off. There's no per-block-type or
per-layout-override use case here the way there is for header/footer, so
there's no reason to let a document declare watermark config without
intending it to show.

**`font_size` units.** Same resolver as `page.margins`
(`core/util.ParseLengthInches`): `cm`, `mm`, `in`, `pt`, `px` only. A CSS
relative unit like `rem`/`em`/`%` degrades to a `FRONT007` warning and is
conserved verbatim — it would resolve fine in slidelang/doclang's own HTML
output, but PPTX and DOCX have no notion of "relative to the root font
size" to resolve it against, so the same input would silently mean two
different sizes depending on output format. Rejecting it uniformly avoids
that split.

Only include this if you actually need a repeating overlay — most decks
and documents don't.

**Fidelity varies by output format (issue #179's PR).** slidelang renders
this identically across html/pdf; doclang the same across html/pdf; PPTX
and DOCX each hit a real limit of their own underlying library:

- **HTML/PDF (both CLIs).** Full fidelity: real opacity, real rotation,
  tiled or single instance, author's `color`. Drawn on top of content
  (`pointer-events: none`, so nothing under it becomes unclickable) rather
  than behind it — the conventional placement for a document/print
  watermark, unlike PPTX below. In doclang, a single `position: fixed`
  overlay covers every printed page — including the table of contents and
  any page-view block that overflows onto more than one physical sheet —
  since Chromium repeats fixed-positioned elements per printed page.
- **PPTX.** `pptxgo`'s `drawingml.Color`/`Paragraph.Color` carry no alpha
  channel — `SrgbClr.Alpha` exists in the OOXML struct but has no public
  setter in the library as pinned today. The opacity is instead
  **pre-blended**: the author's color is mixed toward the slide's
  background (white, since `pptx.go` never calls `Slide.Background`) by
  the requested opacity, and the resulting flat color is drawn as the
  **first** shape on the slide — behind everything else, the only
  placement where a pre-blended flat color reads correctly (an opaque
  image or table on top of it looks exactly like it does over any other
  slide background). `repeat: true` is **ignored**: PPTX always draws a
  single centered instance, not a tile — a diagonal grid would mean dozens
  of individual shapes per slide (PowerPoint's own selection panel would
  list every one of them), and behind opaque content that reads as visual
  noise rather than a watermark the moment a table or image crosses it. A
  single centered mark is the conventional Office watermark shape and the
  only one that survives "behind content" placement cleanly. A `--format
  pptx` build with `watermark:` set logs a WARNING noting both
  divergences.
- **DOCX.** `docxgo` exposes no `w:pict`/VML/text-box API — the classic
  Word watermark mechanism — so there is no way to place a native,
  editable text watermark. Instead, the resolved watermark (tiled and
  rotated, exactly like HTML/PDF) is **rasterized to a PNG** at 150 DPI
  sized to the document's actual page dimensions
  (`section.PageSize()`, not `page:` front matter — see below), embedded
  as a floating image anchored in the header with `BehindText: true`, the
  same mechanism Word's own watermark feature uses internally. The
  tradeoff: the text renders in the embedded Go Regular font, not the
  document's theme font, and it's a bitmap, not editable/selectable text
  in Word — real opacity and rotation are preserved exactly, since they're
  baked into the pixels.
- **DOCX page size caveat.** The PNG is sized from `section.PageSize()`
  (docxgo's own page geometry, currently always its A4 default — `docx.go`
  never calls `SetPageSize` or reads `page:` front matter at all), not
  from the `page:` namespace above. If a future change wires `page:` into
  the DOCX backend's actual page size, the watermark PNG sizing needs to
  move with it; until then, sizing from `page:` instead of the real page
  geometry would produce a mis-scaled watermark the moment they disagree.
- **Markdown.** No page concept exists, so `watermark:` is round-tripped
  back into the front matter verbatim (a build → re-parse cycle doesn't
  lose it) but nothing is rendered from it — same treatment as
  `header:`/`footer:`/`layout_defaults:`.

## Theme resolution priority (both CLIs)

**CLI `--theme` flag > frontmatter `theme:` > default.** If you want a
specific theme to be guaranteed regardless of how the file is built,
prefer passing `--theme` at build time over relying on frontmatter alone.
