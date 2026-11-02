// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

// TestDocxUnavailableNote: misma regla de idioma que los marcadores de
// HTML/PDF: "en" con cualquier subetiqueta es inglés y lo demás, español.
func TestDocxUnavailableNote(t *testing.T) {
	for _, tc := range []struct {
		kind     docxUnavailableKind
		lang, is string
	}{
		{docxUnavailableMap, "", "Mapa no disponible"},
		{docxUnavailableMap, "en-GB", "Map not available"},
		{docxUnavailableChart, "es", "Gráfica no disponible"},
		{docxUnavailableChart, "EN", "Chart not available"},
		{docxUnavailableDiagram, "fr", "Diagrama no disponible"},
		{docxUnavailableDiagram, "en_US", "Diagram not available"},
		{docxUnavailableEquation, "", "Ecuación no disponible"},
		{docxUnavailableEquation, "en", "Equation not available"},
		{docxUnavailableImage, "", "Imagen no disponible"},
		{docxUnavailableImage, "en", "Image not available"},
	} {
		if got := docxUnavailableNote(tc.kind, tc.lang); got != tc.is {
			t.Errorf("docxUnavailableNote(%d, %q) = %q, want %q", tc.kind, tc.lang, got, tc.is)
		}
	}
}

// TestDOCX_PlaceholdersFollowDocumentLang: sin Chromium, un mermaid, una
// ecuación, una imagen que falta y un video sin fuente dejan marcadores en
// el idioma del documento y sin el detalle técnico de antes ("Mermaid
// Diagram: flowchart (not rendered: Chromium not available)", "[Image not
// found: …]"), que queda en el aviso.
func TestDOCX_PlaceholdersFollowDocumentLang(t *testing.T) {
	for lang, want := range map[string][]string{
		"es": {"[Diagrama no disponible]", "[Ecuación no disponible]", "[Imagen no disponible: Logo]", "[video sin fuente]"},
		"en": {"[Diagram not available]", "[Equation not available]", "[Image not available: Logo]", "[video without a source]"},
	} {
		pos := diagnostics.NewPosition(1, 1)
		doc := astWithElements(
			ast.NewMermaidElement(pos, "flowchart", "graph TD; A-->B"),
			ast.NewMathElement(pos, "E = mc^2"),
			ast.NewImageElement(pos, "falta.png", "Logo"),
			ast.NewMediaElement(pos, "video", ""),
		)
		doc.FrontMatter = ast.NewFrontMatterNode(pos)
		doc.FrontMatter.Lang = lang
		logger := newTestLogger()
		gen := NewDOCXGenerator(logger, t.TempDir())
		out := filepath.Join(t.TempDir(), "out.docx")
		if err := gen.Generate(doc, out, GeneratorOptions{Format: "docx", ChromiumPath: filepath.Join(t.TempDir(), "no-chromium")}); err != nil {
			t.Fatal(err)
		}
		if gen.chromiumRenderer != nil {
			t.Skip("a Chromium was found anyway; this test needs it missing")
		}
		text := docxVisibleText(docxDocumentXML(t, out))
		for _, w := range want {
			if !strings.Contains(text, w) {
				t.Errorf("lang %s: missing %q in %q", lang, w, text)
			}
		}
		for _, old := range []string{"Mermaid Diagram", "render failed", "Chromium", "not found", "could not be inserted"} {
			if strings.Contains(text, old) {
				t.Errorf("lang %s: old technical text %q in the document: %q", lang, old, text)
			}
		}
		if len(logger.warns) < 3 {
			t.Errorf("lang %s: the details must stay in the warnings, got %v", lang, logger.warns)
		}
	}
}

// Un chart que no se puede dibujar sin Chromium deja su marcador; antes se
// omitía sin nada en su lugar.
func TestDOCX_ChartWithoutChromiumLeavesPlaceholder(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	chart := ast.NewChartElement(pos, "radar")
	if _, ok, err := renderer.RenderChartNativePNG(chart, 800, 500); ok && err == nil {
		t.Skip("this chart type renders natively; the test needs one that does not")
	}
	doc := astWithElements(chart)
	logger := newTestLogger()
	gen := NewDOCXGenerator(logger, t.TempDir())
	out := filepath.Join(t.TempDir(), "out.docx")
	if err := gen.Generate(doc, out, GeneratorOptions{Format: "docx", ChromiumPath: filepath.Join(t.TempDir(), "no-chromium")}); err != nil {
		t.Fatal(err)
	}
	if gen.chromiumRenderer != nil {
		t.Skip("a Chromium was found anyway; this test needs it missing")
	}
	if text := docxVisibleText(docxDocumentXML(t, out)); !strings.Contains(text, docxUnavailableNote(docxUnavailableChart, "")) {
		t.Errorf("chart placeholder missing: %q", text)
	}
}

// El título o la leyenda de un diagrama, una ecuación o un chart sale debajo
// de su marcador cuando no se pudo dibujar; antes se perdía con el elemento.
func TestDOCX_PlaceholdersKeepCaptions(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	mermaid := ast.NewMermaidElement(pos, "flowchart", "graph TD; A-->B")
	mermaid.Title = "S-mermaid-title"
	math := ast.NewMathElement(pos, "E = mc^2")
	math.Caption = "S-math-caption"
	chart := ast.NewChartElement(pos, "radar")
	chart.Title = "S-chart-title"
	doc := astWithElements(mermaid, math, chart)
	gen := NewDOCXGenerator(newTestLogger(), t.TempDir())
	out := filepath.Join(t.TempDir(), "out.docx")
	if err := gen.Generate(doc, out, GeneratorOptions{Format: "docx", ChromiumPath: filepath.Join(t.TempDir(), "no-chromium")}); err != nil {
		t.Fatal(err)
	}
	if gen.chromiumRenderer != nil {
		t.Skip("a Chromium was found anyway; this test needs it missing")
	}
	text := docxVisibleText(docxDocumentXML(t, out))
	for _, want := range []string{"S-mermaid-title", "S-math-caption", "S-chart-title"} {
		if !strings.Contains(text, want) {
			t.Errorf("%s missing next to its placeholder: %q", want, text)
		}
	}
}
