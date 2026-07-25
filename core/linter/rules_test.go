// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/elements"
)

// findDiagnosticByRuleID busca por el campo RuleID (el que usa rules.go vía
// WithRuleID, p. ej. "CODEGROUP002" o "SPECIAL001"). No reutiliza
// findDiagnostic (definido en layout_validation_test.go) porque ese helper
// compara contra el campo Code, que es el que usa layout_validation.go
// (construcción directa de Diagnostic{Code: "LAYOUT..."}) — un campo
// distinto y no poblado por WithRuleID.
func findDiagnosticByRuleID(diags []diagnostics.Diagnostic, ruleID string) *diagnostics.Diagnostic {
	for i := range diags {
		if diags[i].RuleID == ruleID {
			return &diags[i]
		}
	}
	return nil
}

// Issue #174: un ":::code-item{title="..."}" es azúcar de normalizador —el
// parser real (elements/code_group.go) nunca lo entiende como parte de un
// code-group— así que si CodeGroupFormatterRule no lo reescribió antes de
// parsear, cae como un SpecialBlockElement huérfano con BlockType
// `code-item{...}`. ElementStructureRule debe detectarlo y emitir un ERROR
// distintivo (CODEGROUP002), no solo el warning genérico SPECIAL001, para
// que el problema sea visible aunque el fix del normalizador falle o se
// desactive (defense-in-depth).
func TestElementStructureRule_OrphanedCodeItem_EmitsCODEGROUP002(t *testing.T) {
	pos := diagnostics.NewPosition(10, 1)
	block := ast.NewSpecialBlockElement(pos, `code-item{title="a.go"}`, "")
	slide := &ast.ContentBlock{
		Elements: []ast.Element{block},
	}

	diags := (&ElementStructureRule{}).Check(slide)

	diag := findDiagnosticByRuleID(diags, "CODEGROUP002")
	if diag == nil {
		t.Fatalf("se esperaba un diagnóstico CODEGROUP002, obtenidos: %+v", diags)
	}
	if diag.Severity != diagnostics.Error {
		t.Errorf("severity = %v, want %v (Error)", diag.Severity, diagnostics.Error)
	}
	if !strings.Contains(diag.Message, "code-item") {
		t.Errorf("el mensaje debería mencionar 'code-item', obtenido: %s", diag.Message)
	}

	// No debe además emitirse el warning genérico SPECIAL001 para el mismo
	// elemento — un code-item huérfano ya tiene su propio diagnóstico
	// específico, duplicar con SPECIAL001 sería ruido.
	if findDiagnosticByRuleID(diags, "SPECIAL001") != nil {
		t.Errorf("no se esperaba también un SPECIAL001 junto a CODEGROUP002, obtenidos: %+v", diags)
	}
}

// TestElementStructureRule_MergedCells_NoFalsePositive covers issue #20: a
// table with merged cells (colspan on the header) must pass TABLE003 (Error
// severity: "inconsistent column count") without diagnostics, because
// ast.FlattenCellsToRows derives Headers/Rows as a rectangular grid —
// TABLE003 only compares len(row) against len(Headers) with no concept of
// span, so a naive derivation (Headers with 2 entries from the colspan but a
// body row with 3 real cells) would trigger this Error on every merged
// table, a regression this very feature would introduce if not for the
// rectangular derivation.
func TestElementStructureRule_MergedCells_NoFalsePositive(t *testing.T) {
	parser := &elements.TableParser{}
	ctx := &elements.ParseContext{
		Mode: "flex",
		Lines: []string{
			"TABLE",
			"  cells:",
			"    - [{content: A, header: true, colspan: 2}, {content: B, header: true}]",
			"    - [{content: 1}, {content: 2}, {content: 3}]",
		},
	}
	result := parser.Parse(ctx, 0)
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is not TableElement: %+v", result.Element)
	}

	slide := &ast.ContentBlock{Elements: []ast.Element{table}}
	diags := (&ElementStructureRule{}).Check(slide)

	if diag := findDiagnosticByRuleID(diags, "TABLE003"); diag != nil {
		t.Errorf("unexpected TABLE003 false positive on a merged-cell table: %+v (Headers=%v, Rows=%v)",
			diag, table.Headers, table.Rows)
	}
}

