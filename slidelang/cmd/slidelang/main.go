// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"go.ziradocs.com/slidelang/internal/cli"
	"github.com/spf13/cobra"
)

// version is stamped at build time via -ldflags "-X main.version=vX.Y.Z"
// (goreleaser); "dev" is the fallback for a plain local `go build`.
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "slidelang",
	Short: "SlideLang CLI - Create presentations from DSL files",
	Long: `SlideLang CLI allows you to create beautiful presentations
from simple DSL files using either Strict or Flex syntax modes.

HTML Generation modes:
  By default, HTML output creates separate CSS and JS files for better performance and caching.
  Use --embed-assets to create a single self-contained HTML file.

Examples:
  slidelang build presentation.slidelang
  slidelang build slides.slidelang --format html --output ./build
  slidelang build slides.slidelang --format html --embed-assets
  slidelang build presentation.slidelang --log-level debug
  slidelang build --help`,
	Version: version,
}

func main() {
	// Add subcommands
	rootCmd.AddCommand(cli.NewBuildCommand())
	rootCmd.AddCommand(cli.NewThemesCommand())
	rootCmd.AddCommand(cli.NewMCPCommand())
	rootCmd.AddCommand(cli.NewFmtCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
