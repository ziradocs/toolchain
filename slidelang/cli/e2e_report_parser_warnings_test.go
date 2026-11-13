// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/report"
)

// El reporte de `build --report json` lleva también los avisos del parser
// (aquí STRICT003, una línea no reconocida en un slide strict), como la tool
// `lint` del MCP. Antes solo salían por consola.
func TestE2E_ReportIncludesParserWarnings(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "d.slidelang")
	src := "---\nmode: strict\ntitle: \"D\"\n---\n\nSLIDE content\n  title: \"T\"\n  RARO línea\n  TEXT\n    hola\n"
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "report.json")
	if err := runCLI(t, Options{}, []string{"build", in, "--lint-only", "--report", "json", "--report-out", out}); err != nil {
		t.Fatalf("build: %v", err)
	}
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
		if f.RuleID == "STRICT003" && f.Position.Line == 8 {
			found = true
		}
	}
	if !found {
		t.Errorf("want STRICT003 at line 8 in the report, got %+v", rep.Findings)
	}
}

// buildReport corre `build --lint-only --report json` sobre src y devuelve el
// reporte y el error del build.
func buildReport(t *testing.T, src string) (report.ReportJSON, error) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "d.slidelang")
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "report.json")
	buildErr := runCLI(t, Options{}, []string{"build", in, "--lint-only", "--report", "json", "--report-out", out})
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

// La telemetría del normalizador y del detector de IA (info sin regla) no va
// al reporte: no es un hallazgo que alguien pueda atender ni suprimir. El
// título con "Resumen y Action Items" hace que el detector reporte
// "Contenido AI detectado" en la consola.
func TestE2E_ReportOmitsParserTelemetry(t *testing.T) {
	src := "---\ntitle: \"D\"\nmode: flex\n---\n\n# Resumen y Action Items\n\n- uno\n- dos\n"
	rep, err := buildReport(t, src)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("want the linter findings in the report, got none")
	}
	for _, f := range rep.Findings {
		if f.Source == "normalizer" || f.Source == "detector" || (f.RuleID == "" && f.Code == "") {
			t.Errorf("telemetry in the report: %+v", f.Diagnostic)
		}
	}
}

// Los hallazgos del reporte van por línea, sin importar si los dio el parser
// o el linter: STRICT003 (parser, línea 12) va después de LAYOUT003 (linter,
// línea 6).
func TestE2E_ReportFindingsSortedByLine(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"D\"\n---\n\nSLIDE content\n  TEXT\n    hola\n\nSLIDE content\n  title: \"T2\"\n  RARO línea\n  TEXT\n    hola\n\nSLIDE content\n  title: \"T3\"\n  TEXT\n    fin\n"
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
	want := []string{"LAYOUT003@6", "STRICT003@12"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("findings = %v, want %v", got, want)
	}
}

// Los avisos del parser respetan la política del frontmatter: con
// STRICT003 apagado el hallazgo queda suprimido en el reporte, y con
// severidad error el build falla.
func TestE2E_ReportParserWarningsFollowPolicy(t *testing.T) {
	body := "\nSLIDE content\n  title: \"T\"\n  RARO línea\n  TEXT\n    hola\n"

	t.Run("enabled false", func(t *testing.T) {
		src := "---\nmode: strict\ntitle: \"D\"\nlint_policy:\n  rules:\n    STRICT003:\n      enabled: false\n---\n" + body
		rep, err := buildReport(t, src)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		var waived, active int
		for _, f := range rep.Findings {
			if f.RuleID != "STRICT003" {
				continue
			}
			if f.Waived {
				waived++
			} else {
				active++
			}
		}
		if waived != 1 || active != 0 {
			t.Errorf("STRICT003 waived=%d active=%d, want 1 and 0: %+v", waived, active, rep.Findings)
		}
	})

	t.Run("severity error", func(t *testing.T) {
		src := "---\nmode: strict\ntitle: \"D\"\nlint_policy:\n  rules:\n    STRICT003:\n      severity: error\n---\n" + body
		rep, err := buildReport(t, src)
		if err == nil {
			t.Fatal("build passed, want a lint failure for STRICT003 raised to error")
		}
		found := false
		for _, f := range rep.Findings {
			if f.RuleID == "STRICT003" && f.Severity == diagnostics.Error && !f.Waived {
				found = true
			}
		}
		if !found {
			t.Errorf("want STRICT003 as an active error in the report, got %+v", rep.Findings)
		}
	})

	// En la consola, el aviso que la política subió a error sale en la lista
	// de errores de LINT; antes decía "Validación completada sin problemas"
	// y el build fallaba sin nombrar el error.
	t.Run("severity error on the console", func(t *testing.T) {
		src := "---\nmode: strict\ntitle: \"D\"\nlint_policy:\n  rules:\n    STRICT003:\n      severity: error\n---\n" + body
		dir := t.TempDir()
		in := filepath.Join(dir, "d.slidelang")
		if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		stderr := captureStderr(t, func() {
			if err := runCLI(t, Options{}, []string{"build", in, "--lint-only", "--no-colors"}); err == nil {
				t.Error("build passed, want a lint failure")
			}
		})
		if !strings.Contains(stderr, "❌ error[STRICT003]") {
			t.Errorf("want STRICT003 listed as a lint error on stderr, got:\n%s", stderr)
		}
		if strings.Contains(stderr, "Validación completada sin problemas") {
			t.Errorf("the console says there were no problems:\n%s", stderr)
		}
	})
}

// captureStderr devuelve lo que fn escribe en os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	defer func() { os.Stderr = orig }()
	fn()
	_ = w.Close()
	os.Stderr = orig
	return <-done
}
