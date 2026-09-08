// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// `data-interactive` dice si el visor puede TOCAR algo del slide. El criterio
// quedó fijado en #290 al arreglar media —"hay algo que el visor puede
// tocar"— pero dos elementos seguían contradiciéndolo.
//
// Los negativos no se conforman con el atributo: verifican además que el HTML
// del slide efectivamente no traiga ningún control. Si mañana una cita o un
// diagrama ganan un botón, este test falla y obliga a decidir a mano en vez de
// dejar el metadato mintiendo en la otra dirección.
func TestRenderHTMLPreview_InteractiveMetadataMatchesWhatTheViewerCanTouch(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	quiz := ast.NewQuizElement(pos)
	quiz.Question = "¿Dos más dos?"
	quiz.Options = []string{"3", "4"}
	quiz.Answer = 1

	for _, tc := range []struct {
		name string
		elem ast.Element
		want bool
		// tipo esperado dentro de data-interactive-types, solo para los
		// positivos.
		wantType string
	}{
		// Los negativos, que es lo que este cambio corrige.
		{name: "quote", elem: ast.NewQuoteElement(pos, "Una cita estática."), want: false},
		{name: "mermaid", elem: ast.NewMermaidElement(pos, "graph", "graph TD\nA-->B"), want: false},
		{name: "mermaid como fence de código", elem: ast.NewCodeElement(pos, "mermaid", "graph TD\nA-->B"), want: false},

		// Y los positivos, para que "quitar los que no van" no se convierta en
		// quitar de más: cada uno de estos tiene un control verificable.
		{name: "chart", elem: ast.NewChartElement(pos, "bar"), want: true, wantType: "chart"},
		{name: "map", elem: ast.NewMapElement(pos, "osm"), want: true, wantType: "map"},
		{name: "code", elem: ast.NewCodeElement(pos, "go", "fmt.Println()"), want: true, wantType: "code"},
		{name: "quiz", elem: quiz, want: true, wantType: "quiz"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			block := ast.NewContentBlock(pos, "content")
			block.Title = "Slide"
			block.Elements = []ast.Element{tc.elem}
			doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

			html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
			if err != nil {
				t.Fatalf("RenderHTMLPreview: %v", err)
			}

			want := "false"
			if tc.want {
				want = "true"
			}
			if got := slideDivAttr(t, html, "data-interactive", 0); got != want {
				t.Errorf("data-interactive = %q, se esperaba %q", got, want)
			}
			// El JSON embebido es el otro consumidor del mismo dato; los dos
			// tienen que decir lo mismo.
			if !strings.Contains(html, `"hasInteractive": `+want) {
				t.Errorf("el JSON embebido no dice hasInteractive = %s", want)
			}

			if tc.want {
				if got := slideDivAttr(t, html, "data-interactive-types", 0); !strings.Contains(got, tc.wantType) {
					t.Errorf("data-interactive-types = %q, se esperaba que incluyera %q", got, tc.wantType)
				}
				return
			}

			if got := slideDivAttr(t, html, "data-interactive-types", 0); got != "" {
				t.Errorf("data-interactive-types = %q, se esperaba vacío", got)
			}
			// Y lo que respalda el "false": el elemento no tiene con qué.
			assertNoControls(t, html, tc.name)
		})
	}
}

// assertNoControls falla si el markup del elemento trae algo clickeable. Es lo
// que separa "el metadato dice false" de "el metadato dice la verdad".
func assertNoControls(t *testing.T, html, name string) {
	t.Helper()

	// El botón de copiar lo cuelga initCopyButtons sobre
	// `.slidelang-element.slidelang-code`; el resto de los controles del visor
	// salen del template.
	body := slideBodyHTML(t, html)
	for _, needle := range []string{"<button", "onclick", "slidelang-copy-button", "slidelang-option", "<details"} {
		if strings.Contains(body, needle) {
			t.Errorf("el HTML de %s trae %q: si ganó un control, el metadato tiene que decir true", name, needle)
		}
	}
}

// slideBodyHTML devuelve el contenido del contenedor de elementos del primer
// slide, sin el chrome del visor (que sí trae botones de navegación).
func slideBodyHTML(t *testing.T, html string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<div class="slidelang-content-elements".*?</div>`).FindString(html)
	if m == "" {
		// Algunos layouts no envuelven en content-elements; caer al div del
		// slide es más ruidoso pero nunca menos estricto.
		m = regexp.MustCompile(`(?s)<div class="slidelang-slide.*?<script`).FindString(html)
	}
	if m == "" {
		t.Fatalf("no se pudo aislar el cuerpo del slide")
	}
	return m
}
