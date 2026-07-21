// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"go.ziradocs.com/doclang/internal/cli"
	"github.com/spf13/cobra"
)

// version is stamped at build time via -ldflags "-X main.version=vX.Y.Z"
// (goreleaser); "dev" is the fallback for a plain local `go build`.
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "doclang",
	Short: "DocLang CLI - Create documents from DSL files",
	Long: `DocLang CLI allows you to create professional documents
from simple DSL files using either Strict or Flex syntax modes.

Supported output formats:
  - HTML: Single-page or multi-page documents
  - PDF: Professional PDF documents
  - DOCX: Microsoft Word compatible documents
  - Markdown: Clean Markdown output

Examples:
  doclang build document.doclang
  doclang build doc.doclang --format pdf --output ./dist
  doclang build spec.doclang --format docx
  doclang build report.doclang --theme technical
  doclang init my-document`,
	Version: version,
}

func main() {
	// Add subcommands
	rootCmd.AddCommand(cli.NewBuildCommand())
	rootCmd.AddCommand(cli.NewInitCommand())
	rootCmd.AddCommand(cli.NewFmtCommand())
	rootCmd.AddCommand(cli.NewMCPCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
