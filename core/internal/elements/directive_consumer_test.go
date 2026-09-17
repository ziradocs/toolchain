// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import "testing"

// Una directiva se acepta solo si hay un consumidor real. Estos tests cubren
// la frontera del parser: las grafías retiradas y las desconocidas no vuelven
// a llegar al renderer genérico sin avisar (issue #341).
func TestDirectiveParser_DiagnosesDirectivesWithoutConsumer(t *testing.T) {
	tests := []struct {
		line     string
		wantRule string
	}{
		{"@reveal: fade-in", "DIRECTIVE001"},
		{"@layout: content", "DIRECTIVE002"},
		{"@inventada valor", "DIRECTIVE003"},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := (&DirectiveParser{}).Parse(&ParseContext{Lines: []string{tt.line}}, 0)
			if len(result.Diagnostics) != 1 {
				t.Fatalf("diagnostics = %#v, se esperaba exactamente uno", result.Diagnostics)
			}
			if got := result.Diagnostics[0].RuleID; got != tt.wantRule {
				t.Errorf("RuleID = %q, se esperaba %q", got, tt.wantRule)
			}
		})
	}
}

func TestDirectiveParser_KnownDirectiveDoesNotDiagnoseMissingConsumer(t *testing.T) {
	result := (&DirectiveParser{}).Parse(&ParseContext{Lines: []string{"@timer 30"}}, 0)
	if len(result.Diagnostics) != 0 {
		t.Errorf("@timer produjo diagnostics inesperados: %#v", result.Diagnostics)
	}
}
