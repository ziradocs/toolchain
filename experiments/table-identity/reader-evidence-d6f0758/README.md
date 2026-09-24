# Reader probe evidence

Synthetic only. No proposed AST version has been assigned. `99.0.0` is a
deliberately unsupported sentinel.

- Reader/schema/formatter and pre-decode guard probe: exact checkout
  `d6f0758e3d105399f05131507245f9db8e4efb1e`, run on Ubuntu with
  `GOWORK=/mac-remote-development/evidence/ziradocs-toolchain/table-identity-reader-73d4/go.work`.
- CLI external-filter probe: exact checkout
  `73d4de65829b25deb86718cab044ce89588c3b95`, also on Ubuntu.
  The filter script did not change between these commits. The binary SHA-256
  is in `cli.sha256`.
- Baseline fixture is the synthetic `regular_table_id` AST in
  `../evidence-6b88c45/regular_table_id/build/input.json`.

Commands used (paths abbreviated only through `src` and `out` variables):

```sh
cd "$src/core"
GOWORK="$work/go.work" GOMODCACHE=/mac-remote-development/caches/go-mod \
  GOCACHE=/mac-remote-development/caches/go-build \
  go run ../experiments/table-identity/reader_probe.go \
  "$src/experiments/table-identity/evidence-6b88c45/regular_table_id/build/input.json" \
  "$src/schema/ast.schema.json" "$out/reader"

cd "$src/slidelang"
GOWORK="$work/go.work" GOMODCACHE=/mac-remote-development/caches/go-mod \
  GOCACHE=/mac-remote-development/caches/go-build \
  go build -o "$out/slidelang" ./cmd/slidelang
TABLE_ID_FILTER_OUTPUT="$out/filter-output.json" \
  "$out/slidelang" build \
  "$src/experiments/table-identity/evidence-6b88c45/regular_table_id/input.slidelang" \
  --filter "$src/experiments/table-identity/filter_candidate.py" \
  --format json --output "$out/filter-build" --no-colors
```

`reader/*.json` includes four input payloads, the decoded forms, and observed
results. `reader/*.formatted.slidelang` is the formatter output. `filter-output.json`
is what the filter emitted; `filter-build.json` is what the current CLI emitted
after decoding it. `filter.stderr` records the successful CLI run. The durable
Ubuntu copies are under `/mac-remote-development/evidence/ziradocs-toolchain/`.
