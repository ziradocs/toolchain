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

// TestExtractSubsections_DepthCutoff cubre issue #123: maxDepth es el nivel
// de heading más profundo a incluir (el título de la sección misma cuenta
// como nivel 1, fuera de esta función), no una cuenta de "N niveles desde
// h2". maxDepth=1 no debe incluir nada, maxDepth=2 solo h2, maxDepth=3
// h2-h3, y así sucesivamente — antes del fix, maxDepth=2 incluía h2 Y h3.
func TestExtractSubsections_DepthCutoff(t *testing.T) {
	slide := slideWithTextElement(
		`<h2 id="a">A</h2><h3 id="b">B</h3><h4 id="c">C</h4>`, true)

	tests := []struct {
		maxDepth   int
		wantLevels []int
	}{
		{1, nil},
		{2, []int{2}},
		{3, []int{2, 3}},
		{4, []int{2, 3, 4}},
		{6, []int{2, 3, 4}}, // no hay h5/h6 en el fixture; no debe fallar ni inventar entradas
	}

	for _, tt := range tests {
		subsections := extractSubsections(slide, tt.maxDepth, nil)
		gotLevels := make([]int, len(subsections))
		for i, s := range subsections {
			gotLevels[i] = s.Level
		}
		if len(gotLevels) != len(tt.wantLevels) {
			t.Errorf("maxDepth=%d: got levels %v, want %v", tt.maxDepth, gotLevels, tt.wantLevels)
			continue
		}
		for i := range gotLevels {
			if gotLevels[i] != tt.wantLevels[i] {
				t.Errorf("maxDepth=%d: got levels %v, want %v", tt.maxDepth, gotLevels, tt.wantLevels)
				break
			}
		}
	}
}

// TestExtractSubsections_OrdersByDocumentPositionNotByLevel cubre issue #71:
// la versión anterior recorría los niveles en un loop EXTERNO (todo h2 antes
// que cualquier h3), así que si un único TextElement traía un h3 antes que
// un h2 en la fuente, el resultado salía en orden de nivel (h2, h3) en vez
// de orden de documento (h3, h2). No alcanzable desde ninguno de los dos
// parsers hoy (buildHeadingElement produce un solo <hN> por TextElement en
// ambos dialectos) — solo desde un AST JSON hecho a mano o producido
// externamente, que es como se descubrió.
func TestExtractSubsections_OrdersByDocumentPositionNotByLevel(t *testing.T) {
	slide := slideWithTextElement(`<h3 id="b">B</h3><h2 id="a">A</h2>`, true)

	subsections := extractSubsections(slide, 3, nil)

	gotLevels := make([]int, len(subsections))
	for i, s := range subsections {
		gotLevels[i] = s.Level
	}
	wantLevels := []int{3, 2}
	if len(gotLevels) != len(wantLevels) || gotLevels[0] != wantLevels[0] || gotLevels[1] != wantLevels[1] {
		t.Fatalf("got levels %v, want %v (document order, not level order)", gotLevels, wantLevels)
	}
	if subsections[0].Anchor != "b" || subsections[1].Anchor != "a" {
		t.Errorf("got anchors [%q, %q], want [\"b\", \"a\"]", subsections[0].Anchor, subsections[1].Anchor)
	}
}

// TestExtractSubsections_KeepsBothHeadingsOfSameLevel cubre la segunda mitad
// de issue #71: el loop por nivel tomaba siempre el PRIMER strings.Index de
// ese nivel en todo el content y nunca avanzaba el cursor de búsqueda, así
// que un segundo heading del mismo nivel en el mismo TextElement se perdía
// por completo, no solo se reordenaba.
func TestExtractSubsections_KeepsBothHeadingsOfSameLevel(t *testing.T) {
	slide := slideWithTextElement(`<h2 id="a">A</h2><h2 id="b">B</h2>`, true)

	subsections := extractSubsections(slide, 3, nil)
	if len(subsections) != 2 {
		t.Fatalf("expected both h2 headings to survive, got %d: %+v", len(subsections), subsections)
	}
	if subsections[0].Anchor != "a" || subsections[1].Anchor != "b" {
		t.Errorf("got anchors [%q, %q], want [\"a\", \"b\"]", subsections[0].Anchor, subsections[1].Anchor)
	}
}

