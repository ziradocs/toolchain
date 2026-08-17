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

// TestBuildPrintToPDFParams_ZeroValueDefaults confirma el comportamiento de
// un PDFOptions{} vacío para los campos SIN el contrato especial de Scale
// (ver TestBuildPrintToPDFParams_OnlyZeroMeansUnset para ese). Reemplaza a
// TestBuildPrintToPDFParams_ZeroMarginsOmitted (issue #165): ese nombre y su
// doc comment prometían verificar que márgenes/paper en cero quedaran "en su
// default de cdproto en vez de forzar un cero explícito" — pero el body
// nunca inspeccionaba ni una margen ni el paper size, solo
// DisplayHeaderFooter/Landscape/Scale. La promesa además era falsa: cdproto
// declara los campos de margen sin `omitempty` (page/page.go), así que un
// PDFOptions{} SIEMPRE serializa margen 0 explícito hacia Chromium, exista o
// no una llamada a WithMarginTop — nunca hubo un "default de cdproto" al que
// caer.
func TestBuildPrintToPDFParams_ZeroValueDefaults(t *testing.T) {
	params := buildPrintToPDFParams(PDFOptions{})

	if params.DisplayHeaderFooter {
		t.Error("expected DisplayHeaderFooter = false by default")
	}
	if params.Landscape {
		t.Error("expected Landscape = false by default")
	}
	if params.Scale != 0 {
		t.Errorf("expected Scale left at cdproto's default for a zero-value PDFOptions, got %v — sending an explicit 0 is rejected by CDP", params.Scale)
	}
	if params.MarginTop != 0 || params.MarginBottom != 0 || params.MarginLeft != 0 || params.MarginRight != 0 {
		t.Errorf("margins = %+v, want all four at 0 for a zero-value PDFOptions", params)
	}
	if params.PaperWidth != 0 || params.PaperHeight != 0 {
		t.Errorf("paper size = %v x %v, want 0 x 0 for a zero-value PDFOptions (opts.PaperWidth > 0 && opts.PaperHeight > 0 gate stays untouched by issue #165)", params.PaperWidth, params.PaperHeight)
	}
}

// TestBuildPrintToPDFParams_MarginsAlwaysPropagated cubre issue #165: el
// guard `> 0` que precedía a esto era un no-op para cualquier valor no
// negativo (ver el comentario en buildPrintToPDFParams) pero seguía siendo
// código que mentía sobre lo que hacía — un futuro cdproto que agregara
// `omitempty` a estos campos habría reintroducido en silencio el mismo bug
// que el guard `> 0` de Scale ya tuvo una vez (ver
// TestBuildPrintToPDFParams_OnlyZeroMeansUnset). Cubre solo valores
// no-negativos: para negativos ver
// TestBuildPrintToPDFParams_NegativeMarginsClamped — a diferencia de Scale,
// CDP rechaza un margen negativo con un error duro en vez de aceptarlo, así
// que "propagar tal cual" no es el contrato correcto para ese caso (hallazgo
// de code review: la primera versión de este test incluía -1 esperando que
// se propagara, lo que convertía un margen negativo de "silenciosamente
// clampeado a 0" en "build entero abortado").
func TestBuildPrintToPDFParams_MarginsAlwaysPropagated(t *testing.T) {
	for _, margin := range []float64{0, 0.4, 1.5} {
		opts := PDFOptions{MarginTop: margin, MarginBottom: margin, MarginLeft: margin, MarginRight: margin}
		params := buildPrintToPDFParams(opts)
		if params.MarginTop != margin || params.MarginBottom != margin || params.MarginLeft != margin || params.MarginRight != margin {
			t.Errorf("margins for input %v = %+v, want all four = %v", margin, params, margin)
		}
	}
}

