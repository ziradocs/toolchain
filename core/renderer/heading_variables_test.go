// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// headingVariableCases son textos de una sola línea (sin "- " al inicio, para
// que la variante de una línea y la general no puedan diferir) con
// {{variables}} cuyo valor trae sintaxis inline.
var headingVariableCases = []struct {
	name  string
	input string
	vars  map[string]interface{}
}{
	{"destino con esquema bloqueado", "[a]({{v}})", map[string]interface{}{"v": "javascript:alert(1)"}},
	{"destino permitido", "Ver [doc]({{u}})", map[string]interface{}{"u": "https://x.com/p?a=1&b=2"}},
	{"imagen con src de variable", "![logo]({{v}})", map[string]interface{}{"v": "javascript:alert(1)"}},
	{"variable como parte del esquema", "[x](java{{w}})", map[string]interface{}{"w": "script:alert(1)"}},
	{"cursiva en el valor", "Hola {{n}}", map[string]interface{}{"n": "*Ana*"}},
	{"negrita en el valor", "Hola {{n}}", map[string]interface{}{"n": "**Ana**"}},
	{"enlace en el valor", "Hola {{n}}", map[string]interface{}{"n": "[x](https://x.com)"}},
	{"código en el valor", "Hola {{n}}", map[string]interface{}{"n": "`c`"}},
	{"span de clase en el valor", "Hola {{n}}", map[string]interface{}{"n": "[x]{.danger}"}},
	{"span de idioma en el valor", "Hola {{n}}", map[string]interface{}{"n": "[bonjour]{lang=fr}"}},
	{"HTML en el valor", "Hola {{n}}", map[string]interface{}{"n": "<b>x</b>"}},
	{"entidad en el valor", "Hola {{n}}", map[string]interface{}{"n": "a &amp; b"}},
	{"variable dentro del valor", "Hola {{n}}", map[string]interface{}{"n": "{{w}}", "w": "no"}},
	{"valor que cierra la negrita de afuera", "**{{n}} fin", map[string]interface{}{"n": "a**"}},
	{"variable dentro de código", "`{{n}}`", map[string]interface{}{"n": "a*b*"}},
	{"variable sin definir", "Hola {{nada}}", map[string]interface{}{"n": "x"}},
	{"sin variables", "Hola **mundo**", nil},
}

// Un encabezado tiene que salir igual que un párrafo con la misma entrada y
// las mismas variables. Antes el encabezado aplicaba el Markdown inline al
// texto con los {{placeholders}} sin resolver y sustituía después, sobre el
// HTML ya emitido: el valor de una variable usada como destino entraba al
// href sin pasar por el filtro de esquemas, y el Markdown de un valor salía
// literal en el encabezado pero formateado en el párrafo. Se comprueban todos
// los caminos que producen el HTML de un encabezado: el nodo tipado
// (TextHTML y RenderElementToHTML), la forma legada (ContentHTML y
// renderTextElement) y el título del TOC.
func TestHeadingVariables_MatchParagraphText(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	for _, tc := range headingVariableCases {
		t.Run(tc.name, func(t *testing.T) {
			body := ProcessTextWithVariablesAndMarkdownSecure(tc.input, tc.vars)
			wantHeading := fmt.Sprintf(`<h2 id="h">%s</h2>`, body)

			typed := ast.NewHeadingElement(pos, 2, tc.input, "h")
			populateElementHTML(typed, tc.vars)
			if typed.TextHTML != body {
				t.Errorf("HeadingElement.TextHTML\n got %q\nwant %q", typed.TextHTML, body)
			}
			if got := RenderElementToHTML(ast.NewHeadingElement(pos, 2, tc.input, "h"), tc.vars, NewDefaultRenderContext()); got != wantHeading {
				t.Errorf("RenderElementToHTML(HeadingElement)\n got %q\nwant %q", got, wantHeading)
			}

			legacy := LegacyHeadingElement(ast.NewHeadingElement(pos, 2, tc.input, "h"))
			if got := renderTextElement(legacy, tc.vars); got != wantHeading {
				t.Errorf("renderTextElement(legado)\n got %q\nwant %q", got, wantHeading)
			}
			populateElementHTML(legacy, tc.vars)
			if legacy.ContentHTML != wantHeading {
				t.Errorf("TextElement.ContentHTML (legado)\n got %q\nwant %q", legacy.ContentHTML, wantHeading)
			}

			block := ast.ContentBlock{Elements: []ast.Element{LegacyHeadingElement(ast.NewHeadingElement(pos, 2, tc.input, "h"))}}
			subsections := extractSubsections(block, 3, tc.vars)
			if len(subsections) != 1 {
				t.Fatalf("extractSubsections devolvió %d subsecciones, quería 1", len(subsections))
			}
			if subsections[0].Title != body {
				t.Errorf("título en el TOC\n got %q\nwant %q", subsections[0].Title, body)
			}
		})
	}
}

