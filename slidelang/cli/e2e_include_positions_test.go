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

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/report"
)

// Un diagnóstico de un archivo traído con @include apunta a ese archivo y a
// su línea ahí, y uno del documento a su línea en el documento, en la
// consola, el reporte JSON, el SARIF y el hook PostLint. El parser y el
// linter cuentan líneas sobre el contenido expandido: sin llevarlas a su
// origen (include.SourceMap.MapDiagnostics), un aviso escrito en la línea 4
// de parts/a.slidelang salía como la línea 15 del documento, sin archivo.

// writeIncludeTree escribe files (ruta relativa → contenido) en un directorio
// temporal y entra en él, para que la ruta del documento sea relativa, como
// la escribe quien corre `slidelang build root.slidelang`.
func writeIncludeTree(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
}

// where es "archivo:línea", con el archivo vacío para el documento.
func where(d diagnostics.Diagnostic) string {
	return fmt.Sprintf("%s:%d", d.File, d.Position.Line)
}

// whereOf lista where de cada diagnóstico de la regla ruleID, en orden.
func whereOf(diags []diagnostics.Diagnostic, ruleID string) []string {
	var out []string
	for _, d := range diags {
		if d.RuleID == ruleID {
			out = append(out, where(d))
		}
	}
	return out
}

// includeRun es lo que devuelve un build con --report json: la consola, el
// reporte y lo que recibió PostLint.
type includeRun struct {
	err      error
	stderr   string
	report   report.ReportJSON
	postLint []diagnostics.Diagnostic
}

func runIncludeBuild(t *testing.T, opts Options, extra ...string) includeRun {
	t.Helper()
	var run includeRun
	opts.PostLint = func(_ *ast.AST, active []diagnostics.Diagnostic, _ []linter.WaivedDiagnostic) error {
		run.postLint = append([]diagnostics.Diagnostic(nil), active...)
		return nil
	}
	args := append([]string{"build", "root.slidelang", "--lint-only", "--no-colors", "--report", "json", "--report-out", "report.json"}, extra...)
	run.stderr = captureStderr(t, func() { run.err = runCLI(t, opts, args) })
	if data, err := os.ReadFile("report.json"); err == nil {
		if err := json.Unmarshal(data, &run.report); err != nil {
			t.Fatalf("report: %v", err)
		}
	}
	return run
}

func reportDiagnostics(rep report.ReportJSON) []diagnostics.Diagnostic {
	out := make([]diagnostics.Diagnostic, 0, len(rep.Findings))
	for _, f := range rep.Findings {
		out = append(out, f.Diagnostic)
	}
	return out
}

func assertWhere(t *testing.T, label string, got, want []string) {
	t.Helper()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("%s: got %q, want %q", label, got, want)
	}
}

// includeTree tiene un @include simple (parts/a.slidelang) y uno anidado
// (parts/a.slidelang trae parts/b.slidelang), un slide del documento antes y
// otro después de los @include, y marcadores node-id en el documento y en el
// archivo incluido, que el parser quita antes de contar líneas.
//
// LINK001 (del linter, en la línea del TEXT): root 9, a 5, b 3 y root 17.
// DIRECTIVE001 (del parser): a 4.
var includeTree = map[string]string{
	"root.slidelang": `---
mode: strict
title: "Includes"
---

<!-- node-id: intro -->
SLIDE content
  title: "A"
  TEXT
    one [x](javascript:alert(1))

@include parts/a.slidelang

SLIDE content
  title: "C"
  <!-- node-id: closing-text -->
  TEXT
    three [x](javascript:alert(3))
`,
	"parts/a.slidelang": `<!-- node-id: middle -->
SLIDE content
  title: "B"
  @reveal
  TEXT
    two [x](javascript:alert(2))

@include parts/b.slidelang
`,
	"parts/b.slidelang": `SLIDE content
  title: "D"
  TEXT
    four [x](javascript:alert(4))
`,
}

