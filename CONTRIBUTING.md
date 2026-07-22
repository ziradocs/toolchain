# Contributing to SlideLang / DocLang

Thanks for your interest in contributing. This document covers what you need to know to send a
useful pull request: the legal requirement (DCO), how the repo is laid out and built, and the
conventions we follow.

## Developer Certificate of Origin (DCO)

Every commit must be signed off to certify you wrote it (or otherwise have the right to submit
it) under the project's Apache-2.0 license, per the [Developer Certificate of Origin](https://developercertificate.org/).

Sign off by adding `-s` to your commit:

```bash
git commit -s -m "fix: handle empty frontmatter in flex parser"
```

This appends a `Signed-off-by: Your Name <your.email@example.com>` trailer to the commit message,
using the name and email from your git config (`git config user.name` / `user.email`). Pull
requests with unsigned commits will be blocked until they're signed off — if you forgot, you can
amend and force-push:

```bash
git commit --amend -s
git push --force-with-lease
```

For a PR with multiple unsigned commits, `git rebase --exec 'git commit --amend --no-edit -s' -i main`
(or an interactive rebase adding `-s` at each step) fixes them all at once.

There is no CLA at this stage — DCO is the only requirement. This may be re-evaluated if the
project takes on substantial external contributions that need it.

## Repository layout

This is a Go monorepo for two sibling DSLs that share one core library:

- **`.slidelang`** files → presentations (HTML/PDF), built by `slidelang`
- **`.doclang`** files → documents (HTML/PDF/DOCX/Markdown), built by `doclang`
- Both parse and render through **`core`** (parser, AST, elements, renderer, linter, AI
  normalizer)

```
core/   # shared library: parser, ast, elements, renderer, linter, ai/
slidelang/     # presentations CLI (slidelang binary)
doclang/       # documents CLI (doclang binary)
docs/              # user & developer documentation
examples/          # sample .slidelang / .doclang files
```

## Development setup

There are **three independent Go modules** — you cannot build or test
the whole repo from the root. Each CLI module has a `replace` directive pointing at the local
core, and a gitignored root `go.work` also lists all three for local multi-module editing:

| Module | Path | Go version |
|---|---|---|
| `core` | `go.ziradocs.com/core/v2` | 1.26.5 |
| `slidelang` | `go.ziradocs.com/slidelang/v2` | 1.26.5 |
| `doclang` | `go.ziradocs.com/doclang/v2` | 1.26.5 |

`slidelang/go.mod` and `doclang/go.mod` both contain:

```
replace go.ziradocs.com/core/v2 => ../core
```

so when you edit `core` and build/test either CLI from a checkout that has both
directories side by side (which is how this monorepo is laid out), your local core changes are
picked up automatically — no extra step, no `go mod` command needed.

**`go install` over the vanity import now works — confirmed end-to-end against a clean module
cache.** The website (`ziradocs/website`, separate repo) serves a 4-field `go-import` meta tag
per module (`prefix vcs reporoot subdir`, e.g. `go.ziradocs.com/core/v2 git
https://github.com/ziradocs/toolchain core`) — a form supported since Go 1.25 that declares the
module's physical subdirectory explicitly, instead of making `go` derive it by stripping the
major-version suffix (which is ambiguous when the module's subdirectory has the same name
regardless of major version, as here, and was silently resolving to the wrong module or failing
outright). Requires Go ≥1.25 on the installing machine for the initial handshake — not a new
constraint, since this repo already requires 1.26.5.

**A casualty of the same investigation: `core/v2.1.0`, `slidelang/v2.1.0`, and `doclang/v2.1.0`
are permanently broken tags — never use them, and never reuse that version number.** They were
cut 2026-07-21, a day before the module paths were bumped to `/v2` (commit `5820531`,
2026-07-22), so the `go.mod` at those revisions still declares the unversioned path. Since
`v2.1.0 > v2.0.6` in semver, `@latest` resolved to the broken tag by default — and so did an
*explicit* `@v2.0.6` request, because `go` enumerates all matching tags during resolution and
aborts on the first invalid one it finds. The fix was a new `v2.1.1` release cut from current
`HEAD`, not touching (or reusing) the already-published `v2.1.0` tags.

**The `replace` directives stay committed anyway — this is now a deliberate release-model
decision, not a blocker.** Removing them means CI/goreleaser (neither sets up a `go.work`) would
fetch `core` over the network on every build instead of using the local checkout — i.e. `core`
would need to be tagged and published *before* slidelang/doclang can build against it. That's a
real workflow change worth making on purpose, not as a side effect of an unrelated fix.

