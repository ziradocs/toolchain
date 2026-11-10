// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"sort"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

// ReportableParserDiagnostics son los diagnósticos del parser que van al
// reporte de lint (--report) junto con los del linter: todos menos la
// telemetría del normalizador y del detector (info sin RuleID ni Code), que
// sigue solo en la consola.
func ReportableParserDiagnostics(diags []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	out := make([]diagnostics.Diagnostic, 0, len(diags))
	for _, d := range diags {
		if d.Severity == diagnostics.Info && d.RuleID == "" && d.Code == "" {
			continue
		}
		out = append(out, d)
	}
	return out
}

// SortByPosition ordena diags por línea y columna, estable.
func SortByPosition(diags []diagnostics.Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i].Position, diags[j].Position
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
}

// SortWaivedByPosition es SortByPosition para los diagnósticos suprimidos.
func SortWaivedByPosition(waived []linter.WaivedDiagnostic) {
	sort.SliceStable(waived, func(i, j int) bool {
		a, b := waived[i].Diagnostic.Position, waived[j].Diagnostic.Position
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
}
