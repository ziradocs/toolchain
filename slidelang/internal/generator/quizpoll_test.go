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
	tmpl "go.ziradocs.com/slidelang/v2/internal/generator/template"
)

func astWithQuizAndPoll() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)

	quiz := ast.NewQuizElement(pos)
	quiz.Question = "¿Cuál usarías?"
	quiz.Options = []string{"Primera", "Segunda", "Tercera"}
	quiz.Answer = 1
	quiz.Explanation = "Porque hay ejemplos etiquetados."

	poll := ast.NewPollElement(pos)
	poll.Question = "¿Tu nivel?"
	poll.Options = []string{"Inicial", "Avanzado"}
	poll.Multiple = true

	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = []ast.Element{quiz, poll}
	return &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}
}

func renderQuizPoll(t *testing.T) string {
	t.Helper()
	html, err := New(util.NewNoop()).RenderHTMLPreview(
		astWithQuizAndPoll(), GeneratorOptions{}, renderer.NewDefaultRenderContext())
	if err != nil {
		t.Fatalf("RenderHTMLPreview: %v", err)
	}
	return html
}

func TestRenderHTMLPreview_QuizAndPollMarkup(t *testing.T) {
	html := renderQuizPoll(t)

	for _, want := range []string{
		`data-element-type="quiz"`,
		`data-answer="1"`,
		`data-element-type="poll"`,
		`data-multiple="true"`,
		`data-index="0"`,
		"¿Cuál usarías?",
		"Segunda",
		"Porque hay ejemplos etiquetados.",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("falta %q en el HTML", want)
		}
	}

	// Las opciones son <button>: el JS se ata al clic, y un <button> es
	// enfocable y accionable con teclado sin trabajo extra.
	if !strings.Contains(html, `<button type="button" class="slidelang-option"`) {
		t.Error("las opciones del quiz no son botones")
	}
}

// El bug que ningún test unitario habría visto: namespaceTemplateClasses
// (template/base.go) le pone el prefijo `slidelang-` a TODA clase estática del
// template, así que un CSS o un JS escrito contra la clase "pelada" no matchea
// nada. La primera versión de este PR tenía las tres piezas desalineadas y el
// elemento salía sin estilo y sin interacción.
//
// Este test las ata: para cada clase del contrato, el HTML, el CSS y el JS
// tienen que nombrar exactamente la misma.
func TestQuizPoll_HTMLCSSAndJSAgreeOnClassNames(t *testing.T) {
	html := renderQuizPoll(t)
	js := tmpl.GetQuizPollJS()

	// Todas las clases del contrato: el HTML las emite y el CSS las estila.
	for _, class := range []string{
		"slidelang-question",
		"slidelang-option",
		"slidelang-poll-option",
		"slidelang-explanation",
	} {
		if !strings.Contains(html, `class="`+class+`"`) {
			t.Errorf("el HTML no emite la clase %q", class)
		}
		if !strings.Contains(html, "."+class) {
			t.Errorf("el CSS del bundle no tiene ninguna regla para .%s", class)
		}
	}

	// Y las que el JS necesita para la interacción — la pregunta no está acá
	// porque el JS no la toca.
	for _, class := range []string{
		"slidelang-option",
		"slidelang-poll-option",
		"slidelang-explanation",
		"slidelang-poll-results",
	} {
		if !strings.Contains(js, "."+class) {
			t.Errorf("el JS no se ata a .%s", class)
		}
	}

	// Y ninguna clase del contrato puede quedar sin prefijo: si aparece
	// "pelada" en el HTML es que el namespacer no la tocó y el CSS no la
	// alcanzará.
	for _, bare := range []string{`class="question"`, `class="option"`, `class="poll-option"`} {
		if strings.Contains(html, bare) {
			t.Errorf("el HTML emite %s sin prefijo", bare)
		}
	}
}

// Sin JavaScript —PDF, o el HTML abierto con JS deshabilitado— el quiz tiene
// que enseñar la respuesta igual. El CSS de impresión revela la explicación.
func TestQuizPoll_PrintStylesRevealTheAnswer(t *testing.T) {
	html := renderQuizPoll(t)

	if !strings.Contains(html, "@media print") {
		t.Fatal("el bundle no trae reglas de impresión")
	}
	if !strings.Contains(html, ".slidelang-explanation[hidden]") {
		t.Error("la explicación no se revela al imprimir: un quiz en PDF quedaría sin respuesta")
	}
}