**Always `cd` into the specific module before running Go tooling.** `go build ./...` at the repo
root will fail — there's no module there.

```bash
# core (library, no binary)
cd core
go build ./...
go test ./...

# slidelang
cd slidelang
go build -o slidelang ./cmd/slidelang
./slidelang build ../examples/02_diagrams_and_charts/02_diagrams_and_charts_flex.slidelang
./slidelang build slides.slidelang --theme modern-blue --format html
./slidelang build slides.slidelang --lint-only   # parse+lint only, no output

# doclang
cd doclang
go build -o doclang ./cmd/doclang
./doclang build ../examples/advanced_elements_test.doclang --output output
./doclang build doc.doclang --format docx --toc --numbering
```

Requirements: Go 1.26.5+. For PDF output and offline diagram/chart/map rendering you also need
Chrome/Chromium available, or run with `--install-chromium` to have the CLI fetch a pinned build.

### Running tests

Run tests from inside each module directory — there's no single top-level `go test ./...`:

```bash
cd core && go test ./...          # all tests in the module
go test -run TestName ./parser/             # a single test / package
go test -v -cover ./...                     # verbose + coverage
go vet ./...                                # static checks
gofmt -l .                                  # formatting check (gofmt -w . to fix)
```

Most tests live in `core` (parser, elements, renderer, AI normalizer). If you touch
`core`, also build and run the test suites of both CLIs — the `replace` directive means
your changes affect them immediately, and a change that looks safe in isolation can break a
downstream consumer.

Before opening a PR, at minimum:

```bash
cd core && go build ./... && go vet ./... && go test ./...
cd slidelang   && go build ./... && go vet ./... && go test ./...
cd doclang     && go build ./... && go vet ./... && go test ./...
```

CI runs the same matrix across all three modules plus `golangci-lint` and `govulncheck` — a PR
that fails any of these won't be merged.

## Where changes go

- Parsing logic → `core/parser/`
- New/changed element types → `core/internal/elements/` (register in `GetDefaultRegistry()`, add
  an `ast.NodeType`, add renderer support)
- HTML/Markdown/document rendering → `core/renderer/`
- Normalization rules → `core/internal/normalize/normalizer/rules/`
- Presentation-only features → `slidelang/internal/`
- Document-only features (DOCX/PDF/markdown output) → `doclang/internal/generator/`

Keep CLI-specific code out of `core` — it's the shared library both CLIs depend on.

## Conventions

- **Code comments and log messages are in Spanish**; match the surrounding language when editing
  existing files — most of `docs/` today is still in Spanish, and that's expected during this
  transition. **New public-facing documentation (README, new `docs/` guides, root-level files
  like this one, godoc for exported API) should be written in English going forward** — this is a
  known in-progress migration (see the docs-translation tracking issue), not yet the state of the
  whole tree. When editing an existing Spanish doc, keep it in Spanish unless you're doing a
  dedicated translation pass; when adding a brand-new doc file, prefer English unless it sits
  alongside closely-related Spanish files you're not translating as part of the same change.
- Format Go code with `gofmt` before committing; run `go vet ./...` in each module you touched.
- User-rendered HTML output must go through `core/renderer/sanitizer.go` — it escapes
  user content and blocks dangerous URL protocols (`javascript:`, `data:`, `vbscript:`, `file:`).
  Don't bypass it for new rendered output.
- Commit messages: short, descriptive, imperative mood (`fix: handle empty frontmatter`,
  `feat: add PDF export for slidelang`). Reference the issue you're closing where relevant
  (`Closes #123`).

## Making a change

1. Fork the repo and create a branch off `main`.
2. Make your change, following the conventions above.
3. Add or update tests in the module(s) you touched.
4. Run the build/vet/test sequence for every module you touched (see above).
5. Commit with `git commit -s` (DCO sign-off — see above).
6. Open a pull request describing the change and, if applicable, the issue it addresses.

For parser changes, a full raw-AST diff of the example corpus against `main`
(`--format json` output for everything under `examples/`, diffed) is expected for anything beyond
a trivial fix — parser behavior is easy to regress silently.

For renderer/template changes, a byte-diff of the generated HTML corpus against `main` is the
standard way to confirm you haven't changed unrelated output.

## Reporting bugs / requesting features

Open a GitHub issue. For bugs, include: SlideLang/DocLang version, OS, Go version (for build
issues), a minimal `.slidelang`/`.doclang` reproduction, and expected vs. actual output. For
features, include the use case and, if you have one, the syntax/output you'd expect.

Security vulnerabilities should **not** be reported as public issues — see
[SECURITY.md](SECURITY.md).

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). By participating, you're
expected to uphold it.
