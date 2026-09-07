// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/layouts"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/generator/css"
)

func astWithSlideTypes(types ...string) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := &ast.AST{}
	for _, t := range types {
		block := ast.NewContentBlock(pos, t)
		block.Title = "Slide " + t
		block.Elements = []ast.Element{ast.NewTextElement(pos, "Contenido.")}
		doc.ContentBlocks = append(doc.ContentBlocks, *block)
	}
	return doc
}

func TestDetectRequiredLayoutsFromAST(t *testing.T) {
	g := New(util.NewNoop())

	for _, tc := range []struct {
		name  string
		types []string
		want  []string
	}{
		{"solo los que tienen CSS", []string{"comparison", "content", "stats"}, []string{"comparison", "stats"}},
		{"sin repetir", []string{"stats", "stats", "stats"}, []string{"stats"}},
		{"ordenado", []string{"testimonial", "comparison", "hero"}, []string{"comparison", "hero", "testimonial"}},
		{"nombre inventado se filtra", []string{"comparision", "no_existe"}, []string{}},
		{"solo content", []string{"content", "title"}, []string{}},
		{"mayúsculas se normalizan", []string{"STATS"}, []string{"stats"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := g.detectRequiredLayoutsFromAST(astWithSlideTypes(tc.types...))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("= %v, se esperaba %v", got, tc.want)
			}
		})
	}

	if got := g.detectRequiredLayoutsFromAST(nil); got != nil {
		t.Errorf("con AST nil se esperaba nil, se obtuvo %v", got)
	}
}

// El punto entero del issue #254: que el CSS del layout llegue al bundle y que
// su selector matchee lo que el template emite.
func TestRenderHTMLPreview_LayoutCSSReachesTheDOM(t *testing.T) {
	doc := astWithSlideTypes("comparison")

	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	if !strings.Contains(html, `data-slide-type="comparison"`) {
		t.Fatal("el slide no lleva data-slide-type")
	}
	// Se afirma sobre el SELECTOR y no sobre el comentario de sección: la ruta
	// de preview minifica el CSS y los comentarios no sobreviven. El selector
	// sí, y es lo que de verdad importa — que exista una regla capaz de
	// matchear el `data-slide-type` que el template acaba de emitir, que es
	// exactamente lo que faltaba en #254.
	if !strings.Contains(html, `[data-slide-type="comparison"]`) {
		t.Error("el CSS emitido no tiene un selector que matchee el DOM — es el bug de #254")
	}
	if !strings.Contains(html, "grid-template-columns") {
		t.Error("el CSS del layout comparison no llegó al bundle")
	}
}