// TestExtractSubsections_DepthCutoffWithOutOfOrderLevels combina los dos
// bugs de issue #71 con el corte de profundidad de issue #123: un heading
// que excede maxDepth debe saltarse sin detener el escaneo de los que
// vienen después en la fuente, incluida una vuelta a un nivel más superficial.
func TestExtractSubsections_DepthCutoffWithOutOfOrderLevels(t *testing.T) {
	slide := slideWithTextElement(`<h4 id="c">C</h4><h2 id="a">A</h2><h3 id="b">B</h3>`, true)

	subsections := extractSubsections(slide, 3, nil) // maxDepth=3: excluye h4, incluye h2/h3
	gotLevels := make([]int, len(subsections))
	for i, s := range subsections {
		gotLevels[i] = s.Level
	}
	wantLevels := []int{2, 3}
	if len(gotLevels) != len(wantLevels) || gotLevels[0] != wantLevels[0] || gotLevels[1] != wantLevels[1] {
		t.Fatalf("got levels %v, want %v (h4 skipped, h2/h3 kept in document order)", gotLevels, wantLevels)
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

// TestGenerateDocumentHTML_Lang cubre el prerrequisito de los issues
// #62/#63: antes de esto, <html lang> estaba hardcodeado a "es" sin importar
// lo que el frontmatter declarara, así que un documento con `lang: fr`
// emitía `<html lang="es">` — un falso negativo para cualquier regla que
// verifique el idioma declarado contra la salida real.
func TestGenerateDocumentHTML_Lang(t *testing.T) {
	tests := []struct {
		name        string
		frontMatter *ast.FrontMatterNode
		wantLang    string
	}{
		{"declared lang is honored", &ast.FrontMatterNode{Lang: "fr"}, "fr"},
		{"declared region variant is honored", &ast.FrontMatterNode{Lang: "pt-BR"}, "pt-BR"},
		{"no frontmatter falls back to es", nil, "es"},
		{"frontmatter without lang falls back to es", &ast.FrontMatterNode{Title: "Doc"}, "es"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &ast.AST{FrontMatter: tt.frontMatter}
			html := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, nil)

			want := fmt.Sprintf(`<html lang="%s"`, tt.wantLang)
			if !strings.Contains(html, want) {
				t.Errorf("expected output to contain %q, got head: %.200s", want, html)
			}
		})
	}
}

// TestGenerateDocumentHTML_Lang_EscapesAttribute confirma que un valor de
// lang hostil no puede escapar el atributo — mismo vector que cualquier otro
// campo de frontmatter interpolado en un atributo HTML.
func TestGenerateDocumentHTML_Lang_EscapesAttribute(t *testing.T) {
	doc := &ast.AST{FrontMatter: &ast.FrontMatterNode{Lang: `"><script>alert(1)</script>`}}
	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, nil)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Errorf("lang attribute was not escaped, output contains raw <script>: %.300s", html)
	}
}

// TestGenerateDocumentHTML_PreservesElementOrder es el guard de regresión
// para la mitad "(a)" de issue #62 (orden de lectura): ningún renderer debe
// permutar ast.ContentBlock.Elements — ast.Walk ya documenta este orden
// como el contrato ("mismo orden que renderer.PopulateInlineHTML,
// generalizado"), este test lo verifica contra la salida HTML real. Cubre
// tanto generateDocumentBody (modo estándar) como generatePageViewBody
// (ShowHeaders/ShowFooters) — son dos loops de emisión distintos
// (document_html.go) que podrían divergir en un cambio futuro.
func TestGenerateDocumentHTML_PreservesElementOrder(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{
			{
				BlockType: "content",
				Elements: []ast.Element{
					ast.NewTextElement(pos, "order-marker-alpha"),
					ast.NewTextElement(pos, "order-marker-bravo"),
					ast.NewTextElement(pos, "order-marker-charlie"),
				},
			},
		},
	}

	for _, tt := range []struct {
		name string
		opts DocumentHTMLOptions
	}{
		{"standard mode", DocumentHTMLOptions{}},
		{"page-view mode", DocumentHTMLOptions{ShowHeaders: true, ShowFooters: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html := GenerateDocumentHTML(doc, tt.opts, nil)

			iAlpha := strings.Index(html, "order-marker-alpha")
			iBravo := strings.Index(html, "order-marker-bravo")
			iCharlie := strings.Index(html, "order-marker-charlie")

			if iAlpha == -1 || iBravo == -1 || iCharlie == -1 {
				t.Fatalf("expected all three markers in output, got positions alpha=%d bravo=%d charlie=%d", iAlpha, iBravo, iCharlie)
			}
			if iAlpha >= iBravo || iBravo >= iCharlie {
				t.Errorf("elements were not emitted in AST order: alpha@%d bravo@%d charlie@%d", iAlpha, iBravo, iCharlie)
			}
		})
	}
}

