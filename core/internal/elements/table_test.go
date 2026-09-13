// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func TestTableParser_CanParse(t *testing.T) {
	parser := &TableParser{}

	tests := []struct {
		name     string
		line     string
		mode     string
		expected bool
	}{
		{"strict TABLE keyword", "TABLE", "strict", true},
		// issue #20: TABLE must be recognized in flex too (doclang only
		// parses flex) so the explicit "cells:" merged-cell syntax is
		// authorable in doclang.
		{"flex TABLE keyword", "TABLE", "flex", true},
		{"markdown table row", "| A | B |", "flex", true},
		{"plain text", "just some text", "flex", false},
		// Regression from issue #245: TableParser runs before Quote/
		// Checklist/Points/Text in the registry (GetDefaultRegistry) —
		// without requiring a leading "|", any line with 2+ pipes stole the
		// element from its real parser.
		{"bullet with 2+ pipes is not a table (issue #245)", "- Compara pandas | numpy | scipy", "flex", false},
		{"quote with 2+ pipes is not a table", "> revenue | costs | margin", "flex", false},
		{"checklist with 2+ pipes is not a table", "- [ ] a | b | c", "flex", false},
		{"markdown table row with leading whitespace still matches", "  | A | B |", "flex", true},
		// Regression: widening the TABLE keyword to flex mode means it now
		// runs over ordinary doclang prose too. HasPrefix(trimmed, "TABLE")
		// would swallow a heading/paragraph that merely starts with the
		// word "TABLE" as a (bogus) table-block start, dropping the real
		// content. Only the bare "TABLE" token is the block keyword.
		{"prose starting with TABLE is not a table block", "TABLE OF CONTENTS", "flex", false},
		{"prose \"TABLE N.N: ...\" is not a table block", "TABLE 3.1: Resultados", "flex", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parser.CanParse(tt.line, tt.mode); got != tt.expected {
				t.Errorf("CanParse(%q, %q) = %v, want %v", tt.line, tt.mode, got, tt.expected)
			}
		})
	}
}

func TestTableParser_ParseMarkdownTable(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Header A | Header B |",
			"|---|---|",
			"| val1 | val2 |",
			"| val3 | val4 |",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatal("Element is not TableElement")
	}

	wantHeaders := []string{"Header A", "Header B"}
	if len(table.Headers) != len(wantHeaders) {
		t.Fatalf("len(Headers) = %d, want %d", len(table.Headers), len(wantHeaders))
	}
	for i, h := range wantHeaders {
		if table.Headers[i] != h {
			t.Errorf("Headers[%d] = %q, want %q", i, table.Headers[i], h)
		}
	}

	if len(table.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(table.Rows))
	}
}

// TestTableParser_ParseMarkdownTable_AutoDerivesCells covers issue #20: a
// simple markdown table (no merged cells) must populate Cells by deriving
// it from Headers/Rows — the header row marked IsHeader+Scope="col", the
// body as plain cells with no span — so an A11Y rulepack can walk the cell
// structure regardless of which authoring syntax was used.
func TestTableParser_ParseMarkdownTable_AutoDerivesCells(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Header A | Header B |",
			"|---|---|",
			"| val1 | val2 |",
		},
	}

	result := parser.Parse(ctx, 0)
	table := result.Element.(*ast.TableElement)

	if len(table.Cells) != 2 {
		t.Fatalf("len(Cells) = %d, want 2 (1 header row + 1 body row)", len(table.Cells))
	}
	headerRow := table.Cells[0]
	if len(headerRow) != 2 || !headerRow[0].IsHeader || headerRow[0].Scope != "col" || headerRow[0].Content != "Header A" {
		t.Errorf("Cells[0] = %+v, want header row with IsHeader=true, Scope=col", headerRow)
	}
	bodyRow := table.Cells[1]
	if len(bodyRow) != 2 || bodyRow[0].IsHeader || bodyRow[0].Content != "val1" {
		t.Errorf("Cells[1] = %+v, want plain body row", bodyRow)
	}
}

