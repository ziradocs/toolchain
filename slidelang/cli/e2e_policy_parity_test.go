// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/mcp"
)

// mcpLint llama a la tool `lint` del servidor MCP de slidelang en memoria.
func mcpLint(t *testing.T, src string) (diags []diagnostics.Diagnostic, valid bool) {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	if _, err := mcp.NewServer(util.NewNoop()).Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "t", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "lint", Arguments: map[string]any{"source": src, "fileName": "d.slidelang"}})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Diagnostics []diagnostics.Diagnostic `json:"diagnostics"`
		Valid       bool                     `json:"valid"`
	}
	text := res.Content[0].(*sdkmcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("lint result %q: %v", text, err)
	}
	return out.Diagnostics, out.Valid
}

func diagKeys(diags []diagnostics.Diagnostic) []string {
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

// Con una política sobre un ID del parser (STRICT003), el reporte de
// `slidelang build` y la tool `lint` del MCP dicen lo mismo: apagado no sale
// en ninguno, subido a error sale como error y deja valid=false, y un waiver
// vencido agrega el POLICY001 en los dos.
func TestE2E_PolicyOnParserIDs_BuildAndMCPLintAgree(t *testing.T) {
	body := "\nSLIDE content\n  title: \"T\"\n  RARO línea\n  TEXT\n    hola\n"
	past := time.Now().AddDate(-1, 0, 0).Format(time.RFC3339)
	cases := []struct {
		name, policy, want string
		wantErr            bool
	}{
		{"enabled false", "    STRICT003:\n      enabled: false\n", "", false},
		{"severity error", "    STRICT003:\n      severity: error\n", "STRICT003:error", true},
		{"expired waiver", "    STRICT003:\n      expires_at: \"" + past + "\"\n      reason: \"r\"\n      approved_by: \"e2e\"\n", "STRICT003:warning", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "---\nmode: strict\ntitle: \"D\"\nlint_policy:\n  rules:\n" + c.policy + "---\n" + body
			rep, buildErr := buildReport(t, src)
			if (buildErr != nil) != c.wantErr {
				t.Fatalf("build error = %v, want error %v", buildErr, c.wantErr)
			}
			var built []diagnostics.Diagnostic
			for _, f := range rep.Findings {
				if !f.Waived {
					built = append(built, f.Diagnostic)
				}
			}
			linted, valid := mcpLint(t, src)
			if valid == c.wantErr {
				t.Errorf("lint valid = %v, want %v", valid, !c.wantErr)
			}
			b, l := diagKeys(built), diagKeys(linted)
			if strings.Join(b, ",") != strings.Join(l, ",") {
				t.Errorf("build and MCP lint disagree:\n build %v\n lint  %v", b, l)
			}
			got := ""
			for _, k := range b {
				if strings.HasPrefix(k, "STRICT003:") {
					got = k[:strings.Index(k, "@")]
				}
			}
			if got != c.want {
				t.Errorf("STRICT003 = %q, want %q (%v)", got, c.want, b)
			}
			if c.name == "expired waiver" && !strings.Contains(strings.Join(b, ","), "POLICY001") {
				t.Errorf("want POLICY001 for the expired waiver, got %v", b)
			}
		})
	}
}

// La tool `lint` del MCP no devuelve la telemetría del detector, igual que
// el reporte del build.
func TestE2E_MCPLintOmitsParserTelemetry(t *testing.T) {
	linted, _ := mcpLint(t, "---\ntitle: \"D\"\nmode: flex\n---\n\n# Resumen y Action Items\n\n- uno\n- dos\n")
	if len(linted) == 0 {
		t.Fatal("want the linter findings, got none")
	}
	for _, d := range linted {
		if d.Source == "normalizer" || d.Source == "detector" || (d.RuleID == "" && d.Code == "") {
			t.Errorf("telemetry in MCP lint: %+v", d)
		}
	}
}

// El resumen de `build --lint-only` cuenta los warnings del parser, y
// "Encontrados N warnings" dice lo mismo que `warnings:`. Un waiver vencido
// sobre un ID del parser deja su POLICY001 en la lista de LINT.
func TestE2E_LintSummaryCountsParserWarnings(t *testing.T) {
	run := func(t *testing.T, src string) string {
		t.Helper()
		in := filepath.Join(t.TempDir(), "d.slidelang")
		if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return captureStderr(t, func() {
			if err := runCLI(t, Options{}, []string{"build", in, "--lint-only", "--no-colors"}); err != nil {
				t.Errorf("build: %v", err)
			}
		})
	}
	body := "\nSLIDE content\n  title: \"T\"\n  RARO línea\n  TEXT\n    hola\n"

	t.Run("parser warning only", func(t *testing.T) {
		stderr := run(t, "---\nmode: strict\ntitle: \"D\"\n---\n"+body)
		if !strings.Contains(stderr, "warnings: 1\n") {
			t.Errorf("want `warnings: 1` in the summary:\n%s", stderr)
		}
		if !strings.Contains(stderr, "Encontrados 1 warnings, 0 errores") {
			t.Errorf("want the LINT count to match the summary:\n%s", stderr)
		}
	})

	t.Run("expired waiver", func(t *testing.T) {
		past := time.Now().AddDate(-1, 0, 0).Format(time.RFC3339)
		stderr := run(t, "---\nmode: strict\ntitle: \"D\"\nlint_policy:\n  rules:\n    STRICT003:\n      expires_at: \""+past+"\"\n      reason: \"r\"\n      approved_by: \"e2e\"\n---\n"+body)
		found := false
		for _, line := range strings.Split(stderr, "\n") {
			if strings.HasPrefix(line, "⚠️") && strings.Contains(line, "waiver for STRICT003 expired") {
				found = true
			}
		}
		if !found {
			t.Errorf("want the POLICY001 warning in the LINT list:\n%s", stderr)
		}
		if !strings.Contains(stderr, "warnings: 2\n") || !strings.Contains(stderr, "Encontrados 2 warnings, 0 errores") {
			t.Errorf("want STRICT003 and POLICY001 counted in both places:\n%s", stderr)
		}
	})
}