// TestGenerateDocumentHTML_PreservesElementOrder_HeterogeneousTypes es un
// segundo guard para la misma mitad "(a)" de issue #62, con fixtures de
// código distinto (code review de la segunda ronda: el test original solo
// usa TextElement tres veces, así que un bug hipotético que agrupara la
// salida POR TIPO de elemento — en vez de respetar el orden de
// ContentBlock.Elements — pasaría inadvertido, porque agrupar un único tipo
// consigo mismo no cambia nada observable). Mezclar Text/Quote/Code fuerza
// a que un bug de agrupamiento por tipo mueva un marcador fuera de orden.
func TestGenerateDocumentHTML_PreservesElementOrder_HeterogeneousTypes(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{
			{
				BlockType: "content",
				Elements: []ast.Element{
					ast.NewTextElement(pos, "order-marker-alpha"),
					ast.NewQuoteElement(pos, "order-marker-bravo"),
					ast.NewCodeElement(pos, "text", "order-marker-charlie"),
				},
			},
		},
	}

	for _, tt := range []struct {
		name string
		opts DocumentHTMLOptions
	}{
		{"standard mode", DocumentHTMLOptions{}},
		{"page-view mode", DocumentHTMLOptions{ShowHeaders: true, ShowFooters: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html := GenerateDocumentHTML(doc, tt.opts, nil)

			iAlpha := strings.Index(html, "order-marker-alpha")
			iBravo := strings.Index(html, "order-marker-bravo")
			iCharlie := strings.Index(html, "order-marker-charlie")

			if iAlpha == -1 || iBravo == -1 || iCharlie == -1 {
				t.Fatalf("expected all three markers in output, got positions alpha=%d bravo=%d charlie=%d", iAlpha, iBravo, iCharlie)
			}
			if iAlpha >= iBravo || iBravo >= iCharlie {
				t.Errorf("elements of different types were not emitted in AST order (possible grouping-by-type bug): alpha@%d bravo@%d charlie@%d", iAlpha, iBravo, iCharlie)
			}
		})
	}
}

// TestGenerateDocumentHTML_PreambleBlockNotCountedTowardNumbering covers
// issue #100's second bug: a document's first ContentBlock (block_type
// "title", parsed from the document's first `# ` heading) carries its text
// in Heading, not Title. It must still render its heading, but must not
// consume a number or shift every real section's number up by one — same
// resolveSectionTitle rule doclang/internal/generator/markdown.go now
// applies, so both generators agree on which block counts.
func TestGenerateDocumentHTML_PreambleBlockNotCountedTowardNumbering(t *testing.T) {
	doc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{
			{BlockType: "title", Heading: "Preamble"},
			{BlockType: "content", Title: "Real Section"},
		},
	}

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{TOC: true, Numbering: true}, nil)

	// The preamble keeps its heading, unnumbered, in the TOC and body.
	if !strings.Contains(html, `<li><a href="#preamble">Preamble</a></li>`) {
		t.Errorf("expected an unnumbered TOC entry for the preamble, got:\n%s", html)
	}
	if !strings.Contains(html, `<h1 id="preamble">Preamble</h1>`) {
		t.Errorf("expected an unnumbered heading for the preamble, got:\n%s", html)
	}

	// The real section is "1.", not "2." — the preamble must not have
	// consumed a number ahead of it.
	if !strings.Contains(html, `<li><a href="#real-section">1. Real Section</a></li>`) {
		t.Errorf("expected the real section to be numbered '1.' (preamble doesn't count), got:\n%s", html)
	}
	if !strings.Contains(html, `<h1 id="real-section">1. Real Section</h1>`) {
		t.Errorf("expected the real section's heading to be numbered '1.', got:\n%s", html)
	}

	if strings.Contains(html, "1. Preamble") || strings.Contains(html, "2. Real Section") {
		t.Errorf("preamble must never receive a number and must not shift the real section's number, got:\n%s", html)
	}
}
