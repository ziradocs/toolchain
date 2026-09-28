// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func slideWith(layout string, els ...ast.Element) *ast.ContentBlock {
	b := ast.NewContentBlock(diagnostics.NewPosition(1, 1), layout)
	b.Title = "T"
	b.Elements = els
	return b
}

func metric(label, value string) *ast.MetricElement {
	m := ast.NewMetricElement(diagnostics.NewPosition(1, 1))
	m.Label, m.Value = label, value
	return m
}

func hasCode(diags []diagnostics.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code || d.RuleID == code {
			return true
		}
	}
	return false
}

// Las señales expresadas con elementos tipados ya no dan falsos positivos, y
// un slide que de verdad no las tiene sigue reportándose.
func TestLayoutRulesReadTypedContent(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	for _, tc := range []struct {
		name  string
		check func(*ast.ContentBlock) []diagnostics.Diagnostic
		code  string
		ok    *ast.ContentBlock
		bad   *ast.ContentBlock
	}{
		{"before/after with metrics", validateBeforeAfterSlide, "LAYOUT012",
			slideWith("before_after", metric("Before", "12 h"), metric("After", "2 h")),
			slideWith("before_after", ast.NewTextElement(pos, "Only one side."))},
		{"before/after with headings", validateBeforeAfterSlide, "LAYOUT012",
			slideWith("before_after", ast.NewHeadingElement(pos, 3, "Antes", "heading-antes"), ast.NewHeadingElement(pos, 3, "Después", "heading-despues")),
			slideWith("before_after", ast.NewHeadingElement(pos, 3, "Antes", "heading-antes"))},
		{"pricing with metric", validatePricingSlide, "LAYOUT013",
			slideWith("pricing", metric("Pro", "49 USD")),
			slideWith("pricing", ast.NewTextElement(pos, "Plans coming soon."))},
		{"cta in points", validateCallToActionSlide, "LAYOUT016",
			slideWith("call_to_action", &ast.PointsElement{BaseNode: ast.NewBaseNode(ast.NodeTypePoints, pos), Items: []ast.PointItem{*ast.NewPointItem(pos, "Get started today")}}),
			slideWith("call_to_action", ast.NewTextElement(pos, "Thanks for reading."))},
		{"cta as link", validateCallToActionSlide, "LAYOUT016",
			slideWith("call_to_action", ast.NewTextElement(pos, "[Docs](https://example.com)")),
			slideWith("call_to_action", metric("Users", "10k"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if hasCode(tc.check(tc.ok), tc.code) {
				t.Fatalf("false positive %s", tc.code)
			}
			if !hasCode(tc.check(tc.bad), tc.code) {
				t.Fatalf("true positive %s lost", tc.code)
			}
		})
	}
}

// Un slide de título con solo encabezado no está vacío; uno sin nada, sí.
func TestSlideNotEmptyConsidersHeading(t *testing.T) {
	titleOnly := ast.NewContentBlock(diagnostics.NewPosition(1, 1), "title")
	titleOnly.Heading = "Deck"
	if hasCode((&SlideNotEmptyRule{}).Check(titleOnly), "SLIDE002") {
		t.Fatal("heading-only slide reported as empty")
	}
	if !hasCode((&SlideNotEmptyRule{}).Check(ast.NewContentBlock(diagnostics.NewPosition(1, 1), "content")), "SLIDE002") {
		t.Fatal("really empty slide not reported")
	}
}
