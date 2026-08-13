// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// pdf_tagging_output_test.go verifica el tagging en el PDF PRODUCIDO, no en
// los parámetros que se le pidieron al motor.
//
// La distinción no es académica. pdf_options_test.go afirma que
// buildPrintToPDFParams pone GenerateTaggedPDF=true, y eso es todo lo que
// afirmaba este repo hasta ahora: un motor de PDF que acepte el parámetro y
// lo ignore (o un chrome-headless-shell corriendo el pipeline de impresión
// legacy, o un backend alternativo enchufado por renderer.PDFBackend) pasaría
// ese test con los PDFs saliendo sin árbol de tags, sin /Lang y sin orden de
// lectura — o sea sin nada de lo que hace alcanzables las cláusulas PDF/UA
// que el crosswalk de accesibilidad cita. Este archivo cierra ese hueco, y es
// el gate que cualquier swap de motor tiene que pasar antes de considerarse
// equivalente.
//
// Sin dependencias externas a propósito: Chrome no mete el catálogo en object
// streams comprimidos (no hay /ObjStm en su salida), así que los marcadores
// viven en texto plano dentro del PDF y un escaneo de bytes basta. No hace
// falta qpdf/mutool instalados en CI, que era el otro camino posible.
//
// Verificado empíricamente que la aserción tiene dientes: forzando
// WithGenerateTaggedPDF(false) en buildPrintToPDFParams, los tres marcadores
// desaparecen del PDF y este test falla.

// taggedPDFHTML trae la estructura mínima que un árbol de tags debe reflejar
// —heading, párrafo, lista y tabla con header— más lang en el <html>, que es
// de donde Chrome deriva el /Lang del StructElem raíz.
const taggedPDFHTML = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Tagging fixture</title></head>
<body>
  <h1>Document title</h1>
  <p>A paragraph of body text.</p>
  <ul><li>First item</li><li>Second item</li></ul>
  <table>
    <thead><tr><th>Column</th></tr></thead>
    <tbody><tr><td>Value</td></tr></tbody>
  </table>
</body>
</html>`

func TestRenderHTMLToPDF_OutputIsActuallyTagged(t *testing.T) {
	r := newTestChromiumRenderer(t)

	outputPath := filepath.Join(t.TempDir(), "tagged.pdf")
	if err := r.RenderHTMLToPDF(context.Background(), taggedPDFHTML, outputPath, DefaultPDFOptions()); err != nil {
		t.Fatalf("RenderHTMLToPDF: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading generated PDF: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (first bytes: %q)", data[:min(8, len(data))])
	}

	// Los tres marcadores que distinguen un PDF tageado de uno que solo pidió
	// serlo. /MarkInfo con /Marked true es la declaración del documento;
	// /StructTreeRoot es la raíz del árbol; /StructElem confirma que el árbol
	// tiene nodos de verdad y no está vacío.
	for _, marker := range []struct{ needle, why string }{
		{"/MarkInfo", "the document does not declare itself as marked content"},
		{"/Marked true", "/MarkInfo is present but does not set /Marked true"},
		{"/StructTreeRoot", "there is no structure tree root — the PDF carries no tag tree"},
		{"/StructElem", "the structure tree has no elements — an empty tree is not a reading order"},
	} {
		if !bytes.Contains(data, []byte(marker.needle)) {
			t.Errorf("generated PDF is missing %s: %s", marker.needle, marker.why)
		}
	}
}

// TestRenderHTMLToPDF_OutputCarriesLang cubre /Lang aparte porque su origen
// es distinto: no lo activa GenerateTaggedPDF sino el atributo lang del
// <html>, y un renderer que pierda el atributo por el camino produciría un
// PDF tageado pero sin idioma declarado — falla de PDF/UA aunque el árbol de
// tags esté bien formado.
func TestRenderHTMLToPDF_OutputCarriesLang(t *testing.T) {
	r := newTestChromiumRenderer(t)

	outputPath := filepath.Join(t.TempDir(), "lang.pdf")
	if err := r.RenderHTMLToPDF(context.Background(), taggedPDFHTML, outputPath, DefaultPDFOptions()); err != nil {
		t.Fatalf("RenderHTMLToPDF: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading generated PDF: %v", err)
	}

	if !bytes.Contains(data, []byte("/Lang (en)")) {
		t.Error(`generated PDF does not declare /Lang (en) — the lang attribute on <html> did not survive into the PDF`)
	}
}