// TestBuildPrintToPDFParams_NegativeMarginsClamped cubre el hallazgo de code
// review sobre la primera versión de este PR: quitar el guard `> 0` sin más
// dejaba pasar un margen negativo tal cual hacia CDP, que lo rechaza con un
// error duro ("top margin is negative", -32602) en vez de ignorarlo como
// hacía el guard viejo — un doclang con `page.margins.top: "-1in"` en su
// front matter pasaba de generar un PDF (con margen en 0) a tronar el build
// entero. nonNegativeMargin clampea a 0 en su lugar, preservando la
// tolerancia que tenía el código antes de este PR sin resucitar el guard
// `> 0` (que también descartaba valores positivos legítimos vía un
// `omitempty` que nunca existió).
func TestBuildPrintToPDFParams_NegativeMarginsClamped(t *testing.T) {
	params := buildPrintToPDFParams(PDFOptions{MarginTop: -1, MarginBottom: -2.5, MarginLeft: -0.1, MarginRight: -100})
	if params.MarginTop != 0 || params.MarginBottom != 0 || params.MarginLeft != 0 || params.MarginRight != 0 {
		t.Errorf("negative margins = %+v, want all four clamped to 0", params)
	}
}

// TestBuildPrintToPDFParams_ScaleIsApplied cubre que PDFOptions.Scale deje de
// ser un campo público inerte: DefaultPDFOptions() lo inicializaba en 1.0 y
// slidesPDFOptions() lo seteaba explícitamente, pero buildPrintToPDFParams
// nunca llamaba WithScale, así que Scale: 0.5 producía un PDF a tamaño
// completo en silencio.
func TestBuildPrintToPDFParams_ScaleIsApplied(t *testing.T) {
	for _, scale := range []float64{0.5, 1.0, 2.0} {
		params := buildPrintToPDFParams(PDFOptions{Scale: scale})
		if params.Scale != scale {
			t.Errorf("Scale = %v, want %v", params.Scale, scale)
		}
	}

	// El default documentado (1.0) debe llegar al navegador, no quedarse en
	// el cero de cdproto.
	if params := buildPrintToPDFParams(DefaultPDFOptions()); params.Scale != 1.0 {
		t.Errorf("DefaultPDFOptions().Scale did not reach the print params: got %v, want 1.0", params.Scale)
	}
}

// TestBuildPrintToPDFParams_OnlyZeroMeansUnset es la corrección de un
// hallazgo de code-review: la primera versión de este bloque usaba un guard
// `> 0` mientras el comentario prometía que un valor fuera de rango llegaría
// al navegador para fallar ruidosamente. Las dos cosas no podían ser ciertas
// —un Scale: -1 se descartaba en silencio, exactamente el bug que el cambio
// venía a arreglar— y el test de entonces cementaba la contradicción.
//
// Contrato vigente: SOLO el cero significa "no especificado". Cualquier otro
// valor se propaga; el rango lo valida ValidatePDFScale (ver abajo).
func TestBuildPrintToPDFParams_OnlyZeroMeansUnset(t *testing.T) {
	if params := buildPrintToPDFParams(PDFOptions{Scale: 0}); params.Scale != 0 {
		t.Errorf("Scale 0 must be left at cdproto's default, got %v", params.Scale)
	}
	// Un negativo NO se descarta: se propaga para que la validación —y no un
	// silencio— sea quien lo rechace.
	if params := buildPrintToPDFParams(PDFOptions{Scale: -1}); params.Scale != -1 {
		t.Errorf("a non-zero Scale must be propagated verbatim, got %v (silently dropping it is the bug this replaced)", params.Scale)
	}
}

func TestValidatePDFScale(t *testing.T) {
	valid := []float64{0, 0.1, 0.5, 1.0, 2.0}
	for _, s := range valid {
		if err := ValidatePDFScale(s); err != nil {
			t.Errorf("ValidatePDFScale(%v) = %v, want nil", s, err)
		}
	}

	invalid := []float64{-1, 0.09, 2.01, 99}
	for _, s := range invalid {
		if err := ValidatePDFScale(s); err == nil {
			t.Errorf("ValidatePDFScale(%v) = nil, want an error naming the valid range", s)
		}
	}
}
