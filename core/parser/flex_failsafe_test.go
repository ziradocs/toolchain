// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// findFLEX001 busca el primer diagnóstico FLEX001 en diags, o nil si no hay
// ninguno.
func findFLEX001(diags []diagnostics.Diagnostic) *diagnostics.Diagnostic {
	for i := range diags {
		if diags[i].RuleID == "FLEX001" {
			return &diags[i]
		}
	}
	return nil
}

// TestFlexParser_Failsafe_WarnsOnUnrecognizedLine es la regresión de issue
// #192: antes, una línea que ningún ElementParser reclamaba (Element==nil,
// ConsumedLines==0) desaparecía en silencio del documento — sin error, sin
// warning, --lint-only reportaba éxito. Ahora el failsafe emite un
// diagnóstico Warning con RuleID FLEX001. Severidad Warning (no Error) a
// propósito: un Error rompería cualquier .slidelang existente que ya
// dependiera del descarte silencioso, mismo argumento que STRICT003 (#70).
func TestFlexParser_Failsafe_WarnsOnUnrecognizedLine(t *testing.T) {
	content := "# Title\n\n<<charrt>>\n"
	parser := NewFlexParser(content, util.NewNoop())
	_, diags := parser.Parse()

	d := findFLEX001(diags)
	if d == nil {
		t.Fatalf("expected a FLEX001 diagnostic for an unrecognized <<charrt>> tag, got none: %+v", diags)
	}
	if !d.IsWarning() {
		t.Errorf("severity = %v, want Warning", d.Severity)
	}
}

// TestDocumentFlexParser_Failsafe_WarnsOnUnrecognizedLine es el equivalente
// del test anterior para el dialecto documental (document_flex.go), cuyo
// failsafe era el más silencioso de los dos: las dos ramas exitosas del
// dispatch hacen `continue` antes de llegar a este camino, así que ni
// siquiera miraba result.Error/result.Diagnostics.
func TestDocumentFlexParser_Failsafe_WarnsOnUnrecognizedLine(t *testing.T) {
	content := "# Title\n\n<<charrt>>\n"
	parser := NewDocumentFlexParser(content, util.NewNoop())
	_, diags := parser.Parse()

	d := findFLEX001(diags)
	if d == nil {
		t.Fatalf("expected a FLEX001 diagnostic for an unrecognized <<charrt>> tag, got none: %+v", diags)
	}
	if !d.IsWarning() {
		t.Errorf("severity = %v, want Warning", d.Severity)
	}
}

// TestIsFlexFailsafeExempt_KnownResidues cubre los dos habitantes
// confirmados de la zona muerta — derivados corriendo el registro completo
// sobre examples/ (issue #192, paso b/c del plan), no adivinados:
//
//   - "<<end>>": cierre genérico de GRID/COLUMN/chart/map, contrato fijado
//     por TestTextParser_Parse_OrphanEndTagStillDropped.
//   - ":::" (bare): residuo del cierre de un bloque padre cuando un hijo
//     anidado corta la colección de contenido temprano (issue #57).
//
// Cualquier otra forma con la misma pinta superficial (p. ej. un tag
// desconocido "<<charrt>>", o un ":::type" con tipo) NO debe eximirse — el
// punto de FLEX001 es justamente sacarlas a la luz.
func TestIsFlexFailsafeExempt_KnownResidues(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"<<end>> exacto está exento", "<<end>>", true},
		{"::: bare está exento", ":::", true},
		{"<<charrt>> mal escrito NO está exento", "<<charrt>>", false},
		{":::info con tipo NO está exento (SpecialBlockParser sí lo reclama)", ":::info", false},
		{"prosa normal no está exenta", "some prose", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFlexFailsafeExempt(tt.line); got != tt.want {
				t.Errorf("isFlexFailsafeExempt(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// TestFlexParser_Failsafe_ExemptsKnownResidues confirma que las formas
// eximidas efectivamente NO producen FLEX001, en un documento real, no solo
// contra el predicado aislado.
func TestFlexParser_Failsafe_ExemptsKnownResidues(t *testing.T) {
	// <<end>> huérfano, sin <<chart>>/<<map>>/<<grid>> que lo abra — el
	// mismo caso que TestTextParser_Parse_OrphanEndTagStillDropped fija a
	// nivel de TextParser; acá se confirma que tampoco dispara el failsafe.
	content := "# Title\n\n<<end>>\n"
	parser := NewFlexParser(content, util.NewNoop())
	_, diags := parser.Parse()

	if d := findFLEX001(diags); d != nil {
		t.Errorf("expected no FLEX001 for an orphan <<end>>, got: %+v", d)
	}
}

// TestMeasureFLEX001Noise (nota histórica): la medición real que derivó la
// lista de exenciones de arriba corrió el registro completo contra
// examples/ (33 hits iniciales sin exenciones, 0 tras exentar <<end>>/:::
// y sanear el resto del corpus — <</chart>>/<</mermaid>> hacia <<end>>,
// <<info>> hacia :::info, y los bloques <<poll>>/<<quiz>> sin parser hacia
// :::note/:::tip). No queda un test permanente para esto porque examples/
// no es una fixture de este paquete — el guard real es que el corpus no
// emita FLEX001, verificado manualmente antes de este PR y cubierto en
// spirit por TestFormatDocument_RoundTrip_Corpus (formatter) y por
// html-validate.yml en CI.
