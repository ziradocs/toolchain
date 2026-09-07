// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func quizDiagIDs(diags []diagnostics.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.RuleID)
	}
	return out
}

func newQuiz(question string, options []string, answer int) *ast.QuizElement {
	q := ast.NewQuizElement(diagnostics.NewPosition(1, 1))
	q.Question = question
	q.Options = options
	q.Answer = answer
	return q
}

func newPoll(question string, options []string) *ast.PollElement {
	p := ast.NewPollElement(diagnostics.NewPosition(1, 1))
	p.Question = question
	p.Options = options
	return p
}

// QUIZ001 es el único Error de los dos elementos: un índice fuera de rango deja
// al quiz sin poder señalar la respuesta, así que no sirve ni como lectura.
func TestCheckQuizElement_AnswerOutOfRangeIsError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer int
	}{
		{"por arriba", 7},
		{"negativo", -2},
		{"no declarado", -1},
		{"justo fuera", 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := checkQuizElement(newQuiz("Q", []string{"a", "b", "c", "d"}, tc.answer))
			ids := quizDiagIDs(diags)
			if len(ids) != 1 || ids[0] != "QUIZ001" {
				t.Fatalf("diagnósticos = %v, se esperaba [QUIZ001]", ids)
			}
			if !diags[0].IsError() {
				t.Errorf("QUIZ001 debe ser Error, es %v", diags[0].Severity)
			}
			// El mensaje tiene que explicar la convención 0-based: es el error
			// que más se comete al escribir un quiz a mano.
			if !strings.Contains(diags[0].Message, "answer: 1 selects the second option") {
				t.Errorf("el mensaje no explica el índice 0-based: %q", diags[0].Message)
			}
		})
	}
}

func TestCheckQuizElement_ValidAnswerIsClean(t *testing.T) {
	for _, answer := range []int{0, 1, 3} {
		if diags := checkQuizElement(newQuiz("Q", []string{"a", "b", "c", "d"}, answer)); len(diags) != 0 {
			t.Errorf("answer=%d produjo %v", answer, quizDiagIDs(diags))
		}
	}
}

func TestCheckQuizElement_TooFewOptionsAndEmptyQuestion(t *testing.T) {
	diags := checkQuizElement(newQuiz("  ", []string{"solo una"}, 0))
	ids := quizDiagIDs(diags)
	if len(ids) != 2 || ids[0] != "QUIZ002" || ids[1] != "QUIZ003" {
		t.Fatalf("diagnósticos = %v, se esperaba [QUIZ002 QUIZ003]", ids)
	}
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("%s debe ser Warning, no Error", d.RuleID)
		}
	}
}

// Un poll no tiene respuesta correcta, así que no puede existir un equivalente
// de QUIZ001.
func TestCheckPollElement_NoAnswerRule(t *testing.T) {
	if diags := checkPollElement(newPoll("Q", []string{"a", "b"})); len(diags) != 0 {
		t.Errorf("un poll válido produjo %v", quizDiagIDs(diags))
	}

	diags := checkPollElement(newPoll("", []string{"a"}))
	ids := quizDiagIDs(diags)
	if len(ids) != 2 || ids[0] != "POLL001" || ids[1] != "POLL002" {
		t.Fatalf("diagnósticos = %v, se esperaba [POLL001 POLL002]", ids)
	}
}

// El despacho real: ElementStructureRule tiene que ver los dos elementos dentro
// de un slide, no solo las funciones sueltas.
func TestElementStructureRule_DispatchesQuizAndPoll(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = []ast.Element{
		newQuiz("Q", []string{"a", "b"}, 9),
		newPoll("P", []string{"solo una"}),
	}
	// ElementStructureRule.Check recibe el ContentBlock, no el AST: el linter
	// visita cada bloque por separado.
	rule := &ElementStructureRule{}
	ids := quizDiagIDs(rule.Check(block))

	want := map[string]bool{"QUIZ001": false, "POLL001": false}
	for _, id := range ids {
		if _, ok := want[id]; ok {
			want[id] = true
		}
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("%s no se emitió; ids = %v", id, ids)
		}
	}
}
