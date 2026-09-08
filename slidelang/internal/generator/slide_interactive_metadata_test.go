// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// jsControlSelectors son las marcas de clase que el JavaScript del visor busca
// para colgarle un handler a un elemento. Es la definición operativa de "el
// visor puede tocar esto", y por eso el test mide contra ellas y no contra
// `<button` u `onclick`.
//
// Buscar esos dos en el HTML sería VACUO, y esa fue la primera versión de este
// test: por el CSP con nonce (core/renderer/csp.go) el visor no puede usar
// atributos `onXXX=`, así que `initCopyButtons` crea el botón con
// `document.createElement` y `initInteractiveElements` asigna `.onclick` desde
// el script (template/utilities.go:100-127). En el HTML servido no hay ni un
// `<button` ni un `onclick` para un slide con código —medido: los dos ausentes
// del documento entero— y el guard pasaba sobre elementos que SÍ tienen
// control.
//
// La lista sale de los `querySelectorAll` de template/utilities.go y de los
// módulos de assets/js/modules/. Cada entrada es un conjunto de clases que
// tienen que aparecer juntas en el mismo elemento.
var jsControlSelectors = []struct {
	name    string
	classes []string
}{
	{"botón de copiar (initCopyButtons)", []string{"slidelang-element", "slidelang-code"}},
	{"tabs de code-group (initInteractiveElements)", []string{"slidelang-tab"}},
	{"toggle de details (initInteractiveElements)", []string{"slidelang-details"}},
	{"opción de quiz (quizpoll.js)", []string{"slidelang-option"}},
	{"opción de poll (quizpoll.js)", []string{"slidelang-poll-option"}},
	{"mapa de Leaflet (maps.js)", []string{"slidelang-map-container"}},
	{"canvas de Chart.js (charts.js)", []string{"slidelang-chart-canvas"}},
}

// `data-interactive` dice si el visor puede TOCAR algo del slide. El criterio
// quedó fijado en #290 al arreglar media —"hay algo que el visor puede
// tocar"— pero dos elementos seguían contradiciéndolo.
//
// El test afirma las dos direcciones contra el markup real: un slide marcado
// como interactivo tiene que traer al menos un selector de control del JS, y
// uno marcado como no interactivo no puede traer ninguno. Eso es lo que impide
// que el metadato y lo que se ve se vuelvan a separar, en cualquiera de los
// dos sentidos.
func TestRenderHTMLPreview_InteractiveMetadataMatchesTheControlsInTheMarkup(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	quiz := ast.NewQuizElement(pos)
	quiz.Question = "¿Dos más dos?"
	quiz.Options = []string{"3", "4"}
	quiz.Answer = 1

	for _, tc := range []struct {
		name string
		elem ast.Element
		want bool
		// wantType es el tipo esperado dentro de data-interactive-types, y
		// wantControl el selector de control que tiene que estar en el markup;
		// los dos solo aplican a los positivos.
		wantType    string
		wantControl []string
	}{
		// Los negativos, que es lo que este cambio corrige.
		{name: "quote", elem: ast.NewQuoteElement(pos, "Una cita estática."), want: false},
		{name: "mermaid", elem: ast.NewMermaidElement(pos, "graph", "graph TD\nA-->B"), want: false},

		// Y los positivos, para que "quitar los que no van" no se convierta en
		// quitar de más.
		//
		// El primero es el que este test corrigió: un CodeElement con lenguaje
		// mermaid renderiza `.slidelang-code` —la plantilla decide por tipo de
		// nodo, no por lenguaje— y sí recibe el botón de copiar. La primera
		// versión del cambio lo marcaba como no interactivo.
		{name: "code con lenguaje mermaid", elem: ast.NewCodeElement(pos, "mermaid", "graph TD\nA-->B"), want: true, wantType: "code",
			wantControl: []string{"slidelang-element", "slidelang-code"}},
		{name: "chart", elem: ast.NewChartElement(pos, "bar"), want: true, wantType: "chart",
			wantControl: []string{"slidelang-chart-canvas"}},
		{name: "map", elem: ast.NewMapElement(pos, "osm"), want: true, wantType: "map",
			wantControl: []string{"slidelang-map-container"}},
		{name: "code", elem: ast.NewCodeElement(pos, "go", "fmt.Println()"), want: true, wantType: "code",
			wantControl: []string{"slidelang-element", "slidelang-code"}},
		{name: "quiz", elem: quiz, want: true, wantType: "quiz",
			wantControl: []string{"slidelang-option"}},
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
				if !hasElementWithClasses(html, tc.wantControl) {
					t.Errorf("el metadato dice interactivo pero el markup no trae %v", tc.wantControl)
				}
				return
			}

			if got := slideDivAttr(t, html, "data-interactive-types", 0); got != "" {
				t.Errorf("data-interactive-types = %q, se esperaba vacío", got)
			}
			for _, sel := range jsControlSelectors {
				if hasElementWithClasses(html, sel.classes) {
					t.Errorf("el markup trae %s (%v): si ganó un control, el metadato tiene que decir true",
						sel.name, sel.classes)
				}
			}
		})
	}
}