func TestE2E_IncludedDiagnosticsPointToTheirFileAndLine(t *testing.T) {
	writeIncludeTree(t, includeTree)
	run := runIncludeBuild(t, Options{})
	if run.err != nil {
		t.Fatalf("build: %v\n%s", run.err, run.stderr)
	}

	// El reporte va por archivo: el documento primero y después cada
	// archivo incluido, por nombre (core/report y findings.SortByPosition).
	wantLink := []string{":9", ":17", "parts/a.slidelang:5", "parts/b.slidelang:3"}
	wantDirective := []string{"parts/a.slidelang:4"}
	rep := reportDiagnostics(run.report)
	assertWhere(t, "report LINK001", whereOf(rep, "LINK001"), wantLink)
	assertWhere(t, "report DIRECTIVE001", whereOf(rep, "DIRECTIVE001"), wantDirective)
	assertWhere(t, "PostLint LINK001", whereOf(run.postLint, "LINK001"), wantLink)
	assertWhere(t, "PostLint DIRECTIVE001", whereOf(run.postLint, "DIRECTIVE001"), wantDirective)

	// La consola nombra el archivo antes de la posición (Diagnostic.String).
	for _, want := range []string{
		`javascript:alert(1)" uses the "javascript:" scheme`,
		"(9:1)",
		"(17:1)",
		"(parts/a.slidelang:5:1)",
		"(parts/b.slidelang:3:1)",
		"warning[DIRECTIVE001]: @reveal fue retirada; usá una directiva modificadora como @fade-in (parts/a.slidelang:4:1)",
	} {
		if !strings.Contains(run.stderr, want) {
			t.Errorf("console lacks %q:\n%s", want, run.stderr)
		}
	}
	// Las posiciones del contenido expandido que salían antes.
	for _, stale := range []string{"(15:1)", "(16:1)", "(21:1)", "(29:1)"} {
		if strings.Contains(run.stderr, stale) {
			t.Errorf("console still has an expanded-content position %q:\n%s", stale, run.stderr)
		}
	}
}

