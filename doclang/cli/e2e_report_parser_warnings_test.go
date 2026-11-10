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