// Un slide sin elementos no puede disparar ninguno de los selectores, o el
// guard de arriba estaría midiendo el chrome del visor en vez del contenido.
// Sin esto, agregar un botón de navegación con una de esas clases volvería
// rojos los negativos por la razón equivocada.
func TestRenderHTMLPreview_ChromeCarriesNoControlSelectors(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide vacío"
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	for _, sel := range jsControlSelectors {
		if hasElementWithClasses(html, sel.classes) {
			t.Errorf("el chrome del visor ya trae %s (%v); los negativos de interactividad medirían eso",
				sel.name, sel.classes)
		}
	}
}

// hasElementWithClasses dice si algún atributo class del HTML lleva todas las
// clases pedidas a la vez. Se mira el documento completo a propósito: aislar
// "el cuerpo del slide" con una expresión regular es lo que hizo vacua la
// primera versión —un `</div>` interno, como el del pie de una cita con autor,
// truncaba el fragmento antes del control—, y el chrome no lleva ninguna de
// estas clases, cosa que fija el test de acá arriba.
func hasElementWithClasses(html string, classes []string) bool {
	if len(classes) == 0 {
		return false
	}
	rest := html
	for {
		i := strings.Index(rest, `class="`)
		if i < 0 {
			return false
		}
		rest = rest[i+len(`class="`):]
		j := strings.Index(rest, `"`)
		if j < 0 {
			return false
		}
		present := strings.Fields(rest[:j])
		if containsAll(present, classes) {
			return true
		}
		rest = rest[j:]
	}
}

func containsAll(have []string, want []string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if h == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Todo bloque `::: tipo` que la función reconozca por NOMBRE tiene que traer un
// control en el markup, y ninguno de los que no reconoce puede traerlo.
//
// Las ramas de `chart`/`charts` y `map`/`maps` existían por el nombre del
// bloque, no por lo que se renderiza: `::: chart` con una línea de texto emite
// un contenedor de prosa, el linter tira SPECIAL001, no hay ni un `<canvas>` —y
// el slide salía con `data-interactive="true"`. Enumerar la lista completa acá
// es lo que impide volver a agregar una rama por el nombre.
func TestRenderHTMLPreview_EverySpecialBlockTypeMatchesItsMarkup(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	for _, tc := range []struct {
		blockType string
		want      bool
	}{
		// Los que la función reconoce hoy.
		{"details", true},

		// Los que se retiraron en este cambio.
		{"chart", false},
		{"charts", false},
		{"map", false},
		{"maps", false},
		// `::: code-group` (separado) renderiza sin tabs — #300. El
		// `<<code-group>>` real llega como CodeGroupElement, no por acá.
		{"code-group", false},
		{"codegroup", false},

		// Y los tipos de bloque que nunca estuvieron, como control: si alguno
		// empezara a traer un control, este test lo dice.
		{"note", false},
		{"tip", false},
		{"warning", false},
		{"info", false},
		{"collapsible", false},
	} {
		t.Run(tc.blockType, func(t *testing.T) {
			block := ast.NewContentBlock(pos, "content")
			block.Title = "Slide"
			sb := ast.NewSpecialBlockElement(pos, tc.blockType, "Contenido de prosa.")
			block.Elements = []ast.Element{sb}
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
			// Las DOS direcciones, sin atajos. La primera versión de este
			// test hacía `if tc.want { return }` justo acá, o sea que no
			// verificaba ningún positivo — y los positivos eran precisamente
			// donde estaba el residuo: `code-group` decía `true` sin traer un
			// solo `.slidelang-tab`. Un test que solo comprueba la mitad que
			// ya sabés que está bien no es un test.
			if tc.want {
				var found string
				for _, sel := range jsControlSelectors {
					if hasElementWithClasses(html, sel.classes) {
						found = sel.name
						break
					}
				}
				if found == "" {
					t.Errorf("`::: %s` dice interactivo y su markup no trae ningún selector de control", tc.blockType)
				}
				return
			}
			for _, sel := range jsControlSelectors {
				if hasElementWithClasses(html, sel.classes) {
					t.Errorf("el markup de `::: %s` trae %s (%v)", tc.blockType, sel.name, sel.classes)
				}
			}
		})
	}
}

// Un chart y un mapa PRE-RENDERIZADOS no son interactivos: en los modos offline
// el generador los dibuja al build y emite un <img>, sin <canvas> y sin Chart.js
// ni Leaflet en el bundle. El metadato decía `true` igual.
//
// El test recorre los tres modos con el MISMO deck, y afirma también los
// elementos que NO dependen del modo: el botón de copiar y el quiz siguen
// enganchados en offline, así que un gate por modo sobre toda la función sería
// el error simétrico al que esto corrige.
func TestRenderHTMLPreview_OfflineModesDropRasterizedInteractivity(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	for _, mode := range []string{"browser", "offline-assets", "offline-inline"} {
		t.Run(mode, func(t *testing.T) {
			chartBlock := ast.NewContentBlock(pos, "content")
			chartBlock.Title = "Chart"
			chartBlock.Elements = []ast.Element{ast.NewChartElement(pos, "bar")}

			codeBlock := ast.NewContentBlock(pos, "content")
			codeBlock.Title = "Code"
			codeBlock.Elements = []ast.Element{ast.NewCodeElement(pos, "go", "fmt.Println()")}

			doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*chartBlock, *codeBlock}}

			html, err := New(util.NewNoop()).RenderHTMLPreview(doc,
				GeneratorOptions{RenderMode: mode}, renderer.NewDefaultRenderContext())
			if err != nil {
				t.Fatalf("RenderHTMLPreview(%s): %v", mode, err)
			}

			wantChart := "true"
			if mode != "browser" {
				wantChart = "false"
			}
			if got := slideDivAttr(t, html, "data-interactive", 0); got != wantChart {
				t.Errorf("el slide del chart en %s: data-interactive = %q, se esperaba %q", mode, got, wantChart)
			}
			// El código no depende del modo: initCopyButtons le cuelga el
			// botón en los tres.
			if got := slideDivAttr(t, html, "data-interactive", 1); got != "true" {
				t.Errorf("el slide del código en %s: data-interactive = %q, se esperaba \"true\"", mode, got)
			}
		})
	}
}

