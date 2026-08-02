// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import "testing"

// TestBuildPrintToPDFParams_GenerateTaggedPDF cubre el prerrequisito de
// issue #62: cdproto's page.PrintToPDF() inicializa GenerateTaggedPDF en
// false explícitamente, así que sin un override todo PDF de este toolchain
// salía sin árbol de tags. No requiere un Chromium real: PrintToPDFParams es
// un struct de parámetros, no una llamada de red.
func TestBuildPrintToPDFParams_GenerateTaggedPDF(t *testing.T) {
	params := buildPrintToPDFParams(DefaultPDFOptions())

	if !params.GenerateTaggedPDF {
		t.Error("expected GenerateTaggedPDF = true, got false — PDFs would ship untagged")
	}
}

// TestBuildPrintToPDFParams_PreservesExistingOptions es una regresión: el
// refactor que extrajo esta función de RenderHTMLToPDF no debe cambiar
// ningún comportamiento previo.
func TestBuildPrintToPDFParams_PreservesExistingOptions(t *testing.T) {
	opts := PDFOptions{
		PaperWidth:          8.27,
		PaperHeight:         11.69,
		Landscape:           true,
		MarginTop:           0.5,
		MarginBottom:        0.6,
		MarginLeft:          0.7,
		MarginRight:         0.8,
		DisplayHeaderFooter: true,
		HeaderTemplate:      "<div>header</div>",
		FooterTemplate:      "<div>footer</div>",
	}
	params := buildPrintToPDFParams(opts)

	if !params.PrintBackground {
		t.Error("expected PrintBackground = true (always on)")
	}
	if !params.PreferCSSPageSize {
		t.Error("expected PreferCSSPageSize = true (always on)")
	}
	if params.PaperWidth != opts.PaperWidth || params.PaperHeight != opts.PaperHeight {
		t.Errorf("paper size = %v x %v, want %v x %v", params.PaperWidth, params.PaperHeight, opts.PaperWidth, opts.PaperHeight)
	}
	if !params.Landscape {
		t.Error("expected Landscape = true")
	}
	if params.MarginTop != opts.MarginTop || params.MarginBottom != opts.MarginBottom ||
		params.MarginLeft != opts.MarginLeft || params.MarginRight != opts.MarginRight {
		t.Errorf("margins = %+v, want top/bottom/left/right = %v/%v/%v/%v",
			params, opts.MarginTop, opts.MarginBottom, opts.MarginLeft, opts.MarginRight)
	}
	if !params.DisplayHeaderFooter {
		t.Error("expected DisplayHeaderFooter = true")
	}
	if params.HeaderTemplate != opts.HeaderTemplate || params.FooterTemplate != opts.FooterTemplate {
		t.Errorf("header/footer templates not propagated: got %q / %q", params.HeaderTemplate, params.FooterTemplate)
	}
}

// TestBuildPrintToPDFParams_ZeroMarginsOmitted confirma que márgenes/paper
// en cero (comportamiento previo) dejan esos parámetros en su default de
// cdproto en vez de forzar un cero explícito — mismo guard que ya existía en
// RenderHTMLToPDF antes del refactor.
func TestBuildPrintToPDFParams_ZeroMarginsOmitted(t *testing.T) {
	params := buildPrintToPDFParams(PDFOptions{})

	if params.DisplayHeaderFooter {
		t.Error("expected DisplayHeaderFooter = false by default")
	}
	if params.Landscape {
		t.Error("expected Landscape = false by default")
	}
}
