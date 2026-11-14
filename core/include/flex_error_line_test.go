// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// El error de un elemento flex dentro de un archivo incluido apunta a la línea
// donde abre ese elemento en el archivo incluido, no a la que sigue después de
// consumirlo, y el remapeo de @include conserva el archivo.
func TestFlexElementErrorLineInsideAnInclude(t *testing.T) {
	part := "## Parte\n\ntexto\n\n```python main.py extra30\nprint(1)\n```\n\n<<mermaid theme=\"dark\">>\ngraph TD\nA-->B\n<<end>>\n"
	files := map[string]string{abs(t, "/base/part.doclang"): part}
	root := "---\nmode: flex\ntitle: \"D\"\n---\n\n# Doc\n\n@include part.doclang\n\n## Fin\n\n```go bad extra\nx\n```\n"
	text, sm, err := ExpandWithSourceMap(root, abs(t, "/base"), memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	_, diags := parser.New(util.NewNoop()).ParseDocument(text, "d.doclang")
	mapped := sm.MapDiagnostics(diags, abs(t, "/base"))
	got := map[string]int{}
	for _, d := range mapped {
		if !d.IsError() {
			continue
		}
		switch {
		case strings.Contains(d.Message, "takes at most a language and a filename"):
			got["fence@"+d.File] = d.Position.Line
		case strings.Contains(d.Message, "unknown <<mermaid>> attribute"):
			got["mermaid@"+d.File] = d.Position.Line
		}
	}
	want := map[string]int{"fence@part.doclang": 5, "mermaid@part.doclang": 9, "fence@": 12}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: want line %d, got %d (all: %v)", k, w, got[k], got)
		}
	}
}
