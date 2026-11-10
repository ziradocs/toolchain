// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package findings

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

// Active y Waived van por línea y columna, vengan del parser o del linter.
func TestPolicyResult_ActiveAndWaivedAreOrdered(t *testing.T) {
	at := func(line, col int, id string) diagnostics.Diagnostic {
		return diagnostics.NewWarning(id, diagnostics.NewPosition(line, col), "t").WithRuleID(id)
	}
	r := PolicyResult{
		ParserActive: []diagnostics.Diagnostic{at(9, 1, "P9"), at(2, 5, "P2")},
		LintActive:   []diagnostics.Diagnostic{at(2, 1, "L2"), at(5, 1, "L5")},
		ParserWaived: []linter.WaivedDiagnostic{{Diagnostic: at(7, 1, "PW7")}},
		LintWaived:   []linter.WaivedDiagnostic{{Diagnostic: at(1, 1, "LW1")}},
	}
	var active, waived []string
	for _, d := range r.Active() {
		active = append(active, d.RuleID)
	}
	for _, w := range r.Waived() {
		waived = append(waived, w.Diagnostic.RuleID)
	}
	if strings.Join(active, ",") != "L2,P2,L5,P9" || strings.Join(waived, ",") != "LW1,PW7" {
		t.Errorf("Active %v, Waived %v", active, waived)
	}
}
