// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// TestRunFmt_TranspileFlexToStrict cubre issue #206: un documento
// 'mode: flex'/'flex-ai' ahora se transpila a strict en vez de ser
// rechazado. El AST que produce parser.New(...).Parse() para flex ya viene
// completamente normalizado (mismo camino que un build regular), así que
// FormatStrict puede serializarlo directamente — este test confirma que el
// resultado con --write (a) trae 'mode: strict' en el frontmatter y (b)
// re-parsea sin errores como contenido strict válido.
func TestRunFmt_TranspileFlexToStrict(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "deck.slidelang")
	source := `---
title: Flex Deck
mode: flex
---

# Introduction

Welcome to the deck.

## Highlights

- First point
- Second point
`
	if err := os.WriteFile(inputFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	opts := &FmtOptions{InputFile: inputFile, Strict: true, Write: true}
	if err := runFmt(opts); err != nil {
		t.Fatalf("runFmt: %v", err)
	}

	out, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("failed to read transpiled output: %v", err)
	}
	if !strings.Contains(string(out), "mode: strict") {
		t.Fatalf("transpiled output missing 'mode: strict' in frontmatter:\n%s", out)
	}
	if !strings.Contains(string(out), "SLIDE") {
		t.Fatalf("transpiled output missing SLIDE marker(s):\n%s", out)
	}

	p := parser.New(util.NewNoop())
	astNode, diags := p.Parse(string(out), inputFile)
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("transpiled output failed to re-parse: %s\n--- output ---\n%s", d.String(), out)
		}
	}
	if len(astNode.ContentBlocks) != 2 {
		t.Fatalf("re-parsed transpiled output has %d content blocks, want 2 (one per '# '/'## ' heading in the flex source)", len(astNode.ContentBlocks))
	}
}

// TestRunFmt_TranspileFlexToStrict_Grid: el modo strict ahora TIENE sintaxis
// de grid (<<grid>>/<<column>>/<<end>>), así que un GRID en el documento flex
// de origen ya no es un gap — se transpila correctamente. (Antes este test
// afirmaba lo contrario: que la transpilación fallaba con
// UnsupportedElementError.) Confirma que la salida trae la sintaxis de grid
// strict y re-parsea a un GridElement con las mismas columnas.
func TestRunFmt_TranspileFlexToStrict_Grid(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "grid.slidelang")
	source := `---
title: Grid Deck
mode: flex
---

# Layout

::: grid
::: column
Left column.
:::
::: column
Right column.
:::
:::
`
	if err := os.WriteFile(inputFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	opts := &FmtOptions{InputFile: inputFile, Strict: true, Write: true}
	if err := runFmt(opts); err != nil {
		t.Fatalf("runFmt: unexpected error transpiling a GRID to strict: %v", err)
	}

	out, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("failed to read transpiled output: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "mode: strict") {
		t.Fatalf("transpiled output missing 'mode: strict':\n%s", got)
	}
	for _, marker := range []string{"<<grid>>", "<<column>>", "<<end>>"} {
		if !strings.Contains(got, marker) {
			t.Fatalf("transpiled output missing grid marker %q:\n%s", marker, got)
		}
	}

	p := parser.New(util.NewNoop())
	astNode, diags := p.Parse(got, inputFile)
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("transpiled grid output failed to re-parse: %s\n--- output ---\n%s", d.String(), got)
		}
	}
	var grid *ast.GridElement
	for _, b := range astNode.ContentBlocks {
		for _, el := range b.Elements {
			if g, ok := el.(*ast.GridElement); ok {
				grid = g
			}
		}
	}
	if grid == nil {
		t.Fatalf("re-parsed transpiled output has no GridElement:\n%s", got)
	}
	if len(grid.Columns) != 2 {
		t.Fatalf("transpiled grid has %d columns, want 2: %+v", len(grid.Columns), grid.Columns)
	}
	if grid.Columns[0].Content != "Left column." || grid.Columns[1].Content != "Right column." {
		t.Errorf("transpiled grid columns = %q / %q, want %q / %q",
			grid.Columns[0].Content, grid.Columns[1].Content, "Left column.", "Right column.")
	}
}

// TestRunFmt_AlreadyStrict_RegressionCheck confirma que el path pre-#206
// (documento ya 'mode: strict') sigue funcionando sin cambios: reformatear
// es un no-op cuando el archivo ya está en forma canónica.
func TestRunFmt_AlreadyStrict_RegressionCheck(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "strict.slidelang")
	source := `---
mode: strict
---

SLIDE content
  title: "Hello"
  TEXT
    Hello, strict world.
`
	if err := os.WriteFile(inputFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	opts := &FmtOptions{InputFile: inputFile, Strict: true, Write: true}
	if err := runFmt(opts); err != nil {
		t.Fatalf("runFmt: %v", err)
	}

	out, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if string(out) != source {
		t.Fatalf("expected no-op reformat of already-canonical strict source, got a diff:\n--- want ---\n%s\n--- got ---\n%s", source, out)
	}
}

// TestCheckFailureMessage_DistinguishesTranspileFromDrift cubre el hallazgo
// de code-review en PR #217: --check reportaba el mismo mensaje genérico
// para un documento flex (que NUNCA puede pasar el check, por definición)
// que para uno strict simplemente desactualizado, sin advertir que --write
// transpilaría en vez de solo reformatear. Extraída como función pura para
// poder testear el contenido del mensaje sin invocar os.Exit (--check
// llama a os.Exit(1) directamente, matando el proceso de test si se
// invoca runFmt con Check:true en proceso).
func TestCheckFailureMessage_DistinguishesTranspileFromDrift(t *testing.T) {
	transpileMsg := checkFailureMessage("deck.slidelang", true)
	if !strings.Contains(transpileMsg, "transpilaría") || !strings.Contains(transpileMsg, "irreversible") {
		t.Errorf("transpile-case message should warn about the irreversible dialect rewrite, got: %q", transpileMsg)
	}

	driftMsg := checkFailureMessage("deck.slidelang", false)
	if strings.Contains(driftMsg, "transpilaría") {
		t.Errorf("non-transpile-case message should not mention transpiling, got: %q", driftMsg)
	}
	if !strings.Contains(driftMsg, "--write") {
		t.Errorf("non-transpile-case message should still point at --write, got: %q", driftMsg)
	}

	if transpileMsg == driftMsg {
		t.Fatal("expected distinct messages for the transpile vs. plain-drift cases, got identical text")
	}
}

func TestTranspileWriteNotice_MentionsFileAndStrictMode(t *testing.T) {
	msg := transpileWriteNotice("deck.slidelang")
	if !strings.Contains(msg, "deck.slidelang") {
		t.Errorf("notice should name the file, got: %q", msg)
	}
	if !strings.Contains(msg, "mode: strict") {
		t.Errorf("notice should mention the resulting mode, got: %q", msg)
	}
}