// PostLint recibe también los avisos del parser, con la política aplicada:
// STRICT003 activo y, apagado por la política, entre los suprimidos.
func TestE2E_PostLintReceivesParserDiagnostics(t *testing.T) {
	body := "\nSLIDE content\n  title: \"T\"\n  RARO línea\n  TEXT\n    hola\n"
	for _, c := range []struct {
		name, policy string
		wantActive   bool
	}{
		{"active", "", true},
		{"waived", "lint_policy:\n  rules:\n    STRICT003:\n      enabled: false\n", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := filepath.Join(t.TempDir(), "d.slidelang")
			if err := os.WriteFile(in, []byte("---\nmode: strict\ntitle: \"D\"\n"+c.policy+"---\n"+body), 0o644); err != nil {
				t.Fatal(err)
			}
			var active, waived bool
			hook := func(_ *ast.AST, a []diagnostics.Diagnostic, w []linter.WaivedDiagnostic) error {
				for _, d := range a {
					active = active || d.RuleID == "STRICT003"
				}
				for _, d := range w {
					waived = waived || d.Diagnostic.RuleID == "STRICT003"
				}
				return nil
			}
			if err := runCLI(t, Options{PostLint: hook}, []string{"build", in, "--lint-only"}); err != nil {
				t.Fatalf("build: %v", err)
			}
			if active != c.wantActive || waived == c.wantActive {
				t.Errorf("STRICT003 active=%v waived=%v, want active=%v", active, waived, c.wantActive)
			}
		})
	}
}

// Una política no apaga un error del parser: con CHART006 (unit: null)
// apagado o bajado a warning, el build falla y `lint` del MCP da
// valid=false, sin la telemetría del normalizador.
func TestE2E_MCPLintPolicyDoesNotSilenceParserErrors(t *testing.T) {
	for _, policy := range []string{"enabled: false", "severity: warning"} {
		src := "---\nmode: strict\ntitle: \"D\"\nlint_policy:\n  rules:\n    CHART006:\n      " + policy + "\n---\n\nSLIDE content\n  title: \"T\"\n  <<chart: bar>>\n    data: [[\"A\", 1], [\"B\", 2]]\n    unit: null\n  <<end>>\n"
		in := filepath.Join(t.TempDir(), "d.slidelang")
		if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runCLI(t, Options{}, []string{"build", in, "--lint-only"}); err == nil {
			t.Fatalf("%s: build passed, want the CHART006 parse error", policy)
		}
		diags, valid := mcpLint(t, src)
		if valid {
			t.Errorf("%s: MCP lint valid=true, want false like the build: %v", policy, diags)
		}
		for _, d := range diags {
			if d.Source == "normalizer" || d.Source == "detector" {
				t.Errorf("%s: telemetry in MCP lint: %+v", policy, d)
			}
		}
	}
}

// La info no cuenta como warning, ni la del linter ni la del parser: con
// LAYOUT003 (linter) y STRICT003 (parser) bajados a info por la política,
// el resumen y "Encontrados" dicen 0 warnings.
func TestE2E_LintSummaryDoesNotCountInfo(t *testing.T) {
	src := "---\nmode: strict\ntitle: \"D\"\nlint_policy:\n  rules:\n    LAYOUT003:\n      severity: info\n    STRICT003:\n      severity: info\n---\n\nSLIDE content\n  TEXT\n    hola\n\nSLIDE content\n  title: \"T2\"\n  RARO línea\n  TEXT\n    hola\n"
	in := filepath.Join(t.TempDir(), "d.slidelang")
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr := captureStderr(t, func() {
		if err := runCLI(t, Options{}, []string{"build", in, "--lint-only", "--no-colors"}); err != nil {
			t.Errorf("build: %v", err)
		}
	})
	if !strings.Contains(stderr, "warnings: 0\n") || !strings.Contains(stderr, "Encontrados 0 warnings, 0 errores") {
		t.Errorf("want info left out of both counts:\n%s", stderr)
	}
	if !strings.Contains(stderr, "ℹ️  info[LAYOUT003]") && !strings.Contains(stderr, "ℹ️  info:") {
		t.Errorf("want the linter info listed with ℹ️:\n%s", stderr)
	}
}
