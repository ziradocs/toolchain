// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/elements"
	"go.ziradocs.com/core/v2/util"
)

// reparseQuiz corre la salida del formatter por el parser real. Es lo único que
// prueba de verdad que `fmt` no rompe un deck: comparar strings solo dice que la
// salida no cambió, no que siga siendo válida.
func reparseQuiz(t *testing.T, formatted string) *ast.QuizElement {
	t.Helper()
	lines := strings.Split(formatted, "\n")
	ctx := &elements.ParseContext{Mode: "flex", Lines: lines, Logger: util.NewNoop()}
	result := (&elements.QuizParser{}).Parse(ctx, 0)
	if len(result.Diagnostics) > 0 {
		t.Fatalf("la salida del formatter no re-parsea limpio: %v\n%s", result.Diagnostics, formatted)
	}
	quiz, ok := result.Element.(*ast.QuizElement)
	if !ok {
		t.Fatalf("re-parseo dio %T", result.Element)
	}
	return quiz
}

func TestFormatQuiz_RoundTrip(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	original := ast.NewQuizElement(pos)
	original.Question = "Which type of ML would you use for email spam detection?"
	original.Options = []string{"Unsupervised learning", "Supervised learning", "Reinforcement"}
	original.Answer = 1
	original.Explanation = "We have labeled examples."

	formatted, err := formatQuiz(original)
	if err != nil {
		t.Fatalf("formatQuiz: %v", err)
	}
	if !strings.HasPrefix(formatted, "<<quiz>>\n") || !strings.HasSuffix(formatted, "\n<<end>>") {
		t.Errorf("delimitadores inesperados:\n%s", formatted)
	}

	got := reparseQuiz(t, formatted)
	if got.Question != original.Question {
		t.Errorf("Question: %q → %q", original.Question, got.Question)
	}
	if strings.Join(got.Options, "|") != strings.Join(original.Options, "|") {
		t.Errorf("Options: %#v → %#v", original.Options, got.Options)
	}
	if got.Answer != original.Answer {
		t.Errorf("Answer: %d → %d", original.Answer, got.Answer)
	}
	if got.Explanation != original.Explanation {
		t.Errorf("Explanation: %q → %q", original.Explanation, got.Explanation)
	}
}

// El motivo real de serializar con yaml.Marshal en vez de armar líneas a mano:
// un texto con comillas, dos puntos o `#` tiene que sobrevivir el round-trip.
func TestFormatQuiz_RoundTripsAwkwardText(t *testing.T) {
	for _, text := range []string{
		`Con "comillas" adentro`,
		"Con: dos puntos",
		"Con # numeral",
		"Con 'comilla simple'",
		"- empieza como guion",
		"true",
		"123",
		"Con \\ backslash",
		"Con [corchetes] y {llaves}",
	} {
		t.Run(text, func(t *testing.T) {
			pos := diagnostics.NewPosition(1, 1)
			original := ast.NewQuizElement(pos)
			original.Question = text
			original.Options = []string{text, "otra"}
			original.Answer = 0

			formatted, err := formatQuiz(original)
			if err != nil {
				t.Fatalf("formatQuiz: %v", err)
			}
			got := reparseQuiz(t, formatted)
			if got.Question != text {
				t.Errorf("Question: %q → %q\n%s", text, got.Question, formatted)
			}
			if len(got.Options) != 2 || got.Options[0] != text {
				t.Errorf("Options: %#v\n%s", got.Options, formatted)
			}
		})
	}
}

// `answer: -1` es el centinela de "no declarado" del AST. Emitirlo produciría un
// documento que el linter rechaza con QUIZ001: fmt convertiría un deck con un
// aviso en uno que no compila.
func TestFormatQuiz_OmitsSentinelAnswer(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	quiz := ast.NewQuizElement(pos)
	quiz.Question = "Q"
	quiz.Options = []string{"a", "b"}

	formatted, err := formatQuiz(quiz)
	if err != nil {
		t.Fatalf("formatQuiz: %v", err)
	}
	if strings.Contains(formatted, "answer") {
		t.Errorf("se emitió answer con el centinela:\n%s", formatted)
	}
	if got := reparseQuiz(t, formatted); got.Answer != -1 {
		t.Errorf("el round-trip perdió el centinela: %d", got.Answer)
	}
}

// answer: 0 SÍ tiene que emitirse: es "la primera opción", no "no declarado".
func TestFormatQuiz_EmitsZeroAnswer(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	quiz := ast.NewQuizElement(pos)
	quiz.Question = "Q"
	quiz.Options = []string{"a", "b"}
	quiz.Answer = 0

	formatted, err := formatQuiz(quiz)
	if err != nil {
		t.Fatalf("formatQuiz: %v", err)
	}
	if got := reparseQuiz(t, formatted); got.Answer != 0 {
		t.Errorf("Answer = %d, se esperaba 0:\n%s", got.Answer, formatted)
	}
}

func TestFormatPoll_RoundTrip(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	poll := ast.NewPollElement(pos)
	poll.Question = "What's your level?"
	poll.Options = []string{"Beginner", "Expert"}
	poll.Multiple = true

	formatted, err := formatPoll(poll)
	if err != nil {
		t.Fatalf("formatPoll: %v", err)
	}

	lines := strings.Split(formatted, "\n")
	ctx := &elements.ParseContext{Mode: "flex", Lines: lines, Logger: util.NewNoop()}
	result := (&elements.PollParser{}).Parse(ctx, 0)
	if len(result.Diagnostics) > 0 {
		t.Fatalf("no re-parsea limpio: %v\n%s", result.Diagnostics, formatted)
	}
	got := result.Element.(*ast.PollElement)
	if got.Question != poll.Question || len(got.Options) != 2 || !got.Multiple {
		t.Errorf("round-trip perdió datos: %#v\n%s", got, formatted)
	}
}

// El formatter es idempotente: formatear la salida del formatter da lo mismo.
func TestFormatQuiz_Idempotent(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	quiz := ast.NewQuizElement(pos)
	quiz.Question = "Q"
	quiz.Options = []string{"a", "b"}
	quiz.Answer = 1

	first, err := formatQuiz(quiz)
	if err != nil {
		t.Fatalf("formatQuiz: %v", err)
	}
	second, err := formatQuiz(reparseQuiz(t, first))
	if err != nil {
		t.Fatalf("formatQuiz (2a): %v", err)
	}
	if first != second {
		t.Errorf("no idempotente:\n--- 1a ---\n%s\n--- 2a ---\n%s", first, second)
	}
}
