// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

func parseQuiz(t *testing.T, lines ...string) (*ast.QuizElement, *ParseResult) {
	t.Helper()
	ctx := &ParseContext{Mode: "flex", Lines: lines, Logger: util.NewNoop()}
	result := (&QuizParser{}).Parse(ctx, 0)
	quiz, ok := result.Element.(*ast.QuizElement)
	if !ok {
		t.Fatalf("se esperaba *ast.QuizElement, se obtuvo %T", result.Element)
	}
	return quiz, result
}

func parsePoll(t *testing.T, lines ...string) (*ast.PollElement, *ParseResult) {
	t.Helper()
	ctx := &ParseContext{Mode: "flex", Lines: lines, Logger: util.NewNoop()}
	result := (&PollParser{}).Parse(ctx, 0)
	poll, ok := result.Element.(*ast.PollElement)
	if !ok {
		t.Fatalf("se esperaba *ast.PollElement, se obtuvo %T", result.Element)
	}
	return poll, result
}

func ruleIDs(result *ParseResult) []string {
	out := make([]string, 0, len(result.Diagnostics))
	for _, d := range result.Diagnostics {
		out = append(out, d.RuleID)
	}
	return out
}

// La forma canónica del issue #198: cuerpo en COLUMNA 0 (como los dos decks
// originales) y opciones como lista YAML.
func TestQuizParser_ColumnZeroBodyWithYAMLList(t *testing.T) {
	quiz, result := parseQuiz(t,
		"<<quiz>>",
		`question: "Which type of ML would you use for email spam detection?"`,
		"options:",
		`  - "Unsupervised learning"`,
		`  - "Supervised learning"`,
		`  - "Reinforcement learning"`,
		"answer: 1",
		`explanation: "We have labeled examples."`,
		"<<end>>",
		"Prosa después.",
	)

	if quiz.Question != "Which type of ML would you use for email spam detection?" {
		t.Errorf("Question = %q", quiz.Question)
	}
	if len(quiz.Options) != 3 || quiz.Options[1] != "Supervised learning" {
		t.Errorf("Options = %#v", quiz.Options)
	}
	if quiz.Answer != 1 {
		t.Errorf("Answer = %d, se esperaba 1", quiz.Answer)
	}
	if quiz.Explanation != "We have labeled examples." {
		t.Errorf("Explanation = %q", quiz.Explanation)
	}
	// 1 tag + 7 líneas de cuerpo + <<end>> = 9; la prosa NO se consume.
	if result.ConsumedLines != 9 {
		t.Errorf("ConsumedLines = %d, se esperaba 9", result.ConsumedLines)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("diagnósticos inesperados: %v", ruleIDs(result))
	}
}

// La otra forma que el issue fijó: array inline.
func TestPollParser_InlineArrayOptions(t *testing.T) {
	poll, result := parsePoll(t,
		"<<poll>>",
		`question: "What's your programming experience level?"`,
		`options: ["Beginner", "Intermediate", "Advanced", "Expert"]`,
		"multiple: false",
		"<<end>>",
	)

	if len(poll.Options) != 4 || poll.Options[0] != "Beginner" || poll.Options[3] != "Expert" {
		t.Errorf("Options = %#v", poll.Options)
	}
	if poll.Multiple {
		t.Error("Multiple debería ser false")
	}
	if result.ConsumedLines != 5 {
		t.Errorf("ConsumedLines = %d, se esperaba 5", result.ConsumedLines)
	}
}

func TestPollParser_MultipleTrue(t *testing.T) {
	poll, _ := parsePoll(t,
		"<<poll>>",
		`question: "Pick all that apply"`,
		`options: ["A", "B"]`,
		"multiple: true",
		"<<end>>",
	)
	if !poll.Multiple {
		t.Error("Multiple debería ser true")
	}
}

// Los decks originales (pre-stopgap) cierran con `<</quiz>>`, no con `<<end>>`.
// Tiene que consumirse igual: si quedara sin consumir, el flex failsafe
// emitiría un FLEX001 por cada cierre (isFlexFailsafeExempt solo perdona
// `<<end>>`).
func TestQuizParser_LegacyCloserIsConsumed(t *testing.T) {
	quiz, result := parseQuiz(t,
		"<<quiz>>",
		`question: "Q"`,
		`options: ["a", "b"]`,
		"answer: 0",
		"<</quiz>>",
	)
	if quiz.Question != "Q" {
		t.Errorf("Question = %q", quiz.Question)
	}
	if result.ConsumedLines != 5 {
		t.Errorf("ConsumedLines = %d, se esperaba 5 (el cierre legacy debe consumirse)", result.ConsumedLines)
	}
}

func TestPollParser_LegacyCloserIsConsumed(t *testing.T) {
	_, result := parsePoll(t, "<<poll>>", `question: "Q"`, `options: ["a", "b"]`, "<</poll>>")
	if result.ConsumedLines != 4 {
		t.Errorf("ConsumedLines = %d, se esperaba 4", result.ConsumedLines)
	}
}

