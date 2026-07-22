// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

func singleMermaidAST() *ast.AST {
	return &ast.AST{
		ContentBlocks: []ast.ContentBlock{{
			BlockType: "content",
			Elements: []ast.Element{
				&ast.MermaidElement{DiagramType: "graph", Content: "graph TD; A-->B"},
			},
		}},
	}
}

// TestPrepareTemplateData_BrowserLeavesPreRenderedEmpty: en modo browser no se
// pre-renderiza (PreRenderedHTML vacío) y la metadata client-side se puebla.
func TestPrepareTemplateData_BrowserLeavesPreRenderedEmpty(t *testing.T) {
	data := PrepareTemplateDataWithRenderMode(singleMermaidAST(), "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	el := data.ContentBlocks[0].Elements[0]
	if el.PreRenderedHTML != "" {
		t.Errorf("browser mode should leave PreRenderedHTML empty, got %q", el.PreRenderedHTML)
	}
	if len(data.Diagrams) != 1 {
		t.Errorf("browser mode should populate Diagrams metadata, got %d", len(data.Diagrams))
	}
}

// TestPrepareTemplateData_OfflineInjectsPreRenderedAndSkipsMetadata: en modo
// offline, mermaid/chart/map se pre-renderizan vía RenderElementToHTML y la
// metadata client-side se omite (issue #92). Se usa un RenderContext con fetcher
// nil: RenderElementToHTML devuelve un div de error determinista (no requiere
// Chromium), suficiente para verificar que la inyección y la supresión ocurren.
func TestPrepareTemplateData_OfflineInjectsPreRenderedAndSkipsMetadata(t *testing.T) {
	ctx := &renderer.RenderContext{MermaidMode: "offline-inline"}

	data := PrepareTemplateDataWithRenderMode(singleMermaidAST(), "default", "offline-inline", util.NewNoop(), ctx)
	el := data.ContentBlocks[0].Elements[0]
	if el.PreRenderedHTML == "" {
		t.Fatal("offline mode should inject PreRenderedHTML for a mermaid element")
	}
	if !strings.Contains(string(el.PreRenderedHTML), "mermaid") {
		t.Errorf("PreRenderedHTML should contain mermaid output, got %q", el.PreRenderedHTML)
	}
	if len(data.Diagrams) != 0 {
		t.Errorf("offline mode should skip client-side Diagrams metadata, got %d", len(data.Diagrams))
	}
}