// TestTableParser_ExplicitCells_MergedHeader covers issue #20: the explicit
// "cells:" syntax inside a TABLE block in flex mode (doclang) must parse
// colspan/scope/header, and the derived Headers/Rows must stay
// dimensionally consistent (same column count in headers and in every row)
// — precisely the property that avoids the TABLE003 false positive (see
// TestElementStructureRule_MergedCells_NoFalsePositive in core/linter).
func TestTableParser_ExplicitCells_MergedHeader(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"TABLE",
			"  cells:",
			"    - [{content: A, header: true, colspan: 2}, {content: B, header: true}]",
			"    - [{content: 1}, {content: 2}, {content: 3}]",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatal("Element is not TableElement")
	}

	if len(table.Cells) != 2 {
		t.Fatalf("len(Cells) = %d, want 2", len(table.Cells))
	}
	if got := table.Cells[0][0]; !got.IsHeader || got.ColSpan != 2 || got.Content != "A" {
		t.Errorf("Cells[0][0] = %+v, want {Content:A IsHeader:true ColSpan:2}", got)
	}

	wantHeaders := []string{"A", "A", "B"}
	if len(table.Headers) != len(wantHeaders) {
		t.Fatalf("len(Headers) = %d, want %d (derived from colspan=2 on A)", len(table.Headers), len(wantHeaders))
	}
	for i, h := range wantHeaders {
		if table.Headers[i] != h {
			t.Errorf("Headers[%d] = %q, want %q", i, table.Headers[i], h)
		}
	}

	if len(table.Rows) != 1 || len(table.Rows[0]) != len(table.Headers) {
		t.Fatalf("Rows = %v, want 1 row with %d columns (matching Headers width)", table.Rows, len(table.Headers))
	}
}

// TestTableParser_StrictMode_CaptionPopulated cubre issue #9: TableElement.Caption
// existía en el struct pero el parser strict-mode nunca lo poblaba desde la línea
// "caption:" (a diferencia de image.go, que ya soporta ese patrón).
func TestTableParser_StrictMode_CaptionPopulated(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"TABLE",
			`  headers: ["Q1", "Q2"]`,
			`  caption: "Ventas trimestrales"`,
			"  rows:",
			"      [100, 200]",
			"      [150, 250]",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatal("Element is not TableElement")
	}

	if table.Caption != "Ventas trimestrales" {
		t.Errorf("Caption = %q, want %q", table.Caption, "Ventas trimestrales")
	}
	if len(table.Headers) != 2 {
		t.Errorf("len(Headers) = %d, want 2", len(table.Headers))
	}
	if len(table.Rows) != 2 {
		t.Errorf("len(Rows) = %d, want 2", len(table.Rows))
	}
}

// TestTableParser_StrictMode_CaptionAfterRows cubre el mismo caso que
// TestTableParser_StrictMode_CaptionPopulated pero con "caption:" DESPUÉS del
// bloque "rows:", para verificar que el manejo de índice (i--/continue) del
// sub-loop de rows no rompe el procesamiento de la línea de caption siguiente.
func TestTableParser_StrictMode_CaptionAfterRows(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"TABLE",
			`  headers: ["Q1", "Q2"]`,
			"  rows:",
			"      [100, 200]",
			"      [150, 250]",
			`  caption: "Ventas trimestrales"`,
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	table := result.Element.(*ast.TableElement)

	if table.Caption != "Ventas trimestrales" {
		t.Errorf("Caption = %q, want %q (caption after rows must still be parsed)", table.Caption, "Ventas trimestrales")
	}
	if len(table.Rows) != 2 {
		t.Errorf("len(Rows) = %d, want 2", len(table.Rows))
	}
}

// TestTableParser_StrictMode_NoCaption_StaysEmpty es la contraparte de
// regresión: una tabla strict-mode sin línea "caption:" debe dejar
// TableElement.Caption como "" (sin inventar ni heredar un valor por defecto).
func TestTableParser_StrictMode_NoCaption_StaysEmpty(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"TABLE",
			`  headers: ["Q1", "Q2"]`,
			"  rows:",
			"      [100, 200]",
		},
	}

	result := parser.Parse(ctx, 0)
	table := result.Element.(*ast.TableElement)

	if table.Caption != "" {
		t.Errorf("Caption = %q, want empty when no caption: line is present", table.Caption)
	}
}

