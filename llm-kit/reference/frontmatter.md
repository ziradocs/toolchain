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
variables:             # arbitrary key/value map, used for {{variable}} substitution
  company: "Acme Inc"
header:                # optional rich header config (see below)
footer:                # optional rich footer config (see below)
layout_defaults:       # per-layout header/footer overrides (see below)
lint_policy:           # per-document linter policy (see below)
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
- DocLang parses this field but **ignores it entirely** — the doclang CLI
  always uses its own flex/section parser regardless of what `mode` says.
  Including `mode: flex` in a `.doclang` file is harmless but has no
  effect; you can also omit it.

### Known-ignored keys (do NOT rely on these)

`toc`, `numbering`, `doctype`, `page` — these appear in some `doclang
init` templates and example files but are **not** part of the parsed
schema. Table of contents and page numbering in DocLang output are
controlled by the `--toc` / `--numbering` **CLI flags** passed to
`doclang build`, not by frontmatter.

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
    format: "{current} / {total}"
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

Only include these if you actually need custom header/footer chrome —
most decks and documents don't need them at all.

## Theme resolution priority (both CLIs)

**CLI `--theme` flag > frontmatter `theme:` > default.** If you want a
specific theme to be guaranteed regardless of how the file is built,
prefer passing `--theme` at build time over relying on frontmatter alone.
