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

// slideDivAttr devuelve el valor de attr en el div del slide número idx
// (0-based) del HTML generado.
func slideDivAttr(t *testing.T, html, attr string, idx int) string {
	t.Helper()
	divs := regexp.MustCompile(`<div class="slidelang-slide[^>]*>`).FindAllString(html, -1)
	if idx >= len(divs) {
		t.Fatalf("se pidió el slide %d y solo hay %d", idx, len(divs))
	}
	m := regexp.MustCompile(attr + `="([^"]*)"`).FindStringSubmatch(divs[idx])
	if m == nil {
		t.Fatalf("el div del slide %d no trae %s: %s", idx, attr, divs[idx])
	}
	return m[1]
}

// Los metadatos del div de un slide describen lo que se ve en él. Cuando no lo
// hacen, el que los consume —un visor externo, el JSON embebido, cualquier
// herramienta sobre el HTML— trabaja con una descripción falsa del documento y
// no tiene forma de notarlo.
//
// `data-slide-title` se elegía con `eq .Type "title"`, una comparación contra UN
// tipo. `title_slide`, `cover` e `intro` son slides de título en todas partes
// menos ahí: su texto vive en Heading, la condición leía Title, y salían con el
// atributo vacío teniendo un título bien puesto.
func TestRenderHTMLPreview_SlideTitleReachesTheDOMForEveryTitleAlias(t *testing.T) {
	for _, slideType := range []string{"title", "title_slide", "cover", "intro"} {
		t.Run(slideType, func(t *testing.T) {
			pos := diagnostics.NewPosition(1, 1)
			block := ast.NewContentBlock(pos, slideType)
			// El texto de un slide de título vive en Heading, no en Title:
			// es justamente lo que hacía fallar la condición vieja.
			block.Heading = "Portada real"
			doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

			html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
			if err != nil {
				t.Fatalf("RenderHTMLPreview: %v", err)
			}
			if got := slideDivAttr(t, html, "data-slide-title", 0); got != "Portada real" {
				t.Errorf("data-slide-title = %q, se esperaba %q", got, "Portada real")
			}
			// El mismo título tiene que ir al JSON embebido: son dos
			// consumidores del mismo dato y separarlos es lo que permitió que
			// uno se equivocara sin que el otro lo delatara.
			if !strings.Contains(html, `"title": "Portada real"`) {
				t.Errorf("el JSON embebido no trae el título del slide")
			}
		})
	}
}

// Un quiz y un poll son los únicos elementos que el visor responde con clics.
// Faltaban en detectInteractiveElements, así que un slide con un quiz se
// anunciaba como no interactivo mientras uno con una cita se anunciaba como
// interactivo.
func TestRenderHTMLPreview_QuizAndPollAreInteractive(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	quiz := ast.NewQuizElement(pos)
	quiz.Question = "¿Dos más dos?"
	quiz.Options = []string{"3", "4"}
	quiz.Answer = 1

	poll := ast.NewPollElement(pos)
	poll.Question = "¿Tu nivel?"
	poll.Options = []string{"a", "b"}

	for i, tc := range []struct {
		name string
		elem ast.Element
		want string
	}{
		{"quiz", quiz, "quiz"},
		{"poll", poll, "poll"},
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
			if got := slideDivAttr(t, html, "data-interactive", 0); got != "true" {
				t.Errorf("data-interactive = %q en un slide con un %s", got, tc.name)
			}
			if got := slideDivAttr(t, html, "data-interactive-types", 0); !strings.Contains(got, tc.want) {
				t.Errorf("data-interactive-types = %q, se esperaba que incluyera %q", got, tc.want)
			}
			_ = i
		})
	}
}

// estimateSlideDuration consultaba `slide.Type` —el NodeType del BaseNode
// embebido, "content_block" para todo slide— en vez de `slide.BlockType`.
// Ninguna rama del switch matcheaba nunca, así que el ajuste por tipo no
// existía: una sección y un cierre duraban lo mismo que un slide cualquiera.
//
// Los slides de este test llevan un elemento con texto a propósito. Con CERO
// palabras se dispara un piso de 15 segundos de "tiempo de lectura" que tapa las
// bases menores —el caso `title`, de 10s— y ese piso tiene su propio defecto
// (se aplica con 0 palabras y no con 1, porque `wordCount*60/200` da 0 hasta las
// 49). Es aparte, queda en #288, y acá se evita medir a través de él.
func TestRenderHTMLPreview_SlideDurationUsesTheDeclaredType(t *testing.T) {
	for _, tc := range []struct {
		slideType string
		want      string
	}{
		{"content", "30"},
		{"title", "10"},
		{"title_slide", "10"}, // alias de title
		{"section", "15"},
		{"chapter", "15"}, // alias de section
		{"closing", "20"},
		{"end", "20"},
	} {
		t.Run(tc.slideType, func(t *testing.T) {
			pos := diagnostics.NewPosition(1, 1)
			block := ast.NewContentBlock(pos, tc.slideType)
			block.Title = "Slide"
			block.Elements = []ast.Element{ast.NewTextElement(pos, "Una línea corta.")}
			doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

			html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
			if err != nil {
				t.Fatalf("RenderHTMLPreview: %v", err)
			}
			if got := slideDivAttr(t, html, "data-duration", 0); got != tc.want {
				t.Errorf("data-duration = %q para %q, se esperaba %q", got, tc.slideType, tc.want)
			}
		})
	}
}

// Un quiz impreso tiene que identificar la opción correcta, y solo impreso.
//
// El PDF revelaba la explicación y nada más: tres opciones idénticas y un
// párrafo explicando una que el lector no puede señalar. En el navegador, en
// cambio, marcarla sería dar la respuesta antes de la pregunta.
func TestRenderHTMLPreview_QuizMarksTheCorrectOptionForPrintOnly(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	quiz := ast.NewQuizElement(pos)
	quiz.Question = "¿Dos más dos?"
	quiz.Options = []string{"3", "4", "5"}
	quiz.Answer = 1

	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = []ast.Element{quiz}
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}

	// Exactamente una opción marcada, y es la declarada.
	if n := strings.Count(html, `data-correct="true"`); n != 1 {
		t.Fatalf("hay %d opciones marcadas como correctas, se esperaba 1", n)
	}
	marked := regexp.MustCompile(`data-index="(\d+)"[^>]*data-correct="true"|data-correct="true"[^>]*data-index="(\d+)"`).
		FindStringSubmatch(html)
	if marked == nil {
		t.Fatal("la opción marcada no lleva data-index")
	}
	if marked[1] != "1" && marked[2] != "1" {
		t.Errorf("se marcó la opción %v%v, se esperaba la 1", marked[1], marked[2])
	}

	// Y la regla que la dibuja vive DENTRO de un @media print.
	printBlock := printMediaBlock(t, html)
	if !strings.Contains(printBlock, "[data-correct]") {
		t.Error("no hay ninguna regla para [data-correct] dentro de @media print")
	}
	before, _, _ := strings.Cut(html, "@media print")
	if strings.Contains(before, "[data-correct]") {
		t.Error("hay una regla para [data-correct] FUERA de @media print: revela la respuesta en el navegador")
	}
}

// printMediaBlock devuelve el contenido del bloque @media print del bundle.
func printMediaBlock(t *testing.T, html string) string {
	t.Helper()
	start := strings.Index(html, "@media print")
	if start < 0 {
		t.Fatal("el bundle no trae ningún @media print")
	}
	depth, i := 0, start
	for ; i < len(html); i++ {
		switch html[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return html[start : i+1]
			}
		}
	}
	t.Fatal("el @media print no cierra")
	return ""
}