// Un deck sin layouts con CSS no debe arrastrar la sección.
func TestRenderHTMLPreview_ContentOnlyDeckHasNoLayoutCSS(t *testing.T) {
	html, err := New(util.NewNoop()).RenderHTMLPreview(
		astWithSlideTypes("content", "content"), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	for _, layout := range []string{"comparison", "stats", "hero", "testimonial"} {
		if strings.Contains(html, `[data-slide-type="`+layout+`"]`) {
			t.Errorf("un deck solo-content emitió el CSS de %q", layout)
		}
	}
}

// Tipar un slide no puede quitarle el vestido que ya tenía: la base y los temas
// externos hacen key en .slidelang-content-slide (issue #254).
func TestRenderHTMLPreview_TypedSlidesKeepContentChrome(t *testing.T) {
	html, err := New(util.NewNoop()).RenderHTMLPreview(
		astWithSlideTypes("stats", "title", "section"), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	if !strings.Contains(html, "slidelang-stats-slide slidelang-content-slide") {
		t.Error("un slide `stats` perdió el vestido de contenido")
	}
	// Los que tienen fondo propio no deben recibirlo: se lo pisaría. `hero` y
	// `call_to_action` entran acá aunque su fondo lo ponga el archivo de
	// layout y no la base — es la misma razón, y darles las dos cosas dejaba
	// un `call_to_action` con texto blanco sobre fondo blanco.
	ownChrome, err := New(util.NewNoop()).RenderHTMLPreview(
		astWithSlideTypes("title", "section", "hero", "call_to_action"), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	for _, own := range []string{"title", "section", "hero", "call_to_action"} {
		if strings.Contains(ownChrome, "slidelang-"+own+"-slide slidelang-content-slide") {
			t.Errorf("el slide %q, que tiene vestido propio, recibió el de contenido", own)
		}
	}

	// Un slide `content` NO debe recibirla dos veces: ya la trae por la vía
	// normal (`slidelang-{{.Type}}-slide`), y duplicarla en el atributo es un
	// error de html-validate (no-dup-class). Lo pescó el gate del corpus, no
	// los tests unitarios.
	contentHTML, err := New(util.NewNoop()).RenderHTMLPreview(
		astWithSlideTypes("content"), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	if strings.Contains(contentHTML, "slidelang-content-slide slidelang-content-slide") {
		t.Error("un slide `content` recibió la clase de contenido duplicada")
	}
}

// cssRule es una regla suelta del bundle: su selector y su cuerpo.
type cssRule struct {
	selector string
	body     string
}

// extractCSSRules parte el CSS del bundle en reglas. Es un partidor a la
// bruta —basta para reglas planas, que es todo lo que los archivos de layout
// escriben— y deliberadamente NO entiende @media: una regla dentro de una
// at-rule quedaría con el `@media …{` pegado al selector y no matchearía
// ninguna de las búsquedas de abajo, que es el lado seguro del error.
func extractCSSRules(bundle string) []cssRule {
	var rules []cssRule
	for _, chunk := range strings.Split(bundle, "}") {
		open := strings.Index(chunk, "{")
		if open == -1 {
			continue
		}
		rules = append(rules, cssRule{
			selector: strings.TrimSpace(chunk[:open]),
			body:     strings.TrimSpace(chunk[open+1:]),
		})
	}
	return rules
}

// declaresIn busca una regla cuyo selector contenga TODOS los fragmentos y
// cuyo cuerpo declare `decl`. Un `decl` vacío solo exige que la regla exista
// con cuerpo.
func declaresIn(rules []cssRule, selectorParts []string, decl string) bool {
	for _, rule := range rules {
		matches := true
		for _, part := range selectorParts {
			if !strings.Contains(rule.selector, part) {
				matches = false
				break
			}
		}
		if !matches || rule.body == "" {
			continue
		}
		if decl == "" || strings.Contains(rule.body, decl) {
			return true
		}
	}
	return false
}

// Issue #255: la opción llega al DOM como `data-layout-*` y el CSS la honra
// sin JavaScript. Es el mismo tipo de acoplamiento que rompió quiz/poll —
// atributo emitido por un lado y selector escrito por otro—, así que se ata
// con un test.
//
// El test recorre el REGISTRO (layouts.LayoutsWithOptions), no una lista a
// mano: una versión anterior probaba `comparison` y `hero`, que son justo los
// dos que funcionaban, y no vio que en `stats` y `call_to_action` la opción
// `columns` era inerte. Y no le alcanza con que el selector exista: exige que
// la regla declare algo que pueda surtir efecto, porque el defecto era
// exactamente una regla presente sobre un contenedor que no era grid.
func TestRenderHTMLPreview_LayoutOptionsReachTheDOMAndTheCSS(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	for _, layout := range layouts.LayoutsWithOptions() {
		for _, spec := range layouts.Options(layout) {
			for _, value := range optionTestValues(spec) {
				t.Run(layout+"/"+spec.Name+"="+value, func(t *testing.T) {
					block := ast.NewContentBlock(pos, layout)
					block.Title = "Título"
					block.Elements = []ast.Element{ast.NewTextElement(pos, "Contenido")}

					var config layouts.Config
					if err := layouts.Apply(&config, layout, spec.Name, value); err != nil {
						t.Fatalf("layouts.Apply(%s, %s=%s): %v", layout, spec.Name, value, err)
					}
					block.LayoutConfig = &ast.LayoutConfig{Columns: config.Columns, Align: config.Align}

					doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}
					html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
					if err != nil {
						t.Fatalf("RenderHTMLPreview: %v", err)
					}

					attr := "data-layout-" + spec.Name
					if !strings.Contains(html, attr+`="`+value+`"`) {
						t.Fatalf("el DOM no emite %s=%q", attr, value)
					}

					rules := extractCSSRules(html)
					scope := []string{`[data-slide-type="` + layout + `"]`, attr + `="` + value + `"`}

					switch spec.Type {
					case layouts.OptionInt:
						if !declaresIn(rules, scope, "grid-template-columns") {
							t.Errorf("ninguna regla de %s declara grid-template-columns para %s=%s", layout, spec.Name, value)
						}
						// La trampa que dejó la opción inerte: la regla estaba,
						// pero el contenedor no era una grilla, así que
						// grid-template-columns no hacía nada. `display: grid`
						// tiene que estar DENTRO del alcance de la opción, no
						// depender de qué otra regla ganó la cascada.
						if !declaresIn(rules, []string{`[data-slide-type="` + layout + `"]`, "[" + attr}, "display: grid") {
							t.Errorf("%s no declara `display: grid` bajo [%s]: la opción %s no puede surtir efecto",
								layout, attr, spec.Name)
						}
					case layouts.OptionEnum:
						if !declaresIn(rules, scope, "") {
							t.Errorf("el CSS de %s no tiene ninguna regla para %s=%s — el atributo se emitiría al vacío",
								layout, spec.Name, value)
						}
					}
				})
			}
		}
	}
}

// optionTestValues enumera los valores que el test prueba: todos los de un
// enum, y los extremos más un intermedio de un entero.
func optionTestValues(spec layouts.OptionSpec) []string {
	if spec.Type == layouts.OptionEnum {
		return spec.Values
	}
	var values []string
	for n := spec.Min; n <= spec.Max; n++ {
		values = append(values, strconv.Itoa(n))
	}
	return values
}

// Un slide sin opciones no debe arrastrar atributos vacíos.
func TestRenderHTMLPreview_NoLayoutOptionsEmitsNoAttributes(t *testing.T) {
	html, err := New(util.NewNoop()).RenderHTMLPreview(
		astWithSlideTypes("comparison"), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	// Se mira el <div> del slide, no el documento entero: el bundle SIEMPRE
	// trae los selectores `[data-layout-columns="N"]` del CSS, que no son
	// atributos emitidos.
	start := strings.Index(html, "<div class=\"slidelang-slide")
	if start == -1 {
		t.Fatal("no se encontró el div del slide")
	}
	div := html[start : start+strings.Index(html[start:], ">")]
	for _, attr := range []string{"data-layout-columns", "data-layout-align"} {
		if strings.Contains(div, attr) {
			t.Errorf("un slide sin opciones emitió %s en su div:\n%s", attr, div)
		}
	}
}

// selectorSpecificity cuenta la columna que decide entre estos selectores: la
// de clases, atributos y pseudo-clases. Los selectores de los archivos de
// layout y de los temas no usan ids ni pseudo-elementos, así que esta cuenta
// alcanza; devolver solo esa columna es deliberado —una cuenta completa
// invitaría a creer que el helper es un motor de CSS, y no lo es.
func selectorSpecificity(selector string) int {
	return strings.Count(selector, ".") + strings.Count(selector, "[")
}

// El fondo que declara un layout tiene que ganarle al del tema.
//
// No es hipotético: el CSS del tema se escribe DESPUÉS del de layouts
// (css/builder.go, pasos 4 y 5) y trae
// `.slidelang-slide.slidelang-content-slide { background: … }`. Mientras
// `call_to_action` llevó ADEMÁS esa clase, su fondo perdía por orden con el
// mismo peso (0,2,0) — pero su `color: var(--text-on-primary)` sobrevivía,
// porque el tema no lo declara. Resultado: texto blanco sobre fondo blanco,
// con las viñetas de un ejemplo real invisibles en pantalla.
//
// El test mira las clases que el slide REALMENTE lleva: una regla genérica
// solo compite si su selector le cae a este slide. Así el gate sigue siendo
// cierto si mañana cambia config.UsesContentChrome, que es donde se arregló.
func TestBundle_LayoutBackgroundBeatsThemeBackground(t *testing.T) {
	for _, layout := range css.GetAvailableLayouts() {
		t.Run(layout, func(t *testing.T) {
			html, err := New(util.NewNoop()).RenderHTMLPreview(
				astWithSlideTypes(layout), GeneratorOptions{}, renderer.NewDefaultRenderContext())
			if err != nil {
				t.Fatalf("RenderHTMLPreview: %v", err)
			}
			rules := extractCSSRules(html)
			slideClasses := slideClassesFor(t, html, layout)

			// La regla del layout: la que apunta a la RAÍZ del slide (sin
			// descendiente) y declara fondo.
			root := `[data-slide-type="` + layout + `"]`
			best, bestIdx := -1, -1
			for i, rule := range rules {
				if !strings.Contains(rule.selector, root) || !strings.Contains(rule.body, "background") {
					continue
				}
				if strings.Contains(rule.selector, root+" ") {
					continue
				}
				if s := selectorSpecificity(rule.selector); s > best {
					best, bestIdx = s, i
				}
			}
			if bestIdx == -1 {
				t.Skipf("%s no declara fondo propio", layout)
			}

			for i := bestIdx + 1; i < len(rules); i++ {
				sel := strings.TrimSpace(rules[i].selector)
				if !strings.Contains(rules[i].body, "background") {
					continue
				}
				// Solo compiten las reglas de slide genérico que de verdad le
				// caen a ESTE slide: un selector de otro tipo, o con una clase
				// que este slide no lleva, nunca lo matchea.
				if !appliesToSlide(sel, slideClasses) {
					continue
				}
				if selectorSpecificity(sel) >= best {
					t.Errorf("el fondo de %s (%q, especificidad %d) lo pisa una regla posterior que también le cae (%q, %d).\n"+
						"El slide lleva estas clases: %v — o se le saca la clase genérica (config.UsesContentChrome) o se califica el selector del layout.",
						layout, rules[bestIdx].selector, best, sel, selectorSpecificity(sel), slideClasses)
				}
			}
		})
	}
}

// slideClassesFor devuelve las clases del <div> del slide del tipo pedido.
// Busca entre los DIVS de slide, no la primera aparición del atributo en el
// documento: el bundle CSS va embebido y su primer `data-slide-type="hero"`
// es un selector, no un elemento.
func slideClassesFor(t *testing.T, html, layout string) []string {
	t.Helper()
	marker := `data-slide-type="` + layout + `"`
	const openTag = `<div class="slidelang-slide`

	for at := 0; ; {
		i := strings.Index(html[at:], openTag)
		if i == -1 {
			t.Fatalf("no se encontró el <div> de un slide %q en el HTML", layout)
		}
		at += i
		end := strings.Index(html[at:], ">")
		if end == -1 {
			t.Fatalf("etiqueta <div> sin cerrar para %q", layout)
		}
		tag := html[at : at+end]
		at += end

		if !strings.Contains(tag, marker) {
			continue
		}
		rest := tag[strings.Index(tag, `class="`)+len(`class="`):]
		return strings.Fields(rest[:strings.Index(rest, `"`)])
	}
}

// appliesToSlide reporta si un selector SIMPLE de slide (sin descendientes,
// sin otro data-slide-type) le cae a un slide con estas clases. Es a propósito
// conservador: cualquier forma que no entienda devuelve false, o sea "no
// compite", que es el lado que no inventa fallos.
func appliesToSlide(selector string, slideClasses []string) bool {
	if strings.ContainsAny(selector, " >,") || strings.Contains(selector, "[data-slide-type=") {
		return false
	}
	has := make(map[string]bool, len(slideClasses))
	for _, c := range slideClasses {
		has[c] = true
	}
	parts := strings.Split(strings.TrimPrefix(selector, "."), ".")
	for _, class := range parts {
		if class == "" || !has[class] {
			return false
		}
	}
	return len(parts) > 0
}
