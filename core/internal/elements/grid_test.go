// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func TestGridParser_CanParse(t *testing.T) {
	parser := &GridParser{}

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"grid start", "::: grid", true},
		{"not grid", "::: info", false},
		{"plain text", "some text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parser.CanParse(tt.line, "flex"); got != tt.expected {
				t.Errorf("CanParse(%q) = %v, want %v", tt.line, got, tt.expected)
			}
		})
	}
}

// TestGridParser_MultiColumn_NoBlankLineBetweenColumns cubre issue #9: sin línea en
// blanco entre el ":::" de cierre de una columna y el "::: column" de la siguiente,
// el parser perdía columnas completas (bug de doble-avance de línea en el loop
// principal, que caía dos veces sobre el mismo separador ":::").
func TestGridParser_MultiColumn_NoBlankLineBetweenColumns(t *testing.T) {
	parser := &GridParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"::: grid",
			"::: column",
			"Option A content",
			":::",
			"::: column",
			"Option B content",
			":::",
			"::: column",
			"Option C content",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	grid, ok := result.Element.(*ast.GridElement)
	if !ok {
		t.Fatal("Element is not GridElement")
	}

	if len(grid.Columns) != 3 {
		t.Fatalf("len(Columns) = %d, want 3 (columns must not be lost when there's no blank line separator)", len(grid.Columns))
	}

	want := []string{"Option A content", "Option B content", "Option C content"}
	for i, w := range want {
		if grid.Columns[i].Content != w {
			t.Errorf("Columns[%d].Content = %q, want %q", i, grid.Columns[i].Content, w)
		}
	}

	// El separador ":::" entre columnas no debe filtrarse a GridElement.Content
	if grid.Content != "" {
		t.Errorf("Content = %q, want empty (no stray separator leakage)", grid.Content)
	}
}

// TestGridParser_MultiColumn_WithBlankLineBetweenColumns es el caso "feliz" original
// (con línea en blanco entre columnas), que ya funcionaba antes del fix.
func TestGridParser_MultiColumn_WithBlankLineBetweenColumns(t *testing.T) {
	parser := &GridParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"::: grid",
			"::: column",
			"Option A content",
			":::",
			"",
			"::: column",
			"Option B content",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	grid := result.Element.(*ast.GridElement)

	if len(grid.Columns) != 2 {
		t.Fatalf("len(Columns) = %d, want 2", len(grid.Columns))
	}
	if grid.Columns[0].Content != "Option A content" || grid.Columns[1].Content != "Option B content" {
		t.Errorf("unexpected column contents: %q, %q", grid.Columns[0].Content, grid.Columns[1].Content)
	}
}

// TestGridParser_StrayContentPreserved cubre issue #9: prosa suelta dentro de
// "::: grid ... :::" pero fuera de cualquier "::: column" ya no se descarta
// silenciosamente; se preserva en GridElement.Content.
func TestGridParser_StrayContentPreserved(t *testing.T) {
	parser := &GridParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"::: grid",
			"Texto introductorio antes de cualquier columna",
			"::: column",
			"A",
			":::",
			"::: column",
			"B",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	grid := result.Element.(*ast.GridElement)

	if grid.Content != "Texto introductorio antes de cualquier columna" {
		t.Errorf("Content = %q, want stray prose to be preserved", grid.Content)
	}
	if len(grid.Columns) != 2 {
		t.Fatalf("len(Columns) = %d, want 2", len(grid.Columns))
	}
}

// TestGridParser_StrayContent_PreservesParagraphBreaks cubre una inconsistencia
// encontrada al revisar PR #50: a diferencia de parseColumn (que preserva líneas
// en blanco como separadores de párrafo), la primera versión de la captura de
// contenido suelto las descartaba silenciosamente. Debe comportarse igual que
// una columna: una línea en blanco ENTRE dos líneas de prosa suelta es un salto
// de párrafo real y se preserva.
func TestGridParser_StrayContent_PreservesParagraphBreaks(t *testing.T) {
	parser := &GridParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"::: grid",
			"Primer parrafo",
			"",
			"Segundo parrafo",
			"::: column",
			"A",
			":::",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	grid := result.Element.(*ast.GridElement)

	want := "Primer parrafo\n\nSegundo parrafo"
	if grid.Content != want {
		t.Errorf("Content = %q, want %q (paragraph break preserved)", grid.Content, want)
	}
}

