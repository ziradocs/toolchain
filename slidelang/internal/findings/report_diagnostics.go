// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package findings

import (
	"sort"
	"time"

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

// PolicyResult son los diagnósticos de un documento después de la política
// de lint, separados por origen. Lo usan `slidelang build` y la tool `lint`
// del MCP, para que los dos digan lo mismo.
type PolicyResult struct {
	// ParserActive y ParserWaived son los avisos del parser que van al
	// reporte (ReportableParserDiagnostics) después de la política.
	ParserActive []diagnostics.Diagnostic
	ParserWaived []linter.WaivedDiagnostic
	// LintActive y LintWaived son los del linter (con los avisos del parser
	// que son de una regla del linter, ver MergeParserFindings).
	LintActive []diagnostics.Diagnostic
	LintWaived []linter.WaivedDiagnostic
}

// EvaluatePolicy aplica policy a los avisos del parser (parseDiags, ya sin
// los parserFindings de SplitParserFindings) y a los del linter (lintDiags).
// Un aviso del parser se apaga o se suprime como uno del linter, y si la
// política le sube la severidad cuenta como error de lint.
func EvaluatePolicy(policy *linter.PolicyConfig, parseDiags, lintDiags []diagnostics.Diagnostic, filePath string, now time.Time) PolicyResult {
	var r PolicyResult
	r.ParserActive, r.ParserWaived = policy.Evaluate(ReportableParserDiagnostics(parseDiags), filePath, now)
	r.LintActive, r.LintWaived = policy.Evaluate(lintDiags, filePath, now)
	return r
}

// Active son los diagnósticos activos del parser y del linter, ordenados por
// posición.
func (r PolicyResult) Active() []diagnostics.Diagnostic {
	out := append(append([]diagnostics.Diagnostic{}, r.ParserActive...), r.LintActive...)
	SortByPosition(out)
	return out
}

// Waived son los suprimidos del parser y del linter, ordenados por posición.
func (r PolicyResult) Waived() []linter.WaivedDiagnostic {
	out := append(append([]linter.WaivedDiagnostic{}, r.ParserWaived...), r.LintWaived...)
	SortWaivedByPosition(out)
	return out
}

// HasErrors dice si queda algún error activo, del parser o del linter.
func (r PolicyResult) HasErrors() bool {
	for _, d := range r.ParserActive {
		if d.IsError() {
			return true
		}
	}
	for _, d := range r.LintActive {
		if d.IsError() {
			return true
		}
	}
	return false
}
