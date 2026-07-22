package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.ziradocs.com/core/linter"
	internalcli "go.ziradocs.com/slidelang/internal/cli"
)

// Options holds configuration for the CLI entrypoint, allowing external
// callers to inject custom behavior such as proprietary linting rules.
type Options struct {
	Version     string
	CustomRules []linter.Rule
	RulePacks   []linter.RulePack
}

// NewRootCommand builds the root CLI command with the given options.
func NewRootCommand(opts Options) *cobra.Command {
	version := opts.Version
	if version == "" {
		version = "dev"
	}

	rootCmd := &cobra.Command{
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

	rootCmd.AddCommand(internalcli.NewBuildCommand(opts.CustomRules, opts.RulePacks))
	rootCmd.AddCommand(internalcli.NewThemesCommand())
	rootCmd.AddCommand(internalcli.NewMCPCommand())
	rootCmd.AddCommand(internalcli.NewFmtCommand())

	return rootCmd
}

// Execute is the main entrypoint for the CLI.
func Execute(opts Options) {
	cmd := NewRootCommand(opts)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
