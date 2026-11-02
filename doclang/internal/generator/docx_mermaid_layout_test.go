// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"strings"
	"testing"

	docx "github.com/mmonterroca/docxgo/v2"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

func pngOfSize(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.01 }

// TestFitMermaidPNG: el PNG trae 2 px por px CSS; a 96 px CSS por pulgada
// se inserta a su tamaño natural, nunca más grande, y reducido para caber
// en la caja de texto a lo ancho y a lo alto (menos la leyenda).
func TestFitMermaidPNG(t *testing.T) {
	g := &DOCXGenerator{contentWidthIn: 6.5, contentHeightIn: 9}

	// 480x240 px CSS → 5"x2.5": cabe, no se agranda.
	w, h, err := g.fitMermaidPNG(pngOfSize(t, 960, 480), false)
	if err != nil || !near(w, 5) || !near(h, 2.5) {
		t.Errorf("small diagram: got %.2fx%.2f (%v), want 5x2.5", w, h, err)
	}

	// Muy ancho: se limita al ancho de la caja.
	w, h, _ = g.fitMermaidPNG(pngOfSize(t, 2*1248, 2*312), false)
	if !near(w, 6.5) || !near(h, 1.625) {
		t.Errorf("wide diagram: got %.2fx%.2f, want 6.5x1.625", w, h)
	}

	// Muy alto: se limita al alto de página menos los márgenes de reserva.
	w, h, _ = g.fitMermaidPNG(pngOfSize(t, 2*300, 2*1920), true)
	wantH := 9 - figureSpacingReserveIn - figureCaptionReserveIn
	if !near(h, wantH) || !near(w, wantH*300/1920) {
		t.Errorf("tall diagram: got %.2fx%.2f, want height %.2f", w, h, wantH)
	}
}

func TestDiagramFileHash_DistinguishesSameType(t *testing.T) {
	theme := renderer.DiagramThemeColors{}
	a := diagramFileHash("flowchart", "graph TD; A-->B", theme)
	b := diagramFileHash("flowchart", "graph TD; C-->D", theme)
	if a == b {
		t.Fatal("two flowcharts with different content got the same temp file name")
	}
	if a != diagramFileHash("flowchart", "graph TD; A-->B", theme) {
		t.Fatal("hash is not deterministic")
	}
}

func TestDOCXContentBoxInches_DefaultSection(t *testing.T) {
	w, h := docxContentBoxInches(docx.NewDocument())
	// docxgo arranca en A4 (11906x16838 twips) con márgenes de 1".
	if !near(w, (11906.0-2880)/1440) || !near(h, (16838.0-2880)/1440) {
		t.Errorf("content box = %.2fx%.2f", w, h)
	}
}

// TestDOCXGenerator_MermaidWithoutChromium_WarnsAndLeavesPlaceholder: sin
// Chromium el diagrama ya no desaparece en silencio: queda un marcador en
// el documento y un aviso con la línea y cómo arreglarlo.
func TestDOCXGenerator_MermaidWithoutChromium_WarnsAndLeavesPlaceholder(t *testing.T) {
	logger := newTestLogger()
	g := NewDOCXGenerator(logger, "")
	g.style = GetStyleForTheme("professional", logger)
	doc := docx.NewDocument()
	elem := ast.NewMermaidElement(diagnostics.NewPosition(12, 1), "flowchart", "graph TD; A-->B")

	if err := g.renderMermaid(doc, elem); err != nil {
		t.Fatalf("renderMermaid: %v", err)
	}
	if len(logger.warns) != 1 || !strings.Contains(logger.warns[0], "line 12") || !strings.Contains(logger.warns[0], "--chromium-path") {
		t.Errorf("want one actionable warning naming the line, got %q", logger.warns)
	}
	found := false
	for _, p := range doc.Paragraphs() {
		if strings.Contains(p.Text(), "not rendered: Chromium not available") {
			found = true
		}
	}
	if !found {
		t.Error("no placeholder paragraph left in the document")
	}
}
