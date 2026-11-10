// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// El cuerpo de un diagrama llega al AST con su sangría relativa por
// ParseDocument (con la normalización de flex encendida), sangrado con tabs
// o con espacios: un mindmap depende de ella.
func TestParseDocument_DiagramBodiesKeepRelativeIndentation(t *testing.T) {
	src := "---\ntitle: D\nmode: flex\n---\n\n# A\n\n" +
		"<<mermaid>>\n\tmindmap\n\t\troot((Proyecto))\n\t\t\tAlcance\n\t\t\tRiesgos\n\n" +
		"```mermaid\nmindmap\n  root((Cerca))\n    Alcance\n```\n\n" +
		"```plantuml\n@startuml\nclass A {\n  +x\n}\n@enduml\n```\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d)
		}
	}
	var got []string
	for _, b := range doc.ContentBlocks {
		for _, e := range b.Elements {
			switch el := e.(type) {
			case *ast.MermaidElement:
				got = append(got, el.Content)
			case *ast.PlantUMLElement:
				got = append(got, el.Content)
			}
		}
	}
	want := []string{
		"mindmap\n\troot((Proyecto))\n\t\tAlcance\n\t\tRiesgos",
		"mindmap\n  root((Cerca))\n    Alcance",
		"@startuml\nclass A {\n  +x\n}\n@enduml",
	}
	if len(got) != len(want) {
		t.Fatalf("diagrams = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("diagram %d = %q, want %q", i, got[i], want[i])
		}
	}
}
