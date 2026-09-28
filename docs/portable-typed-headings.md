# Portable typed headings

Status: opt-in AST capability `typed-headings-v1` (schema version 2.17.0).
Without the opt-in, SlideLang and DocLang keep representing a subsection
heading as a raw-HTML `TextElement` (`<hN id="…">`) with `level`, exactly as
before. The strict SlideLang syntax described below is available in both
cases, which is what closes the `slidelang fmt` loss reported in issue #259.

## Authoring

Declare the capability in frontmatter, in either DSL and dialect:

```yaml
---
mode: flex
ast_capabilities: [typed-headings-v1]
---
```

It can be combined with `nested-list-types-v1` in the same list. The flex
syntax does not change: `###`–`######` inside a SlideLang slide, `##`–`######`
inside a DocLang section.

Strict SlideLang gains a heading element inside a `SLIDE`. It reuses the
DocLang `SECTION` keyword and its properties:

```slidelang
SLIDE content
  title: "Quarterly review"
  SECTION "Results **by region**"
    level: 3
  TEXT
    Body text.
  SECTION "Details"
    level: 4
    id: region-details
```

- The title is quoted on the opening line; inline Markdown is allowed and is
  preserved as authored.
- `level:` accepts 3–6 and defaults to 3, matching flex, where `#` opens a
  slide and `##` is its subtitle.
- `id:` overrides the HTML anchor. It is normalized to `[a-z0-9_-]`, and a
  duplicate is an error.
- Unlike DocLang, a heading inside a slide has no body: the elements that
  follow it stay at the slide's element indentation.

Without `id:`, the anchor is `heading-` plus the anchor derived from the text,
with a numeric suffix when an earlier heading in the same deck already used
it. Flex and strict use the same sequence for headings that are direct
children of a slide, so both dialects produce the same AST for them. Flex also
recognizes headings inside `:::` blocks, and those take their place in the
sequence; strict has no heading syntax inside a block, so the strict
formatters refuse a typed heading nested in a block instead of turning it into
text (the DocLang flex formatter keeps it). DocLang anchors keep their
existing rule (derived from the text, or `id:` on a strict `SECTION`).

## AST

```json
{
  "type": "heading",
  "nodeId": "ResultsA",
  "level": 3,
  "text": "Results **by region**",
  "anchor": "heading-results-by-region",
  "textHTML": "Results <strong>by region</strong>"
}
```

`text` is authored inline source, never HTML. `anchor` is the resolved HTML
id. `textHTML` is filled by the renderer like every other `*HTML` field and
is never trusted from a filter. `nodeId` is the optional editorial identity
from `<!-- node-id: Name -->`. It is independent of `anchor`: reordering two
headings with the same text can move the numeric anchor suffix, but each
`nodeId` stays on its own heading, and the formatter writes `id:` whenever an
anchor would not be re-derived in its new position.

## JSON versions and consumers

The version of a document is the version of the newest extension it actually
uses. A source opt-in that produces no heading keeps the lower version.

| Extensions actually present | `schemaVersion` | `capabilities` |
| --- | --- | --- |
| None | `2.14.0` (also reads `2.13.0`) | absent |
| `tableRows` only | `2.15.0` | `["table-rows-v1"]` |
| Typed nested lists, optionally with `tableRows` | `2.16.0` | those capabilities |
| Typed headings, optionally with the others | `2.17.0` | those capabilities |

The decoder and the JSON Schema reject an unknown version or capability, a
duplicate capability, a heading without `typed-headings-v1`, the capability
without any heading, unknown heading fields, a level outside 1–6, empty or
multi-line text, and an anchor outside `[a-z0-9_-]`.

HTML, PDF, Markdown, DOCX, PPTX, the DocLang table of contents, and section
numbering render a typed heading through its legacy form, so the output is
byte-for-byte identical to the same document without the opt-in. JSON output
keeps the typed node.

An external `--filter` receives a 2.17.0 AST only after the capability
handshake advertises `typed-headings-v1`. A filter may edit text or anchors
and reorder headings, but it may not drop the capability, remove or rename an
identified heading, or change its level. A reader built against a core older
than 2.17 does not understand this node and must not be used as a direct
consumer of 2.17.0 JSON.

## Formatting

`slidelang fmt` writes every subsection heading, typed or legacy, as a strict
`SECTION` with `level:`, and adds `ast_capabilities: [typed-headings-v1]` only
when the document uses typed headings. A typed heading keeps its authored
inline Markdown exactly; a legacy heading is recovered from its HTML, which
drops inline emphasis as documented for the DocLang formatter. `doclang fmt`
writes typed headings back as `##`–`######` in flex or `SECTION` in strict. In
flex it refuses a heading whose anchor cannot be derived from its text, since
flex has no syntax to declare it.
