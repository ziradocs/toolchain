// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"strings"
	"testing"

	"go.ziradocs.com/slidelang/v2/internal/generator/data"
)

// TestElementTemplate_PlantUML_RendersObjectAndFallback covers issue #38: the
// "plantuml" branch must emit the <object>/<img> pair pointing at the
// SVG/PNG URLs precomputed in converter.go (data/converter.go's
// PrepareTemplateDataWithRenderMode), and a <h2> title when Title is set.
func TestElementTemplate_PlantUML_RendersObjectAndFallback(t *testing.T) {
	tmpl := mustParseElementTemplate(t)

	got := executeElement(t, tmpl, data.ElementData{
		Type:           "plantuml",
		DiagramType:    "sequence",
		Title:          "Flujo de saludo",
		PlantUMLSVGURL: "https://www.plantuml.com/plantuml/svg/abc123",
		PlantUMLPNGURL: "https://www.plantuml.com/plantuml/png/abc123",
	})

	if !strings.Contains(got, "Flujo de saludo") {
		t.Errorf("expected the title to render, got: %s", got)
	}
	if !strings.Contains(got, `data="https://www.plantuml.com/plantuml/svg/abc123"`) {
		t.Errorf("expected the SVG URL in the <object> data attribute, got: %s", got)
	}
	if !strings.Contains(got, `src="https://www.plantuml.com/plantuml/png/abc123"`) {
		t.Errorf("expected the PNG URL in the fallback <img> src attribute, got: %s", got)
	}
}

// TestElementTemplate_Math_RendersDelimitedLatexWithNumberAndCaption covers
// issue #38: the "math" branch must wrap Content in \[...\] delimiters (the
// convention MathJax's tex-svg.js CDN bundle expects), show the equation
// number only when both MathLabel and MathNumber are set (mirroring
// core/renderer/html.go's renderMathElement), and show the caption when set.
func TestElementTemplate_Math_RendersDelimitedLatexWithNumberAndCaption(t *testing.T) {
	tmpl := mustParseElementTemplate(t)

	got := executeElement(t, tmpl, data.ElementData{
		Type:       "math",
		Content:    "E = mc^2",
		MathLabel:  "eq:einstein",
		MathNumber: 1,
		Caption:    "Equivalencia masa-energía",
	})

	if !strings.Contains(got, `\[E = mc^2\]`) {
		t.Errorf("expected the LaTeX content wrapped in \\[...\\] delimiters, got: %s", got)
	}
	if !strings.Contains(got, "(1)") {
		t.Errorf("expected the equation number (1), got: %s", got)
	}
	if !strings.Contains(got, "Equivalencia masa-energía") {
		t.Errorf("expected the caption, got: %s", got)
	}
}

// TestElementTemplate_Math_NoLabelOmitsNumber covers the unlabeled path: no
// MathLabel/MathNumber means no equation-number span at all, not "(0)".
func TestElementTemplate_Math_NoLabelOmitsNumber(t *testing.T) {
	tmpl := mustParseElementTemplate(t)

	got := executeElement(t, tmpl, data.ElementData{
		Type:    "math",
		Content: "x^2 = z^2 - y^2",
	})

	if strings.Contains(got, "slidelang-math-number") {
		t.Errorf("expected no equation-number span for an unlabeled equation, got: %s", got)
	}
	if !strings.Contains(got, `\[x^2 = z^2 - y^2\]`) {
		t.Errorf("expected the LaTeX content wrapped in \\[...\\] delimiters, got: %s", got)
	}
}

// TestElementTemplate_Math_ContentIsHTMLEscaped is the security-relevant
// regression: Content is raw LaTeX from the author (not markdown, not
// pre-escaped in converter.go — see the case comment there), so
// html/template's own auto-escaping, not a manual EscapeHTML call, is what
// must keep a "<script>" in the LaTeX source from reaching the page
// unescaped. Same invariant core/renderer/math_html.go's BuildMathDiv
// enforces explicitly (issue #73); here the template engine enforces it
// structurally instead.
func TestElementTemplate_Math_ContentIsHTMLEscaped(t *testing.T) {
	tmpl := mustParseElementTemplate(t)

	got := executeElement(t, tmpl, data.ElementData{
		Type:    "math",
		Content: `<script>alert(1)</script>`,
	})

	if strings.Contains(got, "<script>alert(1)</script>") {
		t.Fatalf("LaTeX content was not HTML-escaped, XSS risk: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("expected the LaTeX content to be HTML-escaped, got: %s", got)
	}
}