// TestTableParser_MarkdownMode_NeverSetsCaption cubre issue #9: el soporte de
// caption se agregó solo al parser YAML (strict mode); parseMarkdownTable no
// tiene sintaxis de caption. Esta prueba fija esa asimetría documentada para
// detectar si una futura implementación agrega caption al modo markdown sin
// que sea intencional.
func TestTableParser_MarkdownMode_NeverSetsCaption(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Header A | Header B |",
			"|---|---|",
			"| val1 | val2 |",
		},
	}

	result := parser.Parse(ctx, 0)
	table := result.Element.(*ast.TableElement)

	if table.Caption != "" {
		t.Errorf("Caption = %q, want empty: markdown-mode tables have no caption syntax", table.Caption)
	}
}

// TestTableParser_ParseMarkdownTable_SerializesAsEmptyArrays cubre issue #8:
// headers/rows deben serializar como [] (no JSON null) incluso cuando quedan
// vacíos, round-tripping por json.Marshal (no solo inspeccionando el slice Go).
func TestTableParser_ParseMarkdownTable_SerializesAsEmptyArrays(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Header A | Header B |",
			"|---|---|",
			"| val1 | val2 |",
		},
	}

	result := parser.Parse(ctx, 0)
	table := result.Element.(*ast.TableElement)

	data, err := json.Marshal(table)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if decoded["headers"] == nil {
		t.Errorf("serialized headers is null, want a non-null array: %s", data)
	}
	if decoded["rows"] == nil {
		t.Errorf("serialized rows is null, want a non-null array: %s", data)
	}
}

// TestTableParser_ExplicitCells_SameIndentSequence covers a fix to issue
// #20's "cells:" parsing: idiomatic YAML writes a block sequence at the
// SAME indentation as its mapping key, not indented further
// (`cells:\n  - [...]`), which the original implementation treated as the
// end of the cells: block (indent <= cellsIndent), silently collecting zero
// lines and leaving the table empty. A "-"-prefixed line at the same
// indentation as "cells:" must still be treated as part of the block.
func TestTableParser_ExplicitCells_SameIndentSequence(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"TABLE",
			"  cells:",
			"  - [{content: A, header: true}, {content: B, header: true}]",
			"  - [{content: 1}, {content: 2}]",
		},
	}

	result := parser.Parse(ctx, 0)
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is not TableElement: %+v", result.Element)
	}

	if len(table.Cells) != 2 {
		t.Fatalf("len(Cells) = %d, want 2 (same-indent block sequence must still be parsed)", len(table.Cells))
	}
	if got := table.Cells[0][0]; !got.IsHeader || got.Content != "A" {
		t.Errorf("Cells[0][0] = %+v, want {Content:A IsHeader:true}", got)
	}
}

// TestTableParser_ExplicitCells_MalformedYAML_EmitsDiagnostic covers the
// silent-failure fix: a "cells:" block that isn't valid YAML must surface a
// Warning diagnostic (TABLE004) instead of silently leaving the table empty
// with no signal to the author about what went wrong.
func TestTableParser_ExplicitCells_MalformedYAML_EmitsDiagnostic(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"TABLE",
			"  cells:",
			"    - [{content: A, header: true", // unterminated flow mapping/sequence
		},
	}

	result := parser.Parse(ctx, 0)
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is not TableElement: %+v", result.Element)
	}
	if len(table.Cells) != 0 {
		t.Errorf("Cells = %+v, want empty for malformed YAML", table.Cells)
	}

	if len(result.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic for malformed \"cells:\" YAML, got none")
	}
	found := false
	for _, d := range result.Diagnostics {
		if d.RuleID == "TABLE004" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a TABLE004 diagnostic, got: %+v", result.Diagnostics)
	}
}

