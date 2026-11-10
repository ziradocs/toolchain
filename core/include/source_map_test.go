// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// El SourceMap da archivo y línea de origen de cada línea expandida, contando
// el frontmatter que el archivo incluido tenía aunque la expansión lo quite.
func TestExpandWithSourceMap_FileAndLine(t *testing.T) {
	a, b := abs(t, "/base/a.md"), abs(t, "/base/b.md")
	files := map[string]string{
		a: "---\ntitle: A\n---\nA1\n@include b.md\nA2",
		b: "B1\nB2",
	}
	text, sm, err := ExpandWithSourceMap("R1\n@include a.md\nR2", abs(t, "/base"), memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := Expand("R1\n@include a.md\nR2", abs(t, "/base"), memFS(files)); text != want {
		t.Fatalf("text %q differs from Expand %q", text, want)
	}
	want := SourceMap{Lines: []Origin{
		{"", 1},
		{a, 4}, {b, 1}, {b, 2}, {a, 6},
		{"", 3},
	}, RootLines: 3}
	if !reflect.DeepEqual(sm, want) {
		t.Errorf("source map\n got %v\nwant %v", sm, want)
	}
	_, origins, _ := ExpandWithOrigins("R1\n@include a.md\nR2", abs(t, "/base"), memFS(files))
	if !reflect.DeepEqual(origins, sm.Files()) {
		t.Errorf("ExpandWithOrigins %v, want %v", origins, sm.Files())
	}
}

// Los diagnósticos del parser después de un @include apuntan a su línea en el
// archivo que el autor escribió: los del contenido raíz a su línea ahí (antes
// salían corridos tantas líneas como trajo el include), los de un archivo
// incluido a ese archivo, relativo al directorio del documento, y su línea.
func TestSourceMap_MapDiagnosticsFromTheParser(t *testing.T) {
	files := map[string]string{
		abs(t, "/base/partes/part.doclang"): "  TEXT\n    uno\n  RARO incluido\n",
	}
	root := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n@include partes/part.doclang\n  RARO raiz\n"
	text, sm, err := ExpandWithSourceMap(root, abs(t, "/base"), memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	_, diags := parser.New(util.NewNoop()).ParseDocument(text, "d.doclang")
	mapped := sm.MapDiagnostics(diags, abs(t, "/base"))
	var gotIncluded, gotRoot *diagnostics.Diagnostic
	for i := range mapped {
		switch {
		case strings.Contains(mapped[i].Message, "RARO incluido"):
			gotIncluded = &mapped[i]
		case strings.Contains(mapped[i].Message, "RARO raiz"):
			gotRoot = &mapped[i]
		}
	}
	if gotIncluded == nil || gotIncluded.File != "partes/part.doclang" || gotIncluded.Position.Line != 3 {
		t.Errorf("included diagnostic: want partes/part.doclang:3, got %+v", gotIncluded)
	}
	if gotRoot == nil || gotRoot.File != "" || gotRoot.Position.Line != 8 {
		t.Errorf("root diagnostic: want line 8 of the root, got %+v", gotRoot)
	}
	for _, d := range diags {
		if d.File != "" {
			t.Fatalf("MapDiagnostics must not modify its input: %+v", d)
		}
	}
}

func TestSourceMap_MapDiagnosticsEdges(t *testing.T) {
	b := abs(t, "/b.md")
	sm := SourceMap{Lines: []Origin{{"", 1}, {b, 1}, {b, 2}, {"", 3}}, RootLines: 3}
	end := diagnostics.NewPosition(3, 2)
	ranged := diagnostics.NewError("rango", diagnostics.NewPosition(2, 1), "t").WithEndPosition(end)
	already := diagnostics.NewError("ya mapeado", diagnostics.NewPosition(2, 1), "t")
	already.File = "otro.md"
	in := []diagnostics.Diagnostic{
		diagnostics.NewError("sin posición", diagnostics.NewPosition(0, 0), "t"),
		diagnostics.NewError("después del final", diagnostics.NewPosition(9, 1), "t"),
		ranged,
		diagnostics.NewError("cruza", diagnostics.NewPosition(3, 1), "t").WithEndPosition(diagnostics.NewPosition(4, 1)),
		already,
	}
	out := sm.MapDiagnostics(in, abs(t, "/"))
	if out[0].Position.Line != 0 || out[0].File != "" {
		t.Errorf("a diagnostic without a position must stay: %+v", out[0])
	}
	// La línea 9 está 5 después de la última expandida (la 4); el raíz
	// tiene 3 líneas, así que es la 8 del raíz.
	if out[1].Position.Line != 8 || out[1].File != "" {
		t.Errorf("past the end is the end of the root: want root line 8, got %+v", out[1])
	}
	if out[2].File != "b.md" || out[2].Position.Line != 1 || out[2].EndPosition == nil || out[2].EndPosition.Line != 2 {
		t.Errorf("a range inside one file maps both ends: %+v", out[2])
	}
	if out[3].File != "b.md" || out[3].Position.Line != 2 || out[3].EndPosition != nil {
		t.Errorf("a range that crosses files drops its end: %+v", out[3])
	}
	if out[4].File != "otro.md" || out[4].Position.Line != 2 {
		t.Errorf("a diagnostic that already has File must stay: %+v", out[4])
	}
	if in[2].EndPosition != ranged.EndPosition || in[2].EndPosition.Line != 3 || out[2].EndPosition == in[2].EndPosition {
		t.Errorf("MapDiagnostics must not modify the input EndPosition: in %+v out %+v", in[2].EndPosition, out[2].EndPosition)
	}
	if in[1].Position.Line != 9 || in[2].Position.Line != 2 || in[2].File != "" {
		t.Errorf("MapDiagnostics must not modify its input: %+v", in)
	}
	if got := (SourceMap{}).MapDiagnostics([]diagnostics.Diagnostic{diagnostics.NewError("x", diagnostics.NewPosition(2, 1), "t")}, ""); got[0].File != "" || got[0].Position.Line != 2 {
		t.Errorf("an empty map leaves positions alone: %+v", got[0])
	}
}

// Un raíz que termina en un @include, sin salto de línea final: la última
// línea expandida es del archivo incluido, pero el fin de archivo es el del
// raíz, que es donde termina la expansión.
func TestSourceMap_EndOfFileWhenTheRootEndsInAnInclude(t *testing.T) {
	b := abs(t, "/base/b.md")
	for _, child := range []string{"B1\nB2", "B1\nB2\n"} {
		files := map[string]string{b: child}
		text, sm, err := ExpandWithSourceMap("R1\n@include b.md", abs(t, "/base"), memFS(files))
		if err != nil {
			t.Fatal(err)
		}
		n := len(strings.Split(text, "\n"))
		if last, _ := sm.At(n); last.File != b {
			t.Fatalf("%q: last expanded line should come from b.md, got %v", child, last)
		}
		eof := diagnostics.NewError("eof", diagnostics.NewPosition(n+1, 1), "t")
		got := sm.MapDiagnostics([]diagnostics.Diagnostic{eof}, abs(t, "/base"))[0]
		if got.File != "" || got.Position.Line != 3 {
			t.Errorf("%q: end of file: want root line 3, got %q:%d", child, got.File, got.Position.Line)
		}
	}
}

// Diagnostic.File es relativo al directorio del documento raíz y con "/";
// sin directorio, o si no hay ruta relativa, va la absoluta con "/".
func TestSourceMap_MapDiagnosticsFilePaths(t *testing.T) {
	docDir := abs(t, "/docs")
	inside := abs(t, "/docs/partes/a.md")
	outside := abs(t, "/comun/pie.md")
	sm := SourceMap{Lines: []Origin{{inside, 1}, {outside, 1}}, RootLines: 1}
	diags := []diagnostics.Diagnostic{
		diagnostics.NewError("a", diagnostics.NewPosition(1, 1), "t"),
		diagnostics.NewError("b", diagnostics.NewPosition(2, 1), "t"),
	}
	got := sm.MapDiagnostics(diags, docDir)
	if got[0].File != "partes/a.md" || got[1].File != "../comun/pie.md" {
		t.Errorf("relative to the document directory: got %q, %q", got[0].File, got[1].File)
	}
	got = sm.MapDiagnostics(diags, "")
	if got[0].File != filepath.ToSlash(inside) || strings.Contains(got[0].File, "\\") {
		t.Errorf("without a document directory: got %q, want %q", got[0].File, filepath.ToSlash(inside))
	}
}

// Un archivo incluido que es solo frontmatter, sin salto de línea al final,
// deja una línea vacía que no está en él: su origen es la directiva @include.
// ExpandWithOrigins cambia igual para esa línea.
func TestExpandWithSourceMap_FrontMatterOnlyInclude(t *testing.T) {
	only := abs(t, "/base/solo.md")
	files := map[string]string{only: "---\ntitle: x\n---"}
	text, sm, err := ExpandWithSourceMap("R1\n@include solo.md\nR3", abs(t, "/base"), memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if text != "R1\n\nR3" {
		t.Fatalf("text %q", text)
	}
	if want := []Origin{{"", 1}, {"", 2}, {"", 3}}; !reflect.DeepEqual(sm.Lines, want) {
		t.Errorf("source map\n got %v\nwant %v", sm.Lines, want)
	}
	if _, origins, _ := ExpandWithOrigins("R1\n@include solo.md\nR3", abs(t, "/base"), memFS(files)); origins.File(2) != "" {
		t.Errorf("ExpandWithOrigins line 2 = %q, want the root", origins.File(2))
	}

	// Con salto de línea final la línea vacía sí es del archivo (la 4).
	files[only] = "---\ntitle: x\n---\n"
	_, sm, err = ExpandWithSourceMap("R1\n@include solo.md\nR3", abs(t, "/base"), memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if want := []Origin{{"", 1}, {only, 4}, {"", 3}}; !reflect.DeepEqual(sm.Lines, want) {
		t.Errorf("source map\n got %v\nwant %v", sm.Lines, want)
	}
}

// checkSourceMap verifica el contrato de SourceMap (ver su documentación)
// para un texto expandido desde root con los archivos de files (rutas
// absolutas): una línea por origen, cada línea igual a la de su origen
// (salvo la línea vacía de un include de solo frontmatter, cuyo origen es la
// directiva) y RootLines igual a las líneas de root. Sirve para cualquier
// camino de expansión que devuelva un SourceMap, también uno con opciones
// (p. ej. KeepIncludesInCode): checkSourceMap(t, name, root, files,
// exp.Text, exp.Sources).
func checkSourceMap(t *testing.T, name, root string, files map[string]string, text string, sm SourceMap) {
	t.Helper()
	lines := strings.Split(text, "\n")
	if len(lines) != len(sm.Lines) {
		t.Fatalf("%s: %d lines, %d origins", name, len(lines), len(sm.Lines))
	}
	if want := len(strings.Split(root, "\n")); sm.RootLines != want {
		t.Errorf("%s: RootLines = %d, want %d", name, sm.RootLines, want)
	}
	for i, l := range lines {
		o := sm.Lines[i]
		src, ok := root, true
		if o.File != "" {
			src, ok = files[o.File]
		}
		if !ok {
			t.Errorf("%s: line %d -> %v, unknown file", name, i+1, o)
			continue
		}
		sl := strings.Split(src, "\n")
		if o.Line < 1 || o.Line > len(sl) {
			t.Errorf("%s: line %d %q -> %v, out of range", name, i+1, l, o)
			continue
		}
		if got := sl[o.Line-1]; got != l {
			if _, isInclude := parseIncludeLine(got); isInclude && l == "" {
				continue
			}
			t.Errorf("%s: line %d %q -> %v, which is %q", name, i+1, l, o, got)
		}
	}
}

// sourceMapCorpus son raíces y archivos para checkSourceMap: frontmatter en
// el raíz y en los incluidos, @include en el frontmatter, en código y
// anidado, CRLF, sin frontmatter y un include de solo frontmatter.
func sourceMapCorpus(t *testing.T) (map[string]string, map[string]string) {
	a, bf, fm, only := abs(t, "/base/a.doclang"), abs(t, "/base/b.doclang"), abs(t, "/base/fm.yaml"), abs(t, "/base/solo.doclang")
	files := map[string]string{
		a:    "---\nt: x\n---\n  TEXT\n    from a\n  CODE text\n    @include b.doclang\n    in code\n  @include b.doclang\n  TEXT\n    after b",
		bf:   "---\n---\n  TEXT\n    from b\n",
		fm:   "title: \"T\"",
		only: "---\ntitle: solo\n---",
	}
	roots := map[string]string{
		"strict":       "---\nmode: strict\n@include fm.yaml\n---\n\nSECTION \"A\"\n  CODE text\n    @include b.doclang\n@include a.doclang\n  TEXT\n    tail\n",
		"flex":         "---\nmode: flex\n---\n# A\n\n```\n@include b.doclang\n```\n\n@include a.doclang\nend",
		"nofm":         "# A\n@include a.doclang\n```\n@include b.doclang\n```",
		"crlf":         "---\r\nmode: strict\r\n---\r\nSECTION \"A\"\r\n@include b.doclang\r\n  TEXT\r\n    z\r\n",
		"solo":         "# A\n@include solo.doclang\n@include a.doclang\n",
		"ends-include": "# A\n@include b.doclang",
	}
	return files, roots
}

// Cada línea expandida es la que su origen dice, en el archivo que su origen
// dice (el contrato de SourceMap), sobre la API pública.
func TestExpandWithSourceMap_EveryLineMatchesItsOrigin(t *testing.T) {
	files, roots := sourceMapCorpus(t)
	for name, root := range roots {
		text, sm, err := ExpandWithSourceMap(root, abs(t, "/base"), memFS(files))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		checkSourceMap(t, name, root, files, text, sm)
	}
}

// abs es p (con "/") como ruta absoluta del sistema, para que las pruebas
// de rutas valgan también en Windows.
func abs(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(filepath.FromSlash(p))
	if err != nil {
		t.Fatal(err)
	}
	return a
}
