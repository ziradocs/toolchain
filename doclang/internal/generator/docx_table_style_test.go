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
	widths := docxTableColumnWidths(table, 2, 9000)
	if widths[0]+widths[1] != 9000 {
		t.Errorf("widths %v do not sum to 9000", widths)
	}
	if widths[0] < 9000*4/44 {
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