// TestTableParser_ExplicitCells_HugeSpanIsClampedNotExpanded covers the DoS
// fix (ast.MaxCellSpan): a declared colspan/rowspan far beyond any real
// table must be clamped rather than expanded verbatim — expanding it
// verbatim in ast.FlattenCellsToRows would allocate a slice with that many
// entries at PARSE time, from a few bytes of YAML. A Warning diagnostic
// (TABLE005) must report that the value was clamped.
func TestTableParser_ExplicitCells_HugeSpanIsClampedNotExpanded(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"TABLE",
			"  cells:",
			"    - [{content: A, header: true, colspan: 999999999}]",
		},
	}

	result := parser.Parse(ctx, 0)
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is not TableElement: %+v", result.Element)
	}

	if got := table.Cells[0][0].ColSpan; got != ast.MaxCellSpan {
		t.Errorf("Cells[0][0].ColSpan = %d, want clamped to ast.MaxCellSpan (%d)", got, ast.MaxCellSpan)
	}
	if len(table.Headers) != ast.MaxCellSpan {
		t.Errorf("len(Headers) = %d, want %d (clamped, not %d)", len(table.Headers), ast.MaxCellSpan, 999999999)
	}

	found := false
	for _, d := range result.Diagnostics {
		if d.RuleID == "TABLE005" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a TABLE005 diagnostic for the clamped span, got: %+v", result.Diagnostics)
	}
}

// TestTableParser_ExplicitCells_InvalidScopeIsClearedWithDiagnostic covers a
// code-review finding: an invalid scope value (anything other than the
// HTML allowlist "row"/"col") used to round-trip into the JSON contract
// unchanged while renderer.writeTableCellRow silently never emitted it (its
// own fixed allowlist just skips the attribute) — a typo like "column"
// looked "saved" in --format json but the accessibility structure it
// declared never reached the rendered HTML, with no diagnostic anywhere.
// It must now be cleared to "" at parse time and reported via TABLE006.
func TestTableParser_ExplicitCells_InvalidScopeIsClearedWithDiagnostic(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"TABLE",
			"  cells:",
			"    - [{content: A, header: true, scope: column}]",
		},
	}

	result := parser.Parse(ctx, 0)
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is not TableElement: %+v", result.Element)
	}

	if got := table.Cells[0][0].Scope; got != "" {
		t.Errorf("Cells[0][0].Scope = %q, want cleared to \"\"", got)
	}

	found := false
	for _, d := range result.Diagnostics {
		if d.RuleID == "TABLE006" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a TABLE006 diagnostic for the invalid scope, got: %+v", result.Diagnostics)
	}
}

// TestTableParser_ExplicitCells_ValidScopesNotFlagged is the negative
// counterpart: "row"/"col"/unset must NOT trigger TABLE006.
func TestTableParser_ExplicitCells_ValidScopesNotFlagged(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"TABLE",
			"  cells:",
			"    - [{content: A, header: true, scope: row}, {content: B, header: true, scope: col}, {content: C}]",
		},
	}

	result := parser.Parse(ctx, 0)
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is not TableElement: %+v", result.Element)
	}
	if table.Cells[0][0].Scope != "row" || table.Cells[0][1].Scope != "col" || table.Cells[0][2].Scope != "" {
		t.Fatalf("valid scopes were altered: %+v", table.Cells[0])
	}
	for _, d := range result.Diagnostics {
		if d.RuleID == "TABLE006" {
			t.Errorf("unexpected TABLE006 diagnostic for valid scopes: %+v", result.Diagnostics)
		}
	}
}

// TestTableParser_QuotedCellWithComma cubre el bug de fondo: el splitter de
// headers:/rows: partía por comas sin mirar las comillas, así que una coma
// DENTRO de una celda entrecomillada ("Manual, dual-approval", "1,000",
// "Berlin, Germany" — contenido muy común) generaba una celda de más. El
// síntoma que veía el autor era un error del linter sobre el número de
// columnas (TABLE003), que culpaba a la fila en vez del contenido de la celda.
func TestTableParser_QuotedCellWithComma(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"TABLE",
			`  headers: ["Control", "Aprobación, tipo"]`,
			"  rows:",
			`      ["x", "Manual, dual-approval"]`,
			`      ["1,000", "Berlin, Germany"]`,
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is not TableElement: %+v", result.Element)
	}

	wantHeaders := []string{"Control", "Aprobación, tipo"}
	if !equalStrings(table.Headers, wantHeaders) {
		t.Errorf("Headers = %#v, want %#v", table.Headers, wantHeaders)
	}

	wantRows := [][]string{
		{"x", "Manual, dual-approval"},
		{"1,000", "Berlin, Germany"},
	}
	if len(table.Rows) != len(wantRows) {
		t.Fatalf("len(Rows) = %d, want %d (%#v)", len(table.Rows), len(wantRows), table.Rows)
	}
	for i := range wantRows {
		if !equalStrings(table.Rows[i], wantRows[i]) {
			t.Errorf("Rows[%d] = %#v, want %#v", i, table.Rows[i], wantRows[i])
		}
	}
}

