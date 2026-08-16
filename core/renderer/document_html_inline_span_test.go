// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"regexp"
	"testing"

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
