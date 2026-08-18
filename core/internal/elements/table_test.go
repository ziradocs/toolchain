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
