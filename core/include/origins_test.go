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

// TestExpandWithOrigins_LineMap: cada línea expandida dice de qué archivo
// viene, con anidamiento y con el frontmatter del incluido ya descartado.
// El texto es el mismo que devuelve Expand.
func TestExpandWithOrigins_LineMap(t *testing.T) {
	files := map[string]string{
		"/base/a/uno.md": "---\ntitle: Uno\n---\nu1\n@include b/dos.md\nu2",
		"/base/b/dos.md": "d1\nd2",
	}
	content := "r1\n@include a/uno.md\nr2"

	got, origins, err := ExpandWithOrigins(content, "/base", memFS(files))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	plain, err := Expand(content, "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Errorf("el texto difiere de Expand:\ngot  %q\nwant %q", got, plain)
	}

	wantLines := []string{"r1", "u1", "d1", "d2", "u2", "r2"}
	wantFiles := []string{"", "/base/a/uno.md", "/base/b/dos.md", "/base/b/dos.md", "/base/a/uno.md", ""}
	if lines := strings.Split(got, "\n"); strings.Join(lines, ",") != strings.Join(wantLines, ",") {
		t.Fatalf("líneas = %q, want %q", lines, wantLines)
	}
	if len(origins) != len(wantFiles) {
		t.Fatalf("len(origins) = %d, want %d", len(origins), len(wantFiles))
	}
	for i, want := range wantFiles {
		if f := origins.File(i + 1); f != want {
			t.Errorf("línea %d (%q): origen %q, want %q", i+1, wantLines[i], f, want)
		}
	}
	if origins.File(0) != "" || origins.File(len(origins)+1) != "" {
		t.Error("File fuera de rango debe devolver \"\"")
	}
}

// TestExpandWithOrigins_ErrorsMatchExpand: los errores son los de Expand.
func TestExpandWithOrigins_ErrorsMatchExpand(t *testing.T) {
	for _, content := range []string{"@include", "@include ../fuera.md", "@include /abs.md", "@include falta.md"} {
		_, errPlain := Expand(content, "/base", memFS(nil))
		_, origins, err := ExpandWithOrigins(content, "/base", memFS(nil))
		if err == nil || errPlain == nil || err.Error() != errPlain.Error() {
			t.Errorf("%q: error %v, Expand dio %v", content, err, errPlain)
		}
		if origins != nil {
			t.Errorf("%q: con error, origins debe ser nil", content)
		}
	}
}

// TestExpandWithOrigins_ParserPositions: la posición que el parser le da a
// un elemento cuenta líneas sobre el contenido expandido, frontmatter de la
// raíz incluido, así que cruzarla con los orígenes encuentra el archivo que
// escribió cada imagen. Frontmatter en la raíz y en los incluidos, un
// @include anidado (que, como todos, se resuelve desde la raíz) y los dos
// dialectos de documentos.
func TestExpandWithOrigins_ParserPositions(t *testing.T) {
	cases := map[string]struct {
		root  string
		files map[string]string
	}{
		"flex": {
			root: "---\ntitle: Raíz\n---\n\n# Raíz\n\n![Raiz](raiz.png)\n\n@include a/uno.md\n\n@include b/dos.md\n",
			files: map[string]string{
				"/base/a/uno.md":  "---\ntitle: Uno\nauthor: X\n---\n\n## Uno\n\nTexto.\n\n![Uno](uno.png)\n",
				"/base/b/dos.md":  "---\ntitle: Dos\n---\n\n## Dos\n\n![Dos](dos.png)\n\n@include a/tres.md\n",
				"/base/a/tres.md": "Párrafo.\n\n![Tres](tres.png)\n",
			},
		},
		"strict": {
			root: "---\ntitle: Raíz\nmode: strict\n---\n\nSECTION \"Raíz\"\n  IMAGE \"raiz.png\" \"r\"\n\n@include a/uno.md\n\n@include b/dos.md\n",
			files: map[string]string{
				"/base/a/uno.md":  "---\nmode: strict\n---\nSECTION \"Uno\"\n\n  IMAGE \"uno.png\" \"u\"\n",
				"/base/b/dos.md":  "SECTION \"Dos\"\n  IMAGE \"dos.png\" \"d\"\n\n@include a/tres.md\n",
				"/base/a/tres.md": "SECTION \"Tres\"\n\n  IMAGE \"tres.png\" \"t\"\n",
			},
		},
	}
	want := map[string]string{
		"raiz.png": "",
		"uno.png":  "/base/a/uno.md",
		"dos.png":  "/base/b/dos.md",
		"tres.png": "/base/a/tres.md",
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			expanded, origins, err := ExpandWithOrigins(tc.root, "/base", memFS(tc.files))
			if err != nil {
				t.Fatal(err)
			}
			doc, diags := parser.New(util.NewNoop()).ParseDocument(expanded, "raiz.md")
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("parse error: %s\n%s", d.Message, expanded)
				}
			}
			seen := map[string]string{}
			_ = ast.Walk(doc, func(n ast.Node) error {
				if img, ok := n.(*ast.ImageElement); ok {
					seen[img.Source] = origins.File(img.GetPosition().Line)
				}
				return nil
			})
			for src, wantFile := range want {
				got, ok := seen[src]
				if !ok {
					t.Errorf("%s: no apareció en el AST (vistas: %v)", src, seen)
					continue
				}
				if got != wantFile {
					t.Errorf("%s: origen %q, want %q", src, got, wantFile)
				}
			}
		})
	}
}