func TestE2E_IncludedDiagnosticsInSARIF(t *testing.T) {
	writeIncludeTree(t, includeTree)
	err := runCLI(t, Options{}, []string{"build", "root.slidelang", "--lint-only", "--report", "sarif", "--report-out", "report.sarif"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := os.ReadFile("report.sarif")
	if err != nil {
		t.Fatal(err)
	}
	var sarif report.SARIF
	if err := json.Unmarshal(data, &sarif); err != nil {
		t.Fatalf("sarif: %v", err)
	}
	var got []string
	for _, r := range sarif.Runs[0].Results {
		if r.RuleId != "LINK001" && r.RuleId != "DIRECTIVE001" {
			continue
		}
		loc := r.Locations[0].PhysicalLocation
		got = append(got, fmt.Sprintf("%s %s:%d", r.RuleId, loc.ArtifactLocation.Uri, loc.Region.StartLine))
	}
	want := []string{
		"LINK001 root.slidelang:9",
		"LINK001 root.slidelang:17",
		"DIRECTIVE001 parts/a.slidelang:4",
		"LINK001 parts/a.slidelang:5",
		"LINK001 parts/b.slidelang:3",
	}
	assertWhere(t, "SARIF", got, want)
}

// Un archivo incluido dos veces se lintea dos veces: su hallazgo sale una
// vez, en su archivo y su línea, como en doclang (DedupeIncluded).
func TestE2E_FileIncludedTwiceReportsOnce(t *testing.T) {
	writeIncludeTree(t, map[string]string{
		"root.slidelang": `---
mode: strict
title: "Twice"
---

@include parts/c.slidelang

@include parts/c.slidelang
`,
		"parts/c.slidelang": `SLIDE content
  title: "C"
  TEXT
    see [x](javascript:alert(1))
`,
	})
	run := runIncludeBuild(t, Options{})
	if run.err != nil {
		t.Fatalf("build: %v\n%s", run.err, run.stderr)
	}
	assertWhere(t, "report LINK001", whereOf(reportDiagnostics(run.report), "LINK001"), []string{"parts/c.slidelang:3"})
	if n := strings.Count(run.stderr, "warning[LINK001]"); n != 1 {
		t.Errorf("console lists LINK001 %d times, want 1:\n%s", n, run.stderr)
	}
}

// La imagen de bloque de un :::note escrita en un archivo incluido: el parser
// avisa en la línea de la imagen y el linter, desde el texto del bloque, en
// la del ":::". MergeParserFindings los junta sobre las líneas del contenido
// expandido, que son las del AST, y el que queda se lleva después a su
// archivo: sale uno solo, en la línea de la imagen.
func TestE2E_IncludedBlockImageFindingMergedThenMapped(t *testing.T) {
	writeIncludeTree(t, map[string]string{
		"root.slidelang": `---
title: Imagenes
mode: flex
---
# T

Texto.

@include parts/note.slidelang

Fin.
`,
		"parts/note.slidelang": `Antes.

:::note
![c](sub dir/c.png)
:::
`,
	})
	run := runIncludeBuild(t, Options{})
	if run.err != nil {
		t.Fatalf("build: %v\n%s", run.err, run.stderr)
	}
	var got []string
	for _, d := range reportDiagnostics(run.report) {
		if d.RuleID == "LINK001" && strings.Contains(d.Message, "sub dir/c.png") {
			got = append(got, where(d)+" "+d.Source)
		}
	}
	assertWhere(t, "LINK001 for c.png", got, []string{"parts/note.slidelang:4 image-parser"})
}

// Un error del parser corta el build antes del lint: la consola lo nombra en
// su archivo, igual que los avisos apartados para el lint que salen ahí.
func TestE2E_IncludedParseErrorPointsToItsFile(t *testing.T) {
	writeIncludeTree(t, map[string]string{
		"root.slidelang": `---
title: Imagenes
mode: flex
---
# T

@include parts/bad.slidelang
`,
		"parts/bad.slidelang": `Texto.

![c](sub dir/c.png)

<!-- node-id: orphan1 -->
`,
	})
	run := runIncludeBuild(t, Options{})
	if run.err == nil {
		t.Fatalf("build succeeded; want a parse error\n%s", run.stderr)
	}
	for _, want := range []string{
		`orphan node-id "orphan1"`,
		"(parts/bad.slidelang:5:1)",
		"(parts/bad.slidelang:3:1)",
	} {
		if !strings.Contains(run.stderr, want) {
			t.Errorf("console lacks %q:\n%s", want, run.stderr)
		}
	}
}

// slideRangeRule da un aviso por slide con elementos, del SLIDE al último
// elemento: un rango, para ver qué pasa con la posición final.
type slideRangeRule struct{}

func (slideRangeRule) Check(node ast.Node) []diagnostics.Diagnostic {
	slide, ok := node.(*ast.ContentBlock)
	if !ok || len(slide.Elements) == 0 {
		return nil
	}
	last := slide.Elements[len(slide.Elements)-1].GetPosition()
	return []diagnostics.Diagnostic{
		diagnostics.NewWarning("slide range", slide.GetPosition(), "test").WithRuleID("RANGE001").WithEndPosition(last),
	}
}

// Una posición final en otro archivo que la inicial se descarta (un rango no
// cruza archivos) y una en el mismo archivo se lleva a su línea ahí, como en
// doclang: los dos usan include.SourceMap.MapDiagnostics.
func TestE2E_IncludedEndPositionStaysInItsFile(t *testing.T) {
	writeIncludeTree(t, map[string]string{
		"root.slidelang": `---
mode: strict
title: "Ranges"
---

SLIDE content
  title: "A"
@include parts/body.slidelang

@include parts/whole.slidelang
`,
		"parts/body.slidelang": `  TEXT
    one
`,
		"parts/whole.slidelang": `SLIDE content
  title: "W"
  TEXT
    w
  TEXT
    w2
`,
	})
	run := runIncludeBuild(t, Options{CustomRules: []linter.Rule{slideRangeRule{}}})
	if run.err != nil {
		t.Fatalf("build: %v\n%s", run.err, run.stderr)
	}
	for label, diags := range map[string][]diagnostics.Diagnostic{
		"report":   reportDiagnostics(run.report),
		"PostLint": run.postLint,
	} {
		var got []string
		for _, d := range diags {
			if d.RuleID != "RANGE001" {
				continue
			}
			end := "-"
			if d.EndPosition != nil {
				end = fmt.Sprint(d.EndPosition.Line)
			}
			got = append(got, where(d)+" end "+end)
		}
		// El slide A empieza en el documento y su TEXT viene de
		// parts/body.slidelang: sin posición final. El slide W está entero
		// en parts/whole.slidelang: del 1 al 5.
		assertWhere(t, label+" RANGE001", got, []string{":6 end -", "parts/whole.slidelang:1 end 5"})
	}
	for _, want := range []string{"slide range (6:1)", "slide range (parts/whole.slidelang:1:1-5:1)"} {
		if !strings.Contains(run.stderr, want) {
			t.Errorf("console lacks %q:\n%s", want, run.stderr)
		}
	}
}
