// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/report"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/internal/pipeline"
)

// activeKeys reduce diags a "ID:severidad@línea", ordenados, para comparar
// los tres caminos sin depender del orden en que cada uno los arma.
func activeKeys(diags []diagnostics.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		id := d.RuleID
		if id == "" {
			id = d.Code
		}
		out = append(out, fmt.Sprintf("%s:%s@%d", id, d.Severity, d.Position.Line))
	}
	sort.Strings(out)
	return out
}

// Con una política sobre un ID del parser (TABLE004), `doclang build`, la
// tool `lint` y la tool `export` del MCP dicen lo mismo: con enabled:false el
// aviso no sale en ninguno, con severity:error sale como error en los tres
// (lint con valid=false, export con error), y un waiver vencido agrega el
// POLICY001 en los tres.
func TestPolicyOnParserIDs_BuildLintAndExportAgree(t *testing.T) {
	body := "\n# A\n\nTABLE\n  cells:\n    - [ : ]\n"
	past := time.Now().AddDate(-1, 0, 0).Format(time.RFC3339)
	cases := []struct {
		name, policy string
		wantErr      bool
		want         string // TABLE004 esperado entre los activos, "" si no debe estar
	}{
		{"enabled false", "    TABLE004:\n      enabled: false\n", false, ""},
		{"severity error", "    TABLE004:\n      severity: error\n", true, "TABLE004:error"},
		{"expired waiver", "    TABLE004:\n      expires_at: \"" + past + "\"\n      reason: \"r\"\n      approved_by: \"e2e\"\n", false, "TABLE004:warning"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "---\ntitle: D\nmode: flex\nlint_policy:\n  rules:\n" + c.policy + "---\n" + body

			// build: el reporte JSON de pipeline.Prepare.
			dir := t.TempDir()
			reportPath := filepath.Join(dir, "r.json")
			_, buildErr := pipeline.Prepare(util.NewNoop(), []byte(src), pipeline.PrepareOptions{InputFile: "d.md", MaxInputBytes: util.DefaultMaxInputBytes, ReportFormat: "json", ReportOut: reportPath})
			if (buildErr != nil) != c.wantErr {
				t.Fatalf("build error = %v, want error %v", buildErr, c.wantErr)
			}
			data, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			var rep report.ReportJSON
			if err := json.Unmarshal(data, &rep); err != nil {
				t.Fatal(err)
			}
			var built []diagnostics.Diagnostic
			for _, f := range rep.Findings {
				if !f.Waived {
					built = append(built, f.Diagnostic)
				}
			}

			// lint del MCP.
			session, ctx := newTestSession(t)
			res := callTool(t, session, ctx, "lint", map[string]any{"source": src})
			var lint lintOutput
			if err := json.Unmarshal([]byte(resultText(t, res)), &lint); err != nil {
				t.Fatal(err)
			}
			if lint.Valid == c.wantErr {
				t.Errorf("lint valid = %v, want %v", lint.Valid, !c.wantErr)
			}

			// export del MCP.
			f := newExportFixture(t)
			f.write(t, "d.md", src)
			es, ectx := newExportSession(t, Options{Root: f.root})
			var exported []diagnostics.Diagnostic
			if c.wantErr {
				exported = exportFail(t, es, ectx, map[string]any{"path": "d.md", "format": "html"}, "").Diagnostics
			} else {
				exported = exportOK(t, es, ectx, map[string]any{"path": "d.md", "format": "html"}).Diagnostics
			}

			b, l, e := activeKeys(built), activeKeys(lint.Diagnostics), activeKeys(exported)
			if strings.Join(b, ",") != strings.Join(l, ",") || strings.Join(b, ",") != strings.Join(e, ",") {
				t.Errorf("paths disagree:\n build  %v\n lint   %v\n export %v", b, l, e)
			}
			got := ""
			for _, k := range b {
				if strings.HasPrefix(k, "TABLE004:") {
					got = k[:strings.Index(k, "@")]
				}
			}
			if got != c.want {
				t.Errorf("TABLE004 = %q, want %q (build %v)", got, c.want, b)
			}
			if c.name == "expired waiver" && !strings.Contains(strings.Join(b, ","), "POLICY001") {
				t.Errorf("want POLICY001 for the expired waiver, got %v", b)
			}
		})
	}
}
