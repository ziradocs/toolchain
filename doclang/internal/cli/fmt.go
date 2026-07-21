// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/formatter"
	"go.ziradocs.com/core/parser"
	"go.ziradocs.com/core/util"
	"github.com/spf13/cobra"
)

type fmtOptions struct {
	inputFile string
	write     bool
	check     bool
}

// NewFmtCommand crea el comando 'fmt' de doclang.
func NewFmtCommand() *cobra.Command {
	opts := &fmtOptions{}
	cmd := &cobra.Command{
		Use:   "fmt [file]",
		Short: "Format a .doclang file to its canonical source form",
		Long: `Parse a .doclang file and re-emit it in canonical form: "# título"
por sección, listas/código/imágenes/citas/checklists en su sintaxis
Markdown estándar, y los bloques especiales (:::, <<mermaid>>, <<chart:...>>,
<<map>>, @directivas) tal cual.

DocLang no tiene un modo strict separado — siempre usa el mismo dialecto
flex — así que fmt no necesita una bandera de dialecto. La salida es
determinista e idempotente: formatear el mismo documento dos veces produce
texto byte-idéntico.

Examples:
  # Print the canonical form to stdout
  doclang fmt document.doclang

  # Rewrite the file in place
  doclang fmt document.doclang --write

  # CI check: fail if the file isn't already in canonical form
  doclang fmt document.doclang --check`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.inputFile = args[0]
			return runFmt(opts)
		},
	}
	cmd.Flags().BoolVarP(&opts.write, "write", "w", false, "Write result to the input file instead of stdout")
	cmd.Flags().BoolVar(&opts.check, "check", false, "Exit with status 1 if the file is not already in canonical form; don't write output")
	return cmd
}

func runFmt(opts *fmtOptions) error {
	content, err := os.ReadFile(opts.inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if err := util.RunGuarded(util.DefaultParseTimeout, func() error {
		docParser := parser.NewDocumentFlexParserWithNormalization(string(content), util.NewNoop())
		doc, diags = docParser.Parse()
		return nil
	}); err != nil {
		return err
	}
	for _, d := range diags {
		if d.IsError() {
			return fmt.Errorf("fmt: el archivo tiene errores de parseo, corrígelos antes de formatear:\n%s", d.String())
		}
	}

	out, err := formatter.FormatDocument(doc)
	if err != nil {
		return fmt.Errorf("fmt: %w", err)
	}

	if opts.check {
		if out != string(content) {
			fmt.Fprintf(os.Stderr, "%s no está en forma canónica (correr con --write para reformatear)\n", opts.inputFile)
			os.Exit(1)
		}
		return nil
	}

	if opts.write {
		if out == string(content) {
			return nil
		}
		return os.WriteFile(opts.inputFile, []byte(out), 0644)
	}

	fmt.Print(out)
	return nil
}
