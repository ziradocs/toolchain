// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"golang.org/x/net/html"
)

// headingAllowedAttrs es inlineAllowedAttrs más el envoltorio <hN id> de un
// encabezado.
func headingAllowedAttrs() map[string]map[string]bool {
	allowed := make(map[string]map[string]bool, len(inlineAllowedAttrs)+6)
	for tag, attrs := range inlineAllowedAttrs {
		allowed[tag] = attrs
	}
	for _, h := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		allowed[h] = map[string]bool{"id": true}
	}
	return allowed
}

// headingHTMLOutputs devuelve el HTML de cada camino que arma un encabezado
// con variables: el nodo tipado, la forma legada que se reconstruye desde la
// fuente, la forma legada sin fuente (como llega de un --filter externo, que
// sustituye sobre el HTML ya armado) y el título del TOC, además del párrafo
// con la misma entrada.
func headingHTMLOutputs(input string, vars map[string]interface{}) map[string]string {
	pos := diagnostics.NewPosition(1, 1)
	out := map[string]string{
		"párrafo": ProcessTextWithVariablesAndMarkdownSecure(input, vars),
	}

	typed := ast.NewHeadingElement(pos, 2, input, "h")
	populateElementHTML(typed, vars)
	out["tipado"] = typed.TextHTML

	legacy := LegacyHeadingElement(ast.NewHeadingElement(pos, 2, input, "h"))
	out["legado"] = renderTextElement(legacy, vars)

	raw := ast.NewRawHTMLTextElement(pos, HeadingHTML(2, input, "h"))
	raw.Level = 2
	out["sin fuente"] = renderTextElement(raw, vars)

	block := ast.ContentBlock{Elements: []ast.Element{LegacyHeadingElement(ast.NewHeadingElement(pos, 2, input, "h"))}}
	for _, sub := range extractSubsections(block, 6, vars) {
		out["TOC"] += sub.Title
	}
	return out
}

func assertHeadingHTMLAllowlisted(t *testing.T, input string, vars map[string]interface{}) {
	t.Helper()
	allowed := headingAllowedAttrs()
	for path, out := range headingHTMLOutputs(input, vars) {
		if problems := htmlProblems(out, allowed); len(problems) > 0 {
			t.Errorf("%s(%q, %v) = %q\nfuera de la allowlist: %v", path, input, vars, out, problems)
		}
	}
}

// FuzzHeadingVariablesAllowlist compone la propiedad de
// FuzzInlineMarkdownAllowlist con la sustitución de {{variables}} y con el
// envoltorio <hN>: para cualquier entrada y cualquier valor de variable, el
// HTML de un encabezado (por cualquiera de sus caminos, incluido el que no
// tiene fuente y sustituye sobre el HTML ya armado) y el del párrafo
// equivalente solo llevan etiquetas y atributos permitidos, y ningún href o
// src que el filtro de esquemas rechace.
func FuzzHeadingVariablesAllowlist(f *testing.F) {
	for _, tc := range headingVariableCases {
		for _, value := range tc.vars {
			f.Add(tc.input, value.(string))
		}
	}
	for _, s := range []string{
		"[a]({{v}})", "![a]({{v}})", "[x](java{{v}})", "[c](&#123;&#123;v}})",
		"**{{v}}** `{{v}}` [{{v}}]{.danger}", "<a href=\"{{v}}\">a</a>",
	} {
		f.Add(s, "javascript:alert(1)")
		f.Add(s, `"><img src=x>`)
	}
	f.Fuzz(func(t *testing.T, input, value string) {
		assertHeadingHTMLAllowlisted(t, input, map[string]interface{}{"v": value})
	})
}

// inlineVoidTags son las etiquetas de la allowlist que no se cierran.
var inlineVoidTags = map[string]bool{"img": true, "br": true}

// unbalancedTags tokeniza out y devuelve cada cierre que no corresponde a la
// última etiqueta abierta y cada etiqueta que queda abierta al final.
func unbalancedTags(out string) []string {
	var stack, problems []string
	z := html.NewTokenizer(strings.NewReader(out))
	for {
		switch z.Next() {
		case html.ErrorToken:
			for _, open := range stack {
				problems = append(problems, "<"+open+"> sin cerrar")
			}
			return problems
		case html.StartTagToken:
			if name, _ := z.TagName(); !inlineVoidTags[string(name)] {
				stack = append(stack, string(name))
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			if len(stack) == 0 || stack[len(stack)-1] != string(name) {
				problems = append(problems, "</"+string(name)+"> fuera de orden")
				continue
			}
			stack = stack[:len(stack)-1]
		}
	}
}

// inlineUnbalancedKnown son entradas que hoy dejan una etiqueta sin cerrar.
// Es la clase de ziradocs/toolchain#394: el destino de un enlace cruza el
// cierre de un formato anterior, que termina dentro del href. Cuando se
// arregle ese issue, la prueba de abajo falla aquí para que se quite la
// excepción (y el chequeo de balance pase al fuzz).
var inlineUnbalancedKnown = map[string]string{
	"**[a](b**)": "#394",
	"==[a](b==)": "#394",
	"~~[a](b~~)": "#394",
	"*[a](b*)":   "#394",
}

// El HTML inline cierra cada etiqueta que abre, en orden, sobre un corpus fijo
// (las tablas de casos de este paquete, con y sin envoltorio de encabezado y
// con variables). No corre en el fuzz mientras exista la excepción de #394.
func TestInlineHTML_TagsBalanced(t *testing.T) {
	type entry struct {
		input string
		vars  map[string]interface{}
	}
	var corpus []entry
	for _, tc := range inlineAttributeCases {
		corpus = append(corpus, entry{tc.input, nil})
	}
	for _, tc := range linkDestinationCases {
		corpus = append(corpus, entry{tc.input, nil})
	}
	for _, tc := range headingVariableCases {
		corpus = append(corpus, entry{tc.input, tc.vars})
	}
	for input := range inlineUnbalancedKnown {
		corpus = append(corpus, entry{input, nil})
	}
	for _, s := range []string{
		"**a** *b* ***c*** **d *e*** ==f== ~~g~~ `h`",
		"[See [important]{.danger}](url)",
		"**[a](b)** y [**c**](d)",
		"[![a](i.png) **b**](https://x.com)",
		"- ![a](b)\n- [c](d)",
	} {
		corpus = append(corpus, entry{s, nil})
	}

	for _, e := range corpus {
		issue, known := inlineUnbalancedKnown[e.input]
		for path, out := range headingHTMLOutputs(e.input, e.vars) {
			problems := unbalancedTags(out)
			switch {
			case known && len(problems) == 0:
				t.Errorf("%s(%q) ya sale balanceado (%q): quita la excepción de %s", path, e.input, out, issue)
			case !known && len(problems) > 0:
				t.Errorf("%s(%q, %v) = %q\netiquetas sin balancear: %v", path, e.input, e.vars, out, problems)
			}
		}
	}
}
