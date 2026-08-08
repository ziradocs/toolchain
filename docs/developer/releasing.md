# Releasing

This repo has two release scripts, for two separate concerns. Both live in `scripts/` at the
repo root.

## `scripts/release.sh` — the coordinated binary release

```bash
scripts/release.sh vX.Y.Z
```

Cuts a coordinated release of the three CLIs' binaries: creates the bare `vX.Y.Z` tag plus the
three module tags (`core/vX.Y.Z`, `doclang/vX.Y.Z`, `slidelang/vX.Y.Z`) on the current commit,
pushes the `vX.Y.Z` tag first (in its own push — see the comment in the script for why: empirically,
pushing all four tags at once has failed to trigger `on: push: tags:` reliably), then pushes the
three module tags together. `vX.Y.Z` matches `.github/workflows/release.yml`'s trigger, which runs
`goreleaser` and publishes the actual binaries/packages.

Use this when cutting an actual product release — a version users install.

Guards: requires SemVer starting with `v`, a clean working tree, the tagged major version to
match the `/vN` suffix in `core/go.mod`, and no unpushed commits touching
`.github/workflows/` (pushing a workflow change and a release tag together with local
credentials silently blocks the trigger — this bit the project at `v2.0.4`). It then verifies
`GOWORK=off go build ./...` in both CLIs *before* creating any tag: goreleaser runs in CI without
a `go.work`, so it resolves the `core` version each CLI pins in its `require` line, and a local
`go.work` hides a stale pin completely. Without that check, a stale pin surfaces as a
half-finished release — with all four tags already pushed, and therefore burned. If `gh` is
installed, it also verifies the workflow actually started and force-triggers it if the tag push
didn't.

## `scripts/bump-core.sh` — the core→CLIs dance (issue #25)

```bash
scripts/bump-core.sh vX.Y.Z [ref]
```

