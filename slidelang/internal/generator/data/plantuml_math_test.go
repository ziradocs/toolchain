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

// TestPrepareTemplateData_PlantUMLElement covers issue #38: a PlantUMLElement
// must reach ElementData with its fields populated (including the SVG/PNG
// request URLs precomputed via core's exported encoder, since the template
// itself never builds them), not the near-empty struct the converter's
// default case produced before this type had its own case.
func TestPrepareTemplateData_PlantUMLElement(t *testing.T) {
	astDoc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{{
			BlockType: "content",
			Elements: []ast.Element{
				&ast.PlantUMLElement{DiagramType: "sequence", Content: "A->B: hola", Title: "Flujo"},
			},
		}},
	}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	el := got.ContentBlocks[0].Elements[0]

	if el.DiagramType != "sequence" {
		t.Errorf("DiagramType = %q, want %q", el.DiagramType, "sequence")
	}
	if el.Title != "Flujo" {
		t.Errorf("Title = %q, want %q", el.Title, "Flujo")
	}
	if el.PlantUMLSVGURL == "" || !strings.Contains(el.PlantUMLSVGURL, "/svg/") {
		t.Errorf("expected a /svg/ PlantUML URL, got %q", el.PlantUMLSVGURL)
	}
	if el.PlantUMLPNGURL == "" || !strings.Contains(el.PlantUMLPNGURL, "/png/") {
		t.Errorf("expected a /png/ PlantUML URL, got %q", el.PlantUMLPNGURL)
	}
}

// TestPrepareTemplateData_MathElement covers issue #38: a MathElement must
// reach ElementData with Content (raw LaTeX), MathLabel/MathNumber (the xref
// mechanism), and Caption populated.
func TestPrepareTemplateData_MathElement(t *testing.T) {
	astDoc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{{
			BlockType: "content",
			Elements: []ast.Element{
				&ast.MathElement{Content: "E = mc^2", Label: "eq:einstein", Number: 1, Caption: "Equivalencia masa-energía"},
			},
		}},
	}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	el := got.ContentBlocks[0].Elements[0]

	if el.Content != "E = mc^2" {
		t.Errorf("Content = %q, want %q", el.Content, "E = mc^2")
	}
	if el.MathLabel != "eq:einstein" {
		t.Errorf("MathLabel = %q, want %q", el.MathLabel, "eq:einstein")
	}
	if el.MathNumber != 1 {
		t.Errorf("MathNumber = %d, want %d", el.MathNumber, 1)
	}
	if el.Caption != "Equivalencia masa-energía" {
		t.Errorf("Caption = %q, want %q", el.Caption, "Equivalencia masa-energía")
	}
}

// TestPrepareTemplateData_MathElement_NoLabelStaysUnnumbered covers the
// unlabeled path: without Label/Number, the template's number span must not
// render — MathNumber staying at its zero value is what the {{if and
// .MathLabel (gt .MathNumber 0)}} guard in template/base.go relies on.
func TestPrepareTemplateData_MathElement_NoLabelStaysUnnumbered(t *testing.T) {
	astDoc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{{
			BlockType: "content",
			Elements: []ast.Element{
				&ast.MathElement{Content: "x^2 + y^2 = z^2"},
			},
		}},
	}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	el := got.ContentBlocks[0].Elements[0]

	if el.MathLabel != "" || el.MathNumber != 0 {
		t.Errorf("expected no label/number for an unlabeled equation, got MathLabel=%q MathNumber=%d", el.MathLabel, el.MathNumber)
	}
}
