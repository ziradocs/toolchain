// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package themes embeds the manifest + stylesheet of every named theme
// shipped under this directory (modern-blue, cyberpunk-neon, ...), for
// consumers that can't rely on a real filesystem to find them the way the
// CLI does (internal/generator/css/themes.ThemeLoader searches "./themes"
// and "~/.slidelang/themes" on disk). The WASM build (cmd/wasm) has no such
// filesystem to search against, so it reads these named themes from here
// instead — see cmd/wasm/theme.go.
package themes

import "embed"

//go:embed */theme.json */styles.css
var FS embed.FS

// Names lists the embedded theme directory names — each has a theme.json
// and a styles.css under FS.
var Names = []string{
	"aurora-holographic",
	"cyberpunk-neon",
	"elegant-minimal",
	"modern-blue",
	"neomorphism-glass",
	"startup-tech",
	"startup-tech-solid",
}
