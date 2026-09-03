# ZiraDocs — Flex Mode (`flex` / `flex-full` / `auto`)

Flex mode is Markdown-like: `#`/`##` headings, `-`/`*`/`1.` lists, fenced
code, pipe tables, `> ` quotes, plus ZiraDocs's own tags for charts,
mermaid, maps, special blocks, and directives. Anything that doesn't match
a known element falls through to plain text — very little is a hard parse
error.

`flex`, `flex-full`, and `auto` are, in practice, **the same parser** (the
CLI always runs the AI normalizer, which is what actually differs between
these three names in the *library*, not in `slidelang build`/`slidelang
build --lint-only`). Prefer `mode: flex` when you write clean Markdown
yourself, and `mode: flex-full`/`auto` only if you're deliberately leaning
on loose/messy input and want the intent signaled in the frontmatter.
`flex-ai` is the older name for `flex-full` — it still works and is
permanently supported as a deprecated alias, but prefer `flex-full` in new
files.

## Minimal valid file

```
---
mode: flex
title: "My Presentation"
---

# Opening Slide Title
## Optional subtitle
Intro paragraph.

---
# Second Slide Title
- Bullet one
- Bullet two
```

## Structure rules

- **Frontmatter is still mandatory** — the file must start with `---`,
  same as strict mode. Only the *body* grammar is looser.
- Slides are separated by a line containing only `---`.
- The first heading after a separator is the slide's title — prefer an
  `#`/`##` immediately after each separator. A slide with no heading and no
  elements will warn or, in `content`-type slides, fail the title check the
  linter runs.
- Without a `layout:` block, a slide is typed `title` (the deck's first
  `# ` heading only) or `content` (everything else).
- **To type a slide, put a `layout:` block right before its heading:**

  ```
  ---
  layout: stats
  ---
  ## Quarterly Metrics
  ```

  That sets the slide's layout, and the linter then validates it against
  that layout's schema — the same 19 schemas strict mode uses, listed in
  `../validation-checklist.md` §5. An explicit `layout:` also overrides the
  positional rule above, so a mid-deck slide can be `title` and the first
  one need not be.

  The name must be one of the recognized layouts; an unknown one still
  types the slide but warns (`LAYOUT_UNKNOWN`). Only `layout` is read —
  any other key in that block is reported as having no effect (`FLEX002`).
  Read the schema first: several layouts cap how many elements a slide may
  hold, so declaring `layout: testimonial` on a slide with eight elements
  trades a plain `content` slide for a pile of warnings.

## Elements

See `elements.md` for the complete per-element table. In flex
mode you can use either the tag form (`<<mermaid>>`, `<<chart:type>>`,
`<<map>>`) or, for mermaid, a fenced ` ```mermaid ` block — both work, and
the AI normalizer (always on via the CLI) rewrites the fenced form into the
canonical tag form before parsing.

## What the AI normalizer fixes for you (and what it doesn't)

Because the normalizer always runs in the CLI (`slidelang build`), you have
more slack in flex mode than the raw grammar suggests. It will, among other
things:

- Rewrite a ` ```mermaid ` fenced block into `<<mermaid>>...`.
- Rewrite `:::code-item{title="..."}` tab wrappers into canonical
  ` ```lang [label] ` fenced blocks inside a code-group.
- Strip comments and trailing commas from a raw Chart.js-style JSON chart
  payload. The payload is kept verbatim in `rawJSON` (`isJSONMode: true`);
  it is **not** converted to the `data`/`series` schema, so a chart written
  that way reaches the AST with `data` empty.
- Repair loose `#`/`##` heading soup into title/subtitle/section structure.
- Escape/repair minor frontmatter YAML issues.

It will **not** invent missing data, fix a chart's series/column mismatch,
or turn an unsupported element (a poll, a quiz, presenter notes) into a
real one — those still need to be written correctly or represented as
plain-text placeholders (see `elements.md`'s "no unsupported
closing tags" section).

**Don't rely on the normalizer as a validity crutch** if you also want the
output to be portable to strict mode later, or if you're emitting via a
path that might skip it (the library's `enableAI=false` path, or a
`mode: strict` file — the normalizer never runs there at all). When in
doubt, write the canonical form directly.

## Worked example (from `examples/gallery/02_flex_mode_essentials.slidelang`)

```
---
mode: flex
title: "ZiraDocs Flex Mode Essentials"
author: "ZiraDocs Team"
theme: "modern-blue"
---

# ZiraDocs Flex Mode
## Markdown-like authoring for hand-written decks

*A tour of the everyday building blocks: text, lists, tables, images, and quotes.*

---

## Lists, Ordered and Not

**Unordered:**
- First idea
- Second idea

**Ordered:**
1. Draft the outline
2. Fill in the content

---

## A Markdown Table

| Feature | strict | flex | flex-full |
|---------|--------|------|---------|
| Explicit SLIDE blocks | Yes | No | No |
| Markdown headings | No | Yes | Yes |
```