// El cuerpo indentado es la forma de strict, donde el bloque va bajo un SLIDE.
func TestQuizParser_IndentedBody(t *testing.T) {
	quiz, result := parseQuiz(t,
		"    <<quiz>>",
		`      question: "Q"`,
		"      options:",
		`        - "a"`,
		`        - "b"`,
		"      answer: 1",
		"    <<end>>",
	)
	if quiz.Question != "Q" || len(quiz.Options) != 2 || quiz.Answer != 1 {
		t.Errorf("cuerpo indentado mal parseado: q=%q opts=%#v answer=%d", quiz.Question, quiz.Options, quiz.Answer)
	}
	if result.ConsumedLines != 7 {
		t.Errorf("ConsumedLines = %d, se esperaba 7", result.ConsumedLines)
	}
}

// Sin cerrador, el bloque termina en la frontera y NO la consume: la línea es
// del documento (regla del issue #192).
func TestQuizParser_StopsAtBoundaryWithoutConsumingIt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		boundary string
	}{
		{"separador de slide", "---"},
		{"heading H1", "# Otro slide"},
		{"heading H2", "## Otra sección"},
		{"SLIDE strict", "SLIDE content"},
		{"SECTION doclang", `SECTION "Intro"`},
		{"otro tag embebido", "<<chart: bar>>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quiz, result := parseQuiz(t,
				"<<quiz>>",
				`question: "Q"`,
				`options: ["a", "b"]`,
				"answer: 0",
				tc.boundary,
			)
			if quiz.Question != "Q" {
				t.Errorf("el cuerpo antes de la frontera se perdió: %q", quiz.Question)
			}
			if result.ConsumedLines != 4 {
				t.Errorf("ConsumedLines = %d, se esperaba 4 (la frontera no se consume)", result.ConsumedLines)
			}
		})
	}
}

// Un dedent por debajo del cuerpo cierra el bloque sin consumir: es el caso de
// strict donde sigue un elemento hermano.
func TestQuizParser_DedentEndsBlock(t *testing.T) {
	_, result := parseQuiz(t,
		"    <<quiz>>",
		`      question: "Q"`,
		`      options: ["a", "b"]`,
		"      answer: 0",
		"    TEXT",
	)
	if result.ConsumedLines != 4 {
		t.Errorf("ConsumedLines = %d, se esperaba 4", result.ConsumedLines)
	}
}

// Con el cuerpo en columna 0 el dedent NO puede disparar (bodyIndent es 0), así
// que el bloque tiene que llegar hasta el cerrador aunque haya líneas raras.
func TestQuizParser_ColumnZeroBodyIsNotCutByIndent(t *testing.T) {
	quiz, _ := parseQuiz(t,
		"<<quiz>>",
		`question: "Q"`,
		"options:",
		`  - "a"`,
		`  - "b"`,
		"answer: 1",
		"<<end>>",
	)
	if len(quiz.Options) != 2 {
		t.Errorf("Options = %#v; el array de columna 0 se cortó", quiz.Options)
	}
}

// YAML roto: el bloque se consume igual (no reaparece como prosa) y se reporta.
func TestQuizParser_InvalidYAMLIsReportedAndConsumed(t *testing.T) {
	quiz, result := parseQuiz(t,
		"<<quiz>>",
		`question: "sin cerrar`,
		"  - [{",
		"<<end>>",
	)
	if result.ConsumedLines != 4 {
		t.Errorf("ConsumedLines = %d, se esperaba 4", result.ConsumedLines)
	}
	if quiz.Question != "" {
		t.Errorf("se esperaba un quiz vacío, Question = %q", quiz.Question)
	}
	ids := ruleIDs(result)
	if len(ids) != 1 || ids[0] != "QUIZ004" {
		t.Errorf("diagnósticos = %v, se esperaba [QUIZ004]", ids)
	}
}

// Una llave que el tag no reconoce se reporta en vez de desaparecer: `feedback:`
// (que el website documentaba) y `answer:` dentro de un poll.
func TestQuizPollParser_UnknownKeysAreReported(t *testing.T) {
	_, result := parseQuiz(t,
		"<<quiz>>",
		`question: "Q"`,
		`options: ["a", "b"]`,
		"answer: 0",
		`feedback: "algo"`,
		"<<end>>",
	)
	ids := ruleIDs(result)
	if len(ids) != 1 || ids[0] != "QUIZ005" {
		t.Fatalf("diagnósticos = %v, se esperaba [QUIZ005]", ids)
	}
	if !strings.Contains(result.Diagnostics[0].Message, "feedback") {
		t.Errorf("el mensaje no nombra la llave: %q", result.Diagnostics[0].Message)
	}

	_, pollResult := parsePoll(t,
		"<<poll>>",
		`question: "Q"`,
		`options: ["a", "b"]`,
		"answer: 1",
		"<<end>>",
	)
	ids = ruleIDs(pollResult)
	if len(ids) != 1 || ids[0] != "POLL004" {
		t.Errorf("diagnósticos del poll = %v, se esperaba [POLL004]", ids)
	}
}

