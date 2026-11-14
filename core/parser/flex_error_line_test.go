// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// errorLines devuelve las líneas (1-based) de los diagnósticos de severidad
// Error cuyo mensaje contiene sub.
func errorLines(diags []diagnostics.Diagnostic, sub string) []int {
	var lines []int
	for _, d := range diags {
		if d.IsError() && strings.Contains(d.Message, sub) {
			lines = append(lines, d.Position.Line)
		}
	}
	return lines
}

func wantErrorLine(t *testing.T, diags []diagnostics.Diagnostic, sub string, want int) {
	t.Helper()
	got := errorLines(diags, sub)
	if len(got) != 1 || got[0] != want {
		t.Errorf("error %q: want one error on line %d, got lines %v (diags: %+v)", sub, want, got, diags)
	}
}

const (
	flexFenceMsg   = "takes at most a language and a filename"
	flexMermaidMsg = "unknown <<mermaid>> attribute"
)

// El error de un elemento que consume varias líneas se ancla en la línea donde
// abre, no en la que queda después de consumirlo (antes salía 3 líneas más
// abajo para una cerca de 3 líneas y 1 más abajo para <<mermaid>>).
func TestDocumentFlex_ElementErrorAnchoredAtOpeningLine(t *testing.T) {
	src := "---\nmode: flex\ntitle: \"F\"\n---\n\n# Doc\n\n## s1\n\n```python main.py extra30\nprint(1)\n```\n\n## after\n\n<<mermaid theme=\"dark\">>\ngraph TD\nA-->B\n<<end>>\n\ntexto\n"
	_, diags := NewDocumentFlexParser(src, util.NewNoop()).Parse()
	wantErrorLine(t, diags, flexFenceMsg, 10)
	wantErrorLine(t, diags, flexMermaidMsg, 16)
}

// Con el cuerpo desplazado por el frontmatter (lineOffset) y con un node-id
// delante del elemento, la línea sigue siendo la del archivo.
func TestParseDocument_FlexElementErrorLineWithNodeID(t *testing.T) {
	src := "---\nmode: flex\ntitle: \"F\"\n---\n\n# Doc\n\n## s1\n\n<!-- node-id: Fence -->\n```python main.py extra30\nprint(1)\n```\n"
	_, diags := New(util.NewNoop()).ParseDocument(src, "d.doclang")
	wantErrorLine(t, diags, flexFenceMsg, 11)

	plain := strings.Replace(src, "<!-- node-id: Fence -->\n", "", 1)
	_, diags = New(util.NewNoop()).ParseDocument(plain, "d.doclang")
	wantErrorLine(t, diags, flexFenceMsg, 10)
}

func TestFlex_ElementErrorAnchoredAtOpeningLine(t *testing.T) {
	src := "---\nmode: flex\ntitle: \"F\"\n---\n\n# Deck\n\n## s1\n\n```python main.py extra30\nprint(1)\n```\n\n## after\n\n<<mermaid theme=\"dark\">>\ngraph TD\nA-->B\n<<end>>\n"
	_, diags := NewFlexParser(src, util.NewNoop()).Parse()
	wantErrorLine(t, diags, flexFenceMsg, 10)
	wantErrorLine(t, diags, flexMermaidMsg, 16)
}

// Mismo caso por la entrada de documentos del Parser (frontmatter ya
// consumido: lineOffset != 0) para presentaciones.
func TestFlex_ElementErrorLineWithNodeID(t *testing.T) {
	src := "---\nmode: flex\ntitle: \"F\"\n---\n\n# Deck\n\n## s1\n\n<!-- node-id: Fence -->\n```python main.py extra30\nprint(1)\n```\n"
	_, diags := New(util.NewNoop()).Parse(src, "d.slidelang")
	wantErrorLine(t, diags, flexFenceMsg, 11)
}

// Los elementos anidados en un callout o en una columna ya anclaban en su
// línea (lo hacen los parsers de grid y special_block, con la línea del
// elemento); se fija aquí para que ningún cambio del bucle de arriba los mueva.
func TestFlex_NestedElementErrorsStayOnTheirLine(t *testing.T) {
	src := "---\nmode: flex\ntitle: \"F\"\n---\n\n# Doc\n\n## s1\n\n::: note\n```python main.py extra30\nprint(1)\n```\n<<mermaid theme=\"dark\">>\ngraph TD\nA-->B\n<<end>>\n:::\n\n::: grid\n::: column typed\n```python main.py extra30\nprint(1)\n```\n<<mermaid theme=\"dark\">>\ngraph TD\nA-->B\n<<end>>\n:::\n:::\n"
	_, diags := NewDocumentFlexParser(src, util.NewNoop()).Parse()
	var fence, mermaid []int
	for _, d := range diags {
		switch {
		case strings.Contains(d.Message, flexFenceMsg):
			fence = append(fence, d.Position.Line)
		case strings.Contains(d.Message, flexMermaidMsg):
			mermaid = append(mermaid, d.Position.Line)
		}
	}
	if len(fence) != 2 || fence[0] != 11 || fence[1] != 22 {
		t.Errorf("fence diagnostics: want lines [11 22], got %v", fence)
	}
	if len(mermaid) != 2 || mermaid[0] != 14 || mermaid[1] != 25 {
		t.Errorf("mermaid diagnostics: want lines [14 25], got %v", mermaid)
	}
}
