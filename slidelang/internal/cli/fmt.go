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
	"go.ziradocs.com/core/v2/fragment"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/transform"
	"go.ziradocs.com/core/v2/util"
)

type FmtOptions struct {
	ContentFragment            bool
	FragmentNamespace          string
	FragmentReceipt            string
	InputFile                  string
	Strict                     bool
	Write                      bool
	Check                      bool
	NormalizeRectangularCharts bool
	ChartNormalizationReceipt  string
	Output                     string
}

func NewFmtCommand() *cobra.Command {
	opts := &FmtOptions{}
	cmd := &cobra.Command{
		Use:   "fmt [file]",
		Short: "Format a .slidelang file to its canonical strict-mode source",
		Long: `Parse a .slidelang file and re-emit it in canonical strict-mode form
(the SLIDE-marker dialect, see 'mode: strict' in frontmatter).

The output is deterministic and idempotent: formatting the same document
twice produces byte-identical text, and formatting already-canonical text
is a no-op. This makes strict-mode source usable as an auditable, diffable
artifact — e.g. to check that agent-generated decks are well-formed, or to
detect drift in CI.

--strict is the only accepted target dialect (the flag exists to name it
explicitly for when other canonical forms are added later), but the SOURCE
document can be either dialect:
  - A 'mode: strict' document is reformatted to its own canonical form.
  - A 'mode: flex'/'flex-full' (or its deprecated alias flex-ai) document
    is TRANSPILED to strict: parsed (with normalization, same as a regular
    build) and re-emitted as SLIDE-marker text, with 'mode: strict' in the
    output frontmatter. The output is meant to build to the same AST as the
    source; when the document uses a construct strict mode cannot represent,
    fmt fails with an error naming the element instead of writing text that
    builds differently.

Explicit rectangular chart normalization preserves originals and writes new source:
  slidelang fmt original.slidelang --normalize-rectangular-charts --chart-normalization-receipt receipt.json --output effective.slidelang
Only the bounded one-series bar profile is converted; unsupported JSON fails closed.

Explicit Markdown content context (no document/slide boundaries):
  slidelang fmt original.md --content-fragment --fragment-namespace document/slot/block --fragment-receipt receipt.json --output fragment.strict
The output contains only strict elements. Declare receipt.sourceCapabilities
when assembling the body, then use ordinary build/compose/pack readers.

Examples:
  # Print the canonical strict form to stdout (works for strict OR flex input)
  slidelang fmt deck.slidelang

  # Rewrite the file in place — for a flex source, this replaces it with
  # transpiled strict text (mode: flex -> mode: strict)
  slidelang fmt deck.slidelang --write

  # CI check: fail if the file isn't already in canonical strict form
  # (a flex file always fails this check — it is not, by definition, in
  # strict form; use the no-flag form to preview its transpiled output)
  slidelang fmt deck.slidelang --check`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.InputFile = args[0]
			return runFmt(opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Strict, "strict", true, "Target dialect (only 'strict' is supported today)")
	cmd.Flags().BoolVarP(&opts.Write, "write", "w", false, "Write result to the input file instead of stdout")
	cmd.Flags().BoolVar(&opts.Check, "check", false, "Exit with status 1 if the file is not already in canonical form; don't write output")
	cmd.Flags().BoolVar(&opts.NormalizeRectangularCharts, "normalize-rectangular-charts", false, "Explicitly convert the supported rectangular bar JSON profile to portable source")
	cmd.Flags().StringVar(&opts.ChartNormalizationReceipt, "chart-normalization-receipt", "", "External normalization audit receipt (required with --normalize-rectangular-charts)")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "Write portable source to a new file (required with --normalize-rectangular-charts)")
	cmd.Flags().BoolVar(&opts.ContentFragment, "content-fragment", false, "Parse an explicit Markdown content fragment without document boundaries")
	cmd.Flags().StringVar(&opts.FragmentNamespace, "fragment-namespace", "", "Stable authoral namespace for generated fragment anchors and identities")
	cmd.Flags().StringVar(&opts.FragmentReceipt, "fragment-receipt", "", "External content-fragment receipt (required with --content-fragment)")
	return cmd
}