// TestSplitInlineArray cubre el splitter directamente, incluidos los casos
// borde que el comentario de splitInlineArray declara explícitamente: escape
// de comillas, comillas desbalanceadas (fallback legacy), array vacío y
// contenido sin comillas.
func TestSplitInlineArray(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"celda entrecomillada con coma", `"x", "Manual, dual-approval"`, []string{"x", "Manual, dual-approval"}},
		{"varias comas en una celda", `"a, b, c", "d"`, []string{"a, b, c", "d"}},
		{"miles con coma", `"1,000", "2,500"`, []string{"1,000", "2,500"}},
		{"sin comillas (legacy)", `A, B, C`, []string{"A", "B", "C"}},
		{"numérico sin comillas", `100, 200`, []string{"100", "200"}},
		{"mezcla entrecomillada y no", `A, "B, C", 3`, []string{"A", "B, C", "3"}},
		// El backslash NO es escape (ver splitInlineArray): una celda
		// terminada en `\` conserva su comilla de cierre en vez de comérsela,
		// que es lo que hacía la primera versión de este splitter — dejaba las
		// comillas desbalanceadas, caía al fallback y reproducía el bug de las
		// tres celdas justo en la fila que tuviera una ruta de Windows.
		{"celda terminada en backslash no se come la comilla", `"Berlin, Germany", "C:\"`, []string{"Berlin, Germany", `C:\`}},
		{"backslash literal pasa verbatim", `"C:\ruta, x"`, []string{`C:\ruta, x`}},
		// Una comilla doble literal dentro de una celda sigue sin ser
		// representable en esta forma (limitación pre-existente del dialecto,
		// ver checkQuotable en formatter/util.go): `\"` no se desescapa, se
		// conserva tal cual.
		{"backslash-comilla no se interpreta como escape", `"dijo \"hola\", y se fue", "B"`, []string{`dijo \"hola\", y se fue`, "B"}},
		{"celda vacía entrecomillada", `"", "B"`, []string{"", "B"}},
		{"corchete dentro de la celda", `"[borrador]", "B"`, []string{"[borrador]", "B"}},
		{"array vacío no produce celda fantasma", ``, []string{}},
		{"comillas desbalanceadas cae a legacy", `A", B`, []string{"A", "B"}},
		{"coma final conserva la celda vacía (legacy)", `"A",`, []string{"A", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitInlineArray(tt.input)
			if !equalStrings(got, tt.want) {
				t.Errorf("splitInlineArray(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

// TestTableParser_EmptyHeadersArray fija la decisión deliberada de que
// `headers: []` produzca CERO headers en vez del header vacío que devolvía
// strings.Split("", ",") — ese header fantasma falseaba el conteo de columnas
// del linter.
func TestTableParser_EmptyHeadersArray(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "strict",
		Lines: []string{
			"TABLE",
			"  headers: []",
			"  rows:",
			`      ["a", "b"]`,
		},
	}

	result := parser.Parse(ctx, 0)
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is not TableElement: %+v", result.Element)
	}
	if len(table.Headers) != 0 {
		t.Errorf("len(Headers) = %d, want 0 (%#v)", len(table.Headers), table.Headers)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestTableParser_ParseMarkdownTable_StopsOnProseWithInternalPipe covers
// issue #191: parseMarkdownTable used to stop only on lines with zero "|"
// anywhere, so ordinary prose right after a table that happened to contain
// a "|" not at the start (e.g. "Total anual | 2024") got absorbed as a
// garbage extra row instead of ending the table. The fix requires the same
// leading "|" that TableParser.CanParse/IsNewElement/strict mode's own
// table continuation already require.
func TestTableParser_ParseMarkdownTable_StopsOnProseWithInternalPipe(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Header A | Header B |",
			"|---|---|",
			"| 1 | 2 |",
			"Total anual | 2024",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatal("Element is not TableElement")
	}

	if len(table.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1 (la línea de prosa no debe absorberse como fila)", len(table.Rows))
	}
	if result.ConsumedLines != 3 {
		t.Errorf("ConsumedLines = %d, want 3 (header + separador + 1 fila, sin la línea de prosa)", result.ConsumedLines)
	}
}

// TestTableParser_ParseMarkdownTable_MultiRowStillConsumesAll es la
// no-regresión de issue #191: una tabla markdown canónica multi-fila, donde
// cada fila real abre con "|", debe seguir consumiéndose exactamente igual
// que antes del fix.
func TestTableParser_ParseMarkdownTable_MultiRowStillConsumesAll(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Header A | Header B |",
			"|---|---|",
			"| val1 | val2 |",
			"| val3 | val4 |",
			"| val5 | val6 |",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}

	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatal("Element is not TableElement")
	}

	if len(table.Rows) != 3 {
		t.Fatalf("len(Rows) = %d, want 3", len(table.Rows))
	}
	if result.ConsumedLines != 5 {
		t.Errorf("ConsumedLines = %d, want 5", result.ConsumedLines)
	}
}

// TestTableParser_ParseMarkdownTable_CodeSpanWithPipe cubre F10 del audit
// 2026-09-11: una celda con un code span que contiene "|" ("`User | null`")
// se partía en 2 celdas de más — strings.Split(line, "|") no distinguía un
// "|" real de uno dentro de backticks — así que una fila con menos columnas
// de las que tiene salía marcada por TABLE003 (linter/rules.go) contra una
// tabla que en realidad está bien formada.
func TestTableParser_ParseMarkdownTable_CodeSpanWithPipe(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| Method | Return |",
			"|---|---|",
			"| `getUser()` | `User | null` |",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatal("Element is not TableElement")
	}

	if len(table.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(table.Rows))
	}
	row := table.Rows[0]
	if len(row) != 2 {
		t.Fatalf("row = %#v, want 2 celdas (el \"|\" del code span no debe partir la fila)", row)
	}
	if row[1] != "`User | null`" {
		t.Errorf("row[1] = %q, want \"`User | null`\" (pipe intacto dentro del code span)", row[1])
	}
}

// TestTableParser_ParseMarkdownTable_EscapedPipe cubre la otra mitad de
// F10: un "\|" fuera de un code span es un pipe escapado, no un separador —
// se decodifica a "|" literal en la celda, la misma convención de GFM.
func TestTableParser_ParseMarkdownTable_EscapedPipe(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| A | B |",
			"|---|---|",
			`| x \| y | z |`,
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	table := result.Element.(*ast.TableElement)
	if len(table.Rows) != 1 || len(table.Rows[0]) != 2 {
		t.Fatalf("Rows = %#v, want 1 fila de 2 celdas", table.Rows)
	}
	if table.Rows[0][0] != "x | y" {
		t.Errorf("Rows[0][0] = %q, want \"x | y\"", table.Rows[0][0])
	}
}

// TestTableParser_ParseMarkdownTable_RowPositions cubre F10: RowPositions
// tiene que ser paralelo a Rows y apuntar a la línea REAL de cada fila
// (no a la del inicio de la tabla), para que TABLE003 pueda señalar la fila
// que tiene el problema.
func TestTableParser_ParseMarkdownTable_RowPositions(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| A | B |",
			"|---|---|",
			"| 1 | 2 |",
			"| 3 | 4 |",
		},
	}

	result := parser.Parse(ctx, 0)
	table := result.Element.(*ast.TableElement)

	if len(table.RowPositions) != 2 {
		t.Fatalf("len(RowPositions) = %d, want 2", len(table.RowPositions))
	}
	if table.RowPositions[0].Line != 3 {
		t.Errorf("RowPositions[0].Line = %d, want 3 (la línea de \"| 1 | 2 |\")", table.RowPositions[0].Line)
	}
	if table.RowPositions[1].Line != 4 {
		t.Errorf("RowPositions[1].Line = %d, want 4 (la línea de \"| 3 | 4 |\")", table.RowPositions[1].Line)
	}
}

