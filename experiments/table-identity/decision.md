# Portable table row and cell identity: design probe

Status: **proposal only**. No source grammar, AST, schema, formatter, or consumer
contract in this directory is implemented by the current CLI. The companion
`run_probe.py` exercises only synthetic input against an exact CLI commit.

## Current boundary

At base `7f08d545c20a24e333deb4e961e869dcf3f4a460` (AST 2.14.0),
`TableElement` has `nodeId` through `BaseNode`. `Headers []string`,
`Rows [][]string`, and `Cells [][]TableCell` have no authored row/cell IDs.
`RowPositions` points to source rows for diagnostics; its length can be shorter
than `Rows`, especially for merged cells. For pipe/simple tables, `Cells` is
derived from `Headers/Rows`; for explicit `cells:` tables, the reverse happens
with spans expanded into a rectangular flat view. `ast.Walk` and the node ID
validator do not visit rows or cells. The formatter emits pipe syntax if the
cell structure is simple and there is no caption/label; otherwise it emits
`TABLE/cells:`. It serializes only currently known cell keys. The current
`<!-- node-id: ... -->` directive names an AST node opener, not a row or cell.

## Decision proposed for a separate implementation PR

Use **one authored structured table form** whenever row or cell identity is
needed. A conceptual example (not accepted syntax) is:

```yaml
TABLE
  rows:
    - nodeId: HeaderRow
      section: header
      cells:
        - {nodeId: KeyHeader, content: Key, header: true, scope: col}
        - {nodeId: ValueHeader, content: Value, header: true, scope: col}
    - nodeId: DataRowA
      section: body
      cells:
        - {nodeId: KeyA, content: A}
        - {nodeId: ValueA, content: "1"}
```

This reuses the existing `TABLE` construct and cell attributes. It introduces
row records as the **only authored authority** for an identified table, not a
second content grammar. Proposed AST `tableRows` (name subject to review)
contains row records with `nodeId`, optional `section`, and owned cell records
with `nodeId`, content, header/scope/span. Legacy `headers`, `rows`, and
`cells` are read-only compatibility projections in output. They must be
computed from `tableRows` before serialization and after every transform;
an input AST/filter that supplies conflicting projections must fail with a
named diagnostic rather than choosing one silently. The formatter serializes
only the canonical row records for this form. Its parser regenerates the same
projections after reparse. Pipe and existing `TABLE/cells:` sources remain
valid and get no fabricated row/cell IDs. A source migration tool could
optionally rewrite an existing table to row records, but must ask the author
to supply IDs for targets they want stable; normal `fmt` must never invent
them.

**ID policy.** IDs remain optional for each row and each authored cell, even
within the structured form. A consumer may require IDs for the particular
targets it references; the core parser should not require a complete grid of
IDs. Empty IDs carry no durable identity. All nonempty row/cell IDs obey
`ValidNodeID` and share one document-wide registry with existing node IDs,
including slides and tables. Duplicating an identified row/cell without
changing IDs is a duplicate error. No implicit rename. References to an ID
belong to a separate portable treatment layer; that layer validates targets
after transforms and rejects orphan IDs. The current core has no row/cell
reference field, so an orphan treatment cannot currently be exercised as a
CLI feature. An orphan source directive can be exercised today.

**Semantics and spans.** A row's `section` would be portable content semantics
(`header`, `body`, `footer`), separate from its identity. A cell's `header`
and `scope` retain their current accessibility meaning. Other general
semantics, such as a status value, need a separately justified typed field;
they must not be inferred from text or carried in an appearance treatment.
Private emphasis, tone, or appearance remain outside this public model.
A merged cell has one identity on the authored anchor cell. Coordinates and
expanded flat placeholders have no IDs and are never reference targets.
Validation must detect overlap, out-of-grid spans, and row reorder/insert/
delete that makes coverage invalid, with diagnostics attached to the affected
authored row/cell ID. A valid span may change coordinates without changing its
anchor identity. The current flattening function explicitly tolerates some
overlap/undersized cases, so this would require new validation for the new
form rather than treating the old flat projection as proof of validity.

## Alternatives considered

