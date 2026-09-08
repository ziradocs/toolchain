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
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return version
	}
	return info.Main.Version
}

func main() {
	cli.Execute(cli.Options{Version: resolveVersion()})
}
