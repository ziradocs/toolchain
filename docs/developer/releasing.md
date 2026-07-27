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
credentials silently blocks the trigger — this bit the project at `v2.0.4`). If `gh` is
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
  against `core/go.mod`, same as `release.sh`.
- Rejects if `core/vX.Y.Z` already exists **on the remote** (`git ls-remote`, not just a local
  `git tag -l`) — this is the check that would have caught the collision that forced
  `core/v2.1.3` to exist (an intermediate tag cut between two coordinated releases).
- Tags **only** `core/vX.Y.Z` — never a bare `vX.Y.Z`. `release.yml` triggers on both
  `v[0-9]*.[0-9]*.[0-9]*` and a bare `v*`; a module-only tag like `core/vX.Y.Z` doesn't start
  with `v` at all, so it can't accidentally trigger a goreleaser run.
- Polls `proxy.golang.org`/`sum.golang.org` for the new version before touching `go.mod`, with a
  `GOPROXY=direct GOSUMDB=off` fallback if the sumdb still hasn't caught up.
- Bumps and `go mod tidy`s both CLIs, then verifies `GOWORK=off go build ./...` in each.
- If `gh` is installed, opens a branch + PR with the `go.mod`/`go.sum` bump. Otherwise, leaves
  the bump uncommitted in the working tree with instructions to commit/PR it by hand.

Use this whenever a `core`-only PR needs to become a real dependency for the CLIs, without
cutting a full product release of the binaries.

### Why two scripts, not one, and not zero (collapsing to a single module)

The alternative to automating the dance was collapsing `core`/`slidelang`/`doclang` into one Go
module, which would eliminate this whole class of problem. Rejected on purpose: this repo wants
to preserve the option of splitting `core` out into its own independently-published,
independently-versioned library later (see the Go API stability policy in `core/doc.go`).
Collapsing modules now would go in the opposite direction. Hence: keep the multi-module
architecture, automate the friction it creates instead of removing the architecture.
