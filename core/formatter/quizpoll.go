// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// quizYAML y pollYAML fijan el ORDEN de las llaves en la salida del formatter.
// yaml.Marshal ordena los mapas alfabéticamente, pero sobre un struct respeta
// el orden de los campos, que es lo que queremos: `question` antes que
// `options` antes que `answer` se lee como la fuente que el autor escribió, no
// como un volcado alfabético.
type quizYAML struct {
	Question    string   `yaml:"question"`
	Options     []string `yaml:"options,omitempty"`
	Answer      *int     `yaml:"answer,omitempty"`
	Explanation string   `yaml:"explanation,omitempty"`
}

type pollYAML struct {
	Question string   `yaml:"question"`
	Options  []string `yaml:"options,omitempty"`
	Multiple bool     `yaml:"multiple,omitempty"`
}

// formatQuiz y formatPoll re-emiten el bloque tal como el parser lo acepta
// (issue #198), en los dos dialectos: el cuerpo es YAML, así que la misma
// función sirve para strict y para flex.
//
// El cuerpo se serializa con yaml.Marshal y no armando líneas a mano: eso
// garantiza el reparse para CUALQUIER texto de pregunta u opción —comillas,
// dos puntos, `#`, saltos de línea—, que es justo lo que quote()/checkQuotable
// no pueden prometer para un valor arbitrario.
//
// `answer` se omite cuando vale -1 (el centinela de "no declarado" del AST):
// emitir `answer: -1` produciría un documento que el linter rechaza con
// QUIZ001, o sea que fmt convertiría un deck con un aviso en uno que no
// compila.
func formatQuiz(e *ast.QuizElement) (string, error) {
	body := quizYAML{
		Question:    e.Question,
		Options:     e.Options,
		Explanation: e.Explanation,
	}
	if e.Answer >= 0 {
		answer := e.Answer
		body.Answer = &answer
	}
	return formatQuizPollBlock("quiz", body)
}

func formatPoll(e *ast.PollElement) (string, error) {
	return formatQuizPollBlock("poll", pollYAML{
		Question: e.Question,
		Options:  e.Options,
		Multiple: e.Multiple,
	})
}

func formatQuizPollBlock(tag string, body interface{}) (string, error) {
	yamlText, err := marshalYAMLIndent2(body)
	if err != nil {
		return "", fmt.Errorf("formatter: cuerpo de %s no serializable a YAML: %w", tag, err)
	}
	return fmt.Sprintf("<<%s>>\n%s\n<<end>>", tag, strings.TrimRight(yamlText, "\n")), nil
}
