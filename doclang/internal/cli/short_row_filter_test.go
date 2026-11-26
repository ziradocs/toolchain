// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

// coreCompletesDecodedShortRows dice si el core con el que se compila
// completa en ast.DecodeAST una fila con menos celdas que el encabezado.
// Contra un core publicado que todavía no lo hace, la prueba de abajo se
// salta.
func coreCompletesDecodedShortRows() bool {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Headers = []string{"A", "B"}
	table.Rows = [][]string{{"1"}}
	block := ast.NewContentBlock(pos, "content")
	block.Elements = []ast.Element{table}
	doc := ast.NewAST(pos)
	doc.ContentBlocks = []ast.ContentBlock{*block}
	raw, err := json.Marshal(doc)
	if err != nil {
		return false
	}
	decoded, err := ast.DecodeAST(raw)
	if err != nil || len(decoded.ContentBlocks) == 0 || len(decoded.ContentBlocks[0].Elements) == 0 {
		return false
	}
	got, ok := decoded.ContentBlocks[0].Elements[0].(*ast.TableElement)
	return ok && len(got.Rows) == 1 && len(got.Rows[0]) == 2
}

// Un --filter que le quita celdas a una fila: la fila llega completa a HTML,
// Markdown y DOCX (una celda por columna), y el build da TABLE010, un aviso
// en la línea de la fila, en vez de dejar la fila corta con un aviso que
// dice otra cosa.
func TestRunBuild_FilterThatTrimsARow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el filtro es un script de sh")
	}
	if _, err := exec.LookPath("sed"); err != nil {
		t.Skip("sed no disponible en PATH")
	}
	if !coreCompletesDecodedShortRows() {
		t.Skip("this core does not complete a short row in ast.DecodeAST")
	}
	dir := t.TempDir()
	src := "---\ntitle: \"Filtro\"\n---\n\n# Uno\n\n| A | B | C |\n|---|---|---|\n| 1 | 2 | 3 |\n| 4 | 5 | 6 |\n"
	if err := os.WriteFile(filepath.Join(dir, "doc.doclang"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	filter := filepath.Join(dir, "trim.sh")
	script := "#!/bin/sh\nsed -e 's/\\[\"4\",\"5\",\"6\"\\]/[\"4\"]/' -e 's/{\"content\":\"4\"},{\"content\":\"5\"},{\"content\":\"6\"}/{\"content\":\"4\"}/'\n"
	if err := os.WriteFile(filter, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	build := func(format string) ([]byte, []diagnostics.Diagnostic) {
		t.Helper()
		out := filepath.Join(dir, "out-"+format)
		var active []diagnostics.Diagnostic
		postLint := func(_ *ast.AST, diags []diagnostics.Diagnostic, _ []linter.WaivedDiagnostic) error {
			active = diags
			return nil
		}
		cmd := NewBuildCommand(nil, nil, nil, nil, postLint)
		cmd.SetArgs([]string{filepath.Join(dir, "doc.doclang"), "--filter", filter, "--format", format, "--output", out})
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		if err := cmd.Execute(); err != nil {
			t.Fatalf("build %s: %v", format, err)
		}
		ext := map[string]string{"html": ".html", "markdown": ".md", "docx": ".docx"}[format]
		data, err := os.ReadFile(filepath.Join(out, "doc"+ext))
		if err != nil {
			t.Fatal(err)
		}
		return data, active
	}

	html, diags := build("html")
	if !strings.Contains(string(html), "<tr><td>4</td><td></td><td></td></tr>") {
		t.Errorf("HTML does not carry the completed row:\n%s", regexp.MustCompile(`(?s)<table.*?</table>`).Find(html))
	}
	found := false
	for _, d := range diags {
		if d.RuleID == "TABLE010" {
			found = d.IsWarning() && d.Position.Line == 10
		}
		if d.RuleID == "TABLE003" {
			t.Errorf("TABLE003 for a row the filter trimmed: %v", d)
		}
	}
	if !found {
		t.Errorf("no TABLE010 warning on line 10: %v", diags)
	}

	md, _ := build("markdown")
	if !strings.Contains(string(md), "| 4 |  |  |") {
		t.Errorf("Markdown does not carry the completed row:\n%s", md)
	}

	docx, _ := build("docx")
	zr, err := zip.NewReader(bytes.NewReader(docx), int64(len(docx)))
	if err != nil {
		t.Fatal(err)
	}
	var document string
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			document = string(b)
		}
	}
	for i, row := range regexp.MustCompile(`(?s)<w:tr[ >].*?</w:tr>`).FindAllString(document, -1) {
		if n := strings.Count(row, "<w:tc>"); n != 3 {
			t.Errorf("DOCX row %d has %d cells, want 3", i, n)
		}
	}
}
