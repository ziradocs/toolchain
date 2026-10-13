// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/formatter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/transform"
	"go.ziradocs.com/core/v2/util"
)

type fmtOptions struct {
	inputFile string
	write     bool
	check     bool
	strict    bool
	// strictSet distingue "no pasó --strict" de "pasó --strict=false", que
	// significan cosas distintas: preservar el dialecto del documento vs.
	// pedir explícitamente flex.
	strictSet                  bool
	normalizeRectangularCharts bool
	chartNormalizationReceipt  string
	output                     string
}

// NewFmtCommand crea el comando 'fmt' de doclang.
func NewFmtCommand() *cobra.Command {
	opts := &fmtOptions{}
	cmd := &cobra.Command{
		Use:   "fmt [file]",
		Short: "Format a .doclang file to its canonical source form",
		Long: `Parse a .doclang file and re-emit it in canonical form.

Por defecto se PRESERVA el dialecto del documento: uno flex se reescribe en
flex ("# título" por sección, listas/código/imágenes/citas/checklists en
sintaxis Markdown estándar) y uno que declara 'mode: strict' se reescribe en
strict (bloques SECTION declarados). En ambos casos los bloques especiales
(:::, <<mermaid>>, <<chart:...>>, <<map>>, @directivas) van tal cual, y la
salida es determinista e idempotente.

--strict fuerza el dialecto strict. Sobre un documento flex eso NO es un
formateo sino una TRANSPILACIÓN: reescribe el documento a otro dialecto, y
es la forma de promover un borrador a artefacto auditable. A diferencia de
'slidelang fmt', acá no es el default — DocLang tiene dos dialectos
legítimos, y transpilar sin pedirlo reescribiría cada documento flex
existente.

Explicit rectangular chart normalization preserves originals and writes new source:
  doclang fmt original.doclang --normalize-rectangular-charts --chart-normalization-receipt receipt.json --output effective.doclang
Only the bounded one-series bar profile is converted; unsupported JSON fails closed.

Examples:
  # Print the canonical form to stdout, in the document's own dialect
  doclang fmt document.doclang

  # Rewrite the file in place
  doclang fmt document.doclang --write

  # Promote a flex draft to the strict dialect (transpiles)
  doclang fmt draft.doclang --strict --write

  # CI check: fail if the file isn't already in canonical form
  doclang fmt document.doclang --check`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.inputFile = args[0]
			opts.strictSet = cmd.Flags().Changed("strict")
			return runFmt(opts)
		},
	}
	cmd.Flags().BoolVar(&opts.strict, "strict", false, "Emit the strict dialect; on a flex document this transpiles it (default: keep the document's own dialect)")
	cmd.Flags().BoolVarP(&opts.write, "write", "w", false, "Write result to the input file instead of stdout")
	cmd.Flags().BoolVar(&opts.check, "check", false, "Exit with status 1 if the file is not already in canonical form; don't write output")
	cmd.Flags().BoolVar(&opts.normalizeRectangularCharts, "normalize-rectangular-charts", false, "Explicitly convert the supported rectangular bar JSON profile to portable source")
	cmd.Flags().StringVar(&opts.chartNormalizationReceipt, "chart-normalization-receipt", "", "External normalization audit receipt (required with --normalize-rectangular-charts)")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Write portable source to a new file (required with --normalize-rectangular-charts)")
	return cmd
}

