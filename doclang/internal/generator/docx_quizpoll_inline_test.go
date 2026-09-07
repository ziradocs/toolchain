// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// La pregunta, las opciones y la explicación de un quiz son texto del autor y
// tienen que pasar por el mismo renderer inline que un párrafo.
//
// Los tres se escribían con un SetText directo — el único lugar del generador
// DOCX que se saltaba walkDocxInlinePatterns—, así que salían crudos: una
// pregunta con `**negrita**` llegaba al .docx con los asteriscos a la vista, y
// un `[Ctrl]{.kbd}` con las llaves. El .docx es una salida de entrega, no un
// volcado del fuente.
func TestDOCXGenerator_QuizContentGoesThroughTheInlineRenderer(t *testing.T) {
	q := ast.NewQuizElement(diagnostics.NewPosition(1, 1))
	q.Question = "¿Cuál usa **negrita** y [Ctrl]{.kbd}?"
	q.Options = []string{"La *cursiva*", "La `código`"}
	q.Answer = 1
	q.Explanation = "Porque **esto** importa."

	output := filepath.Join(t.TempDir(), "quiz.docx")
	if err := New(newTestLogger()).Generate(astWithElements(q), output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	xml := docxDocumentXML(t, output)

	// Ningún delimitador de markdown ni de token puede quedar como texto.
	for _, literal := range []string{"**negrita**", "[Ctrl]{.kbd}", "*cursiva*", "`código`", "**esto**"} {
		if strings.Contains(xml, escapeForDocxText(literal)) {
			t.Errorf("%q salió literal al .docx; el contenido no pasó por el renderer inline", literal)
		}
	}

	// Y el texto sí está, partido en runs con su formato.
	for _, want := range []string{
		"<w:t>negrita</w:t>",
		"<w:t>Ctrl</w:t>",
		"<w:t>cursiva</w:t>",
		"<w:t>código</w:t>",
		"<w:t>esto</w:t>",
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("falta %s en document.xml — se perdió el texto, no solo el formato", want)
		}
	}

	// El numerador va en su propio run, separado del texto del autor: pegarlos
	// y mandar la cadena entera al renderer desplaza los offsets de los
	// patterns respecto de lo que el autor escribió.
	// El espacio final obliga a xml:space="preserve", así que se busca por el
	// contenido del <w:t> y no por la etiqueta pelada.
	if !strings.Contains(xml, ">1. </w:t>") {
		t.Errorf("el numerador no salió en su propio run; document.xml:\n%s", xml)
	}
}

// El estilo base del quiz (negrita en la pregunta, cursiva en la explicación)
// tiene que seguir puesto sobre CADA run que el renderer inline produzca, no
// solo sobre el primero: si no, el tramo `**negrita**` de una pregunta pierde
// la negrita base y se ve MENOS marcado que el texto que lo rodea.
func TestDOCXGenerator_QuizBaseStyleReachesEveryRun(t *testing.T) {
	q := ast.NewQuizElement(diagnostics.NewPosition(1, 1))
	q.Question = "Antes **medio** después"
	q.Options = []string{"a", "b"}
	q.Answer = 0

	output := filepath.Join(t.TempDir(), "quiz-base.docx")
	if err := New(newTestLogger()).Generate(astWithElements(q), output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	xml := docxDocumentXML(t, output)

	for _, text := range []string{"Antes ", "medio", " después"} {
		run := docxRunContaining(t, xml, text)
		if !strings.Contains(run, "<w:b") {
			t.Errorf("el run de %q no lleva la negrita base de la pregunta:\n%s", text, run)
		}
	}
}

// escapeForDocxText escapa un literal como lo haría el serializador de XML,
// para buscarlo en document.xml tal como aparecería si hubiera sobrevivido.
func escapeForDocxText(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// docxRunContaining devuelve el <w:r>…</w:r> cuyo <w:t> contiene text.
func docxRunContaining(t *testing.T, xml, text string) string {
	t.Helper()
	needle := "<w:t" // los runs pueden traer xml:space
	for pos := 0; ; {
		start := strings.Index(xml[pos:], "<w:r>")
		if start < 0 {
			break
		}
		start += pos
		end := strings.Index(xml[start:], "</w:r>")
		if end < 0 {
			break
		}
		run := xml[start : start+end+len("</w:r>")]
		if strings.Contains(run, needle) && strings.Contains(run, ">"+escapeForDocxText(text)+"<") {
			return run
		}
		pos = start + end
	}
	t.Fatalf("no hay ningún run con el texto %q en:\n%s", text, xml)
	return ""
}