// El metadato del slide no era el único lugar que ramificaba por el NOMBRE del
// bloque: `generateFeaturesSummary`, `getRequiredLibraries` (data/converter.go)
// y el detector de módulos (modules/detector.go) hacían lo mismo, y el test de
// arriba no los veía porque solo mira selectores de control por slide.
//
// Medido antes del arreglo: un deck con `::: chart` y `::: map` de pura prosa
// salía con `data-interactive="false"` en todos sus slides —correcto— y a la
// vez con `"hasMaps": true` y el módulo de Leaflet empaquetado.
func TestRenderHTMLPreview_ProseSpecialBlocksDoNotPullLibraries(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	for _, blockType := range []string{"chart", "charts", "map", "maps", "mermaid", "diagram", "code-group"} {
		t.Run(blockType, func(t *testing.T) {
			block := ast.NewContentBlock(pos, "content")
			block.Title = "Slide"
			block.Elements = []ast.Element{ast.NewSpecialBlockElement(pos, blockType, "Contenido de prosa.")}
			doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

			html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
			if err != nil {
				t.Fatalf("RenderHTMLPreview: %v", err)
			}

			// Ni el resumen de features ni la lista de librerías del JSON
			// embebido pueden declarar algo que el markup no tiene.
			for _, claim := range []string{`"hasCharts": true`, `"hasMaps": true`, `"hasMermaid": true`} {
				if strings.Contains(html, claim) {
					t.Errorf("`::: %s` declara %s y su markup es prosa", blockType, claim)
				}
			}
			for _, lib := range []string{`"chartjs"`, `"leaflet"`, `"mermaid"`} {
				if strings.Contains(html, `"libraries": [`) && strings.Contains(librariesOf(html), lib) {
					t.Errorf("`::: %s` pide la librería %s y su markup es prosa", blockType, lib)
				}
			}
		})
	}
}

// Y el positivo, para que "no pedir librerías" no se cumpla no pidiéndolas
// nunca: un chart de verdad sí las pide.
func TestRenderHTMLPreview_RealChartStillPullsItsLibrary(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = []ast.Element{ast.NewChartElement(pos, "bar")}
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	if !strings.Contains(html, `"hasCharts": true`) {
		t.Error("un ChartElement real dejó de declarar hasCharts")
	}
	if !strings.Contains(librariesOf(html), `"chartjs"`) {
		t.Errorf("un ChartElement real dejó de pedir chartjs; libraries = %s", librariesOf(html))
	}
}

// librariesOf extrae el arreglo "libraries" del JSON embebido.
func librariesOf(html string) string {
	i := strings.Index(html, `"libraries": [`)
	if i < 0 {
		return ""
	}
	rest := html[i:]
	j := strings.Index(rest, "]")
	if j < 0 {
		return rest
	}
	return rest[:j+1]
}
