// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// TestSpecialBlockParser_TableNestedInside cubre F9 (audit 2026-09-11): una
// tabla dentro de un ":::info" se reconoce como un TableElement tipado en
// Elements, no como texto crudo sin estructura.
func TestSpecialBlockParser_TableNestedInside(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			":::info",
			"Antes de la tabla.",
			"| A | B |",
			"|---|---|",
			"| 1 | 2 |",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	block, ok := result.Element.(*ast.SpecialBlockElement)
	if !ok {
		t.Fatalf("Element is not SpecialBlockElement: %T", result.Element)
	}
	if result.ConsumedLines != 6 {
		t.Errorf("ConsumedLines = %d, want 6", result.ConsumedLines)
	}
	if len(block.Elements) != 1 {
		t.Fatalf("len(Elements) = %d, want 1: %+v", len(block.Elements), block.Elements)
	}
	table, ok := block.Elements[0].(*ast.TableElement)
	if !ok {
		t.Fatalf("Elements[0] is not TableElement: %T", block.Elements[0])
	}
	if len(table.Rows) != 1 || table.Rows[0][0] != "1" {
		t.Errorf("table.Rows = %#v, want [[\"1\" \"2\"]]", table.Rows)
	}
	// Content sigue teniendo TODAS las líneas del cuerpo, incluida la tabla
	// (Content y Elements son vistas paralelas, no una partición — ver el
	// comentario de Parse).
	wantContent := "Antes de la tabla.\n| A | B |\n|---|---|\n| 1 | 2 |"
	if block.Content != wantContent {
		t.Errorf("Content = %q, want %q", block.Content, wantContent)
	}
}

// TestSpecialBlockParser_NestedSpecialBlock cubre el repro real de
// examples/01_title_and_content/01.7_advanced_inline_syntax_flex.slidelang
// (":::reveal" con varios ":::reveal_content" adentro): antes de este fix,
// un ":::tipo" anidado CERRABA el bloque padre sin consumirlo (tratándolo
// como si fuera un hermano de nivel superior), fragmentando el documento y
// perdiendo el cierre real del padre.
func TestSpecialBlockParser_NestedSpecialBlock(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"::: reveal",
			"### Level 1",
			"Texto del nivel 1.",
			"::: reveal_content",
			"Contenido revelado.",
			":::",
			"### Level 2",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	block, ok := result.Element.(*ast.SpecialBlockElement)
	if !ok {
		t.Fatalf("Element is not SpecialBlockElement: %T", result.Element)
	}
	if result.ConsumedLines != 8 {
		t.Fatalf("ConsumedLines = %d, want 8 (el bloque completo, incluido su propio cierre)", result.ConsumedLines)
	}
	if block.BlockType != "reveal" {
		t.Errorf("BlockType = %q, want \"reveal\"", block.BlockType)
	}
	if len(block.Elements) != 1 {
		t.Fatalf("len(Elements) = %d, want 1 (el reveal_content anidado)", len(block.Elements))
	}
	nested, ok := block.Elements[0].(*ast.SpecialBlockElement)
	if !ok {
		t.Fatalf("Elements[0] is not SpecialBlockElement: %T", block.Elements[0])
	}
	if nested.BlockType != "reveal_content" {
		t.Errorf("nested.BlockType = %q, want \"reveal_content\"", nested.BlockType)
	}
	if nested.Content != "Contenido revelado." {
		t.Errorf("nested.Content = %q, want \"Contenido revelado.\"", nested.Content)
	}
	// "### Level 2" viene DESPUÉS del reveal_content anidado y sigue siendo
	// prosa suelta del padre — no se promueve a heading (eso lo hace el
	// parser de nivel superior, no éste) ni se pierde.
	if !containsLine(block.Content, "### Level 2") {
		t.Errorf("Content no incluye \"### Level 2\": %q", block.Content)
	}
}

// TestSpecialBlockParser_SelfClosingOneLiner_DoesNotConsumeFollowingContent
// cubre F13/C25 (audit 2026-09-11): un ":::tipo attrs ... :::" que abre y
// cierra en la MISMA línea no debe escanear hacia adelante buscando un
// cierre que ya pasó — antes de este fix, un bloque de este tipo colgaba
// escaneando el resto del documento y adoptaba como hijos anidados a los
// bloques ":::" siguientes (regresión encontrada contra el fixture real
// 01.7_advanced_inline_syntax_flex.slidelang: "External Widgets" y otro
// ":::embed" completo desaparecían, absorbidos como prosa/hijos del
// primer embed, roto).
func TestSpecialBlockParser_SelfClosingOneLiner_DoesNotConsumeFollowingContent(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			`::: embed type="youtube" video_id="abc123" :::`,
			"",
			"## Next Heading",
			"",
			"Prosa después, no debería tocarse.",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	if result.ConsumedLines != 1 {
		t.Fatalf("ConsumedLines = %d, want 1 (el bloque ya cerró en su propia línea)", result.ConsumedLines)
	}
	block, ok := result.Element.(*ast.SpecialBlockElement)
	if !ok {
		t.Fatalf("Element is not SpecialBlockElement: %T", result.Element)
	}
	if block.BlockType != "embed" {
		t.Errorf("BlockType = %q, want \"embed\"", block.BlockType)
	}
	if block.Content != "" {
		t.Errorf("Content = %q, want \"\" (nada que escanear tras el autocierre)", block.Content)
	}
	if len(block.Elements) != 0 {
		t.Errorf("Elements = %+v, want vacío", block.Elements)
	}
}

// TestSpecialBlockParser_StopsAtSlideSeparator_WithoutConsuming cubre la
// otra mitad de F9: un ":::bloque" sin su propio cierre no debe tragarse el
// "---" que separa slides/frontmatter en modo flex — antes de este fix solo
// se chequeaba "SLIDE " (el límite de modo strict), así que en flex un
// bloque sin cerrar fusionaba dos slides en una sola en silencio.
func TestSpecialBlockParser_StopsAtSlideSeparator_WithoutConsuming(t *testing.T) {
	parser := &SpecialBlockParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			":::info",
			"Contenido sin cierre.",
			"---",
			"# Siguiente slide",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	if result.ConsumedLines != 2 {
		t.Fatalf("ConsumedLines = %d, want 2 (no debe consumir el \"---\")", result.ConsumedLines)
	}
}

func containsLine(content, want string) bool {
	for _, line := range splitLinesForTest(content) {
		if line == want {
			return true
		}
	}
	return false
}

func splitLinesForTest(s string) []string {
	var lines []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}
