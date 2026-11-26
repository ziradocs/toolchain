// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"testing"

	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Un documento sin frontmatter enciende el normalizador, y su MermaidRule
// quita el ``` de cierre de cada bloque. Con bloques mermaid antes del
// @include y dentro del archivo incluido, el TABLE003 de una fila del
// incluido y el de una fila del documento después del @include salen cada
// uno en su archivo y su línea.
func TestNormalizedLinesMapThroughAnInclude(t *testing.T) {
	part := "## Parte\n\n```mermaid\ngraph TD\n  A-->B\n```\n\n| a | b |\n|---|---|\n| 1 | 2 | 3 |\n"
	files := map[string]string{abs(t, "/base/part.doclang"): part}
	root := "# Doc\n\n```mermaid\ngraph TD\n  C-->D\n```\n\n```mermaid\ngraph TD\n  E-->F\n```\n\n@include part.doclang\n\n## Fin\n\n| x | y |\n|---|---|\n| 4 | 5 | 6 |\n"
	text, sm, err := ExpandWithSourceMap(root, abs(t, "/base"), memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	doc, diags := parser.New(util.NewNoop()).ParseDocument(text, "d.doclang")
	all := append(diags, linter.New().Lint(doc)...)
	got := map[string]int{}
	for _, d := range sm.MapDiagnostics(all, abs(t, "/base")) {
		if d.RuleID == "TABLE003" {
			got[d.File] = d.Position.Line
		}
	}
	want := map[string]int{"part.doclang": 10, "": 19}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("TABLE003 in %q: want line %d, got %d (all: %v)", k, w, got[k], got)
		}
	}
}
