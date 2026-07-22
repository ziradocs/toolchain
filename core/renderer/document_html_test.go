// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// slideWithTextElement crea un ContentBlock de un solo TextElement, para las
// pruebas de extractSubsections de este archivo. isRawHTML refleja el flag
// del propio AST: true simula lo que parseSubsectionHeading construye (HTML
// ya seguro), false simula el fallback genérico de TextParser (texto de
// usuario guardado sin escapar).
func slideWithTextElement(content string, isRawHTML bool) ast.ContentBlock {
	pos := diagnostics.NewPosition(1, 1)
	slide := *ast.NewContentBlock(pos, "content")

	var textElem *ast.TextElement
	if isRawHTML {
		textElem = ast.NewRawHTMLTextElement(pos, content)
	} else {
		textElem = ast.NewTextElement(pos, content)
	}
	slide.Elements = append(slide.Elements, textElem)

	return slide
}

// TestExtractSubsections_EscapesVariableValues es una regresión encontrada en
// code-review de la PR de XSS (docs/SECURITY_AUDIT_2026-07.md, CR-2): esta PR
// escapó el título de nivel superior de un documento, pero extractSubsections
// (usada por el TOC y el sidebar) sustituía {{variable}} en headings h2-h6 vía
// ProcessVariables (sin escapar), dejando el mismo vector de XSS abierto un
// nivel de heading más abajo. Ahora usa ProcessVariablesEscapeValues.
func TestExtractSubsections_EscapesVariableValues(t *testing.T) {
	// Simula el HTML ya renderizado por parseSubsectionHeader: un heading con
	// un placeholder de variable sin resolver todavía.
	slide := slideWithTextElement(`<h2 id="config">Config {{evil}}</h2>`, true)

	variables := map[string]interface{}{"evil": "<script>alert(1)</script>"}

	subsections := extractSubsections(slide, 3, variables)
	if len(subsections) != 1 {
		t.Fatalf("expected 1 subsection, got %d: %+v", len(subsections), subsections)
	}

	title := subsections[0].Title
	if strings.Contains(title, "<script>") {
		t.Errorf("subsection title leaked raw <script> from a frontmatter variable: %q", title)
	}
	if !strings.Contains(title, "&lt;script&gt;") {
		t.Errorf("expected the variable's value to be HTML-escaped in the subsection title, got: %q", title)
	}
}

// TestExtractSubsections_DoesNotReintroduceInsecureLinkHref es una regresión
// para docs/SECURITY_AUDIT_2026-07.md, ME-9 (issue #31): extractSubsections
// solía re-procesar el título con la variante insegura ProcessInlineMarkdown,
// cuyo manejo de enlaces (`ProcessInlineMarkdownFormats`) construye
// `<a href="$2">` directo desde la captura del regex, sin pasar por
// SanitizeURL. El título que parseSubsectionHeading realmente produce hoy ya
// llega seguro (ProcessInlineMarkdownSecureLine descarta cualquier
// `[texto](javascript:...)` antes de construir el <hN>), así que ese caso
// concreto no tiene sintaxis `[..](..)` cruda que sobreviva hasta aquí. Lo
// que este test verifica es la causa raíz: que extractSubsections en sí
// mismo ya NO ejecuta ningún procesamiento de markdown/links sobre el
// título — si por cualquier vía futura (un cambio en parseSubsectionHeading,
// otro llamador, un caso límite) llegara sintaxis `[texto](url)` sin
// procesar, extractSubsections no debe convertirla en un href ejecutable.
// Se usa directamente ese texto crudo como fixture para demostrarlo: con el
// código pre-fix este test falla (la re-interpretación insegura reconstruye
// el href), con el fix aplicado el texto pasa intacto.
func TestExtractSubsections_DoesNotReintroduceInsecureLinkHref(t *testing.T) {
	slide := slideWithTextElement(`<h2 id="x">[x](javascript:alert(1)) and **not bold**</h2>`, true)

	subsections := extractSubsections(slide, 3, nil)
	if len(subsections) != 1 {
		t.Fatalf("expected 1 subsection, got %d: %+v", len(subsections), subsections)
	}

	title := subsections[0].Title
	if strings.Contains(title, `href="javascript:`) {
		t.Errorf("subsection title contains an executable javascript: href: %q", title)
	}
	if strings.Contains(title, "<strong>") || strings.Contains(title, "<a href=") {
		t.Errorf("extractSubsections re-interpreted raw markdown/link syntax instead of leaving it untouched: %q", title)
	}
}