| Approach | Result |
| --- | --- |
| Parallel `rowIds[]` / `cellIds[][]` beside `Rows` | Reject: index coupling breaks on reorder/spans and creates competing authorities. |
| Content, coordinates, source positions, or hashes | Reject: duplicates and edits make identity ambiguous or unstable. |
| Put `nodeId` directly into existing `Cells[][]` only | Incomplete: no row identity; current simple-table formatter can drop cell metadata and merged flattening duplicates content. |
| Use inline comments before each row/cell | Insufficient as the sole form: no distinct AST row/cell node opener today, especially in flow-style YAML and spanning cells. Could be sugar later if proven unambiguous. |
| Structured authored rows with owned cells | Recommended: one authority, explicit identity, stable under edits and reorder, reuses existing table/cell vocabulary. |

## Compatibility gate before implementation

| Surface | Current 2.14.0 | Proposed candidate gate |
| --- | --- | --- |
| Legacy source | Pipe or `TABLE/cells:`; no row/cell IDs | Parse identically; no generated IDs; old `fmt` behavior preserved. |
| New source | Unsupported; unknown keys may be ignored | New form parses explicitly or fails; never silently treats it as legacy `rows:`. |
| JSON AST | Closed schema; table fields fixed | Versioned `tableRows`; regenerate schema and TS types, define strict projection consistency. |
| Go `DecodeAST` | `encoding/json` ignores unknown fields | Reject unsupported `schemaVersion`/identity-bearing unknown fields in old readers or provide explicit downgrade rejection; never silently erase IDs. |
| Formatter | Pipe or `cells:` output | Structured form whenever canonical row records exist; parse→format→reparse preserves IDs, section, spans, and content. |
| Consumers/filters | May expect only 2.14 fields | Capability/version gate; explicit rejection or a deliberate lossy export requiring opt-in. |

A minor bump is **not automatically safe** despite optional JSON fields: a
2.14 Go decoder can silently discard an unknown `tableRows` field, and a
formatter using the remaining projections can then erase identities. The JSON
schema is closed (`additionalProperties: false`), so old validators reject the
new field. The implementation decision must choose a version and migration
policy after testing real reader behavior, and ensure all identity-bearing
paths fail visibly on unsupported readers. Keeping the current `cells`
projection while adding canonical rows is an additive shape only for readers
that ignore new fields *and do not round-trip them*; it is not a preservation
guarantee.

## Probe interpretation

`run_probe.py` writes source, CLI build JSON, formatter output, reparse JSON,
exit status, diagnostics, SHA-256 digests, and a manifest into a user-chosen
directory. Its inputs cover duplicate content, reorder, insert, edit, delete,
table ID, row/cell ID attempts, duplicate/orphan directives, and regular and
merged tables. The manifest must be read as **current CLI evidence**. The
structured example above is design notation only and has not been fed to the
CLI as a claimed supported form.

### Observed current CLI (exact commit `6b88c45250650bea5e70825de44299d22e2ccbbd`)

The committed [evidence](evidence-6b88c45/manifest.json) contains 18 synthetic
cases and complete source/AST/formatter outputs. CLI binary SHA-256 is
`3848f4f74c8317c65d4fead4b98d8d0e162c731a50c6ce88c8d9e82d1bdc249a`.
On Ubuntu, from a detached exact-SHA checkout with a temporary `go.work` for
`core`, `slidelang`, and `doclang`, the commands were:

```sh
GOWORK="$out/go.work" GOMODCACHE=/mac-remote-development/caches/go-mod \
  GOCACHE=/mac-remote-development/caches/go-build \
  go build -o "$out/slidelang-final" ./cmd/slidelang
python3 experiments/table-identity/run_probe.py \
  "$out/slidelang-final" "$out/probe-final-exact"
```

| Case | Observed result |
| --- | --- |
| Regular duplicate rows, reorder, insert, edit, delete | Build/format/reparse succeed; `rows` and `cells` reflect new order/content, with no row/cell IDs. Duplicate content is indistinguishable by identity. |
| Whole-table `nodeId` | `TableA` survives build/format/reparse. |
| Row marker; cell marker | Build and `fmt` fail with `orphan node-id` (`RowA`, `CellA`). |
| Duplicate node IDs; slide/table collision | Build and `fmt` fail with `duplicate nodeId "Reused"`. |
| Orphan node marker | Build and `fmt` fail with `orphan node-id "Missing"`. |
| `id` keys inside `cells:` | Build/format/reparse succeed, but `HeaderKey`/`CellA` do not appear in the AST or formatted source: silent loss. |
| Valid merged table | Build/format/reparse preserve authored cell spans; expanded `rows` duplicate covered content; `rowPositions` absent. |
| Reordered/deleted/inserted rows affecting rowspan | Build/format/reparse still succeed without a span diagnostic. The expanded view can gain blank placeholders or extra columns; this does not prove the authored grid valid. |

