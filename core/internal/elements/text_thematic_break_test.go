// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import "testing"

func TestIsThematicBreak(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"***", true},
		{"___", true},
		{"* * *", true},
		{"_ _ _", true},
		{"*****", true},
		{"\t*\t*\t*", true},
		{"**", false},        // menos de tres
		{"__", false},        // menos de tres
		{"*-*", false},       // mezcla de marcadores
		{"*_*", false},       // mezcla de marcadores
		{"---", false},       // el separador de bloque del dialecto, fuera a propósito
		{"- - -", false},     // la reclama PointsParser, fuera a propósito
		{"*** texto", false}, // no está sola en la línea
		{"* item", false},    // una viñeta de verdad
		{"", false},          // línea vacía
	}

	for _, tt := range tests {
		if got := isThematicBreak(tt.line); got != tt.want {
			t.Errorf("isThematicBreak(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

// Un thematic break que abre el párrafo se consume y no produce elemento:
// el loop del dialecto avanza sin emitir nada y sin FLEX001.
func TestTextParser_ThematicBreakAloneIsDropped(t *testing.T) {
	result := (&TextParser{}).Parse(&ParseContext{Lines: []string{"***", "After."}, Mode: "flex"}, 0)

	if result.Element != nil {
		t.Errorf("Element = %v, want nil — el thematic break se descarta", result.Element)
	}
	if result.ConsumedLines != 1 {
		t.Errorf("ConsumedLines = %d, want 1 — la línea se consume para que el loop no la reporte como no reconocida", result.ConsumedLines)
	}
}

// A media prosa corta SIN consumir: la siguiente vuelta lo descarta.
func TestTextParser_ThematicBreakMidParagraphEndsIt(t *testing.T) {
	lines := []string{"Before.", "***", "After."}

	result := (&TextParser{}).Parse(&ParseContext{Lines: lines, Mode: "flex"}, 0)

	if result.ConsumedLines != 1 {
		t.Fatalf("ConsumedLines = %d, want 1 — solo %q", result.ConsumedLines, lines[0])
	}
	el := result.Element
	if el == nil {
		t.Fatal("Element = nil, want el párrafo previo al break")
	}
}
