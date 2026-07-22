// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go.ziradocs.com/doclang/v2/cli"
)

// version is stamped at build time via -ldflags "-X main.version=vX.Y.Z"
// (goreleaser); "dev" is the fallback for a plain local `go build`.
var version = "dev"

func main() {
	cli.Execute(cli.Options{Version: version})
}
