# Portable rectangular chart content

The additive `chart-content-v1` capability uses AST schema 2.23.0. A chart can
carry `unit?: string` as literal content and `showDataLabels?: boolean` as an
explicit presentation intent. Absence, empty unit, and `false` stay distinct.
No unit interpretation, trimming, translation, or `Intl.NumberFormat` unit
conversion occurs. `data` keeps category/value rows and `series` keeps names.
Source body keys `unit: "literal"` and `showDataLabels: false` round-trip through
the ordinary parser and formatter. Capability/version are inferred only when
these fields occur; documents without them retain their previous contract.
The optional frontmatter declaration `ast_capabilities: [chart-content-v1]` is
accepted, but does not invent fields or raise the emitted schema without usage.

`transform.NormalizeRectangularCharts(doc)` is a deliberately opt-in API. It
returns a cloned effective AST and an external versioned receipt. It never
mutates the original AST. Each candidate is checked completely before changing
that chart. Rejected charts retain their opaque JSON for the consumer's normal
unsupported-chart gate. The receipt's `accepted` is false if any candidate was
rejected; conversion outcomes refer to the in-memory normalization only.

The recognized v1 profile is **bar, one string series, and 2–24 rows**, each
exactly `[string category, finite JSON number]`. Only `data`, `series`, `unit`
and `showDataLabels` keys are accepted. Optional unit must be a string, optional
label intent must be a boolean; null, duplicate keys, unknown keys, and conflicting
canonical fields are rejected. An empty category/series/unit remains a literal
string. Categories and series containing quotes, backslashes or control characters
are rejected because the historical native chart grammar cannot guarantee their
roundtrip. Unit is a new field with JSON-string encoding and preserves all literal
text independently. Arbitrary Chart.js configuration is not interpreted.

Numbers must retain their decimal value through the consumers' float64-to-JSON
conversion. Thus `0.1`, `120`, and `1e2` are accepted, while `1e-400` and
`9007199254740993` are rejected. Numeric literals are limited to 128 bytes and
an exponent in −400…400 before rational comparison, so giant underflow literals
cannot cause giant rational allocations. This gate does not change the numeric
behavior of historical native chart parsers or consumer decoders.

After conversion, the effective chart has no `rawJSON` or `isJSONMode`; the
receipt retains the exact parsed raw JSON, its SHA-256, node ID, original source
position, and source-to-target JSON pointers for every category, value, series
name, and optional field. `beforeSHA256`/`afterSHA256` hash the in-memory semantic
ASTs, including their source positions; they are not hashes of a later build
whose source positions and frontmatter mode differ.

The companion CLI phase uses the same API and formatter:

```sh
slidelang fmt original.slidelang --normalize-rectangular-charts \
  --chart-normalization-receipt receipt.json --output effective.slidelang
slidelang build effective.slidelang --format json --output ast-output
```

DocLang uses the same flags and explicitly produces its strict dialect. The
normalization CLI requires distinct, fresh source and receipt paths, forbids
`--write`/`--check`, and preserves the original. It publishes the effective
source and sidecar together. On rejection it exits nonzero, writes a receipt
with `accepted: false` and `outputProduced: false`, and publishes no source.
Ordinary `fmt` and `build` perform no rectangular JSON conversion.

The receipt version is `rectangular-chart-normalization-v1`. The publication
receipt also carries `originalSource` (the complete original input, before
preprocessing), `sourceSHA256`, `effectiveSourceSHA256`, and `outputProduced`.
Build, Compose, Pack and reopen must all use the effective portable source;
the original input and its sidecar remain separate audit artifacts.

Core/schema changes ship before the CLI phase: publish the reviewed core-only
version, then update CLI pins and sum entries from that actual publication.
Workspace candidate tests do not certify a binary's published-module provenance.
