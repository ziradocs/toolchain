// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
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
	files := map[string]string{
		"/base/a.md": "---\ntitle: A\n---\nA1\n@include b.md\nA2",
		"/base/b.md": "B1\nB2",
	}
	text, sm, err := ExpandWithSourceMap("R1\n@include a.md\nR2", "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := Expand("R1\n@include a.md\nR2", "/base", memFS(files)); text != want {
		t.Fatalf("text %q differs from Expand %q", text, want)
	}
	want := SourceMap{
		{"", 1},
		{"/base/a.md", 4}, {"/base/b.md", 1}, {"/base/b.md", 2}, {"/base/a.md", 6},
		{"", 3},
	}
	if !reflect.DeepEqual(sm, want) {
		t.Errorf("source map\n got %v\nwant %v", sm, want)
	}
	if got := strings.Split(text, "\n"); len(got) != len(sm) {
		t.Errorf("%d lines, %d origins", len(got), len(sm))
	}
	_, origins, _ := ExpandWithOrigins("R1\n@include a.md\nR2", "/base", memFS(files))
	if !reflect.DeepEqual(origins, sm.Files()) {
		t.Errorf("ExpandWithOrigins %v, want %v", origins, sm.Files())
	}
}

// Los diagnósticos del parser después de un @include apuntan a su línea en el
// archivo que el autor escribió: los del contenido raíz a su línea ahí (antes
// salían corridos tantas líneas como trajo el include), los de un archivo
// incluido a ese archivo y su línea.
func TestSourceMap_MapDiagnosticsFromTheParser(t *testing.T) {
	files := map[string]string{
		"/base/part.doclang": "  TEXT\n    uno\n  RARO incluido\n",
	}
	root := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n@include part.doclang\n  RARO raiz\n"
	text, sm, err := ExpandWithSourceMap(root, "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	_, diags := parser.New(util.NewNoop()).ParseDocument(text, "d.doclang")
	mapped := sm.MapDiagnostics(diags)
	var gotIncluded, gotRoot *diagnostics.Diagnostic
	for i := range mapped {
		switch {
		case strings.Contains(mapped[i].Message, "RARO incluido"):
			gotIncluded = &mapped[i]
		case strings.Contains(mapped[i].Message, "RARO raiz"):
			gotRoot = &mapped[i]
		}
	}
	if gotIncluded == nil || gotIncluded.File != "/base/part.doclang" || gotIncluded.Position.Line != 3 {
		t.Errorf("included diagnostic: want /base/part.doclang:3, got %+v", gotIncluded)
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
	sm := SourceMap{{"", 1}, {"/b.md", 1}, {"/b.md", 2}, {"", 3}}
	end := diagnostics.NewPosition(3, 2)
	in := []diagnostics.Diagnostic{
		diagnostics.NewError("sin posición", diagnostics.NewPosition(0, 0), "t"),
		diagnostics.NewError("fuera", diagnostics.NewPosition(9, 1), "t"),
		diagnostics.NewError("rango", diagnostics.NewPosition(2, 1), "t").WithEndPosition(end),
		diagnostics.NewError("cruza", diagnostics.NewPosition(3, 1), "t").WithEndPosition(diagnostics.NewPosition(4, 1)),
	}
	out := sm.MapDiagnostics(in)
	if out[0].Position.Line != 0 || out[0].File != "" || out[1].Position.Line != 9 || out[1].File != "" {
		t.Errorf("positions out of the map must stay: %+v %+v", out[0], out[1])
	}
	if out[2].File != "/b.md" || out[2].Position.Line != 1 || out[2].EndPosition == nil || out[2].EndPosition.Line != 2 {
		t.Errorf("a range inside one file maps both ends: %+v", out[2])
	}
	if out[3].File != "/b.md" || out[3].Position.Line != 2 || out[3].EndPosition != nil {
		t.Errorf("a range that crosses files drops its end: %+v", out[3])
	}
}