func runFmt(opts *FmtOptions) error {
	if opts.ContentFragment {
		if !opts.Strict || opts.Output == "" || opts.FragmentNamespace == "" || opts.FragmentReceipt == "" || opts.Write || opts.Check || opts.NormalizeRectangularCharts || opts.ChartNormalizationReceipt != "" {
			return fmt.Errorf("content fragment requires --strict, --output, --fragment-namespace and --fragment-receipt; cannot combine with document normalization/write/check")
		}
		content, err := os.ReadFile(opts.InputFile)
		if err != nil {
			return err
		}
		result := fragment.NormalizeMarkdownContent(string(content), parser.ContentOptions{AnchorNamespace: opts.FragmentNamespace})
		if err := fragment.PublishContentFragment(opts.Output, opts.FragmentReceipt, result); err != nil {
			return err
		}
		if !result.Receipt.Accepted {
			return fmt.Errorf("content fragment rejected; no source output produced")
		}
		return nil
	}
	if opts.FragmentNamespace != "" || opts.FragmentReceipt != "" {
		return fmt.Errorf("fragment namespace/receipt require --content-fragment")
	}

	if opts.Output != "" && (opts.Write || opts.Check) {
		return fmt.Errorf("--output cannot be combined with --write or --check")
	}
	if opts.NormalizeRectangularCharts {
		if opts.Output == "" || opts.ChartNormalizationReceipt == "" || opts.Write || opts.Check {
			return fmt.Errorf("normalization requires --output and --chart-normalization-receipt, without --write/--check")
		}
	} else if opts.ChartNormalizationReceipt != "" {
		return fmt.Errorf("--chart-normalization-receipt requires --normalize-rectangular-charts")
	}
	if !opts.Strict {
		return fmt.Errorf("fmt: --strict=false is not supported yet; strict is the only canonical dialect implemented")
	}

	content, err := os.ReadFile(opts.InputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	p := parser.New(util.NewNoop())
	var astNode *ast.AST
	var diags []diagnostics.Diagnostic
	if err := util.RunGuarded(util.DefaultParseTimeout, func() error {
		astNode, diags = p.Parse(string(content), opts.InputFile)
		return nil
	}); err != nil {
		return err
	}
	for _, d := range diags {
		if d.IsError() || (opts.NormalizeRectangularCharts && d.RuleID == "CHART002") {
			return fmt.Errorf("fmt: the file has parse errors, fix them before formatting:\n%s", d.String())
		}
	}
	var receipt transform.ChartNormalizationReceipt
	if opts.NormalizeRectangularCharts {
		astNode, receipt, err = transform.NormalizeRectangularCharts(astNode)
		if err != nil {
			return err
		}
		if !receipt.Accepted {
			if err = transform.PublishNormalizedChartSource(opts.Output, opts.ChartNormalizationReceipt, content, nil, receipt); err != nil {
				return err
			}
			return fmt.Errorf("rectangular chart normalization rejected; no source output produced")
		}
	}

	// Issue #206: si el documento no es ya 'mode: strict', esto transpila —
	// parser.New(...).Parse() ya corrió la normalización completa para
	// flex/flex-full (misma que un build regular), así que el AST resultante
	// ya tiene los mismos node types tipados (QuoteElement, ChecklistElement,
	// etc.) que produciría un parse strict; FormatStrict no distingue el
	// dialecto de origen, solo serializa lo que recibe, y
	// frontMatterOverrides fuerza "mode: strict" en el output sin importar
	// qué mode traía el frontmatter original. Si el doc usa una construcción
	// que el modo strict no puede representar (por ejemplo un bloque ::: con
	// elementos anidados), FormatStrict devuelve UnsupportedElementError y el
	// error se propaga tal cual — nombra el elemento problemático, no falla en
	// silencio.
	isTranspile := astNode.FrontMatter == nil || astNode.FrontMatter.Mode != "strict"

	out, err := formatter.FormatStrict(astNode)
	if err != nil {
		return fmt.Errorf("fmt: %w", err)
	}

	if opts.NormalizeRectangularCharts {
		return transform.PublishNormalizedChartSource(opts.Output, opts.ChartNormalizationReceipt, content, []byte(out), receipt)
	}
	if opts.Output != "" {
		return os.WriteFile(opts.Output, []byte(out), 0644)
	}

	if opts.Check {
		if out != string(content) {
			fmt.Fprint(os.Stderr, checkFailureMessage(opts.InputFile, isTranspile))
			os.Exit(1)
		}
		return nil
	}

	if opts.Write {
		if out == string(content) {
			return nil
		}
		if isTranspile {
			fmt.Fprint(os.Stderr, transpileWriteNotice(opts.InputFile))
		}
		return os.WriteFile(opts.InputFile, []byte(out), 0644)
	}

	fmt.Print(out)
	return nil
}

// transpileWriteNotice y checkFailureMessage están extraídas de runFmt como
// funciones puras (sin I/O) para poder testear su contenido sin invocar
// os.Exit -- code-review en PR #217 notó que --check no tenía NINGÚN test
// (imposible en proceso mientras el os.Exit(1) viviera inline dentro de la
// rama --check), y que --check reportaba el mismo mensaje genérico de
// "drift de formato" para un documento flex que para uno strict
// simplemente desactualizado, sin advertir que --write transpilaría
// (reescritura irreversible de dialecto), no solo reformatearía.

func transpileWriteNotice(inputFile string) string {
	return fmt.Sprintf("fmt: transpiling %q to strict mode: the file will be rewritten in SLIDE syntax (mode: strict)\n", inputFile)
}

func checkFailureMessage(inputFile string, isTranspile bool) string {
	if isTranspile {
		return fmt.Sprintf("%s is not in canonical strict form: it is a flex/flex-full document, and --write would transpile it (an irreversible dialect rewrite, not just a reformat)\n", inputFile)
	}
	return fmt.Sprintf("%s is not in canonical form (run with --write to reformat)\n", inputFile)
}