The CLI currently has no treatment/reference grammar for row/cell IDs, so an
orphan *reference* cannot be tested as current behavior. The orphan directive
cases test the existing node-ID parser only. A future implementation PR needs
focused tests of registry collisions across slide/table/row/cell, of orphan
references after transforms, and of exact projection checks on JSON/filter
input; this proposal does not claim those tests already pass.

## Reader and downgrade seam (isolated follow-up probe)

`reader_probe.go` takes a real 2.14 CLI JSON fixture and injects a conceptual
`tableRows` field with row/cell IDs. It tests four synthetic payloads against
the committed 2.14 JSON Schema, `ast.DecodeAST`, and `formatter.FormatStrict`.
`filter_candidate.py` injects the same field into the live `--filter` response.
`99.0.0` is a deliberately unsupported test sentinel, **not** a proposed
release number. Neither file implements the new table model.

| Payload | 2.14 schema | 2.14 `DecodeAST` | 2.14 formatter after decode | Pre-decode guard sketch |
| --- | --- | --- | --- | --- |
| 2.14 baseline | accepts | accepts | succeeds | accepts |
| Version sentinel only | **accepts** (version is any string) | accepts and retains sentinel | succeeds | rejects unsupported version |
| `tableRows` only, version 2.14 | rejects (closed table shape) | accepts, **drops `tableRows`** | succeeds without row/cell IDs | rejects unsupported field |
| Version sentinel + `tableRows` | rejects field | accepts, retains sentinel but **drops `tableRows`** | succeeds without row/cell IDs | rejects both via version check |

The current CLI `--filter` follows the same loss path: the filter emits the
sentinel version and row records; `RunFilters` calls `DecodeAST` on that output,
then the CLI succeeds and its JSON contains the sentinel version but no row
records. Existing `fmt` parses source, not foreign AST JSON; the formatter
result above is through its public Go API after decode. The schema reports a
generic union error, but `$defs.TableElement.additionalProperties` is `false`
and `tableRows` is absent from its property list. These observations are
captured in the [reader evidence](reader-evidence-73d4/reader-results.json).

**Where to reject:** a new AST/JSON reader must inspect `schemaVersion` and
the raw table object before `json.Unmarshal`/`DecodeAST` can erase unknown
identity-bearing fields. A strict schema validation step can reject unknown
fields, but the current schema does not gate its version string. The proposed
new decoder/filter ingress should combine explicit supported-version and
capability checks with strict unknown-field/projection validation. A new
formatter must reject a table whose canonical records it cannot serialize;
it must never fall back to the legacy `cells`/pipe view when that would erase
IDs. The isolated pre-decode guard in `reader_probe.go` demonstrates the seam;
it is **not installed in the CLI**. Existing binaries cannot be made safe by
changing a future producer's version number alone.

**Recommended policy for the implementation decision:**

1. Keep legacy source and JSON with no row/cell identity on the 2.14 contract,
   preserving current pipe/`cells:` behavior. A new CLI may offer explicit
   2.14 output only when it can prove it contains no new identity/semantics.
2. An authored row/cell ID requires a newly negotiated AST capability and a
   versioned schema/type/formatter contract (version number still undecided).
   Produce only for readers/filters that declare support; otherwise reject
   before handing over JSON or invoking an old `--filter` binary.
3. Reject automatic downgrade from the new contract to 2.14. A separately
   named, explicitly lossy projection export could be offered for display,
   with no promise that row/cell references survive. Never feed that export
   back as an identity-preserving source.
4. At new JSON/filter ingress, check supported version/capabilities on raw
   bytes, validate row/cell IDs and projection equality, then decode. Fail on
   unsupported versions, unknown identity-bearing fields, conflicts, and
   orphan references. Only after this gate may a formatter or transform run.

This policy entails work in schema generation, TypeScript types, Go decode,
filter capability negotiation, both formatters, and cross-version tests. It
does not require rewriting or mutating existing source files. The choice of
release number and the precise source spelling remain gates for a separate
implementation PR.