// TestExtractSubsections_IgnoresUnsafeTextElements es una regresión
// encontrada en code-review de esta misma PR (#31, ME-9): extractSubsections
// escaneaba CUALQUIER *ast.TextElement en busca de patrones <h2>-<h6>, sin
// distinguir los construidos por parseSubsectionHeading (IsRawHTML=true, ya
// pasados por ProcessInlineMarkdownSecureLine) de un TextElement genérico
// (IsRawHTML=false, el fallback de TextParser para texto de usuario
// arbitrario, guardado tal cual sin escapar). Un párrafo con HTML literal
// como `<h2 id="x">Test <a href="javascript:alert(1)">click</a></h2>`
// coincide con el mismo patrón de búsqueda y se extraía crudo hacia el
// TOC/sidebar — XSS de cero interacción alcanzable en el formato HTML por
// defecto sin necesidad de sintaxis markdown/subsection alguna.
func TestExtractSubsections_IgnoresUnsafeTextElements(t *testing.T) {
	// Simula el fallback genérico de TextParser: un párrafo con HTML literal
	// tecleado por el usuario, guardado sin escapar (IsRawHTML=false).
	slide := slideWithTextElement(`<h2 id="x">Test <a href="javascript:alert(1)">click</a></h2>`, false)

	subsections := extractSubsections(slide, 3, nil)
	if len(subsections) != 0 {
		t.Fatalf("expected extractSubsections to ignore a non-raw-HTML TextElement, got %d subsections: %+v", len(subsections), subsections)
	}
}

// TestGenerateInitScripts_MermaidStrictConfig cubre issue #70: el
// mermaid.initialize embebido en el HTML por defecto de doclang debe
// coincidir con la forma canónica que ya usan los raster builders de
// chromium_renderer.go (securityLevel:'strict', htmlLabels:false).
func TestGenerateInitScripts_MermaidStrictConfig(t *testing.T) {
	scripts := generateInitScripts(DocumentHTMLOptions{}, "")

	if !strings.Contains(scripts, "securityLevel: 'strict'") {
		t.Error("expected mermaid.initialize to set securityLevel:'strict'")
	}
	if !strings.Contains(scripts, "htmlLabels: false") {
		t.Error("expected mermaid.initialize to set htmlLabels:false (top-level, #70)")
	}
}

// bareInlineScriptPattern matchea un <script> SIN nonce y sin src= (externo,
// no lo necesita) ni type="application/json" (no ejecutable, exento por la
// propia spec de CSP) — usado para confirmar que ningún <script> inline se
// coló sin el nonce del build. <style> NO se chequea: style-src usa
// 'unsafe-inline' a propósito (ver BuildDefaultOutputCSP — Mermaid inyecta
// su CSS de tema en runtime sin nonce y sin forma de asignarle uno).
var bareInlineScriptPattern = regexp.MustCompile(`<script(\s[^>]*)?>`)

// assertNoUnnoncedInlineScripts falla el test si algún <script> del html
// carece de nonce="..." — src= (externo) está exento.
func assertNoUnnoncedInlineScripts(t *testing.T, html string) {
	t.Helper()
	for _, m := range bareInlineScriptPattern.FindAllString(html, -1) {
		if strings.Contains(m, "src=") || strings.Contains(m, `type="application/json"`) {
			continue
		}
		if !strings.Contains(m, "nonce=") {
			t.Errorf("found an inline <script> with no nonce and no src=: %s", m)
		}
	}
}

// TestGenerateDocumentHTML_EmitsCSPWithMatchingNonces cubre BA-10: la salida
// HTML por defecto de doclang debe llevar una CSP con nonce, y ESE MISMO
// nonce debe estar en cada <script> inline (ThemeVariables, InteractiveViewer,
// CustomCSS/CustomJS ejercitan cada sitio de emisión). Verificado en vivo
// (headless Chrome) que <style> NO lleva/necesita nonce — ver comentario en
// bareInlineScriptPattern.
func TestGenerateDocumentHTML_EmitsCSPWithMatchingNonces(t *testing.T) {
	doc := &ast.AST{}
	opts := DocumentHTMLOptions{
		Title:             "Test Doc",
		ThemeVariables:    map[string]string{"--primary-color": "#ff0000"},
		InteractiveViewer: true,
		CustomCSS:         "body { color: red; }",
		CustomJS:          "console.log('hi');",
	}

	html := GenerateDocumentHTML(doc, opts, nil)

	cspMatch := regexp.MustCompile(`Content-Security-Policy" content="([^"]*)"`).FindStringSubmatch(html)
	if cspMatch == nil {
		t.Fatal("expected output to contain a Content-Security-Policy meta tag")
	}
	csp := cspMatch[1]

	nonceMatch := regexp.MustCompile(`'nonce-([A-Za-z0-9+/=]+)'`).FindStringSubmatch(csp)
	if nonceMatch == nil {
		t.Fatalf("expected CSP to contain a 'nonce-...' source, got: %s", csp)
	}
	nonce := nonceMatch[1]

	styleCount := strings.Count(html, "<style")
	scriptCount := strings.Count(html, "<script")
	nonceAttrCount := strings.Count(html, fmt.Sprintf(`nonce="%s"`, nonce))
	if nonceAttrCount == 0 {
		t.Fatal("expected at least one inline <script> to carry the CSP nonce")
	}
	if styleCount+scriptCount == 0 {
		t.Fatal("expected the test options (ThemeVariables/InteractiveViewer/CustomCSS/CustomJS) to produce at least one style/script tag")
	}

	assertNoUnnoncedInlineScripts(t, html)
}
