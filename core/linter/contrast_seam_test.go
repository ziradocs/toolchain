// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// themeVariablesProbe es una regla mínima que solo registra el mapa que
// recibió vía ThemeAware, para verificar el mecanismo de cableado en sí
// (WithThemeVariables / AddRule), separado de cualquier lógica de
// contraste real (eso lo cubre ThemeContrastRule más abajo).
type themeVariablesProbe struct {
	received map[string]string
	calls    int
}

func (p *themeVariablesProbe) Check(node ast.Node) []diagnostics.Diagnostic { return nil }

func (p *themeVariablesProbe) SetThemeVariables(vars map[string]string) {
	p.received = vars
	p.calls++
}

func newMinimalAST() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	astNode := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	astNode.ContentBlocks = append(astNode.ContentBlocks, *block)
	return astNode
}

// TestLinter_WithThemeVariables_DeliveredToExistingRule cubre el cableado
// de WithThemeVariables sobre una regla ya agregada al Linter.
func TestLinter_WithThemeVariables_DeliveredToExistingRule(t *testing.T) {
	probe := &themeVariablesProbe{}
	l := NewWithRules(probe)

	vars := map[string]string{"--text-color": "#111111", "--bg-color": "#ffffff"}
	l.WithThemeVariables(vars)

	if probe.calls != 1 {
		t.Fatalf("expected SetThemeVariables to be called once, got %d calls", probe.calls)
	}
	if probe.received["--text-color"] != "#111111" {
		t.Errorf("probe did not receive expected theme variables, got %+v", probe.received)
	}
}

// TestLinter_AddRule_AfterWithThemeVariables_StillReceivesMap confirma el
// segundo camino de cableado: una regla agregada DESPUÉS de
// WithThemeVariables también debe recibir el mapa (vía AddRule), no solo
// las reglas presentes al momento de la llamada.
func TestLinter_AddRule_AfterWithThemeVariables_StillReceivesMap(t *testing.T) {
	l := New()
	vars := map[string]string{"--text-color": "#000000", "--bg-color": "#ffffff"}
	l.WithThemeVariables(vars)

	probe := &themeVariablesProbe{}
	l.AddRule(probe)

	if probe.calls != 1 {
		t.Fatalf("expected SetThemeVariables to be called once via AddRule, got %d calls", probe.calls)
	}
	if probe.received["--bg-color"] != "#ffffff" {
		t.Errorf("probe did not receive expected theme variables via AddRule, got %+v", probe.received)
	}
}

// TestLinter_WithThemeVariables_EmptyMapStillInjects es la regresión que
// motivó NO usar "themeVariables != nil" como guardia: un tema puede
// legítimamente resolver a un mapa vacío, y una regla agregada después
// debe seguir recibiendo esa llamada (con un mapa vacío), no ser omitida
// silenciosamente.
func TestLinter_WithThemeVariables_EmptyMapStillInjects(t *testing.T) {
	l := New()
	l.WithThemeVariables(map[string]string{})

	probe := &themeVariablesProbe{}
	l.AddRule(probe)

	if probe.calls != 1 {
		t.Fatalf("expected SetThemeVariables to be called even with an empty map, got %d calls", probe.calls)
	}
}

// TestLinter_AddRule_WithoutWithThemeVariables_NeverCalled confirma el
// no-op: si nunca se llamó WithThemeVariables, AddRule no debe invocar
// SetThemeVariables en absoluto (comportamiento histórico preservado para
// reglas ThemeAware cuando el caller no participa del seam).
func TestLinter_AddRule_WithoutWithThemeVariables_NeverCalled(t *testing.T) {
	l := New()
	probe := &themeVariablesProbe{}
	l.AddRule(probe)

	if probe.calls != 0 {
		t.Fatalf("expected SetThemeVariables to never be called without WithThemeVariables, got %d calls", probe.calls)
	}
}

// TestThemeContrastRule_EndToEnd cierra el loop completo: un Linter real,
// un tema con un par de colores por debajo del umbral AA, cableado vía
// WithThemeVariables, y ThemeContrastRule.Check() disparando CONTRAST001.
func TestThemeContrastRule_EndToEnd(t *testing.T) {
	rule := NewThemeContrastRule([]ContrastPair{
		{Label: "body text on background", FgVariable: "--text-color", BgVariable: "--bg-color"},
	})

	// #777777 sobre #ffffff da ~4.48:1, por debajo del umbral AA de 4.5:1
	// para texto normal.
	vars := map[string]string{"--text-color": "#777777", "--bg-color": "#ffffff"}

	l := NewWithRules(rule).WithThemeVariables(vars)
	diags := l.LintUnfiltered(newMinimalAST())

	found := false
	for _, d := range diags {
		if d.RuleID == "CONTRAST001" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a CONTRAST001 diagnostic for a below-threshold pair, got %+v", diags)
	}
}