func runFmt(opts *fmtOptions) error {
	if opts.output != "" && (opts.write || opts.check) {
		return fmt.Errorf("--output cannot be combined with --write or --check")
	}
	if opts.normalizeRectangularCharts {
		if opts.strictSet && !opts.strict {
			return fmt.Errorf("normalization requires the strict target dialect")
		}
		if opts.output == "" || opts.chartNormalizationReceipt == "" || opts.write || opts.check {
			return fmt.Errorf("normalization requires --output and --chart-normalization-receipt, without --write/--check")
		}
	} else if opts.chartNormalizationReceipt != "" {
		return fmt.Errorf("--chart-normalization-receipt requires --normalize-rectangular-charts")
	}
	content, err := os.ReadFile(opts.inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if err := util.RunGuarded(util.DefaultParseTimeout, func() error {
		doc, diags = parser.New(util.NewNoop()).ParseDocument(string(content), opts.inputFile)
		return nil
	}); err != nil {
		return err
	}
	for _, d := range diags {
		if d.IsError() || (opts.normalizeRectangularCharts && d.RuleID == "CHART002") {
			return fmt.Errorf("fmt: the file has parse errors, fix them before formatting:\n%s", d.String())
		}
	}

	var receipt transform.ChartNormalizationReceipt
	if opts.normalizeRectangularCharts {
		doc, receipt, err = transform.NormalizeRectangularCharts(doc)
		if err != nil {
			return err
		}
		if !receipt.Accepted {
			if err = transform.PublishNormalizedChartSource(opts.output, opts.chartNormalizationReceipt, content, nil, receipt); err != nil {
				return err
			}
			return fmt.Errorf("rectangular chart normalization rejected; no source output produced")
		}
	}

	sourceIsStrict := doc != nil && doc.FrontMatter != nil && doc.FrontMatter.Mode == "strict"

	// Sin --strict se preserva el dialecto del documento. Con --strict=false
	// explícito sobre un documento strict se estaría pidiendo degradarlo a
	// flex, que descarta la declaración del autor sin forma de recuperarla.
	targetStrict := sourceIsStrict || opts.normalizeRectangularCharts
	if opts.strictSet {
		if !opts.strict && sourceIsStrict {
			return fmt.Errorf("fmt: --strict=false would downgrade %q from strict to flex, discarding the dialect it declares; "+
				"if that is what you want, change `mode:` in the front matter by hand", opts.inputFile)
		}
		targetStrict = opts.strict
	}

	var out string
	if targetStrict {
		out, err = formatter.FormatDocumentStrict(doc)
	} else {
		out, err = formatter.FormatDocument(doc)
	}
	if err != nil {
		return fmt.Errorf("fmt: %w", err)
	}

	// Transpilar (flex → strict) reescribe el documento a otro dialecto, no
	// lo reformatea. Se avisa antes de tocar el archivo.
	isTranspile := targetStrict && !sourceIsStrict

	if opts.normalizeRectangularCharts {
		return transform.PublishNormalizedChartSource(opts.output, opts.chartNormalizationReceipt, content, []byte(out), receipt)
	}
	if opts.output != "" {
		return os.WriteFile(opts.output, []byte(out), 0644)
	}

	if opts.check {
		if out != string(content) {
			fmt.Fprint(os.Stderr, checkFailureMessage(opts.inputFile, isTranspile))
			os.Exit(1)
		}
		return nil
	}

	if opts.write {
		if out == string(content) {
			return nil
		}
		if isTranspile {
			fmt.Fprint(os.Stderr, transpileWriteNotice(opts.inputFile))
		}
		return os.WriteFile(opts.inputFile, []byte(out), 0644)
	}

	fmt.Print(out)
	return nil
}

// transpileWriteNotice y checkFailureMessage son puras (sin I/O) para poder
// testear su contenido sin invocar os.Exit — mismo motivo por el que
// slidelang/internal/cli/fmt.go las extrajo (code review de PR #217): con el
// os.Exit(1) inline dentro de la rama --check, esa rama era intesteable en
// proceso, y el mensaje genérico de "drift de formato" no distinguía un
// documento simplemente desactualizado de uno que --write TRANSPILARÍA.

func transpileWriteNotice(inputFile string) string {
	return fmt.Sprintf("fmt: transpiling %q to strict mode: the file will be rewritten in SECTION syntax (mode: strict)\n", inputFile)
}

func checkFailureMessage(inputFile string, isTranspile bool) string {
	if isTranspile {
		return fmt.Sprintf("%s is not in canonical strict form: it is a flex document, and --write would transpile it "+
			"(a dialect rewrite, not just a reformat)\n", inputFile)
	}
	return fmt.Sprintf("%s is not in canonical form (run with --write to reformat)\n", inputFile)
}