// TestTableParser_ParseMarkdownTable_OddBacktickCount_StillSplits cubre un
// hallazgo de revisión: un "`" suelto (una corrida de largo 1 sin otra
// corrida del mismo largo en el resto de la línea que la cierre) no abre
// ningún code span real — codeSpanRanges no lo marca como delimitador, así
// que los "|" reales que vienen después siguen siendo separadores.
func TestTableParser_ParseMarkdownTable_OddBacktickCount_StillSplits(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| A | B |",
			"|---|---|",
			"| use ` for code | see docs | z |",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	table := result.Element.(*ast.TableElement)
	// 3 pipes reales tras el backtick suelto -> 3 celdas de datos, no 1.
	if len(table.Rows) != 1 || len(table.Rows[0]) != 3 {
		t.Fatalf("Rows = %#v, want 1 fila de 3 celdas", table.Rows)
	}
}

// TestTableParser_ParseMarkdownTable_DoubleBacktickSpanWithPipe cubre un
// hallazgo de segunda ronda de revisión: un code span delimitado por una
// corrida de dos o más backticks seguidos (la forma CommonMark para meter
// un backtick literal adentro) se rompía porque la versión anterior
// alternaba "dentro de código" por cada CARÁCTER backtick suelto, no por
// corrida — dos backticks consecutivos se leían como "abre, cierra" en vez
// de "abre un delimitador de largo 2", así que el "|" que en realidad está
// DENTRO del span se trataba como separador real. El fixture de este test
// (sin ningún backtick suelto adentro) ya alcanza para reproducirlo: 4
// backticks en total, pero agrupados en dos corridas de 2, no en 4 corridas
// de 1.
func TestTableParser_ParseMarkdownTable_DoubleBacktickSpanWithPipe(t *testing.T) {
	parser := &TableParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"| A | B |",
			"|---|---|",
			"| x | ``a|b`` |",
		},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	table := result.Element.(*ast.TableElement)
	if len(table.Rows) != 1 || len(table.Rows[0]) != 2 {
		t.Fatalf("Rows = %#v, want 1 fila de 2 celdas (el \"|\" adentro del span de 2 backticks no debe partir la fila)", table.Rows)
	}
	if table.Rows[0][1] != "``a|b``" {
		t.Errorf("Rows[0][1] = %q, want \"``a|b``\" (el span completo, con el pipe adentro intacto)", table.Rows[0][1])
	}
}

