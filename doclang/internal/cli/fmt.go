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

type fmtOptions struct {
	inputFile string
	write     bool
	check     bool
	strict    bool
	// strictSet distingue "no pasó --strict" de "pasó --strict=false", que
	// significan cosas distintas: preservar el dialecto del documento vs.
	// pedir explícitamente flex.
	strictSet bool
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
		doc, diags = parser.New(util.NewNoop()).ParseDocument(string(content), opts.inputFile)
		return nil
	}); err != nil {
		return err
	}
	for _, d := range diags {
		if d.IsError() {
			return fmt.Errorf("fmt: el archivo tiene errores de parseo, corrígelos antes de formatear:\n%s", d.String())
		}
	}

	sourceIsStrict := doc != nil && doc.FrontMatter != nil && doc.FrontMatter.Mode == "strict"

	// Sin --strict se preserva el dialecto del documento. Con --strict=false
	// explícito sobre un documento strict se estaría pidiendo degradarlo a
	// flex, que descarta la declaración del autor sin forma de recuperarla.
	targetStrict := sourceIsStrict
	if opts.strictSet {
		if !opts.strict && sourceIsStrict {
			return fmt.Errorf("fmt: --strict=false degradaría %q de strict a flex, descartando el dialecto que declara; "+
				"si es lo que querés, cambiá `mode:` en el frontmatter a mano", opts.inputFile)
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
	return fmt.Sprintf("fmt: transpilando %q a modo strict — el archivo será reescrito en la sintaxis SECTION (mode: strict)\n", inputFile)
}

func checkFailureMessage(inputFile string, isTranspile bool) string {
	if isTranspile {
		return fmt.Sprintf("%s no está en forma canónica strict — es un documento flex; --write lo transpilaría "+
			"(reescritura de dialecto, no un simple reformateo)\n", inputFile)
	}
	return fmt.Sprintf("%s no está en forma canónica (correr con --write para reformatear)\n", inputFile)
}
