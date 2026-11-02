// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// La sección inicial sin encabezado sobrevive la transpilación en las dos
// direcciones. FormatDocumentStrict la escribe como `SECTION ""`; antes el
// parser strict leía ese primer SECTION como el bloque "title" y corría
// todos los bloques un lugar (el `#` real pasaba a ser un "content"
// numerado), sin aviso.

var headlessTranspileCases = map[string]string{
	"sin frontmatter con preámbulo": "Intro SENTINEL_P.\n\n## Antes\n\n# Título\n\nCuerpo.\n\n# Otra\n\nMás.\n",
	"solo h2":                       "## Solo dos\n\nTexto.\n\n### Tres\n\nMás.\n",
}

func parseDocOrFail(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("no parsea: %s\n%s", d.String(), src)
		}
	}
	return doc
}

type blockShape struct{ blockType, heading, title string }

func blockShapes(doc *ast.AST) []blockShape {
	var out []blockShape
	for _, b := range doc.ContentBlocks {
		out = append(out, blockShape{b.BlockType, b.Heading, b.Title})
	}
	return out
}

func TestFormatDocumentStrict_HeadlessLeadingSection_RoundTrips(t *testing.T) {
	for name, src := range headlessTranspileCases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "doc.doclang")
			flexDoc := parseDocOrFail(t, src)

			// flex → strict → reparse
			strictSrc, err := FormatDocumentStrict(flexDoc)
			if err != nil {
				t.Fatalf("FormatDocumentStrict: %v", err)
			}
			if !strings.Contains(strictSrc, "mode: strict") || !strings.Contains(strictSrc, `SECTION ""`) {
				t.Fatalf("salida strict inesperada:\n%s", strictSrc)
			}
			strictDoc := parseDocOrFail(t, strictSrc)
			if got, want := blockShapes(strictDoc), blockShapes(flexDoc); !equalShapes(got, want) {
				t.Fatalf("flex→strict cambió la estructura\nwant %+v\ngot  %+v\n%s", want, got, strictSrc)
			}
			want, _ := buildDocASTForComparison(t, src, path)
			got, ok := buildDocASTForComparison(t, strictSrc, path)
			if !ok {
				return
			}
			if diffs := diffASTs(want, got); len(diffs) > 0 {
				t.Fatalf("flex→strict cambió el AST:\n  %s\n%s", strings.Join(diffs, "\n  "), strictSrc)
			}

			// strict → flex → reparse. FormatDocument conserva el `mode:` del
			// frontmatter, así que transpilar de vuelta a flex es pasarle el
			// AST strict con el modo cambiado.
			toFlex := *strictDoc
			fmCopy := *strictDoc.FrontMatter
			fmCopy.Mode = "flex"
			toFlex.FrontMatter = &fmCopy
			flexSrc, err := FormatDocument(&toFlex)
			if err != nil {
				t.Fatalf("FormatDocument: %v", err)
			}
			if strings.HasPrefix(flexSrc, "# \n") || strings.Contains(flexSrc, "\n# \n") {
				t.Fatalf("encabezado vacío en la salida flex:\n%s", flexSrc)
			}
			back := parseDocOrFail(t, flexSrc)
			if got, want := blockShapes(back), blockShapes(flexDoc); !equalShapes(got, want) {
				t.Fatalf("strict→flex cambió la estructura\nwant %+v\ngot  %+v\n%s", want, got, flexSrc)
			}
			again, ok := buildDocASTForComparison(t, flexSrc, path)
			if !ok {
				return
			}
			if diffs := diffASTs(want, again); len(diffs) > 0 {
				t.Fatalf("strict→flex cambió el AST:\n  %s\n%s", strings.Join(diffs, "\n  "), flexSrc)
			}
		})
	}
}

func equalShapes(a, b []blockShape) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Un `SECTION ""` que NO es el primero conserva el comportamiento de
// siempre: es un bloque "content" con título vacío, y el primer SECTION con
// título sigue siendo el bloque "title".
func TestDocumentStrict_EmptyTitleOnlyHeadlessWhenFirst(t *testing.T) {
	doc := parseDocOrFail(t, "---\nmode: strict\n---\n\nSECTION \"Uno\"\n\n  TEXT\n    a\n\nSECTION \"\"\n\n  TEXT\n    b\n")
	want := []blockShape{{"title", "Uno", ""}, {"content", "", ""}}
	if got := blockShapes(doc); !equalShapes(got, want) {
		t.Errorf("want %+v, got %+v", want, got)
	}
}
