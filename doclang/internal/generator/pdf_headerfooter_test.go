// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// Estos tests cubren la parte pura (sin Chromium real) del arreglo de PDF
// para issue #117: buildPDFHeaderTemplate/buildPDFFooterTemplate arman el
// HeaderTemplate/FooterTemplate que Chromium usa para imprimir cada
// página, reemplazando las dos constantes hardcodeadas previas que tenían
// dos bugs confirmados — título sin escapar (inyección) y una condición
// ||/&& asimétrica que podía dejar DisplayHeaderFooter=true con un
// template vacío.

func TestBuildPDFHeaderTemplate_NilConfig_EscapesLegacyTitle(t *testing.T) {
	html := buildPDFHeaderTemplate(nil, `<script>alert(1)</script>`, nil)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("expected the legacy title fallback to be HTML-escaped, got:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("expected an escaped script tag, got:\n%s", html)
	}
}

func TestBuildPDFFooterTemplate_NilConfig_UsesChromiumPageNumberClasses(t *testing.T) {
	html := buildPDFFooterTemplate(nil, nil)

	if !strings.Contains(html, `<span class="pageNumber"></span>`) || !strings.Contains(html, `<span class="totalPages"></span>`) {
		t.Errorf("expected the legacy footer fallback to use Chromium's pageNumber/totalPages classes, got:\n%s", html)
	}
}

func TestBuildPDFHeaderTemplate_ZonesEscapeHTML(t *testing.T) {
	header := &ast.HeaderConfig{
		Enabled: true,
		Text: &ast.HeaderFooterText{
			Left:   `<script>alert(1)</script>`,
			Center: "Confidencial",
			Right:  "2026",
		},
	}

	html := buildPDFHeaderTemplate(header, "unused", nil)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("expected header zone text to be HTML-escaped, got:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") || !strings.Contains(html, "Confidencial") || !strings.Contains(html, "2026") {
		t.Errorf("expected all three zones present and escaped, got:\n%s", html)
	}
}

// TestBuildPDFFooterTemplate_PageNumbersFormat cubre la sustitución de
// {{current}}/{{total}} por las clases especiales de Chromium (no por
// ProcessVariables, que resuelve variables del documento, no del motor de
// impresión), y que Position decide a qué zona se suma la numeración.
func TestBuildPDFFooterTemplate_PageNumbersFormat(t *testing.T) {
	footer := &ast.FooterConfig{
		Enabled: true,
		PageNumbers: &ast.PageNumbersConfig{
			Enabled:  true,
			Format:   "Página {{current}} de {{total}}",
			Position: "left",
		},
	}

	html := buildPDFFooterTemplate(footer, nil)

	if !strings.Contains(html, `Página <span class="pageNumber"></span> de <span class="totalPages"></span>`) {
		t.Errorf("expected the format string translated to Chromium's page-number classes, got:\n%s", html)
	}
}

// TestBuildPDFFooterTemplate_PageNumbersFormatEscapesInjection verifica que
// un Format con HTML arbitrario se escapa antes de la sustitución de
// tokens (ninguno de los dos tokens contiene caracteres que EscapeHTML
// toque, así que el orden escapar-luego-reemplazar no los rompe).
func TestBuildPDFFooterTemplate_PageNumbersFormatEscapesInjection(t *testing.T) {
	footer := &ast.FooterConfig{
		Enabled: true,
		PageNumbers: &ast.PageNumbersConfig{
			Enabled: true,
			Format:  `{{current}}<script>alert(1)</script>`,
		},
	}

	html := buildPDFFooterTemplate(footer, nil)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("expected an unescaped script tag in Format to be rejected, got:\n%s", html)
	}
	if !strings.Contains(html, `<span class="pageNumber"></span>`) {
		t.Errorf("expected the {{current}} token to still translate, got:\n%s", html)
	}
}

// TestBuildPDFHeaderTemplate_EmptyZonesFallBackToBlankPlaceholder verifica
// que un header Enabled pero sin texto ni numeración NUNCA retorna "" —
// verificado empíricamente contra Edge/Chromium 151 (ver
// pdfBlankChromeTemplate): con DisplayHeaderFooter=true, un
// HeaderTemplate="" hace que Chromium imprima SU PROPIO template default
// (fecha + título), no un lado en blanco. Dejar pasar ese "" reintroduciría
// el mismo "chrome fantasma" que este fix corrige.
func TestBuildPDFHeaderTemplate_EmptyZonesFallBackToBlankPlaceholder(t *testing.T) {
	html := buildPDFHeaderTemplate(&ast.HeaderConfig{Enabled: true}, "unused", nil)
	if html == "" {
		t.Fatal("buildPDFHeaderTemplate must never return \"\" — it hands DisplayHeaderFooter's default chrome back to Chromium")
	}
	if html != pdfBlankChromeTemplate {
		t.Errorf("expected the blank placeholder for an enabled header with no text/logo, got:\n%s", html)
	}
}

func TestBuildPDFFooterTemplate_EmptyZonesFallBackToBlankPlaceholder(t *testing.T) {
	html := buildPDFFooterTemplate(&ast.FooterConfig{Enabled: true}, nil)
	if html == "" {
		t.Fatal("buildPDFFooterTemplate must never return \"\"")
	}
	if html != pdfBlankChromeTemplate {
		t.Errorf("expected the blank placeholder for an enabled footer with no text/page numbers, got:\n%s", html)
	}
}

// TestBuildPDFHeaderTemplate_HeaderPageNumbersIgnored documenta que el
// header no lleva numeración (ast.HeaderConfig no tiene campo PageNumbers
// — solo FooterConfig lo tiene), a diferencia del footer.
func TestBuildPDFHeaderTemplate_HeaderPageNumbersIgnored(t *testing.T) {
	header := &ast.HeaderConfig{
		Enabled: true,
		Text:    &ast.HeaderFooterText{Center: "Título"},
	}
	html := buildPDFHeaderTemplate(header, "unused", nil)
	if strings.Contains(html, "pageNumber") {
		t.Errorf("expected no page-number markup in a header template, got:\n%s", html)
	}
}
