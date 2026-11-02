# Portable nested list types

Status: opt-in AST capability `nested-list-types-v1` (schema version 2.16.0).
Without the opt-in the AST keeps its legacy shape (no `subListType`, schema
2.14.0), but lists still nest to any depth through the recursive `subPoints`;
see "Without the opt-in" below. The extension uses existing `nodeId` comments for stable
item identity; it does not derive identity from text, numbering, or position.

## Authoring

Declare the capability in frontmatter in either DSL and dialect:

```yaml
---
mode: strict
ast_capabilities: [nested-list-types-v1]
---
```

`table-rows-v1` is inferred from authored `tableRows` and is not a source
`ast_capabilities` entry. The source key accepts the nested-list opt-in and
`typed-headings-v1` (see [portable typed headings](portable-typed-headings.md)),
alone or together.

Then use the existing list markers and indentation. A parent item owns the
list directly beneath it:

```slidelang
POINTS
  1. ParentA
    - ChildA
      1. ChildB
  2. ParentB
    1. ChildC
```

The outer `PointsElement.listType` is `ordered`. ParentA has
`subListType: "unordered"`; ChildA has `subListType: "ordered"`;
ParentB has `subListType: "ordered"`. Leaves have no `subListType`.
`subListType` describes the **children's list**, never the marker used by
the item itself. The first marker at a given level defines its list type.
Mixing ordered and unordered markers among siblings in one nested list is an
error; separate subtrees may use different types. In flex mode, changing the
outer marker starts a new `PointsElement`, as it did before this extension.
`1)` is an ordered marker like `1.`, as in CommonMark; in flex, changing the
delimiter between outer ordered items (`1.` to `1)`) also starts a new
`PointsElement`. The delimiter is not serialized.
Empty items and malformed markers inside an opt-in `POINTS` block are errors.
Numbering is presentation order and is never used as an ID.

To keep identities through edits, place `<!-- node-id: ParentA -->` and
`<!-- node-id: ChildA -->` before the corresponding items. IDs remain
optional for ordinary parsing; existing document-wide uniqueness and syntax
rules apply. A filter processing this capability requires IDs on the points
element and every item so parenthood can be checked without matching prose
or array offsets.

## JSON versions and consumers

| Source features actually present | `schemaVersion` | `capabilities` |
| --- | --- | --- |
| No opt-in list types or authored table rows | `2.14.0` (also reads `2.13.0`) | absent |
| Authored `tableRows` only | `2.15.0` | `["table-rows-v1"]` |
| Typed nested lists only | `2.16.0` | `["nested-list-types-v1"]` |
| Both extensions | `2.16.0` | both capability strings |
| Any of the above plus typed headings | `2.17.0` | adds `typed-headings-v1` |
| Any of the above plus media poster/caption | `2.18.0` | adds `media-figure-v1` |
| Any of the above plus code filenames | `2.19.0` | adds `code-filename-v1` |
| Any of the above plus quiz/poll results | `2.20.0` | adds `quiz-poll-results-v1` |

The capability array is a set when read; duplicate and unknown entries fail.
The writer emits a deterministic order. A source declaration with no nested
child list does not fabricate `subListType` or raise the JSON version.
That unused source-only declaration is not carried through an AST JSON
round-trip because no extension feature is present.
Every parent with `subPoints` in 2.16 must declare `subListType`, and a leaf
cannot declare it. JSON lacking the matching version/capability fails before
unknown fields can be discarded. There is no legacy downgrade for 2.16.

Core HTML, SlideLang HTML, DocLang Markdown, PPTX and DOCX render the opt-in
hierarchy and types. DOCX writes native Word lists: each item is a numbered
paragraph at its level (`w:ilvl`), and every list, including each child list,
has its own numbering definition, so an ordered child list starts at its own
first ordinal. PPTX and DOCX have nine levels; deeper typed lists fail before
output.

## Without the opt-in

A legacy list nests by indentation to any depth, following CommonMark: an
item is the child of the item above it only when its marker starts at or past
that item's content column (the column after the marker and its space); a
shallower indentation makes it a sibling or returns to an outer level.
Checklists do the same. Strict keeps its indentation grammar: any item indented
deeper than the one above it is its child. The JSON shape is the one the schema always allowed:
recursive `subPoints`/`subItems`, no `subListType`, no `start`, schema 2.14.0,
no capability. Earlier parsers flattened every level past the second into the
second, so a reader that only walks two levels now misses the rest; the
readers in this repository (core HTML, SlideLang HTML and PPTX, DocLang DOCX
and Markdown) walk the whole tree. PPTX and DOCX draw an untyped list deeper
than nine levels at their last level instead of failing.

The type of each sublist, the marker of each item and the number written before
an ordered item live in memory only (`SubListMarker`, `Marker`, `Ordinal`). Core
HTML draws each sublist with its own type, starts a new list when the marker
changes between siblings, and writes `<ol start>` from the first number; DOCX
starts its Word numbering there too. After a JSON round-trip that information
is gone: a sublist takes the type of its parent and numbering starts at 1. To
serialize types and starts, declare `nested-list-types-v1` and `list-start-v1`.

Previously published readers cannot be changed retroactively. A reader that
ignores unknown JSON fields must not receive a 2.16 document without an
external version/capability gate: it could discard `subListType`. The
controlled CLI filter path below supplies that gate.

For external filters, the CLI sends no AST bytes until the filter reports
support for the document's schema version and every active capability through
`--ziradocs-capabilities`. A filter may edit content or reorder siblings,
but must retain the declared type on each nested parent and keep node IDs
with their parent. The decoded result is validated again. The handshake
retains its existing timeout and bounded response. Legacy documents still
use the prior filter protocol.

For a list-only document, a compatible filter may respond:

```json
{"astSchemaVersions":["2.16.0"],"features":["nested-list-types-v1"]}
```

For a document with both extensions, `features` must also include
`table-rows-v1` and the filter must handle both contracts.
