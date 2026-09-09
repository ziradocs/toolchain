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

// La duración era la QUINTA función que ramificaba por el nombre del bloque, y
// la última: `estimateSlideDuration` sumaba los 15 segundos de "elemento
// complejo" a un `::: chart` de pura prosa, así que un párrafo pesaba 45
// segundos en vez de 30. Un chart de verdad sí los suma, y eso no cambia.
func TestRenderHTMLPreview_ProseSpecialBlockDoesNotCostComplexTime(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	dur := func(elem ast.Element) string {
		block := ast.NewContentBlock(pos, "content")
		block.Title = "Slide"
		block.Elements = []ast.Element{elem}
		doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}
		html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
		if err != nil {
			t.Fatalf("RenderHTMLPreview: %v", err)
		}
		return slideDivAttr(t, html, "data-duration", 0)
	}

	base := dur(ast.NewSpecialBlockElement(pos, "note", "Contenido de prosa."))
	for _, blockType := range []string{"chart", "map", "mermaid"} {
		t.Run(blockType, func(t *testing.T) {
			if got := dur(ast.NewSpecialBlockElement(pos, blockType, "Contenido de prosa.")); got != base {
				t.Errorf("`::: %s` de prosa dura %s y un `::: note` idéntico dura %s: se le cobró tiempo de elemento complejo", blockType, got, base)
			}
		})
	}

	t.Run("un chart real sí cuesta más", func(t *testing.T) {
		if got := dur(ast.NewChartElement(pos, "bar")); got == base {
			t.Errorf("un ChartElement real dura %s, igual que un bloque de prosa: se perdió el ajuste por elemento complejo", got)
		}
	})
}

// Un `CodeElement` con lenguaje mermaid renderiza `.slidelang-code` —el test de
// interactividad de más arriba fija que hasta recibe el botón de copiar—, así
// que declarar `hasMermaid`, pedir la librería y empaquetar el módulo describen
// un diagrama que no está en la página. Era la misma decisión por LENGUAJE en
// vez de por tipo de nodo, en tres funciones.
func TestRenderHTMLPreview_CodeElementWithMermaidLanguageIsCode(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = []ast.Element{ast.NewCodeElement(pos, "mermaid", "graph TD\nA-->B")}
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	if strings.Contains(html, `"hasMermaid": true`) {
		t.Error(`declara "hasMermaid": true y su markup es un bloque de código`)
	}
	if strings.Contains(librariesOf(html), `"mermaid"`) {
		t.Errorf("pide la librería mermaid; libraries = %s", librariesOf(html))
	}
	// Y sí es código: el markup que initCopyButtons engancha.
	if !hasElementWithClasses(html, []string{"slidelang-element", "slidelang-code"}) {
		t.Error("el markup dejó de ser un bloque de código")
	}
}

// El positivo del par: un MermaidElement de verdad sí declara y sí pide.
func TestRenderHTMLPreview_RealMermaidStillPullsItsLibrary(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = []ast.Element{ast.NewMermaidElement(pos, "graph", "graph TD\nA-->B")}
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	if !strings.Contains(html, `"hasMermaid": true`) {
		t.Error("un MermaidElement real dejó de declarar hasMermaid")
	}
	if !strings.Contains(librariesOf(html), `"mermaid"`) {
		t.Errorf("un MermaidElement real dejó de pedir mermaid; libraries = %s", librariesOf(html))
	}
}

// El elemento REAL de code-group nunca tuvo case en las funciones de features,
// librerías y módulos: el `hasCode` de un deck cuyo código vive en
// `::::code-group` salía de la rama por NOMBRE del bloque especial, o sea por
// accidente. Al retirar esa rama, dos decks del corpus con tabs de verdad
// pasaron a declarar `hasCode: false` — lo cazó el barrido de corpus, no los
// tests, y por eso este test existe.
func TestRenderHTMLPreview_RealCodeGroupDeclaresItself(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	cg := ast.NewCodeGroupElement(pos)
	cg.CodeBlocks = []ast.CodeBlock{
		{Label: "Go", Language: "go", Content: "fmt.Println()"},
		{Label: "Python", Language: "python", Content: "print()"},
	}

	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = []ast.Element{cg}
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	if !hasElementWithClasses(html, []string{"slidelang-tab"}) {
		t.Fatal("el fixture no produjo tabs; el test no estaría midiendo nada")
	}
	if !strings.Contains(html, `"hasCode": true`) {
		t.Error(`un code-group real con tabs no declara "hasCode": true`)
	}
}

