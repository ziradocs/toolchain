# DocLang — Flex Mode

**Flex is DocLang's inferred dialect and its default**: Markdown-shaped input
that the parser interprets, with the normalizer applied. `mode: flex`,
`mode: flex-full`, `mode: auto` and *no frontmatter at all* are all this same
grammar — unlike SlideLang, DocLang has nothing to auto-detect between.

The one other value that does something is **`mode: strict`**, DocLang's
declared dialect (`SECTION` blocks, never normalized) — see
[`doclang-strict.md`](./doclang-strict.md). Pick flex to draft; pick strict for
the version that gets committed and reviewed.

Note that `SLIDE` blocks never parse in a `.doclang` file, in either dialect:
that is SlideLang's strict grammar, not DocLang's.

## Minimal valid file

```
---
title: "Report Title"
---

# Report Title

Some opening paragraph.
```

Frontmatter is **optional to the parser** for DocLang (unlike ZiraDocs,
where it's mandatory) — a bare Markdown document parses fine. In practice,
write one anyway: the linter that `doclang build` runs flags a missing
frontmatter block as an error (`FRONT003`), so a bare file does not
currently build.

## Structure: heading hierarchy, not slides

- `# Heading` starts a new **section** (DocLang's equivalent of a
  ZiraDocs "slide" — the shared AST calls both a generic `ContentBlock`).
  The first `#` becomes the document's title section; every subsequent `#`
  starts a new top-level section.
- `##` / `###` do **not** start new sections — they become nested heading
  elements (`<h2>`/`<h3>`) *inside* the current section. This is the
  opposite of ZiraDocs flex mode, where every `##` can start new content.
- A bare `---` line is treated as an ignorable separator (harmless, but
  unnecessary — you don't need slide-style `---` separators in DocLang).

## Frontmatter fields that actually do something

DocLang shares the exact same frontmatter parser as ZiraDocs, so **only
these keys are recognized**: `mode` (ignored for parsing, but harmless to
include), `title`, `author`, `date`, `theme`, `variables`, `numbering`,
`header`, `footer`, `layout_defaults`, `lint_policy`. See `frontmatter.md`
for details.

**Gap to know about:** some example files write `doctype: ...` into
frontmatter. **That key is not parsed** — YAML silently drops unknown
keys, so it has zero effect on the build. This is exactly the trap
`frontmatter.md` warns about: nothing distinguishes "key that doesn't
exist" from "key that exists and does nothing." (`toc:` and `page:` used
to be in this same "ignored" list too — front matter presence now turns
TOC on by default and `page:` feeds PDF page geometry; see
`frontmatter.md`. `header:`/`footer:`/`layout_defaults:` are parsed and
consumed by every doclang output format — see the "doclang-specific
limitations" note under `frontmatter.md`'s `header`/`footer`/
`layout_defaults` section for what each backend can and can't do with
them.)

## Elements

Same shared element parsers as ZiraDocs flex mode — see
`elements.md` for the full per-element table (charts, mermaid,
maps, tables, special blocks, code, code-groups, checklists, math, etc.).
All of it works identically inside a DocLang section — with one exception:
directives (`@notes`, `@timer`, …) parse, but the document renderer emits
nothing for them — a directive is slidelang presenter-notes metadata, and a
document has no presenter view to show it in. Unlike a truly unrecognized
element, this is not silent: both the Markdown and DOCX generators log a
warning naming the directive and its line. `@include` is the exception to
the exception: it is expanded before parsing, so it works in both formats.

## Validation: DocLang runs the same linter

`doclang build --lint-only` parses and lints without writing output, the
same way `slidelang build --lint-only` does, and `doclang mcp` exposes the
same `lint`, `get_ast`, `list_themes`, `preview` tools over source held in
memory. Rule severity is configurable per document via `--lint-config` or a
`lint_policy:` block in the frontmatter (see `frontmatter.md`) — DocLang's
MCP `lint` tool honours that policy, which ZiraDocs's does not.

The **element** rules are the ones that matter here, and they fire exactly
as they do for ZiraDocs, because the element parsers are shared:
`TABLE003` (every row must match the header's column count — an error),
`CODEGROUP001/002`, `IMG001`, `CODE001`, `CHART001/003/004`, `SPECIAL001`.

What doesn't carry over: the strict-mode rules (DocLang ignores `mode:`,
so they never fire) and the per-layout slide schemas, which are written
against the slide model. The layout schemas can still emit cosmetic
warnings on a DocLang document — a first section carrying body text draws
"Title slides typically should not contain content elements". They are
warnings, they don't block the build, and there is nothing to fix.

So the practical rule is the same as for ZiraDocs: run the linter, fix
errors, and read layout-flavoured warnings as noise rather than as
something to work around.

## Worked example (from `examples/gallery/01_business_report_basics.doclang`)

```
---
title: "Quarterly Business Report — Q4"
author: "DocLang Team"
date: "2025-01-15"
theme: professional
---

# Executive Summary

This report summarizes Northwind Analytics' performance for the fourth
quarter, covering revenue, customer growth, and operational highlights.

::: success
Revenue grew 24% quarter-over-quarter, exceeding the internal target of 18%.
:::

## Key Metrics

| Metric | Q4 | Q3 | Change |
|--------|----|----|--------|
| Revenue | $387K | $312K | +24.0% |

# Revenue Breakdown

## By Region

- North America: 52% of total revenue
- Europe: 31% of total revenue

# Next Quarter Priorities

- [x] Close the Series B funding round
- [ ] Expand the support team by two engineers

> "This was our strongest quarter yet."
>
> **— CEO, Q4 all-hands**
```

Note how `## Key Metrics` and `## By Region` are nested headings *inside*
the `# Executive Summary` / `# Revenue Breakdown` sections, not their own
sections — that's the section-hierarchy rule in action.
