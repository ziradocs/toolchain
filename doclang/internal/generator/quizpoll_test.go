// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func newQuizElement() *ast.QuizElement {
	q := ast.NewQuizElement(diagnostics.NewPosition(1, 1))
	q.Question = "¿Capital de Francia?"
	q.Options = []string{"Berlín", "París", "Madrid"}
	q.Answer = 1
	q.Explanation = "Desde 987."
	return q
}

// Markdown no tiene interacción, así que el quiz sale RESUELTO: la opción
// correcta marcada y la respuesta explícita. Un quiz que no se puede responder
// tiene que al menos enseñar la respuesta — mismo criterio que el HTML estático
// de core, que PPTX y que DOCX.
func TestMarkdownGenerator_QuizIsRenderedSolved(t *testing.T) {
	md := (&MarkdownGenerator{}).renderElement(newQuizElement())

	for _, want := range []string{
		"**¿Capital de Francia?**",
		"1. Berlín",
		"2. París ✅",
		"3. Madrid",
		"**Respuesta:** París",
		"*Desde 987.*",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("falta %q en:\n%s", want, md)
		}
	}
}

// Un poll no tiene respuesta correcta, así que no debe aparecer ninguna marca.
func TestMarkdownGenerator_PollHasNoAnswer(t *testing.T) {
	p := ast.NewPollElement(diagnostics.NewPosition(1, 1))
	p.Question = "¿Tu nivel?"
	p.Options = []string{"Inicial", "Avanzado"}

	md := (&MarkdownGenerator{}).renderElement(p)

	if !strings.Contains(md, "1. Inicial") || !strings.Contains(md, "2. Avanzado") {
		t.Errorf("faltan las opciones:\n%s", md)
	}
	for _, unwanted := range []string{"✅", "Respuesta"} {
		if strings.Contains(md, unwanted) {
			t.Errorf("un poll no debe traer %q:\n%s", unwanted, md)
		}
	}
}

// Un `answer` fuera de rango (que el linter reporta como QUIZ001) no debe
// producir una línea de respuesta inventada ni un índice fuera de los límites.
func TestMarkdownGenerator_QuizWithInvalidAnswerDoesNotPanic(t *testing.T) {
	for _, answer := range []int{-1, 9} {
		q := newQuizElement()
		q.Answer = answer

		md := (&MarkdownGenerator{}).renderElement(q)

		if strings.Contains(md, "**Respuesta:**") {
			t.Errorf("answer=%d produjo una respuesta inventada:\n%s", answer, md)
		}
		if !strings.Contains(md, "1. Berlín") {
			t.Errorf("answer=%d perdió las opciones:\n%s", answer, md)
		}
	}
}

// Issue #243 del lado de DOCX: sin esto, el texto salía con el TOKEN literal
// ("[Ctrl]{.kbd}") porque el pipeline inline de DOCX lee Markdown y no conocía
// esa forma. Y desde que el normalizador reescribe `<kbd>` a su token, ese
// literal es lo que vería cualquiera que escriba la tag.
func TestDOCXSpanTokenPattern_MatchesTheCanonicalForm(t *testing.T) {
	for _, tc := range []struct {
		in, text, class string
	}{
		{"[Ctrl]{.kbd}", "Ctrl", "kbd"},
		{"[2]{.sub}", "2", "sub"},
		{"[x]{.sup}", "x", "sup"},
		{"[esto]{.underline}", "esto", "underline"},
		{"[nota]{.small}", "nota", "small"},
	} {
		m := docxSpanTokenTextPattern.FindStringSubmatch(tc.in)
		if m == nil {
			t.Errorf("%q no matcheó — el token saldría literal en el DOCX", tc.in)
			continue
		}
		if m[1] != tc.text || m[2] != tc.class {
			t.Errorf("%q → texto %q clase %q, se esperaba %q/%q", tc.in, m[1], m[2], tc.text, tc.class)
		}
	}

	// Un span de idioma NO es un token de clase: lo maneja docxLangPattern.
	if docxSpanTokenTextPattern.MatchString("[bonjour]{lang=fr}") {
		t.Error("el patrón de clase se comió un span de idioma")
	}
}
