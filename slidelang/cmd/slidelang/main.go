// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"runtime/debug"

	"go.ziradocs.com/slidelang/v2/cli"
)

// version is stamped at build time via -ldflags "-X main.version=vX.Y.Z"
// (goreleaser); "dev" is the fallback for a plain local `go build`.
var version = "dev"

// resolveVersion recovers the module version for binaries that goreleaser did
// not stamp — which is every `go install go.ziradocs.com/slidelang/v2/cmd/slidelang@latest`.
// Those are real releases: `go version -m` reads v2.32.4 out of them, but
// `--version` printed "dev", because ldflags are a goreleaser step and
// `go install` has no equivalent.
//
// The docs tell people to install with `@latest`, so "dev" was the answer most
// users got, and it is the answer that makes a bug report unattributable.
//
// A local `go build` still reports "dev": ReadBuildInfo returns "(devel)"
// there, which carries no more information than the fallback and reads worse.
//
// The decision lives in pickVersion, a pure function, because that is the only
// shape a test can actually pin: a test that calls resolveVersion() from inside
// `go test` reads the *test* binary's build info, so it exercises whichever
// branch the harness happens to produce and stays green no matter what the
// fallback does. The first version of this fix had exactly that hole — muting
// the fallback left every test passing.
func resolveVersion() string {
	built := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		built = info.Main.Version
	}
	return pickVersion(version, built)
}

// pickVersion resolves the version to print from the ldflags-stamped value and
// the one embedded in the module's build info.
func pickVersion(stamped, fromBuildInfo string) string {
	if stamped != "dev" {
		return stamped
	}
	if fromBuildInfo == "" || fromBuildInfo == "(devel)" {
		return stamped
	}
	return fromBuildInfo
}

func main() {
	cli.Execute(cli.Options{Version: resolveVersion()})
}