// TestGridParser_MultiColumn_WithBlankLineBetweenColumns ya cubre el caso feliz;
// este test confirma explícitamente que una línea en blanco que actúa solo como
// separador ENTRE columnas (no como salto de párrafo de prosa suelta) no se
// filtra a GridElement.Content.
func TestGridParser_BlankLineBetweenColumns_DoesNotLeakIntoContent(t *testing.T) {
	parser := &GridParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"::: grid",
			"::: column",
			"A",
			":::",
			"",
			"::: column",
			"B",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	grid := result.Element.(*ast.GridElement)

	if grid.Content != "" {
		t.Errorf("Content = %q, want empty (blank line between columns is just spacing, not stray prose)", grid.Content)
	}
	if len(grid.Columns) != 2 {
		t.Fatalf("len(Columns) = %d, want 2", len(grid.Columns))
	}
}

// TestGridParser_NoClosingDelimiter_EOF cubre el caso de un bloque grid truncado:
// llega a EOF sin un ":::" de cierre. El parser no debe colgarse ni perder lo ya
// acumulado (columnas y contenido suelto).
func TestGridParser_NoClosingDelimiter_EOF(t *testing.T) {
	parser := &GridParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"::: grid",
			"Texto suelto sin cierre",
			"::: column",
			"Contenido A",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	grid, ok := result.Element.(*ast.GridElement)
	if !ok {
		t.Fatal("Element is not GridElement")
	}

	if result.ConsumedLines != len(ctx.Lines) {
		t.Errorf("ConsumedLines = %d, want %d (all lines consumed even without closing :::)", result.ConsumedLines, len(ctx.Lines))
	}
	if grid.Content != "Texto suelto sin cierre" {
		t.Errorf("Content = %q, want stray content preserved despite missing closing delimiter", grid.Content)
	}
	if len(grid.Columns) != 1 || grid.Columns[0].Content != "Contenido A" {
		t.Errorf("Columns incorrect: %+v", grid.Columns)
	}
}

// TestGridParser_OnlyStrayContent_NoColumns cubre un grid sin ninguna "::: column",
// solo prosa suelta - el caso límite de cero columnas no debe perder el contenido.
func TestGridParser_OnlyStrayContent_NoColumns(t *testing.T) {
	parser := &GridParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"::: grid",
			"Todo esto es prosa suelta",
			"sin ninguna columna definida",
			":::",
		},
	}

	result := parser.Parse(ctx, 0)
	grid := result.Element.(*ast.GridElement)

	want := "Todo esto es prosa suelta\nsin ninguna columna definida"
	if grid.Content != want {
		t.Errorf("Content = %q, want %q", grid.Content, want)
	}
	if len(grid.Columns) != 0 {
		t.Errorf("len(Columns) = %d, want 0", len(grid.Columns))
	}
}

// TestGridParser_StopsAtSlideDirective cubre la rama "SLIDE " del loop principal:
// un bloque grid sin cierre explícito debe cortarse al llegar a la siguiente
// directiva SLIDE, sin consumir esa línea (la deja para el parser principal).
func TestGridParser_StopsAtSlideDirective(t *testing.T) {
	parser := &GridParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"::: grid",
			"::: column",
			"Contenido A",
			":::",
			"SLIDE 2",
		},
	}

	result := parser.Parse(ctx, 0)
	grid := result.Element.(*ast.GridElement)

	if result.ConsumedLines != 4 {
		t.Errorf("ConsumedLines = %d, want 4 (the SLIDE line must not be consumed)", result.ConsumedLines)
	}
	if len(grid.Columns) != 1 || grid.Columns[0].Content != "Contenido A" {
		t.Errorf("Columns incorrect: %+v", grid.Columns)
	}
}
