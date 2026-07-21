# DocLang — Flex Mode (the only mode)

**DocLang has no strict mode and no `mode:` switching.** The `doclang`
CLI *always* parses with the flex/document parser
(`NewDocumentFlexParserWithNormalization`), regardless of what — if
anything — a `mode:` frontmatter key says. If you're used to ZiraDocs's
three-mode system, drop that mental model for DocLang entirely: there is
one grammar.

(A `docs/doclang/DOCLANG_SYNTAX_STRICT.md` file exists in the source repo
describing an aspirational strict syntax — **it is not implemented by the
parser**. Do not emit that syntax; it will not parse the way that document
describes.)

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
include), `title`, `author`, `date`, `theme`, `variables`, `header`,
`footer`, `layout_defaults`, `lint_policy`. See `frontmatter.md` for
details.

**Important gap to know about:** the `doclang init` template and some
example files write `toc: true`, `numbering: true`, `doctype: ...`, or
`page: ...` into frontmatter. **None of these keys are parsed** — YAML
silently drops unknown keys, so they have zero effect on the build. Table
of contents and page numbering are controlled by **CLI flags**,
`--toc`/`--numbering`, not by frontmatter. Don't tell a user "add `toc:
true` to your frontmatter to get a table of contents" — it does nothing;
the flag is what matters.

## Elements

Same shared element parsers as ZiraDocs flex mode — see
`elements.md` for the full per-element table (charts, mermaid,
maps, tables, special blocks, code, code-groups, checklists, math, etc.).
All of it works identically inside a DocLang section — with one exception:
directives (`@notes`, `@timer`, …) parse, but the document renderer emits
nothing for them, so they silently vanish from DocLang output. `@include`
is the exception to the exception: it is expanded before parsing, so it
works in both formats.

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
`CODEGROUP001/002`, `IMG001`, `CODE001`, `CHART001`, `SPECIAL001`.

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