// answer ausente queda en -1 (el centinela), no en 0: "no declarado" no puede
// confundirse con "la primera opción es la correcta".
func TestQuizParser_MissingAnswerIsSentinel(t *testing.T) {
	quiz, _ := parseQuiz(t, "<<quiz>>", `question: "Q"`, `options: ["a", "b"]`, "<<end>>")
	if quiz.Answer != -1 {
		t.Errorf("Answer = %d, se esperaba -1", quiz.Answer)
	}
}

func TestQuizParser_AnswerZeroIsNotTheSentinel(t *testing.T) {
	quiz, _ := parseQuiz(t, "<<quiz>>", `question: "Q"`, `options: ["a", "b"]`, "answer: 0", "<<end>>")
	if quiz.Answer != 0 {
		t.Errorf("Answer = %d, se esperaba 0", quiz.Answer)
	}
}

// Las blancas dentro del cuerpo se consumen y no lo cortan.
func TestQuizParser_BlankLinesInsideBody(t *testing.T) {
	quiz, result := parseQuiz(t,
		"<<quiz>>",
		`question: "Q"`,
		"",
		`options: ["a", "b"]`,
		"",
		"answer: 1",
		"<<end>>",
	)
	if quiz.Answer != 1 || len(quiz.Options) != 2 {
		t.Errorf("las blancas rompieron el parseo: %#v", quiz)
	}
	if result.ConsumedLines != 7 {
		t.Errorf("ConsumedLines = %d, se esperaba 7", result.ConsumedLines)
	}
}

// CanParse exige el tag exacto: sin esto `<<quizzes>>` entraría por prefijo.
func TestQuizPollParser_CanParseRequiresExactTag(t *testing.T) {
	quiz := &QuizParser{}
	poll := &PollParser{}
	for _, tc := range []struct {
		line string
		want bool
	}{
		{"<<quiz>>", true},
		{"  <<quiz>>  ", true},
		{"<<quizzes>>", false},
		{"<<quiz: hard>>", false},
		{"<<quiz", false},
		{"quiz", false},
		{"<<poll>>", false},
	} {
		if got := quiz.CanParse(tc.line, "flex"); got != tc.want {
			t.Errorf("QuizParser.CanParse(%q) = %v, se esperaba %v", tc.line, got, tc.want)
		}
	}
	for _, tc := range []struct {
		line string
		want bool
	}{
		{"<<poll>>", true},
		{"<<polls>>", false},
		{"<<quiz>>", false},
	} {
		if got := poll.CanParse(tc.line, "flex"); got != tc.want {
			t.Errorf("PollParser.CanParse(%q) = %v, se esperaba %v", tc.line, got, tc.want)
		}
	}
}

// El registry tiene que despachar los dos tags: es lo que los hace funcionar en
// flex, donde no hay cadena if/else escrita a mano.
func TestDefaultRegistry_DispatchesQuizAndPoll(t *testing.T) {
	registry := GetDefaultRegistry()
	for tag, wantType := range map[string]string{"<<quiz>>": "quiz", "<<poll>>": "poll"} {
		lines := []string{tag, `question: "Q"`, `options: ["a", "b"]`, "answer: 0", "<<end>>"}
		ctx := &ParseContext{Mode: "flex", Lines: lines, Logger: util.NewNoop()}
		result := registry.Parse(ctx, 0)
		if result.Element == nil {
			t.Fatalf("%s: el registry no produjo elemento", tag)
		}
		if got := string(result.Element.GetType()); got != wantType {
			t.Errorf("%s: tipo %q, se esperaba %q", tag, got, wantType)
		}
	}
}

// Un tab inicial en el cuerpo no debe romper el YAML (YAML rechaza tabs como
// indentación): se expanden antes de deserializar.
func TestQuizParser_TabIndentedBodyIsAccepted(t *testing.T) {
	quiz, _ := parseQuiz(t,
		"<<quiz>>",
		"\tquestion: \"Q\"",
		"\toptions:",
		"\t  - \"a\"",
		"\t  - \"b\"",
		"\tanswer: 0",
		"<<end>>",
	)
	if quiz.Question != "Q" || len(quiz.Options) != 2 {
		t.Errorf("el cuerpo con tabs no parseó: q=%q opts=%#v", quiz.Question, quiz.Options)
	}
}

// EOF sin cerrador: el bloque consume lo que hay y no entra en bucle.
func TestQuizParser_UnterminatedBlockAtEOF(t *testing.T) {
	quiz, result := parseQuiz(t, "<<quiz>>", `question: "Q"`, `options: ["a", "b"]`)
	if quiz.Question != "Q" {
		t.Errorf("Question = %q", quiz.Question)
	}
	if result.ConsumedLines != 3 {
		t.Errorf("ConsumedLines = %d, se esperaba 3", result.ConsumedLines)
	}
}
