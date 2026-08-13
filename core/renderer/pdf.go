// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "context"

// PDFOptions configura la generación de PDF. Vive en core/renderer, no en
// core/renderer/chromium: describe una PÁGINA (tamaño, márgenes, header/
// footer), no un navegador, así que un caller no debería tener que importar
// el paquete chromium solo para nombrar este tipo. Antes de este seam,
// doclang y slidelang importaban chromium únicamente para eso — ver
// PDFBackend más abajo, que es lo que de verdad habilita un motor
// alternativo (Typst, chrome-headless-shell) sin tocar los call sites.
type PDFOptions struct {
	// Paper size (inches)
	PaperWidth  float64
	PaperHeight float64
	Landscape   bool

	// Margins (inches)
	MarginTop    float64
	MarginBottom float64
	MarginLeft   float64
	MarginRight  float64

	// Header/Footer
	DisplayHeaderFooter bool
	HeaderTemplate      string
	FooterTemplate      string

	// Scale
	Scale float64
}

// DefaultPDFOptions retorna opciones por defecto (A4, portrait).
func DefaultPDFOptions() PDFOptions {
	return PDFOptions{
		PaperWidth:   8.27,  // A4 width in inches
		PaperHeight:  11.69, // A4 height in inches
		Landscape:    false,
		MarginTop:    0.4,
		MarginBottom: 0.4,
		MarginLeft:   0.4,
		MarginRight:  0.4,
		Scale:        1.0,
	}
}

// PDFBackend convierte HTML ya renderizado a PDF. Satisfecha
// estructuralmente por *chromium.ChromiumRenderer (evita el ciclo de import
// renderer -> chromium -> renderer, mismo patrón que las interfaces de
// fetcher en fetchers.go) y por cualquier motor alternativo que un caller
// quiera enchufar en su lugar en el futuro.
type PDFBackend interface {
	RenderHTMLToPDF(ctx context.Context, htmlContent string, outputPath string, opts PDFOptions) error
}
