package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.ziradocs.com/core/linter"
	internalcli "go.ziradocs.com/doclang/internal/cli"
)

// Options holds configuration for the CLI entrypoint, allowing external
// callers to inject custom behavior such as proprietary linting rules.
type Options struct {
	Version           string
	CustomRules       []linter.Rule
	RulePacks         []linter.RulePack
	ExternalRulepacks []string
}

// NewRootCommand builds the root CLI command with the given options.
func NewRootCommand(opts Options) *cobra.Command {
	version := opts.Version
	if version == "" {
		version = "dev"
	}

	rootCmd := &cobra.Command{
		Use:     "doclang",
		Short:   "DocLang CLI - Generate documents from .doclang files",
		Long:    `DocLang CLI generates documents (HTML, PDF, DOCX) from .doclang DSL files.`,
		Version: version,
	}

	rootCmd.AddCommand(internalcli.NewBuildCommand(opts.CustomRules, opts.RulePacks, opts.ExternalRulepacks))
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