// `--no-utilities` es el otro interruptor que deja el metadato mintiendo.
//
// utilities.js trae `initCopyButtons` —el botón de copiar de todo
// `.slidelang-element.slidelang-code`— e `initInteractiveElements` —los
// `.onclick` de las tabs de un code-group y del toggle de un details—. Con la
// opción apagada el módulo no se empaqueta y esos tres elementos se quedan sin
// handler, pero salían igual con `data-interactive="true"`.
//
// El test NO puede medir esto con jsControlSelectors: el markup no cambia entre
// los dos builds —`.slidelang-code` sigue ahí, solo que sin botón—, así que un
// guard sobre las clases pasa en los dos y no distingue nada. Lo que distingue
// es el BUNDLE, y por eso las afirmaciones van sobre el JS embebido
// (RenderHTMLPreview fuerza EmbedAssets, así que el módulo, si entra, entra
// inline en el HTML).
//
// El par es una sola presentación construida dos veces: lo único que cambia es
// la opción.
func TestRenderHTMLPreview_NoUtilitiesDropsHandlerDrivenInteractivity(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	quiz := ast.NewQuizElement(pos)
	quiz.Question = "¿Dos más dos?"
	quiz.Options = []string{"3", "4"}
	quiz.Answer = 1

	// Un slide por elemento, en este orden, para poder preguntar por índice.
	slides := []struct {
		name string
		elem ast.Element
		// wantConUtilities y wantSinUtilities son el data-interactive esperado
		// en cada uno de los dos builds.
		wantConUtilities bool
		wantSinUtilities bool
	}{
		{name: "código", elem: ast.NewCodeElement(pos, "go", "fmt.Println()"), wantConUtilities: true, wantSinUtilities: false},
		{name: "code-group", elem: ast.NewCodeGroupElement(pos), wantConUtilities: true, wantSinUtilities: false},
		{name: "details", elem: ast.NewSpecialBlockElement(pos, "details", "Contenido plegable"), wantConUtilities: true, wantSinUtilities: false},
		// Los que NO dependen de utilities.js: filtrar la función entera por
		// la opción sería el error simétrico, y estas dos filas lo impiden.
		{name: "quiz (quizpoll.js)", elem: quiz, wantConUtilities: true, wantSinUtilities: true},
		{name: "chart (charts.js)", elem: ast.NewChartElement(pos, "bar"), wantConUtilities: true, wantSinUtilities: true},
	}

	blocks := make([]ast.ContentBlock, 0, len(slides))
	for _, s := range slides {
		b := ast.NewContentBlock(pos, "content")
		b.Title = s.name
		b.Elements = []ast.Element{s.elem}
		blocks = append(blocks, *b)
	}
	doc := &ast.AST{ContentBlocks: blocks}

	for _, build := range []struct {
		name         string
		opts         GeneratorOptions
		wantHandlers bool
	}{
		{name: "con utilities", opts: GeneratorOptions{}, wantHandlers: true},
		{name: "--no-utilities", opts: GeneratorOptions{NoUtilities: true}, wantHandlers: false},
	} {
		t.Run(build.name, func(t *testing.T) {
			html, err := New(util.NewNoop()).RenderHTMLPreview(doc, build.opts, renderer.NewDefaultRenderContext())
			if err != nil {
				t.Fatalf("RenderHTMLPreview: %v", err)
			}

			// Primero el bundle, que es la premisa de todo lo demás: si los
			// handlers no están donde este test cree, las afirmaciones sobre
			// el metadato no miden nada.
			for _, handler := range []string{"initCopyButtons", "initInteractiveElements"} {
				if got := strings.Contains(html, handler); got != build.wantHandlers {
					t.Fatalf("%s en el JS empaquetado = %v, se esperaba %v", handler, got, build.wantHandlers)
				}
			}

			for i, s := range slides {
				want := "false"
				if (build.wantHandlers && s.wantConUtilities) || (!build.wantHandlers && s.wantSinUtilities) {
					want = "true"
				}
				if got := slideDivAttr(t, html, "data-interactive", i); got != want {
					t.Errorf("slide %d (%s): data-interactive = %q, se esperaba %q",
						i, s.name, got, want)
				}
			}
		})
	}
}
