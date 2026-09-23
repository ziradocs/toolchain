# Portable node identities

Status: implementation contract for the public AST and source dialects.

## Decision

A standalone `<!-- node-id: Name -->` line names exactly the AST node that
starts on the next nonblank line. It works in SlideLang strict and flex and in
DocLang strict and flex through the shared parser. IDs use 1–128 ASCII
characters: an initial letter followed by letters, digits, `.`, `_`, or `-`.
They are case sensitive, document-wide unique, optional, and never generated
by a build or formatter. A copy of an identified node needs a new explicit ID;
reusing the old ID is an error. The formatter preserves explicit IDs, including
when content is inserted or reordered.

`nodeId` is an additive optional field on AST nodes with `BaseNode`. It is
distinct from existing table, equation, chart, and heading `id`/`label`
fields. Those continue to drive cross references and HTML anchors; `nodeId`
does not change their rendering. Existing sources and JSON without `nodeId`
retain their behavior. The JSON schema version is 2.14.0.

The directive is a source annotation, not prose. A directive must precede a
node opener or heading. It does not bind to a delimiter, layout metadata,
plain untyped text, or a text match. A directive in literal code or diagram
content stays literal. Missing targets, multiple nodes on the target line,
invalid IDs, and duplicate IDs are errors with source positions. No fallback
matches by content, hash, or relative position after an edit.

Anchored flex input bypasses heuristic normalization so the authored node
boundaries and source locations remain authoritative; unanchored input keeps
the existing normalization behavior. Identity is available on blocks,
elements, and typed nested nodes that the parser exposes. A raw grid column
body or raw code fragment is not a separate AST node and cannot be identified
independently. The formatter rejects an identified node if its target dialect
cannot preserve that node's structure.

Transforms may intentionally delete nodes. Surviving nodes retain their IDs
through JSON serialization; a transform that creates a node must either leave
it unlabelled or assign a fresh explicit ID. The tree is checked for invalid
and duplicate IDs after each built-in transform and external filter. Consumers
must validate references to IDs against the final transformed tree; deletion
of a referenced node is an error in the referencing consumer, not an implicit
request to restore the deleted node.

## Example

```slidelang
<!-- node-id: SummarySlide -->
SLIDE content
  title: "Summary"
  <!-- node-id: RevenueTable -->
  TABLE
    headers: ["Year", "Revenue"]
    rows:
      - ["2026", "42"]
```

Moving either annotated node, or editing its text, does not change its ID.

## Consumption

The repository has three Go modules. In a candidate checkout, use a temporary
`go.work` referencing `core`, `slidelang`, and `doclang` from the **same
commit**, and record the commit and `go env GOWORK` for every validation.
`GOWORK=off` instead resolves the versions pinned in each module's `go.mod`;
it does not exercise this candidate core until a separate release and pin
update. No absolute `replace` belongs in committed module files.
