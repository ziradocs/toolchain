// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

// El resumen de build cuenta como warnings solo los warnings del parser que
// quedaron activos; la telemetría (info) no suma, y un aviso al que la
// política subió a error cuenta como error.
func TestCountParserSummary(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	active := []diagnostics.Diagnostic{
		diagnostics.NewWarning("línea no reconocida", pos, "parser").WithRuleID("STRICT003"),
		diagnostics.NewWarning("otra", pos, "parser").WithRuleID("TABLE007"),
		{Severity: diagnostics.Info, Message: "Normalización aplicada", Position: pos, Source: "normalizer"},
		{Severity: diagnostics.Info, Message: "info con regla", Position: pos, Source: "parser", RuleID: "X001"},
		{Severity: diagnostics.Error, Message: "subido por la política", Position: pos, Source: "parser", RuleID: "SPECIAL002"},
	}
	warnings, errors := countParserSummary(active)
	if warnings != 2 || errors != 1 {
		t.Errorf("countParserSummary = (%d warnings, %d errors), want (2, 1)", warnings, errors)
	}
	if w, e := countParserSummary(nil); w != 0 || e != 0 {
		t.Errorf("countParserSummary(nil) = (%d, %d), want (0, 0)", w, e)
	}
}
