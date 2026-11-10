// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/util"
)

// Active y Waived ordenan como core/report: primero el documento, después
// cada archivo incluido, y dentro de cada uno por línea y columna.
func TestSortByPositionOrdersByFileFirst(t *testing.T) {
	at := func(file string, line, col int) diagnostics.Diagnostic {
		d := diagnostics.NewWarning("m", diagnostics.NewPosition(line, col), "t")
		d.File = file
		return d
	}
	in := []diagnostics.Diagnostic{at("partes/x.doclang", 5, 1), at("", 9, 1), at("partes/a.doclang", 7, 1), at("", 2, 3), at("", 2, 1)}
	want := []diagnostics.Diagnostic{in[4], in[3], in[1], in[2], in[0]}
	got := PolicyResult{LintActive: in}.Active()
	waivedIn := make([]linter.WaivedDiagnostic, len(in))
	for i, d := range in {
		waivedIn[i] = linter.WaivedDiagnostic{Diagnostic: d}
	}
	waived := PolicyResult{LintWaived: waivedIn}.Waived()
	for i := range want {
		if got[i].File != want[i].File || got[i].Position != want[i].Position {
			t.Errorf("Active()[%d] = %s %v, want %s %v", i, got[i].File, got[i].Position, want[i].File, want[i].Position)
		}
		if w := waived[i].Diagnostic; w.File != want[i].File || w.Position != want[i].Position {
			t.Errorf("Waived()[%d] = %s %v, want %s %v", i, w.File, w.Position, want[i].File, want[i].Position)
		}
	}
}

func TestWhere(t *testing.T) {
	d := diagnostics.NewWarning("m", diagnostics.NewPosition(4, 1), "t")
	if got := Where(d); got != "line 4" {
		t.Errorf("Where(document) = %q", got)
	}
	d.File = "partes/x.doclang"
	if got := Where(d); got != "partes/x.doclang:4" {
		t.Errorf("Where(included) = %q", got)
	}
}

// DocumentDir da el directorio del documento sin resolver symlinks, en la
// forma de la ruta de entrada.
func TestDocumentDir(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(base, "docs")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "enlace")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	if got := DocumentDir(filepath.Join(link, "main.doclang")); got != link {
		t.Errorf("DocumentDir = %q, want %q", got, link)
	}
	if got := DocumentDir(""); got != "" {
		t.Errorf("DocumentDir without a file = %q, want empty", got)
	}
}

// sourcesFromDocDir escribe cada archivo incluido desde el directorio del
// documento: por el symlink si el archivo está debajo de él, y tal cual si
// no; y deja el SourceMap igual si el directorio no tiene symlinks.
func TestSourcesFromDocDir(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(base, "a", "real")
	under := filepath.Join(real, "partes", "x.doclang")
	outside := filepath.Join(base, "inc", "y.doclang")
	for _, f := range []string{under, outside} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(base, "lnk")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	sources := include.SourceMap{Lines: []include.Origin{{File: "", Line: 1}, {File: under, Line: 1}, {File: outside, Line: 1}, {File: under, Line: 2}}, RootLines: 2}
	got := sourcesFromDocDir(sources, link)
	want := []string{"", filepath.Join(link, "partes", "x.doclang"), outside, filepath.Join(link, "partes", "x.doclang")}
	for i, o := range got.Lines {
		if o.File != want[i] || o.Line != sources.Lines[i].Line {
			t.Errorf("line %d: %+v, want %s:%d", i+1, o, want[i], sources.Lines[i].Line)
		}
	}
	if got.RootLines != sources.RootLines || sources.Lines[1].File != under {
		t.Errorf("RootLines %d, or the input was modified: %+v", got.RootLines, sources)
	}
	if same := sourcesFromDocDir(sources, real); same.Lines[1].File != under || same.Lines[2].File != outside {
		t.Errorf("without a symlink the files change: %+v", same)
	}
}

// prepareWithInclude escribe main en un directorio temporal con part en
// partes/p.doclang y corre Prepare como el build, con la raíz de includes y
// docDir en ese directorio.
func prepareWithInclude(t *testing.T, log util.Logger, main, part string) (*Prepared, error) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "partes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "partes", "p.doclang"), []byte(part), 0o644); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "main.doclang")
	return Prepare(log, []byte(main), PrepareOptions{
		InputFile:     input,
		MaxInputBytes: util.DefaultMaxInputBytes,
		IncludeRoot:   dir,
		DocDir:        DocumentDir(input),
	})
}

