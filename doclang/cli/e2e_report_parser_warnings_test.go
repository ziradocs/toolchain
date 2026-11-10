// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/report"
)

// El reporte de `build --report json` lleva también los avisos del parser
// (aquí TABLE004, un `cells:` que no es YAML válido), como la tool `lint`
// del MCP. Antes solo salían por log.
func TestE2E_ReportIncludesParserWarnings(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "d.doclang")
	src := "---\ntitle: D\n---\n\n# A\n\nTABLE\n  cells:\n    - [ : ]\n"
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "report.json")
	_ = runCLI(t, Options{Name: "doclang", Version: "test"}, []string{"build", in, "--lint-only", "--report", "json", "--report-out", out})
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var rep report.ReportJSON
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("report: %v", err)
	}
	found := false
	for _, f := range rep.Findings {
		if f.RuleID == "TABLE004" {
			found = true
		}
	}
	if !found {
		t.Errorf("want TABLE004 in the report, got %+v", rep.Findings)
	}
}

// buildReport corre `build --lint-only --report json` sobre src y devuelve el
// reporte y el error del build.
func buildReport(t *testing.T, src string) (report.ReportJSON, error) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "d.doclang")
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "report.json")
	buildErr := runCLI(t, Options{Name: "doclang", Version: "test"}, []string{"build", in, "--lint-only", "--report", "json", "--report-out", out})
	var rep report.ReportJSON
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("report not written (build: %v): %v", buildErr, err)
	}
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("report: %v", err)
	}
	return rep, buildErr
}

// La telemetría del detector de IA (info sin regla) no va al reporte. El
// título "Resumen y Action Items" dispara el detector; hoy el parser de
// documentos la deja solo en el log, y esta prueba cuida que siga fuera del
// reporte si algún día la devuelve como diagnóstico (como el de slides).
func TestE2E_ReportOmitsParserTelemetry(t *testing.T) {
	src := "---\ntitle: D\nmode: flex\n---\n\n# Resumen y Action Items\n\nTexto.\n\nTABLE\n  cells:\n    - [ : ]\n"
	rep, err := buildReport(t, src)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("want the TABLE findings in the report, got none")
	}
	for _, f := range rep.Findings {
		if f.Source == "normalizer" || f.Source == "detector" || (f.RuleID == "" && f.Code == "") {
			t.Errorf("telemetry in the report: %+v", f.Diagnostic)
		}
	}
}

// Los hallazgos del reporte van por línea, vengan del parser o del linter:
// TABLE004 (parser, línea 13) va después de TABLE002 (linter, línea 8).
func TestE2E_ReportFindingsSortedByLine(t *testing.T) {
	src := "---\ntitle: D\nmode: flex\n---\n\n# A\n\nTABLE\n  headers: [\"a\"]\n\n## B\n\nTABLE\n  cells:\n    - [ : ]\n"
	rep, err := buildReport(t, src)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var got []string
	for _, f := range rep.Findings {
		id := f.RuleID
		if id == "" {
			id = f.Code
		}
		got = append(got, fmt.Sprintf("%s@%d", id, f.Position.Line))
	}
	want := []string{"TABLE002@8", "TABLE004@13", "TABLE001@13", "TABLE002@13"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

// Los avisos del parser respetan la política del frontmatter: con TABLE004
// apagado queda suprimido en el reporte, y con severidad error el build
// falla.
func TestE2E_ReportParserWarningsFollowPolicy(t *testing.T) {
	body := "\n# A\n\nTABLE\n  cells:\n    - [ : ]\n"

	t.Run("enabled false", func(t *testing.T) {
		src := "---\ntitle: D\nmode: flex\nlint_policy:\n  rules:\n    TABLE004:\n      enabled: false\n---\n" + body
		rep, err := buildReport(t, src)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		var waived, active int
		for _, f := range rep.Findings {
			if f.RuleID != "TABLE004" {
				continue
			}
			if f.Waived {
				waived++
			} else {
				active++
			}
		}
		if waived != 1 || active != 0 {
			t.Errorf("TABLE004 waived=%d active=%d, want 1 and 0: %+v", waived, active, rep.Findings)
		}
	})

	t.Run("severity error", func(t *testing.T) {
		src := "---\ntitle: D\nmode: flex\nlint_policy:\n  rules:\n    TABLE004:\n      severity: error\n---\n" + body
		rep, err := buildReport(t, src)
		if err == nil {
			t.Fatal("build passed, want a lint failure for TABLE004 raised to error")
		}
		found := false
		for _, f := range rep.Findings {
			if f.RuleID == "TABLE004" && f.Severity == diagnostics.Error && !f.Waived {
				found = true
			}
		}
		if !found {
			t.Errorf("want TABLE004 as an active error in the report, got %+v", rep.Findings)
		}
	})
}
