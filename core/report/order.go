// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"sort"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

// finding es un diagnóstico del reporte con su waiver, si lo suprimió uno.
type finding struct {
	diag   diagnostics.Diagnostic
	waived bool
	policy *linter.RulePolicy
}

// orderedFindings junta activos y suprimidos en un solo orden, por archivo,
// línea y columna: un hallazgo suprimido va en su lugar del documento, no al
// final. Primero los del documento (Diagnostic.File vacío) y después los de
// cada archivo incluido, por nombre. Entre dos de la misma posición manda el
// orden de llegada (activos primero).
func orderedFindings(active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic) []finding {
	out := make([]finding, 0, len(active)+len(waived))
	for _, d := range active {
		out = append(out, finding{diag: d})
	}
	for _, w := range waived {
		out = append(out, finding{diag: w.Diagnostic, waived: true, policy: w.Policy})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if fa, fb := out[i].diag.File, out[j].diag.File; fa != fb {
			return fa < fb // "" (el documento) va primero
		}
		a, b := out[i].diag.Position, out[j].diag.Position
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
	return out
}