// Un error de parseo en un archivo incluido llega en el DiagnosticsError y
// en el log con el archivo y la línea de ese archivo.
func TestPrepare_ParseErrorInIncludedFile(t *testing.T) {
	log := &warnLogger{Logger: util.NewNoop()}
	_, err := prepareWithInclude(t, log, "---\ntitle: D\nmode: strict\n---\n\nSECTION \"A\"\n  TEXT \"x\"\n\n@include partes/p.doclang\n",
		"SECTION \"B\"\n  level: 99\n  TEXT \"y\"\n")
	var diagErr *DiagnosticsError
	if !errors.As(err, &diagErr) || diagErr.Stage != StageParse {
		t.Fatalf("want a parse DiagnosticsError, got %v", err)
	}
	found := false
	for _, d := range diagErr.Diagnostics {
		if d.IsError() {
			found = true
			if d.File != "partes/p.doclang" || d.Position.Line != 2 {
				t.Errorf("parse error at %s:%d, want partes/p.doclang:2", d.File, d.Position.Line)
			}
		}
	}
	if !found {
		t.Fatalf("no error in %v", diagErr.Diagnostics)
	}
	if !strings.Contains(strings.Join(log.lines, "\n"), "partes/p.doclang:2") {
		t.Errorf("the log does not name partes/p.doclang:2: %v", log.lines)
	}
}

// Un aviso del parser en un archivo incluido que la política sube a error
// llega en el DiagnosticsError de lint con su archivo y su línea.
func TestPrepare_LintErrorInIncludedFile(t *testing.T) {
	log := &warnLogger{Logger: util.NewNoop()}
	_, err := prepareWithInclude(t, log, "---\ntitle: D\nmode: flex\nlint_policy:\n  rules:\n    TABLE004:\n      severity: error\n---\n\n# A\n\n@include partes/p.doclang\n",
		"## T\n\nTABLE\n  cells:\n    - [ : ]\n")
	var diagErr *DiagnosticsError
	if !errors.As(err, &diagErr) || diagErr.Stage != StageLint {
		t.Fatalf("want a lint DiagnosticsError, got %v", err)
	}
	found := false
	for _, d := range diagErr.Diagnostics {
		if d.RuleID == "TABLE004" {
			found = true
			if d.File != "partes/p.doclang" || d.Position.Line != 3 || !d.IsError() {
				t.Errorf("TABLE004 at %s:%d (error %v), want an error at partes/p.doclang:3", d.File, d.Position.Line, d.IsError())
			}
		}
	}
	if !found {
		t.Fatalf("no TABLE004 in %v", diagErr.Diagnostics)
	}
	if !strings.Contains(strings.Join(log.lines, "\n"), "ERROR LINT: Error at partes/p.doclang:3") {
		t.Errorf("the log does not name partes/p.doclang:3: %v", log.lines)
	}
}

// Los LangRuns que Prepare deriva antes del lint (los ve un rulepack
// externo en el AST) dejan literal el código, como el HTML (issue #487).
func TestPrepare_LangRunsKeepCodeLiteral(t *testing.T) {
	src := "---\ntitle: D\nmode: flex\n---\n\n# A\n\nHola [say `{{title}}` and {{title}}]{lang=en}.\n"
	prepared, err := Prepare(util.NewNoop(), []byte(src), PrepareOptions{InputFile: "d.doclang", MaxInputBytes: util.DefaultMaxInputBytes})
	if err != nil {
		t.Fatal(err)
	}
	var runs []string
	for _, b := range prepared.Doc.ContentBlocks {
		for _, e := range b.Elements {
			if text, ok := e.(*ast.TextElement); ok {
				for _, r := range text.LangRuns {
					runs = append(runs, r.Text)
				}
			}
		}
	}
	all := strings.Join(runs, "|")
	if all != "say `{{title}}` and D" {
		t.Errorf("LangRuns = %q, want the code span literal and the prose substituted", all)
	}
}