// TestSplitMarkdownTableRow_BackslashParity cubre un hallazgo de tercera
// ronda de revisión: el split anterior sólo miraba UN backslash hacia
// atrás de un "|", así que "\\|" (un backslash escapado por otro backslash,
// seguido de un pipe real y sin escapar) se leía igual que "\|" (un
// backslash escapando al pipe) — el segundo backslash de la corrida
// "veía" el pipe siguiente sin saber que él mismo ya estaba consumido por
// el anterior. Un backslash escapa al "|" sólo si la corrida de
// backslashes justo antes tiene largo IMPAR.
//
// Los backslashes que SOBREVIVEN como contenido literal se aparean de a
// DOS (CommonMark §2.4: cada backslash escapa al que sigue; un par de
// backslashes colapsa a UNO literal) — hallazgo de cuarta ronda de
// revisión sobre el fix de tercera ronda: la versión anterior determinaba
// bien la paridad (split o no split) pero escribía los backslashes
// literales SIN aparear (una corrida de N escritos como N-1 sueltos en vez
// de (N-1)/2 apareados), así que "\\\|" reconstruía DOS backslashes en vez
// de uno — reventando el round-trip de escapeTableCellPipe (ver
// table_pipe_escape_test.go) apenas la celda ya traía un backslash antes
// del "|".
func TestSplitMarkdownTableRow_BackslashParity(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{
			name: "un backslash escapa el pipe (impar), cero backslashes sobreviven",
			line: `a\|b`,
			want: []string{"a|b"},
		},
		{
			name: "dos backslashes se aparean en uno solo, el pipe separa (par)",
			line: `a\\|b`,
			want: []string{`a\`, "b"},
		},
		{
			name: "tres backslashes: el primer par colapsa a uno, el tercero escapa (impar)",
			line: `a\\\|b`,
			want: []string{`a\|b`},
		},
		{
			name: "cinco backslashes: dos pares colapsan a dos, el quinto escapa (impar)",
			line: `a\\\\\|b`,
			want: []string{`a\\|b`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitMarkdownTableRow(tt.line)
			if len(got) != len(tt.want) {
				t.Fatalf("SplitMarkdownTableRow(%q) = %#v, want %#v", tt.line, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SplitMarkdownTableRow(%q)[%d] = %q, want %q", tt.line, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestSplitMarkdownTableRow_BackslashInsideCodeSpanIsLiteral cubre un
// hallazgo de quinta ronda de revisión: la rama de backslashes corría
// SIEMPRE, sin mirar inSpan, así que una celda como "`a\|b`" (un code
// span de un solo backtick que ya protege el "|" de adentro como no
// separador) perdía igual el backslash — la lógica de apareo/escape de
// pipes lo trataba como si estuviera a nivel de texto. CommonMark §6.1
// es explícito: "backslash escapes do not work in code spans" — el
// contenido entre backticks se preserva BYTE A BYTE, sin importar cuántos
// backslashes consecutivos traiga ni si terminan justo antes de un "|".
// Esto también cierra el round-trip de fmt: el formatter (ver
// escapeTableCellPipe/table_pipe_escape_test.go) ya deja intacto un "|"
// dentro de un span; si el parser desescapaba el backslash del contenido,
// parse→format→reparse no era la identidad.
func TestSplitMarkdownTableRow_BackslashInsideCodeSpanIsLiteral(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{
			name: "un backslash antes del pipe, DENTRO de un span de 1 backtick: sobrevive",
			line: "`a\\|b`",
			want: []string{"`a\\|b`"},
		},
		{
			name: "dos backslashes (corrida par) antes del pipe, dentro de un span: sobreviven los dos, no se aparean",
			line: "`a\\\\|b`",
			want: []string{"`a\\\\|b`"},
		},
		{
			name: "tres backslashes (corrida impar) antes del pipe, dentro de un span: sobreviven los tres",
			line: "`a\\\\\\|b`",
			want: []string{"`a\\\\\\|b`"},
		},
		{
			name: "el mismo contenido FUERA de un span sí aplica el apareo/escape (sanity check, no debe romperse)",
			line: `a\|b`,
			want: []string{"a|b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitMarkdownTableRow(tt.line)
			if len(got) != len(tt.want) {
				t.Fatalf("SplitMarkdownTableRow(%q) = %#v, want %#v", tt.line, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SplitMarkdownTableRow(%q)[%d] = %q, want %q", tt.line, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestCodeSpanRanges_EscapedBacktickDoesNotOpenSpan cubre un hallazgo de
// tercera ronda de revisión, con precedente exacto en el spec de
// CommonMark (sección "Backslash escapes": "\`not code`" se renderiza como
// el texto literal "`not code`", nunca como <code>): un backtick escapado
// (precedido por un backslash) no puede abrir un code span. Antes de este
// fix, un backtick escapado emparejaba con el siguiente backtick real,
// tratando como código un "|" que en realidad debía seguir siendo
// separador de columna.
func TestCodeSpanRanges_EscapedBacktickDoesNotOpenSpan(t *testing.T) {
	line := "a\\`code|pipe`z"
	got := SplitMarkdownTableRow(line)
	want := []string{"a\\`code", "pipe`z"}
	if len(got) != len(want) {
		t.Fatalf("SplitMarkdownTableRow(%q) = %#v, want %#v (el backtick escapado no debe abrir un span, así que el \"|\" sigue siendo separador)", line, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("SplitMarkdownTableRow(%q)[%d] = %q, want %q", line, i, got[i], want[i])
		}
	}
}
