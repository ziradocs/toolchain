// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"regexp"
	"testing"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// classFromOpenTag extrae el nombre de clase de un tag de apertura de
// inlineSpanTokens (p. ej. `<span class="slidelang-text-danger">` →
// "slidelang-text-danger"). "underline" no lleva class (es un <u> pelón) —
// el caller debe filtrarlo aparte.
var openTagClassPattern = regexp.MustCompile(`class="([^"]+)"`)

func classFromOpenTag(openTag string) (string, bool) {
	m := openTagClassPattern.FindStringSubmatch(openTag)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// TestGenerateDocumentHTML_InlineSpanTokens_AllHaveMatchingCSSRule es el
// test de cobertura para issue #81: inlineSpanTokens (sanitizer.go) es
// compartido por los dos CLIs, pero el <style> embebido de doclang
// (document_html.go) tenía cero reglas para las clases que emite. Este
// test itera el mapa fuente en vez de hardcodear la lista de clases, para
// que un token NUEVO agregado a inlineSpanTokens sin su CSS correspondiente
// haga fallar el build en vez de salir sin estilo en silencio, otra vez.
func TestGenerateDocumentHTML_InlineSpanTokens_AllHaveMatchingCSSRule(t *testing.T) {
	html := GenerateDocumentHTML(&ast.AST{}, DocumentHTMLOptions{}, nil)

	for token, tags := range inlineSpanTokens {
		class, hasClass := classFromOpenTag(tags[0])
		if !hasClass {
			// "underline" -> <u></u>, sin class: no hay nada que cubrir en CSS.
			continue
		}
		rule := regexp.MustCompile(`\.` + regexp.QuoteMeta(class) + `\s*\{`)
		if !rule.MatchString(html) {
			t.Errorf("token %q emits class %q but doclang's embedded <style> has no CSS rule for it", token, class)
		}
	}
}

// TestGenerateDocumentHTML_InlineDangerSpan_RendersWithMatchingCSSRule es
// el caso end-to-end concreto: un documento doclang real que usa
// [texto]{.danger} debe producir tanto el span con su clase como la regla
// CSS que lo respalda — la costura completa, no solo el mapa de tokens.
func TestGenerateDocumentHTML_InlineDangerSpan_RendersWithMatchingCSSRule(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := &ast.AST{
		ContentBlocks: []ast.ContentBlock{
			{
				BlockType: "content",
				Elements: []ast.Element{
					ast.NewTextElement(pos, "[atención]{.danger}"),
				},
			},
		},
	}

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, nil)

	if !regexp.MustCompile(`<span class="slidelang-text-danger">atención</span>`).MatchString(html) {
		t.Errorf("expected the rendered span for [atención]{.danger}, got: %.500s", html)
	}
	if !regexp.MustCompile(`\.slidelang-text-danger\s*\{`).MatchString(html) {
		t.Errorf("expected a CSS rule for .slidelang-text-danger in the embedded <style>, none found")
	}
}

// TestGenerateDocumentHTML_InlineSpans_DarkModeMeetsAA es hallazgo de
// code-review sobre PR #158: los fallbacks hex de .slidelang-text-*/
// .slidelang-highlight-* están verificados AA contra un fondo claro, pero
// sin un override de --doclang-*-color/-bg/-text dentro de
// html[data-theme="dark"], var(--doclang-*, fallback) sigue resolviendo al
// MISMO fallback claro sobre --doclang-page-bg: #1a1a1a (3.05:1-3.60:1,
// por debajo de 4.5:1). Este test extrae los hex REALES del HTML generado
// (no una copia hardcodeada que podría desincronizarse) y los verifica con
// a11y.ContrastRatio -- para texto, contra el --doclang-page-bg de dark
// mode; para highlights, contra su propio --doclang-highlight-*-bg.
func TestGenerateDocumentHTML_InlineSpans_DarkModeMeetsAA(t *testing.T) {
	html := GenerateDocumentHTML(&ast.AST{}, DocumentHTMLOptions{InteractiveViewer: true}, nil)

	darkBlock := regexp.MustCompile(`(?s)html\[data-theme="dark"\]\s*\{(.*?)\}`).FindStringSubmatch(html)
	if darkBlock == nil {
		t.Fatal("no html[data-theme=\"dark\"] block found in the generated <style>")
	}
	dark := darkBlock[1]

	hexVar := func(name string) string {
		m := regexp.MustCompile(`--` + regexp.QuoteMeta(name) + `:\s*(#[0-9a-fA-F]{3,8})`).FindStringSubmatch(dark)
		if m == nil {
			t.Fatalf("dark mode block has no override for --%s", name)
		}
		return m[1]
	}

	darkBg := hexVar("doclang-page-bg")

	for _, textVar := range []string{
		"doclang-danger-color", "doclang-info-color", "doclang-success-color",
		"doclang-warning-color", "doclang-accent-color",
	} {
		fg := hexVar(textVar)
		if ratio, ok := a11y.ContrastRatio(fg, darkBg); !ok || ratio < 4.5 {
			t.Errorf("--%s = %s on dark bg %s: ratio = %.2f, want >= 4.5 (AA)", textVar, fg, darkBg, ratio)
		}
	}

	for _, name := range []string{"warning", "info", "success"} {
		bg := hexVar("doclang-highlight-" + name + "-bg")
		fg := hexVar("doclang-highlight-" + name + "-text")
		if ratio, ok := a11y.ContrastRatio(fg, bg); !ok || ratio < 4.5 {
			t.Errorf("--doclang-highlight-%s-text = %s on --doclang-highlight-%s-bg = %s: ratio = %.2f, want >= 4.5 (AA)", name, fg, name, bg, ratio)
		}
	}
}