// Un panic al expandir los @include (con KeepIncludesInCode core parsea el
// raíz y cada archivo incluido) sale como error de Prepare, como uno del
// parser, en vez de tumbar el proceso.
func TestPrepare_PanicWhileExpandingIsAnError(t *testing.T) {
	old := readIncludedFile
	readIncludedFile = func(string) ([]byte, error) { panic("boom") }
	t.Cleanup(func() { readIncludedFile = old })
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("the panic escaped Prepare: %v", r)
			}
		}()
		_, err = prepareWithInclude(t, util.NewNoop(), "---\ntitle: D\nmode: flex\n---\n\n# A\n\n@include partes/p.doclang\n", "## B\n")
	}()
	if err == nil || !strings.Contains(err.Error(), "failed to expand includes") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("want the panic as an include error, got %v", err)
	}
}

// La columna de INCLUDE001 es la del `@`, también con espacios Unicode
// delante (los que core recorta al reconocer la directiva). Cuenta en bytes,
// como las demás posiciones de core.
func TestKeptIncludeDiagnosticsColumn(t *testing.T) {
	lines := []string{"    @include a", "     @include a", "    　@include a", "\t@include a"}
	diags := keptIncludeDiagnostics(lines, []int{1, 2, 3, 4})
	for i, d := range diags {
		want := strings.Index(lines[i], "@") + 1
		if d.Position.Line != i+1 || d.Position.Column != want {
			t.Errorf("%q: position %v, want %d:%d", lines[i], d.Position, i+1, want)
		}
		if !strings.Contains(d.Message, `"@include a"`) {
			t.Errorf("%q: message %q", lines[i], d.Message)
		}
	}
}

// DedupeIncluded deja de cada hallazgo de un archivo incluido tantas copias
// como dio la expansión que más dio (la expansión es la línea expandida),
// y no toca los del documento.
func TestDedupeIncluded(t *testing.T) {
	at := func(file string, line int, msg string) diagnostics.Diagnostic {
		d := diagnostics.NewWarning(msg, diagnostics.NewPosition(line, 1), "linter")
		d.File = file
		return d
	}
	// raw: la línea en el contenido expandido; mapped: la del archivo. b.md
	// está anidado en a.md, que se incluyó dos veces (expansiones en las
	// líneas 5 y 12 de b.md:2).
	raw := []diagnostics.Diagnostic{at("", 5, "x"), at("", 5, "x"), at("", 3, "doc"), at("", 3, "doc"), at("", 12, "x"), at("", 12, "x"), at("", 12, "x"), at("", 9, "y")}
	mapped := []diagnostics.Diagnostic{at("b.md", 2, "x"), at("b.md", 2, "x"), at("", 3, "doc"), at("", 3, "doc"), at("b.md", 2, "x"), at("b.md", 2, "x"), at("b.md", 2, "x"), at("a.md", 4, "y")}
	got := DedupeIncluded(raw, mapped)
	var summary []string
	for _, d := range got {
		summary = append(summary, d.File+":"+strconv.Itoa(d.Position.Line)+" "+d.Message)
	}
	want := "b.md:2 x|b.md:2 x|:3 doc|:3 doc|b.md:2 x|a.md:4 y"
	if strings.Join(summary, "|") != want {
		t.Errorf("DedupeIncluded = %s, want %s", strings.Join(summary, "|"), want)
	}
}

// linkFindingsDelta.apply quita del lint una copia de cada hallazgo que
// agregó el rebase y le suma los que escondió; sin cambios, devuelve el lint
// tal cual.
func TestLinkFindingsDeltaApply(t *testing.T) {
	w := func(msg string, line int) diagnostics.Diagnostic {
		return diagnostics.NewWarning(msg, diagnostics.NewPosition(line, 1), "linter").WithRuleID("LINK001")
	}
	lint := []diagnostics.Diagnostic{w("a", 1), w("added", 2), w("added", 2), w("b", 3)}
	got := linkFindingsDelta{hidden: []diagnostics.Diagnostic{w("hidden", 4)}, added: []diagnostics.Diagnostic{w("added", 2)}}.apply(lint)
	var msgs []string
	for _, d := range got {
		msgs = append(msgs, d.Message)
	}
	if strings.Join(msgs, ",") != "a,added,b,hidden" {
		t.Errorf("apply = %v, want a,added,b,hidden", msgs)
	}
	if same := (linkFindingsDelta{}).apply(lint); len(same) != len(lint) {
		t.Errorf("an empty delta changed the lint: %v", same)
	}
}