// TestElementStructureRule_MergedCells_RowSpan covers a rowspan-only merge
// (no colspan): a cell in row 0 with RowSpan:2 must not desynchronize the
// rectangular derivation, and TABLE003 must not false-positive on it either.
func TestElementStructureRule_MergedCells_RowSpan(t *testing.T) {
	parser := &elements.TableParser{}
	ctx := &elements.ParseContext{
		Mode: "flex",
		Lines: []string{
			"TABLE",
			"  cells:",
			"    - [{content: A, header: true}, {content: B, header: true}]",
			"    - [{content: 1, rowspan: 2}, {content: 2}]",
			"    - [{content: 3}]",
		},
	}
	result := parser.Parse(ctx, 0)
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is not TableElement: %+v", result.Element)
	}

	slide := &ast.ContentBlock{Elements: []ast.Element{table}}
	diags := (&ElementStructureRule{}).Check(slide)

	if diag := findDiagnosticByRuleID(diags, "TABLE003"); diag != nil {
		t.Errorf("unexpected TABLE003 false positive on a rowspan table: %+v (Headers=%v, Rows=%v)",
			diag, table.Headers, table.Rows)
	}
	if len(table.Rows) != 2 || len(table.Rows[1]) != 2 || table.Rows[1][0] != "1" {
		t.Errorf("expected the rowspan carry ('1') in row 1 col 0, got Rows=%v", table.Rows)
	}
}

// TestElementStructureRule_MergedCells_NonHeaderLedFirstRow covers the
// TABLE001 false-positive fix: a `cells:` table whose first row is NOT
// entirely IsHeader (e.g. row-scoped headers embedded in body rows instead
// of a clean top header row) still has header cells — ast.FlattenCellsToRows
// derives Headers as []string{} for this shape (it only populates Headers
// when the ENTIRE first Cells row is IsHeader), so checking len(Headers)
// alone would wrongly report TABLE001 ("should have headers defined") even
// though elem.Cells clearly has header cells.
func TestElementStructureRule_MergedCells_NonHeaderLedFirstRow(t *testing.T) {
	parser := &elements.TableParser{}
	ctx := &elements.ParseContext{
		Mode: "flex",
		Lines: []string{
			"TABLE",
			"  cells:",
			"    - [{content: Region, header: true, scope: row}, {content: 100}, {content: 200}]",
			"    - [{content: Other, header: true, scope: row}, {content: 300}, {content: 400}]",
		},
	}
	result := parser.Parse(ctx, 0)
	table, ok := result.Element.(*ast.TableElement)
	if !ok {
		t.Fatalf("Element is not TableElement: %+v", result.Element)
	}
	if len(table.Headers) != 0 {
		t.Fatalf("test setup assumption broken: expected Headers to be empty for a non-header-led table, got %v", table.Headers)
	}

	slide := &ast.ContentBlock{Elements: []ast.Element{table}}
	diags := (&ElementStructureRule{}).Check(slide)

	if diag := findDiagnosticByRuleID(diags, "TABLE001"); diag != nil {
		t.Errorf("unexpected TABLE001 false positive on a table with row-scoped headers: %+v", diag)
	}
	if diag := findDiagnosticByRuleID(diags, "TABLE002"); diag != nil {
		t.Errorf("unexpected TABLE002 false positive: %+v", diag)
	}
}

// Un bloque especial desconocido que NO parece un code-item huérfano debe
// seguir cayendo en el warning genérico SPECIAL001 de siempre (no debe
// dispararse CODEGROUP002 por error).
func TestElementStructureRule_UnknownSpecialBlock_StillEmitsSPECIAL001(t *testing.T) {
	pos := diagnostics.NewPosition(5, 1)
	block := ast.NewSpecialBlockElement(pos, "dashboard", "")
	slide := &ast.ContentBlock{
		Elements: []ast.Element{block},
	}

	diags := (&ElementStructureRule{}).Check(slide)

	if findDiagnosticByRuleID(diags, "CODEGROUP002") != nil {
		t.Errorf("no se esperaba CODEGROUP002 para un bloque especial que no es un code-item, obtenidos: %+v", diags)
	}
	if findDiagnosticByRuleID(diags, "SPECIAL001") == nil {
		t.Fatalf("se esperaba el warning genérico SPECIAL001, obtenidos: %+v", diags)
	}
}
