# Provenance of the `docxgo` fork (issue #33)

`doclang` generates DOCX exclusively through
`github.com/mmonterroca/docxgo/v2` (`doclang/internal/generator/docx.go`,
~1780 lines, the project's only DOCX generator). This dependency:

- **Is the maintainer's personal external repository** (`github.com/mmonterroca/docxgo`), not an
  org repo — an explicit decision: the project **consumes** it as an external dependency instead
  of **absorbing** it into the org (see the OSS launch plan decisions, 2026-07-12). It requires no
  transfer or org governance.
- **Is a fork of** [`github.com/fumiama/go-docx`](https://github.com/fumiama/go-docx), rewritten
  with its own v2 (`v2.1.1` in `doclang/go.mod`) that adds significant functionality absent
  upstream: a fluent Builder Pattern in addition to direct DOM access, dynamic TOC, a field system
  (PAGE/NUMPAGES/TOC/STYLEREF/SEQ/REF), floating images (WrapSquare/WrapTight), cell merging in
  tables, multi-section layouts (portrait↔landscape), per-section headers/footers, 9 image formats,
  and ~40 built-in paragraph styles.
- `doclang` uses the **Direct Document API** (not the Builder Pattern) — needed for
  conditional logic and iteration over dynamic AST data, with better access to the underlying DOM
  and per-call error handling instead of deferring to `Build()`.
- The module requires the major-version suffix in the path (`github.com/mmonterroca/docxgo/v2`) —
  the initial `v2.0.0-beta`/`v2.1.0` releases had a `go.mod` with the path missing the `/v2`
  suffix, which blocked importing them via `go get`; fixed in `v2.1.1`, the version
  `doclang/go.mod` consumes today.

## Monitoring (resolution of #33, code side)

There's no automatic version bump: `docxgo` releases are controlled by its maintainer, not an
automated PR. What does run in CI against the version pinned in `doclang/go.mod`:

- `govulncheck.yml` (weekly + on `go.mod`/`go.sum` changes) — scans for known CVEs reachable from
  `doclang`'s actual code, including `docxgo`.
- `.github/dependabot.yml` — `docxgo` is on the `doclang` entry's `ignore` list (bumps to this
  dependency are done by hand, not via an automatic Dependabot PR).

If a real vulnerability is found in the future, or a fork with org governance is needed (2FA,
branch protection, `govulncheck` on the fork repo itself), that's an administrative decision and
action for the maintainer on their personal repo — it doesn't block or depend on this monorepo.
