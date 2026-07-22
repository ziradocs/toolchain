package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/linter"
	internalcli "go.ziradocs.com/slidelang/internal/cli"
)

// Options holds configuration for the CLI entrypoint, allowing external
// callers to inject custom behavior such as proprietary linting rules.
type Options struct {
	Name              string // Name of the CLI command (default: "slidelang")
	Version           string
	CustomRules       []linter.Rule
	RulePacks         []linter.RulePack
	ExternalRulepacks []string
	PolicyResolver    func(flagPath string, fm *ast.FrontMatterNode) (*linter.PolicyConfig, error)
	PostLint          func(doc *ast.AST, active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic) error
}

// NewRootCommand builds the root CLI command with the given options.
func NewRootCommand(opts Options) *cobra.Command {
	version := opts.Version
	if version == "" {
		version = "dev"
	}

	name := opts.Name
	if name == "" {
		name = "slidelang"
	}

	rootCmd := &cobra.Command{
		Use:   name,
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
  slidelang fmt presentation.slidelang --write`,
		Version: version,
	}

	rootCmd.AddCommand(internalcli.NewBuildCommand(opts.CustomRules, opts.RulePacks, opts.ExternalRulepacks, opts.PolicyResolver, opts.PostLint))
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
