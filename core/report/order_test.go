// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

// Los hallazgos del reporte (JSON y SARIF) van por línea, con los suprimidos
// en su lugar y no al final.
func TestReport_FindingsOrderedByLineWithWaived(t *testing.T) {
	at := func(line int, id string) diagnostics.Diagnostic {
		return diagnostics.NewWarning(id, diagnostics.NewPosition(line, 1), "t").WithRuleID(id)
	}
	active := []diagnostics.Diagnostic{at(5, "A5"), at(1, "A1")}
	waived := []linter.WaivedDiagnostic{{Diagnostic: at(3, "W3"), Policy: &linter.RulePolicy{Reason: "r"}}}

	data, err := generateJSON(active, waived, "d.doclang", "1", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var rep ReportJSON
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range rep.Findings {
		got = append(got, f.RuleID)
	}
	if len(got) != 3 || got[0] != "A1" || got[1] != "W3" || got[2] != "A5" || !rep.Findings[1].Waived || rep.Findings[0].Waived {
		t.Errorf("JSON findings %v (waived %v %v %v), want A1, W3 (waived), A5", got, rep.Findings[0].Waived, rep.Findings[1].Waived, rep.Findings[2].Waived)
	}

	data, err = generateSARIF(active, waived, "d.doclang", nil)
	if err != nil {
		t.Fatal(err)
	}
	var sarif SARIF
	if err := json.Unmarshal(data, &sarif); err != nil {
		t.Fatal(err)
	}
	res := sarif.Runs[0].Results
	if len(res) != 3 || res[0].RuleId != "A1" || res[1].RuleId != "W3" || res[2].RuleId != "A5" || len(res[1].Suppressions) == 0 || len(res[0].Suppressions) != 0 {
		t.Errorf("SARIF results out of order or suppression misplaced: %+v", res)
	}
}

// Con archivos incluidos (Diagnostic.File), primero va el documento y después
// cada archivo por nombre; dentro de cada uno, por línea.
func TestReport_FindingsOrderedByFileThenLine(t *testing.T) {
	at := func(file string, line int, id string) diagnostics.Diagnostic {
		d := diagnostics.NewWarning(id, diagnostics.NewPosition(line, 1), "t").WithRuleID(id)
		d.File = file
		return d
	}
	active := []diagnostics.Diagnostic{at("b.doclang", 1, "B1"), at("", 9, "R9"), at("a.doclang", 3, "A3"), at("", 2, "R2")}
	waived := []linter.WaivedDiagnostic{{Diagnostic: at("a.doclang", 1, "A1"), Policy: &linter.RulePolicy{Reason: "r"}}}
	data, err := generateJSON(active, waived, "d.doclang", "1", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var rep ReportJSON
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range rep.Findings {
		got = append(got, f.RuleID)
	}
	if want := "R2,R9,A1,A3,B1"; strings.Join(got, ",") != want {
		t.Errorf("findings %v, want %s", got, want)
	}
}
