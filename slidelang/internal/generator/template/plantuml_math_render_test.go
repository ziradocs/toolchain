// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	htmltemplate "html/template"
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

// TestElementTemplate_Math_PreRenderedHTMLTakesPriorityOverRawLatex covers
// the code-review finding on PR #56: in offline/PDF modes,
// data/converter.go's renderOfflineMath populates PreRenderedHTML with a
// pre-typeset SVG. The template must prefer it over the raw \[...\] LaTeX
// (which nothing typesets once buildCDNIncludes strips the MathJax <script>
// in offline mode) — without this branch, PDF/offline output showed the
// untypeset LaTeX source instead of the equation.
func TestElementTemplate_Math_PreRenderedHTMLTakesPriorityOverRawLatex(t *testing.T) {
	tmpl := mustParseElementTemplate(t)

	got := executeElement(t, tmpl, data.ElementData{
		Type:            "math",
		Content:         "E = mc^2",
		PreRenderedHTML: htmltemplate.HTML(`<div class="slidelang-math-diagram slidelang-math-inline"><svg><path d="M0 0"/></svg></div>`),
	})

	if !strings.Contains(got, `<path d="M0 0"/>`) {
		t.Errorf("expected the pre-rendered SVG to appear, got: %s", got)
	}
	if strings.Contains(got, `\[E = mc^2\]`) {
		t.Errorf("expected the raw LaTeX fallback to be suppressed when PreRenderedHTML is set, got: %s", got)
	}
}

// TestElementTemplate_PlantUML_PreRenderedHTMLTakesPriorityOverRemoteObject
// is the PlantUML side of the same finding: with PreRenderedHTML set, the
// template must NOT emit the <object data="..."> pointing at a remote
// PlantUML server — that's exactly what made PDF/offline-inline output
// depend on network access despite claiming to be self-contained.
func TestElementTemplate_PlantUML_PreRenderedHTMLTakesPriorityOverRemoteObject(t *testing.T) {
	tmpl := mustParseElementTemplate(t)

	got := executeElement(t, tmpl, data.ElementData{
		Type:           "plantuml",
		DiagramType:    "sequence",
		PlantUMLSVGURL: "https://www.plantuml.com/plantuml/svg/abc123",
		PlantUMLPNGURL: "https://www.plantuml.com/plantuml/png/abc123",
		PreRenderedHTML: htmltemplate.HTML(
			`<svg class="slidelang-plantuml-diagram slidelang-plantuml-inline"><rect width="10" height="10"/></svg>`),
	})

	if !strings.Contains(got, `<rect width="10" height="10"/>`) {
		t.Errorf("expected the pre-rendered SVG to appear, got: %s", got)
	}
	if strings.Contains(got, "plantuml.com") {
		t.Errorf("expected no remote PlantUML URL when PreRenderedHTML is set, got: %s", got)
	}
}
