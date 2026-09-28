# Portable table row and cell identities

Status: opt-in AST capability `table-rows-v1` (schema version 2.15.0).
Documents without `tableRows` continue to emit AST 2.14.0 without a
`capabilities` field. This does not change the existing pipe table or
`TABLE/cells:` source forms.
Readers also accept existing 2.13.0 documents without `tableRows`.

## Authoring

Use one `TABLE/tableRows:` block as the authority for any table that needs
durable row/cell identity or row-section semantics:

```slidelang
<!-- node-id: TableA -->
TABLE
  tableRows:
    - nodeId: HeaderRow
      section: header
      cells: [{nodeId: KeyHeader, content: Key, header: true, scope: col}, {nodeId: ValueHeader, content: Value, header: true, scope: col}]
    - nodeId: DataRowA
      section: body
      cells: [{nodeId: KeyA, content: A}, {nodeId: ValueA, content: "1"}]
```

`nodeId` is optional on each row and authored cell. Nonempty IDs share the
document-wide namespace and alphabet of other `nodeId` values. Copying an
identified row/cell requires fresh explicit IDs; duplicates are errors. The
parser and formatter never generate or rename IDs. `section` is optional and
means `body` when omitted; valid values are `header`, `body`, and `footer` in
that order. It declares content structure, not emphasis, tone, or styling.
The core HTML renderer emits these sections as `<thead>`, `<tbody>`, and
`<tfoot>`; a rowspan crossing section boundaries is rejected.
Existing cell `header`, `scope`, `colspan`, and `rowspan` keep their current
meaning. A merged cell's ID belongs to its authored anchor. Covered grid
coordinates have no IDs. Invalid overlap, incomplete coverage, or a span that
extends beyond the declared rows is an error.

`headers:`, `rows:`, and `cells:` cannot be authored alongside `tableRows:`.
In AST JSON they remain deterministic compatibility projections computed
from the canonical authored rows. A JSON/filter response with divergent
projections is rejected. Consumers should resolve row/cell IDs through
`ast.ResolveTableIdentity`; the returned record is the authored target and
must be resolved again after a transform replaces the AST. A missing result
is an orphan reference for the consuming layer to reject. This API does not
define a private treatment or appearance grammar.

## JSON compatibility and filters

Opt-in AST JSON has `schemaVersion: "2.15.0"`,
`capabilities: ["table-rows-v1"]`, and `tableRows` on each participating
table. The 2.15 schema and generated TypeScript types describe both legacy
and opt-in shapes; the Go decoder checks the version/capability relationship
and projections on raw JSON before permissive Go unmarshalling can discard
identity. Unsupported versions and malformed row records fail.

For a document with `tableRows`, the CLI asks **each** external filter for
capabilities before sending AST bytes. It runs the filter with
`--ziradocs-capabilities`, closed empty stdin, a timeout, and a bounded
response. A compatible filter responds with exactly:

```json
{"astSchemaVersions":["2.15.0"],"features":["table-rows-v1"]}
```

An incompatible or invalid response aborts the build. After execution, the
CLI validates raw JSON and rejects missing/changed row/cell IDs, owner moves,
missing `tableRows`, version/capability downgrade, and divergent projections.
To verify ownership without content or position matching, a filter processing
`tableRows` requires an explicit table `nodeId`; an identified cell also
requires its authored row to have a `nodeId`. Legacy 2.14 filter invocations
retain the previous protocol with no handshake. This negotiation protects
the CLI's controlled filter path; it cannot change the behavior of old
consumer binaries elsewhere. No automatic downgrade or lossy export is
provided.
