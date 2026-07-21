# ZiraDocs — Strict Mode

Strict mode is a rigid, explicit keyword grammar. No AI normalization runs
over it (it's the only mode that never gets rewritten before parsing), so
whatever you write is exactly what gets parsed — get the syntax right the
first time.

Use strict mode when you want deterministic, reviewable output: generated
decks, CI-checked content, or any pipeline where "the LLM's raw output either
parses or it doesn't" is a feature, not a risk.

## Minimal valid file

```
---
mode: strict
title: "My Presentation"
---

SLIDE title
  heading: "Opening"

SLIDE content
  title: "First Topic"
  TEXT
    Some content here.
```

## Frontmatter

- **Mandatory**: the file must start with a `---` delimiter line, or the
  parser fails immediately with "Missing FrontMatter delimiter" — this is a
  hard requirement in *every* mode, not just strict.
- `mode: strict` should be set explicitly. (If omitted, the parser
  defaults to `auto` with a warning rather than treating the file as
  strict — so never omit `mode` when you intend strict.)
  See `frontmatter.md` for the full field list.

## Top-level structure: `SLIDE <type>` blocks

- Every top-level line must start with `SLIDE`. Anything else at the top
  level is a parse error ("unexpected content").
- `SLIDE <type>` — `<type>` becomes the slide's layout/block type (e.g.
  `title`, `content`, `section`, `closing`, `comparison`, `stats`, ... — see
  the layout table in `../validation-checklist.md` for the full list the
  linter recognizes and what each expects).
- Everything belonging to a slide must be indented **exactly 2 spaces**
  under its `SLIDE` line. A non-indented, non-blank line ends the slide.

## Slide properties

Inside a slide block, `key: value` lines (before any element) set
properties:

| Key | Meaning |
|---|---|
| `title` | Slide title (shown for `content`-type slides) |
| `heading` | Primary heading (used by `title`-type slides) |
| `subtitle` | Secondary heading |
| `logo` | Path to a logo image (must have a valid image extension: `.png/.jpg/.jpeg/.gif/.svg/.webp`) |

Any other key is rejected with "Unknown content block property" — don't
invent new frontmatter-like keys inside a slide block.

## Elements

Inside a slide, elements are introduced by an **uppercase keyword** or a
`<<...>>`/`:::`/`@` tag — see `elements.md` for the complete, per-element
table (charts, mermaid, maps, code, tables, special blocks, etc.). Quote,
checklist, and grid all have strict spellings now (`QUOTE`, `CHECKLIST`, and
the `<<grid>>` block below); the flex `::: grid` / `::: column` form is **not**
recognized in strict mode.

A **grid** (side-by-side columns) uses a delimited block, same house style as
`<<map>>`/`<<chart>>`/`<<math>>`:

```
<<grid>>
<<column>>
## Left
- point one
<<column>>
Right column prose
<<end>>
```

Each `<<column>>` starts a column (body = raw Markdown content); `<<end>>`
closes the grid. Lines before the first `<<column>>` are loose prose spanning
the grid.

**Strict mode explicitly rejects the loose/Markdown spellings of the
diagram/chart/map elements** — this is a strict-only trap, since these
spellings are tolerated (and auto-normalized) in flex mode:

| Wrong (flex-only or invalid) | Correct (strict) |
|---|---|
| `MERMAID` | `<<mermaid>>` |
| `PLANTUML` | `<<plantuml>>` |
| `CHART` | `<<chart:type>>` |
| `MAP` | `<<map>>` |
| `MATH` | `<<math>>` … `<<end>>` |
| `$$ … $$` | `<<math>>` … `<<end>>` |
| `::: grid` / `::: column` | `<<grid>>` / `<<column>>` / `<<end>>` |

## What the linter then checks

Passing the parser is not the same as passing `slidelang build --lint-only`.
Strict mode adds its own rules on top of the general ones — see
`../validation-checklist.md` for the complete, rule-ID-mapped checklist
(`STRICT001`/`STRICT002` are the strict-specific ones: title slides need a
`heading` or `title`; content slides need a `title` or at least one
element).

## Worked example (from `examples/gallery/01_strict_mode_basics.slidelang`)

```
---
mode: strict
title: "ZiraDocs Strict Mode Tour"
author: "ZiraDocs Team"
theme: "modern-blue"
---

SLIDE title
  heading: "ZiraDocs Strict Mode"
  subtitle: "Explicit structure, predictable output"

SLIDE content
  title: "Why Strict Mode?"
  TEXT
    Strict mode trades flexibility for guarantees: every slide type and
    field is declared explicitly with SLIDE blocks — no heuristics.
  POINTS
    - Every slide starts with SLIDE <layout>
    - No ambiguity between headings and content

SLIDE closing
  title: "Thanks for Reading"
  TEXT
    See the flex-mode examples next in this gallery.
```
