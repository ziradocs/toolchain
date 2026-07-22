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
	"go.ziradocs.com/core/v2/util"
)

type FmtOptions struct {
	InputFile string
	Strict    bool
	Write     bool
	Check     bool
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
    output frontmatter. This fails with an error naming the element if the
    document uses a construct strict mode cannot represent yet (e.g. GRID
    — see issue #214).

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
	return cmd
}

func runFmt(opts *FmtOptions) error {
	if !opts.Strict {
		return fmt.Errorf("fmt: --strict=false no está soportado hoy; strict es el único dialecto canónico implementado")
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
		if d.IsError() {
			return fmt.Errorf("fmt: el archivo tiene errores de parseo, corrígelos antes de formatear:\n%s", d.String())
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
	// que el modo strict no puede representar (GRID hoy, ver issue #214),
	// FormatStrict devuelve UnsupportedElementError y el error se propaga tal
	// cual — nombra el elemento problemático, no falla en silencio.
	isTranspile := astNode.FrontMatter == nil || astNode.FrontMatter.Mode != "strict"

	out, err := formatter.FormatStrict(astNode)
	if err != nil {
		return fmt.Errorf("fmt: %w", err)
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
	return fmt.Sprintf("fmt: transpilando %q a modo strict — el archivo será reescrito en la sintaxis SLIDE (mode: strict)\n", inputFile)
}

func checkFailureMessage(inputFile string, isTranspile bool) string {
	if isTranspile {
		return fmt.Sprintf("%s no está en forma canónica strict — es un documento flex/flex-full; --write lo transpilaría (reescritura irreversible de dialecto, no un simple reformateo)\n", inputFile)
	}
	return fmt.Sprintf("%s no está en forma canónica (correr con --write para reformatear)\n", inputFile)
}
