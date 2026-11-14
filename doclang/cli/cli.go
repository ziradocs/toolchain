package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
	internalcli "go.ziradocs.com/doclang/v2/internal/cli"
)

// Options holds configuration for the CLI entrypoint, allowing external
// callers to inject custom behavior such as proprietary linting rules.
type Options struct {
	Name              string // Name of the CLI command (default: "doclang")
	Version           string
	CustomRules       []linter.Rule
	RulePacks         []linter.RulePack
	ExternalRulepacks []string
	PolicyResolver    func(flagPath string, fm *ast.FrontMatterNode) (*linter.PolicyConfig, error)
	// PostLint recibe, después de la política de lint, los diagnósticos del
	// parser y del linter: lo mismo que lleva el reporte de --report. Cada
	// diagnóstico está en su archivo de origen: uno de un archivo traído con
	// @include lleva Diagnostic.File (relativo al directorio del documento) y
	// su línea en ese archivo, y van ordenados por archivo y posición, con el
	// documento primero. doc, en cambio, conserva las posiciones del
	// contenido expandido: una línea del AST no se compara directamente con
	// la de un diagnóstico de un archivo incluido.
	PostLint func(doc *ast.AST, active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic) error
}

// NewRootCommand builds the root CLI command with the given options.
func NewRootCommand(opts Options) *cobra.Command {
	version := opts.Version
	if version == "" {
		version = "dev"
	}

	name := opts.Name
	if name == "" {
		name = "doclang"
	}

	rootCmd := &cobra.Command{
		Use:     name,
		Short:   "DocLang CLI - Generate documents from .doclang files",
		Long:    `DocLang CLI generates documents (HTML, PDF, DOCX) from .doclang DSL files.`,
		Version: version,
	}

	rootCmd.AddCommand(internalcli.NewBuildCommand(opts.CustomRules, opts.RulePacks, opts.ExternalRulepacks, opts.PolicyResolver, opts.PostLint))
	rootCmd.AddCommand(internalcli.NewFmtCommand())
	rootCmd.AddCommand(internalcli.NewInitCommand())
	rootCmd.AddCommand(internalcli.NewMCPCommand())
	rootCmd.AddCommand(internalcli.NewLicensesCommand())

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
