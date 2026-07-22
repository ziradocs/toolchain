// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
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