`slidelang` and `doclang` depend on a **published** `go.ziradocs.com/core/v2` — there's no
`replace` directive (see `CLAUDE.md`'s "Module layout & the go.mod gotcha"). A gitignored root
`go.work` masks this for local development (every build uses the working-tree `core`
automatically), but `GOWORK=off` builds — CI, and any external `go install` — resolve whatever
version `slidelang/go.mod`/`doclang/go.mod` pins in their `require` line. So whenever a `core`
change needs to reach the CLIs as a real dependency (not just for local `go.work` builds), someone
has to:

1. Make sure the `core` change is merged to `main` (this script does **not** merge anything).
2. Cut a `core/vX.Y.Z` tag from that commit.
3. Wait for `proxy.golang.org` and `sum.golang.org` to index the new tag — a fetch immediately
   after pushing a brand-new tag can hit a transient `500` from the sumdb before it catches up.
4. Bump `require go.ziradocs.com/core/v2` to that version in **both** `slidelang/go.mod` and
   `doclang/go.mod`, and run `go mod tidy` in each.
5. Verify both CLIs actually build against the *published* version (`GOWORK=off go build ./...`
   — not the `go.work`-masked version, which always looks fine regardless).

`scripts/bump-core.sh` does all five steps. It:

- Validates SemVer strictly (`vX.Y.Z` exactly — no prerelease/build suffix) and the `/vN` suffix
  against `core/go.mod`, same as `release.sh` — but reads that `go.mod` out of `$REF`
  (`git show "$TAG_COMMIT":core/go.mod`), not out of the working tree. Reading the local tree
  validates the wrong file the moment `HEAD != $REF`: standing on a branch that has already
  migrated to `/v3`, the check would pass and the script would tag `core/v3.0.0` onto an
  `origin/main` still declaring `/v2` — precisely the permanently-broken tag the guard exists
  to prevent (see the `v2.1.0` note in `CLAUDE.md`).
- Rejects if `core/vX.Y.Z` already exists **on the remote** (`git ls-remote`, not just a local
  `git tag -l`) — this is the check that would have caught the collision that forced
  `core/v2.1.3` to exist (an intermediate tag cut between two coordinated releases). The same
  goes for the bump branch, on `origin` as well as locally.
- Both remote guards **fail closed**. `git ls-remote` exits non-zero on network/credential
  failures, and the naive spellings (`git ls-remote … | grep -q .`, or `--exit-code` inside an
  `if`) collapse that into "no match" — so a transient `128` reads as "the ref is free" and the
  script pushes a tag it can never take back. The helper distinguishes "doesn't exist" from
  "couldn't ask" and aborts on the latter. Keep it that way.
- Tags **only** `core/vX.Y.Z` — never a bare `vX.Y.Z`. `release.yml` triggers on both
  `v[0-9]*.[0-9]*.[0-9]*` and a bare `v*`; a module-only tag like `core/vX.Y.Z` doesn't start
  with `v` at all, so it can't accidentally trigger a goreleaser run.
- Polls `proxy.golang.org`/`sum.golang.org` for the new version before touching `go.mod`, with a
  `GOPROXY=direct GOSUMDB=off` fallback if the sumdb still hasn't caught up.
- Cuts the bump branch **from the ref it just tagged** (`git checkout -b … "$TAG_COMMIT"`),
  before running the bump — see "Safe to run from any branch" below.
- Bumps and `go mod tidy`s both CLIs, then verifies `GOWORK=off go build ./...` in each.
- If `gh` is installed, commits the `go.mod`/`go.sum` bump on that branch and opens the PR.
  Otherwise, leaves the bump uncommitted on the branch with instructions to commit/PR it by hand.

Use this whenever a `core`-only PR needs to become a real dependency for the CLIs, without
cutting a full product release of the binaries.

### Safe to run from any branch

**The script must stay safe to run from whatever branch you happen to be on — that's the whole
point of it taking a `ref` argument.** Everything it produces (the bump branch, the `go mod tidy`
result, the `GOWORK=off go build` check) is computed on top of `$REF`, never on top of your HEAD.
It creates the branch with `git checkout -b "$BRANCH" "$TAG_COMMIT"`, before the bump runs and
before the tag is pushed, and if HEAD has commits `$REF` doesn't, it names them and says they will
not be included.

Any future edit that reintroduces a bare `git checkout -b`, or moves branch creation after the
bump, breaks this. Two distinct failures, both silent:

- **Unrelated commits ride along.** This happened on 2026-08-06: the operator's HEAD was on a
  feature branch — a prior `git checkout main` had failed silently because `main` was checked out
  in another git worktree — so the bump branch inherited two feature commits. PR #98 was titled
  and reviewed as a `go.mod` bump but merged an entire `doclang` feature into `main` alongside it.
  Nothing unreviewed shipped (the content had been reviewed in #97, and #98's CI passed in full),
  but the history now records a feature under a `chore:` commit and #97 had to be closed as
  redundant. Note that `git checkout -b NEW main` is immune to the worktree failure that started
  this: it branches *from* `main` without checking `main` out.
- **The wrong dependency set gets committed.** `go mod tidy` derives the `require` set from the
  `.go` files in the working tree. Run it on the wrong tree and an import that branch adds
  produces a spurious `require`, while one it removes drops a `require` that `$REF` still needs —
  and step 8's `GOWORK=off go build ./...`, running on that same wrong tree, catches neither.
  This is why the branch is cut *before* the bump, not at commit time.

### Why two scripts, not one, and not zero (collapsing to a single module)

The alternative to automating the dance was collapsing `core`/`slidelang`/`doclang` into one Go
module, which would eliminate this whole class of problem. Rejected on purpose: this repo wants
to preserve the option of splitting `core` out into its own independently-published,
independently-versioned library later (see the Go API stability policy in `core/doc.go`).
Collapsing modules now would go in the opposite direction. Hence: keep the multi-module
architecture, automate the friction it creates instead of removing the architecture.
