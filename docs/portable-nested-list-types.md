# Portable nested list types

Status: opt-in AST capability `nested-list-types-v1` (schema version 2.16.0).
Without the opt-in, SlideLang and DocLang retain their existing list AST and
rendering behavior. The extension uses existing `nodeId` comments for stable
item identity; it does not derive identity from text, numbering, or position.

## Authoring

Declare the capability in frontmatter in either DSL and dialect:

```yaml
---
mode: strict
ast_capabilities: [nested-list-types-v1]
---
```

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

The capability array is a set when read; duplicate and unknown entries fail.
The writer emits a deterministic order. A source declaration with no nested
child list does not fabricate `subListType` or raise the JSON version.
Every parent with `subPoints` in 2.16 must declare `subListType`, and a leaf
cannot declare it. JSON lacking the matching version/capability fails before
unknown fields can be discarded. There is no legacy downgrade for 2.16.

Core HTML, SlideLang HTML, DocLang Markdown, DOCX, and PPTX render the
opt-in hierarchy and types. PPTX has nine paragraph levels; deeper typed
lists fail before output. Existing non-opt-in output is unchanged.

For external filters, the CLI sends no AST bytes until the filter reports
the exact schema version and every active capability through
`--ziradocs-capabilities`. A filter may edit content or reorder siblings,
but must retain the declared type on each nested parent and keep node IDs
with their parent. The decoded result is validated again. The handshake
retains its existing timeout and bounded response. Legacy documents still
use the prior filter protocol.
