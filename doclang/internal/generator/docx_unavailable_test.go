// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
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
