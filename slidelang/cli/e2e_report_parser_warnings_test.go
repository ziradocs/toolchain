// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
