# DocLang — Strict Mode

DocLang has two dialects. This is the **declared** one: you write the structure
instead of letting the parser infer it, and **the normalizer never runs**. What
you read is what gets parsed — which is what makes a strict document reviewable
in a pull request and meaningful to sign off on.

Use flex to draft. Use strict for the version that gets committed.

Opt in with `mode: strict` in the frontmatter. There is no CLI flag: the dialect
belongs to the document.

## Minimal valid file

```
---
mode: strict
title: "Retention Policy"
---

SECTION "Overview"

  TEXT
    Every block is declared. Nothing is inferred.
```

## Sections

A document is a sequence of `SECTION` blocks. The title is part of the opening
line and **must be quoted**:

```
SECTION "Introduction"
```

Properties go on **indented lines below** the opening line — never inline.
`SECTION "Intro" level: 2` is an error.

| Property | Accepted on | Meaning |
|---|---|---|
| `level:` | any section | 1-6. Defaults to 1. |
| `id:` | levels 2-6 only | Pins the anchor instead of deriving it from the title. |

`level:` declares the hierarchy — **indentation does not**. Sections are never
nested syntactically; an indented `SECTION` is an error, not a subsection.

```
SECTION "Guide"

  TEXT
    Intro paragraph.

SECTION "Installation"
  level: 2
  id: install

  TEXT
    Steps go here.
```

A level-1 section becomes a document section. Levels 2-6 become headings
*inside* the section above them — the same structure `#` and `##` produce in
flex, which is why both dialects render identically.

A level-2 section with no level-1 section before it is an error (there is
nothing to nest it under).

### `id:` normalization

`id:` is normalized to anchor form: lowercased, spaces to hyphens, then narrowed
to `[a-z0-9_-]`. `id: Install_Steps` becomes `install_steps`. That normalized
form is canonical, so `doclang fmt` emits it. An `id:` with no surviving
characters (only emoji, say) is an error rather than an empty anchor.

`doclang fmt` omits `id:` when it would be derivable from the title, and keeps
it when it would not.

## Elements

Inside a section body, indented two spaces, the element vocabulary is **the same
as SlideLang strict** — see `slidelang-strict.md` for the full syntax of each:

`TEXT`, `POINTS`, `CODE <lang>`, `IMAGE "src" "alt"`, `TABLE`, `QUOTE`,
`CHECKLIST`, `:::info` / `:::warning` / `:::danger` / `:::success`,
`:::code-group`, `<<mermaid>>`, `<<plantuml>>`, `<<chart: type>>`, `<<map>>`,
`<<grid>>`, `<<math>>`, `@directives`.

The one thing to unlearn from flex: **there is no `SLIDE`**, and the
slide-only properties (`title:`, `heading:`, `subtitle:`, `logo:`) are errors
inside a `SECTION`. A section's title is the quoted string on its opening line.

### What strict unlocks

`label:` on tables and figures is only reachable in the strict dialect, so
cross-references work:

```
SECTION "Results"

  TABLE
    headers: ["Metric", "Value"]
    rows: [
      ["Throughput", "1200 rps"]
    ]
    caption: "Measured throughput"
    label: tbl-throughput

  TEXT
    See \ref{tbl-throughput} for the numbers.
```

This is the main functional reason to pick strict over flex, beyond
auditability: in flex, a table cannot be labelled at all.

## Not part of the grammar

Early design sketches of this dialect described `numbered:`, `pagebreak:`,
`doctype:`, `<<toc>>`, `<<ref: …>>`, `<<footnote>>`, `<<bibliography>>` and
`<<pagebreak>>`. **None of these parse.** Table of contents and numbering are
CLI flags (`--toc`, `--numbering`); cross-references use `label:` and `\ref{…}`
as shown above. Do not emit the others.

## `@include`

`@include` expands textually, *before* the parser. In strict — where indentation
is syntax — that means **the fragment's own indentation decides where it lands,
not the directive's**:

- A fragment of complete sections is written at column 0.
- A fragment meant for a section body is written already indented.
- Indenting the `@include` line itself does **not** indent the fragment.

## Formatting

```
doclang fmt doc.doclang            # keeps the document's own dialect
doclang fmt draft.doclang --strict --write   # promotes a flex draft to strict
```

`--strict` on a flex document is a **transpilation**, not a reformat — it
rewrites the document into the other dialect. It is not the default (unlike
`slidelang fmt`, where strict is the only canonical form), because DocLang's two
dialects are both legitimate and transpiling unasked would rewrite every
existing flex document.

Downgrading strict → flex is refused: it would discard the author's declaration
with no way back. Change `mode:` by hand if you really want that.

One known lossiness, shared with the flex formatter: inline formatting in a
*section title* (`SECTION "The **important** part"`) degrades to plain text on a
round trip. The stored heading HTML is not invertible — a hand-typed `<strong>`
produces the same element as `**bold**`. The result is stable and re-parses
clean; it just loses the emphasis. Element bodies are unaffected.