// TestThemeContrastRule_MessageNeverContradictsItself pina el hallazgo del
// segundo code review en el punto exacto donde se ve: el ratio #006ffb
// sobre blanco (~4.499888:1) reprueba AA, pero el mensaje ANTES de la
// corrección de FormatRatio mostraba "4.50:1" — indistinguible del umbral
// citado en el mismo mensaje ("4.5:1 required"). El mensaje nunca debe
// mostrar un valor que se lea como igual o mayor al umbral citado cuando
// el diagnóstico dice "below".
func TestThemeContrastRule_MessageNeverContradictsItself(t *testing.T) {
	rule := NewThemeContrastRule([]ContrastPair{
		{Label: "body text on background", FgVariable: "--text-color", BgVariable: "--bg-color"},
	})
	vars := map[string]string{"--text-color": "#006ffb", "--bg-color": "#ffffff"}

	l := NewWithRules(rule).WithThemeVariables(vars)
	diags := l.LintUnfiltered(newMinimalAST())

	var msg string
	for _, d := range diags {
		if d.RuleID == "CONTRAST001" {
			msg = d.Message
		}
	}
	if msg == "" {
		t.Fatalf("expected a CONTRAST001 diagnostic, got %+v", diags)
	}
	if strings.Contains(msg, "4.50:1") {
		t.Errorf("message %q shows the failing ratio as 4.50:1, indistinguishable from the 4.5:1 threshold it cites as unmet", msg)
	}
	if !strings.Contains(msg, "4.49:1") {
		t.Errorf("message %q does not show the expected truncated ratio 4.49:1", msg)
	}
}

// TestThemeContrastRule_PassingPairProducesNoDiagnostic es el contrapunto:
// un par con contraste suficiente no debe producir ningún CONTRAST001.
func TestThemeContrastRule_PassingPairProducesNoDiagnostic(t *testing.T) {
	rule := NewThemeContrastRule([]ContrastPair{
		{Label: "body text on background", FgVariable: "--text-color", BgVariable: "--bg-color"},
	})
	vars := map[string]string{"--text-color": "#000000", "--bg-color": "#ffffff"}

	l := NewWithRules(rule).WithThemeVariables(vars)
	diags := l.LintUnfiltered(newMinimalAST())

	for _, d := range diags {
		if d.RuleID == "CONTRAST001" {
			t.Fatalf("did not expect CONTRAST001 for a passing pair, got %+v", diags)
		}
	}
}

// themeVariablesMutator es una regla ThemeAware que muta el mapa que
// recibió, para probar que WithThemeVariables entrega una copia — no una
// referencia al mapa del caller (que puede ser una entrada process-global
// compartida, p. ej. doclang.EmbeddedThemes o el *Theme.Variables que el
// generador también usa para renderizar).
type themeVariablesMutator struct{ received map[string]string }

func (m *themeVariablesMutator) Check(node ast.Node) []diagnostics.Diagnostic { return nil }

func (m *themeVariablesMutator) SetThemeVariables(vars map[string]string) {
	m.received = vars
	if vars != nil {
		vars["--injected-by-rule"] = "corrupted"
	}
}

// TestLinter_WithThemeVariables_RuleMutationDoesNotAffectCallersMap es la
// regresión para el hallazgo de aislamiento del code review: una regla que
// escribe en el mapa que recibió no debe poder corromper el mapa original
// que el caller (una CLI) pasó a WithThemeVariables — ese mapa puede ser
// una fuente compartida (tema global, o el mismo mapa que alimenta el
// renderer) que un build posterior o el propio render volvería a leer.
func TestLinter_WithThemeVariables_RuleMutationDoesNotAffectCallersMap(t *testing.T) {
	callerMap := map[string]string{"--text-color": "#000000"}

	mutator := &themeVariablesMutator{}
	NewWithRules(mutator).WithThemeVariables(callerMap)

	if _, corrupted := callerMap["--injected-by-rule"]; corrupted {
		t.Fatalf("rule mutation leaked into the caller's original map: %+v", callerMap)
	}
	if _, gotInjected := mutator.received["--injected-by-rule"]; !gotInjected {
		t.Fatalf("expected the rule's own copy to reflect its mutation, got %+v", mutator.received)
	}
}

// TestLinter_WithThemeVariables_RulesDoNotShareTheSameMapInstance es la
// regresión para el hallazgo del segundo code review: WithThemeVariables
// clonaba el mapa del caller UNA sola vez y entregaba esa MISMA instancia
// mutable a cada regla — una regla que mutara lo recibido contaminaba lo
// que veían las reglas siguientes en el mismo lint run (y lo que
// runExternalRulepack serializa para los rulepacks externos). Cada regla
// debe recibir su PROPIA copia independiente.
func TestLinter_WithThemeVariables_RulesDoNotShareTheSameMapInstance(t *testing.T) {
	mutatorA := &themeVariablesMutator{}
	probeB := &themeVariablesProbe{}

	NewWithRules(mutatorA, probeB).WithThemeVariables(map[string]string{"--text-color": "#000000"})

	if _, leaked := probeB.received["--injected-by-rule"]; leaked {
		t.Fatalf("rule A's mutation leaked into rule B's map: %+v", probeB.received)
	}
}

// TestThemeContrastRule_MissingOrUnparseableVariableIsSkipped cubre las dos
// limitaciones aceptadas del seam: una variable ausente del mapa del tema,
// y una variable presente pero con un valor no-hex (gradiente) — ninguna
// de las dos debe producir un diagnóstico fabricado.
func TestThemeContrastRule_MissingOrUnparseableVariableIsSkipped(t *testing.T) {
	tests := []struct {
		name string
		vars map[string]string
	}{
		{"bg variable missing entirely", map[string]string{"--text-color": "#777777"}},
		{"bg variable is a gradient, not hex", map[string]string{
			"--text-color": "#777777",
			"--bg-color":   "linear-gradient(90deg, #fff, #000)",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewThemeContrastRule([]ContrastPair{
				{Label: "body text on background", FgVariable: "--text-color", BgVariable: "--bg-color"},
			})
			l := NewWithRules(rule).WithThemeVariables(tt.vars)
			diags := l.LintUnfiltered(newMinimalAST())
			for _, d := range diags {
				if d.RuleID == "CONTRAST001" {
					t.Fatalf("expected the pair to be skipped silently, got %+v", diags)
				}
			}
		})
	}
}
