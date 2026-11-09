// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// codeContents parsea src como documento (o presentación) y devuelve el
// contenido de cada bloque de código, en orden.
func codeContents(t *testing.T, src string, document bool) []string {
	t.Helper()
	p := parser.New(util.NewNoop())
	var doc *ast.AST
	if document {
		doc, _ = p.ParseDocument(src, "d.doclang")
	} else {
		doc, _ = p.Parse(src, "d.slidelang")
	}
	var out []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch c := n.(type) {
		case *ast.CodeElement:
			out = append(out, c.Content)
		case *ast.CodeGroupElement:
			for _, b := range c.CodeBlocks {
				out = append(out, b.Content)
			}
		}
		return nil
	})
	return out
}

// Issue #487: un `@include` dentro de un cuerpo de código es código. Expand lo
// deja como está, así que el texto que parsea el build da el mismo bloque de
// código que el texto sin expandir (el que ven get_ast y lint).
func TestExpand_CodeBodiesAreLiteral(t *testing.T) {
	files := map[string]string{
		"/base/other.txt": "    INCLUDED-LINE-1\n    INCLUDED-LINE-2\n",
	}
	strictDoc := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n"
	for _, tc := range []struct {
		name     string
		src      string
		document bool
	}{
		{"strict CODE", strictDoc + "  CODE\n    @include other.txt\n", true},
		{"strict CODE with language", strictDoc + "  CODE yaml\n    a: 1\n\n    @include other.txt\n", true},
		{"strict CODE{verbatim}", strictDoc + "  CODE{verbatim} txt\n    @include other.txt\n", true},
		{"strict CODE, nbsp before the directive", strictDoc + "  CODE\n     @include other.txt\n", true},
		{"strict CODE, ideographic space before the directive", strictDoc + "  CODE\n    　@include other.txt\n", true},
		{"strict code-group", strictDoc + "  :::code-group\n  ```txt [a]\n  @include other.txt\n  ```\n  :::\n", true},
		{"flex fence", "# A\n\n```txt\n@include other.txt\n```\n", true},
		{"slide strict CODE", "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  CODE\n    @include other.txt\n", false},
		{"slide flex fence", "---\nmode: flex\ntitle: T\n---\n\n# A\n\n```\n@include other.txt\n```\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Expand(tc.src, "/base", memFS(files))
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			if got != tc.src {
				t.Errorf("a code body must not be expanded:\n%s", got)
			}
			before := codeContents(t, tc.src, tc.document)
			if len(before) == 0 || !strings.Contains(strings.Join(before, "\n"), "@include other.txt") {
				t.Fatalf("fixture: the parser should keep the directive as code, got %q", before)
			}
		})
	}
}

// Fuera del código nada cambia: un `@include` a nivel de elemento después de
// un CODE se sigue expandiendo, y uno dentro de un diagrama también (no es
// código; su cuerpo puede venir de un archivo).
func TestExpand_OutsideCodeStillExpands(t *testing.T) {
	files := map[string]string{
		"/base/frag.doclang": "  TEXT\n    Incluido.\n",
		"/base/flow.mmd":     "    A-->B\n",
	}
	strictDoc := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n"
	for _, tc := range []struct {
		name, src, want string
	}{
		{"after a CODE body", strictDoc + "  CODE\n    x = 1\n\n@include frag.doclang\n", strictDoc + "  CODE\n    x = 1\n\n  TEXT\n    Incluido.\n\n"},
		{"at the CODE line's own indentation", strictDoc + "  CODE\n    x = 1\n  @include frag.doclang\n", strictDoc + "  CODE\n    x = 1\n  TEXT\n    Incluido.\n\n"},
		{"inside a diagram", strictDoc + "  <<mermaid>>\n    graph TD\n    @include flow.mmd\n  <<end>>\n", strictDoc + "  <<mermaid>>\n    graph TD\n    A-->B\n\n  <<end>>\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Expand(tc.src, "/base", memFS(files))
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			if got != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

// Qué es código se decide sobre el texto ya expandido: un `@include` escrito
// dentro del cuerpo de un CODE de un archivo incluido queda literal, y uno que
// se expande justo después de un CODE no cambia dónde termina ese cuerpo.
func TestExpand_CodeRegionsFollowTheExpandedText(t *testing.T) {
	files := map[string]string{
		"/base/with-code.doclang": "  CODE\n    @include secret.txt\n",
		"/base/secret.txt":        "    SECRET\n",
		"/base/frag.doclang":      "  TEXT\n    Incluido.\n",
	}
	strictDoc := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n"
	got, err := Expand(strictDoc+"@include with-code.doclang\n@include frag.doclang\n", "/base", memFS(files))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := strictDoc + "  CODE\n    @include secret.txt\n\n  TEXT\n    Incluido.\n\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
	if strings.Contains(got, "SECRET") {
		t.Error("an @include inside an included CODE body was expanded")
	}
}

// Los orígenes de línea siguen alineados con el texto expandido cuando parte
// de él es código que no se expandió.
func TestExpandWithOrigins_CodeBodyKeepsLineMap(t *testing.T) {
	files := map[string]string{
		"/base/frag.doclang": "  TEXT\n    Incluido.\n",
	}
	src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  CODE\n    @include frag.doclang\n\n@include frag.doclang\n"
	expanded, origins, err := ExpandWithOrigins(src, "/base", memFS(files))
	if err != nil {
		t.Fatalf("ExpandWithOrigins: %v", err)
	}
	lines := strings.Split(expanded, "\n")
	if len(origins) != len(lines) {
		t.Fatalf("%d origins for %d lines", len(origins), len(lines))
	}
	for i, line := range lines {
		wantFile := ""
		if i >= 9 && i <= 11 {
			wantFile = "/base/frag.doclang"
		}
		if origins[i] != wantFile {
			t.Errorf("line %d %q: origin %q, want %q", i+1, line, origins[i], wantFile)
		}
	}
}