// Los LangRuns de un encabezado se derivan del mismo HTML que se emite, así
// que también tienen que coincidir con los del párrafo equivalente.
func TestHeadingVariables_LangRunsMatchParagraphText(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	for _, tc := range headingVariableCases {
		t.Run(tc.name, func(t *testing.T) {
			paragraph := ast.NewTextElement(pos, tc.input)
			populateElementLangRuns(paragraph, tc.vars)

			typed := ast.NewHeadingElement(pos, 2, tc.input, "h")
			populateElementLangRuns(typed, tc.vars)
			if !reflect.DeepEqual(typed.LangRuns, paragraph.LangRuns) {
				t.Errorf("HeadingElement.LangRuns = %#v, el párrafo da %#v", typed.LangRuns, paragraph.LangRuns)
			}

			legacy := LegacyHeadingElement(ast.NewHeadingElement(pos, 2, tc.input, "h"))
			populateElementLangRuns(legacy, tc.vars)
			if !reflect.DeepEqual(legacy.LangRuns, paragraph.LangRuns) {
				t.Errorf("LangRuns del legado = %#v, el párrafo da %#v", legacy.LangRuns, paragraph.LangRuns)
			}
		})
	}
}

// Con un destino que el filtro de esquemas descarta, el encabezado no puede
// llevar enlace: queda solo la etiqueta, como en el párrafo.
func TestHeadingVariables_BlockedDestinationLeavesNoLink(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"v": "javascript:alert(1)"}
	legacy := LegacyHeadingElement(ast.NewHeadingElement(pos, 2, "[a]({{v}})", "h"))

	if got := renderTextElement(legacy, vars); got != `<h2 id="h">a</h2>` {
		t.Errorf("renderTextElement = %q", got)
	}
	block := ast.ContentBlock{Elements: []ast.Element{legacy}}
	if subs := extractSubsections(block, 3, vars); len(subs) != 1 || subs[0].Title != "a" {
		t.Errorf("extractSubsections = %#v", subs)
	}
}

// Un TextElement crudo sin fuente autoral (llegó por un --filter externo, o
// alguien cambió su Content después de parsear) no se puede volver a armar
// desde el Markdown. Ahí la sustitución solo toca el texto entre etiquetas:
// un {{placeholder}} dentro de un atributo ya emitido se queda literal.
func TestHeadingVariables_RawHTMLWithoutSourceKeepsAttributes(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"v": "javascript:alert(1)"}
	content := `<h2 id="h"><a href="{{v}}">a</a> {{v}}</h2>`
	want := `<h2 id="h"><a href="{{v}}">a</a> javascript:alert(1)</h2>`

	raw := ast.NewRawHTMLTextElement(pos, content)
	raw.Level = 2
	if got := renderTextElement(raw, vars); got != want {
		t.Errorf("renderTextElement sin fuente\n got %q\nwant %q", got, want)
	}

	// Content ya no es lo que el parser armó desde HeadingSource: gana
	// Content, que es lo que se habría emitido sin variables.
	edited := LegacyHeadingElement(ast.NewHeadingElement(pos, 2, "[a]({{v}})", "h"))
	edited.Content = content
	if got := renderTextElement(edited, vars); got != want {
		t.Errorf("renderTextElement con Content editado\n got %q\nwant %q", got, want)
	}

	block := ast.ContentBlock{Elements: []ast.Element{raw}}
	subs := extractSubsections(block, 3, vars)
	if len(subs) != 1 || subs[0].Title != `<a href="{{v}}">a</a> javascript:alert(1)` {
		t.Errorf("extractSubsections = %#v", subs)
	}
}

// ProcessVariablesEscapeValues recibe HTML de confianza y solo sustituye en el
// texto entre etiquetas. El fin de una etiqueta es el ">" que queda fuera de
// un valor entre comillas, así que un ">" dentro de un atributo no adelanta
// el cierre ni deja el resto del atributo expuesto a la sustitución.
func TestProcessVariablesEscapeValues_SkipsTags(t *testing.T) {
	vars := map[string]interface{}{"v": `<b>"x"</b>`}
	for _, tc := range []struct {
		name, input, want string
	}{
		{"href", `<a href="{{v}}">{{v}}</a>`, `<a href="{{v}}">&lt;b&gt;&quot;x&quot;&lt;/b&gt;</a>`},
		{"comillas dobles con >", `<img alt="x>{{v}}" src="y"> {{v}}`, `<img alt="x>{{v}}" src="y"> &lt;b&gt;&quot;x&quot;&lt;/b&gt;`},
		{"comillas simples con > y comilla doble", `<a title='">{{v}}'>t</a>`, `<a title='">{{v}}'>t</a>`},
		{"etiqueta sin cerrar", `{{v}} <a href="{{v}}`, `&lt;b&gt;&quot;x&quot;&lt;/b&gt; <a href="{{v}}`},
		{"sin etiquetas", `a {{v}} b`, `a &lt;b&gt;&quot;x&quot;&lt;/b&gt; b`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProcessVariablesEscapeValues(tc.input, vars); got != tc.want {
				t.Errorf("ProcessVariablesEscapeValues(%q)\n got %q\nwant %q", tc.input, got, tc.want)
			}
		})
	}
	if got := ProcessVariablesEscapeValues(`<a href="{{v}}">{{v}}</a>`, nil); got != `<a href="{{v}}">{{v}}</a>` {
		t.Errorf("con variables nil = %q", got)
	}
	if strings.Contains(ProcessVariablesEscapeValues(`{{v}}`, vars), "<b>") {
		t.Error("el valor sustituido no se escapó")
	}
}
