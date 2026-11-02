// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func astWithTable() *ast.AST {
	doc := newTestAST()
	table := ast.NewTableElement(diagnostics.NewPosition(6, 1))
	table.Headers = []string{"Formato", "Red", "Total"}
	table.Rows = [][]string{{"HTML", "CDN", "10"}, {"DOCX", "No", "200"}, {"PDF", "No", "3000"}}
	table.ColumnAlign = []string{"", "center", "right"}
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, table)
	return doc
}

// TestDOCXGenerator_Table_Styled: encabezado con el fondo del tema que se
// repite en cada página, bordes finos, filas alternas, alineación de la
// tabla Markdown y ancho de la caja de texto.
func TestDOCXGenerator_Table_Styled(t *testing.T) {
	for _, theme := range []string{"professional", "academic", "technical", "page-view"} {
		s := GetStyleForTheme(theme, newTestLogger())
		xml := buildDOCXXML(t, astWithTable(), GeneratorOptions{Theme: theme})

		if n := strings.Count(xml, `w:fill="`+s.TableHeaderBg+`"`); n != 3 {
			t.Errorf("%s: want 3 header cells filled %s, got %d", theme, s.TableHeaderBg, n)
		}
		if n := strings.Count(xml, "<w:tblHeader></w:tblHeader>"); n != 1 {
			t.Errorf("%s: want the header row repeated (one w:tblHeader), got %d", theme, n)
		}
		if !strings.Contains(xml, "<w:tblBorders>") || !strings.Contains(xml, `w:color="`+s.TableBorderColor+`"`) {
			t.Errorf("%s: table borders missing", theme)
		}
		// Filas de datos 0,1,2: solo la del medio va con banda.
		if n := strings.Count(xml, `w:fill="`+s.TableStripeBg+`"`); n != 3 {
			t.Errorf("%s: want one banded row (3 cells), got %d cells", theme, n)
		}
	}

	xml := buildDOCXXML(t, astWithTable(), GeneratorOptions{})
	if n := strings.Count(xml, `<w:jc w:val="center">`); n < 4 {
		t.Errorf("want the centered column (header + 3 rows), got %d centered paragraphs", n)
	}
	if n := strings.Count(xml, `<w:jc w:val="right">`); n != 4 {
		t.Errorf("want the right-aligned column (header + 3 rows), got %d", n)
	}
	// docxgo arranca en A4 con márgenes de 1": 11906-2880 = 9026 twips.
	if !strings.Contains(xml, `<w:tblW w:type="dxa" w:w="9026">`) {
		t.Errorf("table width is not the text width:\n%s", xml)
	}
}

func TestDOCXTableColumnWidths_SumToAvailableAndAreBounded(t *testing.T) {
	table := ast.NewTableElement(diagnostics.NewPosition(1, 1))
	table.Headers = []string{"Id", "Descripción"}
	table.Rows = [][]string{{"1", strings.Repeat("palabra ", 50)}}
	widths := docxTableColumnWidths(table, 2, 9000, 22)
	if widths[0]+widths[1] != 9000 {
		t.Errorf("widths %v do not sum to 9000", widths)
	}
	if widths[0] < 9000*6/46 {
		t.Errorf("short column squeezed below its minimum: %v", widths)
	}
}

// TestDOCXStyles_TableHeaderContrast: el texto del encabezado (negrita de
// 11 pt, que WCAG no cuenta como texto grande) llega a 4.5:1 en cada tema.
func TestDOCXStyles_TableHeaderContrast(t *testing.T) {
	for _, theme := range []string{"professional", "academic", "technical", "page-view"} {
		s := GetStyleForTheme(theme, newTestLogger())
		if r := contrastRatio(t, s.TableHeaderColor, s.TableHeaderBg); r < 4.5 {
			t.Errorf("%s: header %s on %s = %.2f, want >= 4.5", theme, s.TableHeaderColor, s.TableHeaderBg, r)
		}
		if r := contrastRatio(t, s.TextColor, s.TableStripeBg); r < 4.5 {
			t.Errorf("%s: text %s on stripe %s = %.2f, want >= 4.5", theme, s.TextColor, s.TableStripeBg, r)
		}
	}
}

// TestMarkdownGenerator_TableKeepsColumnAlign: el Markdown que emite doclang
// conserva la alineación de la tabla de origen.
func TestMarkdownGenerator_TableKeepsColumnAlign(t *testing.T) {
	got := NewMarkdownGenerator(newTestLogger()).renderElement(astWithTable().ContentBlocks[0].Elements[2])
	if !strings.Contains(got, "| --- | :---: | ---: |") {
		t.Errorf("delimiter row lost the alignment:\n%s", got)
	}
}

// TestDOCXTableColumnWidths_BackendEndpointsHeader es la tabla de endpoints
// de docs/architecture/backend.md (slidelang-app), donde "Método" salía
// partida en "Méto/do": la columna recibía 837 twips por reparto
// proporcional. Ningún encabezado puede quedar más angosto que su palabra
// más larga en negrita a 11 pt.
func TestDOCXTableColumnWidths_BackendEndpointsHeader(t *testing.T) {
	table := ast.NewTableElement(diagnostics.NewPosition(1, 1))
	table.Headers = []string{"Endpoint", "Método", "Descripción", "Rate Limited"}
	table.Rows = [][]string{
		{"`/health`", "GET", "Health check del backend", "❌"},
		{"`/api/presentations/health`", "GET", "Health check SlideLang", "✅"},
		{"`/api/presentations/:id`", "GET", "Obtener presentación desde SlideLang", "✅"},
	}
	const available, size = 9026, 22 // A4 con márgenes de 1", 11 pt
	widths := docxTableColumnWidths(table, 4, available, size)

	sum := 0
	for j, w := range widths {
		sum += w
		if min := docxHeaderMinWidth(table.Headers[j], size); w < min {
			t.Errorf("columna %q: %d twips, menos que los %d de su encabezado", table.Headers[j], w, min)
		}
	}
	if sum != available {
		t.Errorf("widths %v suman %d, want %d", widths, sum, available)
	}
	// "Método": 6 caracteres a 0.62 em de 11 pt más el margen de la celda.
	if widths[1] < 1000 {
		t.Errorf("Método: %d twips, want >= 1000", widths[1])
	}
}

func TestDocxHeaderMinWidth_LongestWord(t *testing.T) {
	// "Rate Limited" se puede partir entre palabras: manda "Limited" (7).
	if a, b := docxHeaderMinWidth("Rate Limited", 22), docxHeaderMinWidth("Limited", 22); a != b {
		t.Errorf("Rate Limited = %d, Limited = %d; want equal", a, b)
	}
	// La tilde no agrega ancho: "Método" mide lo mismo que "Metodo".
	if a, b := docxHeaderMinWidth("Método", 22), docxHeaderMinWidth("Metodo", 22); a != b {
		t.Errorf("Método = %d, Metodo = %d", a, b)
	}
}
