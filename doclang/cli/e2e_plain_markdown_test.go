// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/internal/mcp"
)

// Markdown común de proyecto, sin frontmatter, convertido con `doclang
// build`: tiene que construir sin errores, sin perder texto, sin las reglas
// de slides (FRONT003, LAYOUT002) y sin agregar un TOC que el original no
// tiene.

var titleTagRe = regexp.MustCompile(`<title>([^<]*)</title>`)

type plainMarkdownCase struct {
	name      string
	file      string
	content   string
	title     string
	sentinels []string
}

var plainMarkdownCases = []plainMarkdownCase{
	{
		name:      "sin frontmatter",
		file:      "nofm.md",
		content:   "# Mi título\n\nPárrafo SENTINEL_A.\n\n## Sub\n\nTexto SENTINEL_B.\n",
		title:     "Mi título",
		sentinels: []string{"SENTINEL_A", "SENTINEL_B", `<h2 id="sub">`},
	},
	{
		name:      "solo h2",
		file:      "solo-h2.md",
		content:   "## Solo dos\n\nTexto SENTINEL_C.\n\n### Tres\n\nMás SENTINEL_D.\n",
		title:     "solo-h2", // sin `#`: el nombre del archivo
		sentinels: []string{"SENTINEL_C", "SENTINEL_D", `<h2 id="solo-dos">`, `<h3 id="tres">`},
	},
	{
		name:      "texto antes del h1",
		file:      "antes.md",
		content:   "> [!CAUTION]\n> Aviso SENTINEL_E.\n\nIntro SENTINEL_F.\n\n# Título\n\nCuerpo SENTINEL_G.\n",
		title:     "Título",
		sentinels: []string{"SENTINEL_E", "SENTINEL_F", "SENTINEL_G"},
	},
}

func reportRuleIDs(t *testing.T, path string) []string {
	t.Helper()
	var rep struct {
		Findings []struct {
			RuleID string `json:"ruleId"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(readFileString(t, path)), &rep); err != nil {
		t.Fatalf("report: %v", err)
	}
	ids := []string{}
	for _, f := range rep.Findings {
		ids = append(ids, f.RuleID)
	}
	sort.Strings(ids)
	return ids
}

func TestE2E_PlainMarkdownBuilds(t *testing.T) {
	for _, tc := range plainMarkdownCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := writeDoc(t, dir, tc.file, tc.content)
			report := filepath.Join(dir, "report.json")

			if err := runCLI(t, Options{Name: "doclang", Version: "test"},
				[]string{"build", src, "--output", dir, "--report", "json", "--report-out", report}); err != nil {
				t.Fatalf("build: %v", err)
			}
			for _, id := range reportRuleIDs(t, report) {
				switch id {
				case "FRONT003", "LAYOUT002", "LAYOUT_FORBIDDEN_ELEMENT", "LAYOUT_AUTO_CLOSING", "CORE001":
					t.Errorf("regla de slides sobre un documento: %s", id)
				}
			}

			html := readFileString(t, filepath.Join(dir, strings.TrimSuffix(tc.file, ".md")+".html"))
			for _, s := range tc.sentinels {
				if !strings.Contains(html, s) {
					t.Errorf("falta %q en el HTML", s)
				}
			}
			if m := titleTagRe.FindStringSubmatch(html); m == nil || m[1] != tc.title {
				t.Errorf("<title>: want %q, got %v", tc.title, m)
			}
			if strings.Contains(html, "toc-text") {
				t.Error("sin frontmatter no se agrega TOC")
			}
		})
	}
}

// Con frontmatter el comportamiento no cambia: su `title:` gana y el TOC
// sigue prendido por defecto.
func TestE2E_PlainMarkdown_FrontMatterUnchanged(t *testing.T) {
	dir := t.TempDir()
	src := writeDoc(t, dir, "fm.md", "---\nmode: flex\ntitle: Del FM\n---\n\n# Otro\n\nCuerpo SENTINEL_H.\n")
	if err := runCLI(t, Options{Name: "doclang", Version: "test"}, []string{"build", src, "--output", dir}); err != nil {
		t.Fatalf("build: %v", err)
	}
	html := readFileString(t, filepath.Join(dir, "fm.html"))
	if m := titleTagRe.FindStringSubmatch(html); m == nil || m[1] != "Del FM" {
		t.Errorf("<title>: want %q, got %v", "Del FM", m)
	}
	if !strings.Contains(html, "toc-text") || !strings.Contains(html, "SENTINEL_H") {
		t.Error("con frontmatter el TOC sigue por defecto y el cuerpo se conserva")
	}
}

func TestE2E_PlainMarkdown_EmptyDocumentFails(t *testing.T) {
	dir := t.TempDir()
	src := writeDoc(t, dir, "vacio.md", "\n\n")
	report := filepath.Join(dir, "report.json")
	err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", src, "--output", dir, "--report", "json", "--report-out", report})
	if err == nil {
		t.Fatal("un documento vacío debe fallar")
	}
	raw := readFileString(t, report)
	if !strings.Contains(raw, "CORE001") || !strings.Contains(raw, "Document has no content") {
		t.Errorf("CORE001 con mensaje de documento, got %s", raw)
	}
	if strings.Contains(raw, "slide") {
		t.Errorf("el mensaje no debe hablar de slides: %s", raw)
	}
}

// La tool `lint` de `doclang mcp` y `doclang build` dan los mismos
// hallazgos de lint para la misma fuente.
func TestE2E_PlainMarkdown_MCPLintMatchesBuild(t *testing.T) {
	content := "Intro SENTINEL_I.\n\n## Sub\n\n```python\n```\n"
	dir := t.TempDir()
	src := writeDoc(t, dir, "paridad.md", content)
	report := filepath.Join(dir, "report.json")
	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", src, "--lint-only", "--report", "json", "--report-out", report}); err != nil {
		t.Fatalf("build: %v", err)
	}
	buildIDs := reportRuleIDs(t, report)
	if len(buildIDs) == 0 {
		t.Fatal("el fixture debe producir al menos un hallazgo (CODE001) para que la comparación diga algo")
	}

	ctx := context.Background()
	clientT, serverT := sdkmcp.NewInMemoryTransports()
	if _, err := mcp.NewServer(util.NewNoop()).Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "t", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "lint", Arguments: map[string]any{"source": content}})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Valid        bool `json:"valid"`
		SectionCount int  `json:"sectionCount"`
		Diagnostics  []struct {
			RuleID   string `json:"ruleId"`
			Code     string `json:"code"`
			Source   string `json:"source"`
			Severity string `json:"severity"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(*sdkmcp.TextContent).Text), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Valid || out.SectionCount != 1 {
		t.Errorf("want valid con 1 sección, got valid=%v sections=%d", out.Valid, out.SectionCount)
	}
	mcpIDs := []string{}
	for _, d := range out.Diagnostics {
		if d.Source != "linter" {
			continue // el reporte de build solo trae hallazgos del linter
		}
		id := d.RuleID
		if id == "" {
			id = d.Code
		}
		mcpIDs = append(mcpIDs, id)
	}
	sort.Strings(mcpIDs)
	if strings.Join(mcpIDs, ",") != strings.Join(buildIDs, ",") {
		t.Errorf("lint de MCP %v != build %v", mcpIDs, buildIDs)
	}
}
