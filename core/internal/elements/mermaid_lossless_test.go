// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func TestMermaidParser_PreservesOpaqueBodies(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		lines    []string
		want     string
		typeName string
	}{
		{
			name: "strict mindmap indentation", mode: "strict",
			lines: []string{"<<mermaid>>", "    mindmap", "      root((Producto))", "        Mercado", "          Segmentos", "<<end>>"},
			want:  "mindmap\n  root((Producto))\n    Mercado\n      Segmentos", typeName: "mindmap",
		},
		{
			name: "flex frontmatter", mode: "flex",
			lines: []string{"```mermaid", "---", "config:", "  theme: forest", "---", "flowchart LR", "  A --> B", "```"},
			want:  "---\nconfig:\n  theme: forest\n---\nflowchart LR\n  A --> B", typeName: "flowchart",
		},
		{
			name: "quadrant does not become flowchart", mode: "flex",
			lines: []string{"```mermaid", "quadrantChart", "  x-axis Bajo --> Alto", "```"},
			want:  "quadrantChart\n  x-axis Bajo --> Alto", typeName: "quadrantchart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&MermaidParser{}).Parse(&ParseContext{Mode: tt.mode, Lines: tt.lines}, 0)
			elem, ok := got.Element.(*ast.MermaidElement)
			if !ok {
				t.Fatalf("element = %T, want *ast.MermaidElement", got.Element)
			}
			if elem.Content != tt.want {
				t.Errorf("content = %q, want %q", elem.Content, tt.want)
			}
			if elem.DiagramType != tt.typeName {
				t.Errorf("diagramType = %q, want %q", elem.DiagramType, tt.typeName)
			}
		})
	}
}
