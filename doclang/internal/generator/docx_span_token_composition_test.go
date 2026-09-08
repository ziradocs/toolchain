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

// Un token de span puede llevar formato adentro, y el sitio lo documenta con
// `[**combine**]{.danger}`. El apply del token escribía su contenido con un
// SetText único, así que `[**Ctrl**]{.kbd}` llegaba al .docx con los asteriscos
// a la vista mientras el HTML del mismo documento salía
// `<kbd class="slidelang-kbd"><strong>Ctrl</strong></kbd>`.
//
// Es el mismo arreglo que docxApplyLangSpan ya tenía y el que PPTX hace con
// applySpanTokens: recursar por code/bold/italic y estampar el estilo del token
// sobre cada run que salga de ahí.
func TestDOCXGenerator_SpanTokenComposesWithInnerFormatting(t *testing.T) {
	text := ast.NewTextElement(diagnostics.NewPosition(1, 1),
		"Un [**Ctrl**]{.kbd} y un [**combine**]{.danger} y un [*x*]{.underline} y un [`cod`]{.kbd}.")

	output := filepath.Join(t.TempDir(), "span-token.docx")
	if err := New(newTestLogger()).Generate(astWithElements(text), output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	xml := docxDocumentXML(t, output)

	// Ni el delimitador del token ni el de markdown pueden quedar como texto.
	for _, literal := range []string{"[**Ctrl**]{.kbd}", "**Ctrl**", "**combine**", "*x*", "`cod`"} {
		if strings.Contains(xml, escapeForDocxText(literal)) {
			t.Errorf("%q salió literal al .docx", literal)
		}
	}

	// `kbd` compone con la negrita: el run lleva las dos cosas. Que solo
	// sobreviva el texto no alcanza —eso ya pasaba antes, con los asteriscos
	// pegados.
	kbd := docxRunContaining(t, xml, "Ctrl")
	if !strings.Contains(kbd, "<w:b") {
		t.Errorf("el run de kbd perdió la negrita de adentro:\n%s", kbd)
	}
	if !strings.Contains(kbd, `w:ascii="Consolas"`) {
		t.Errorf("el run de kbd perdió la fuente monoespaciada del token:\n%s", kbd)
	}

	// `underline` compone con la cursiva.
	u := docxRunContaining(t, xml, "x")
	if !strings.Contains(u, "<w:i") {
		t.Errorf("el run de underline perdió la cursiva de adentro:\n%s", u)
	}
	if !strings.Contains(u, "<w:u ") {
		t.Errorf("el run de underline perdió el subrayado del token:\n%s", u)
	}

	// Una clase que el generador todavía no representa —`danger` no estila
	// nada— conserva el texto y el formato interno, y no imprime la sintaxis.
	// Que no lo represente NO es un límite de la librería: docxgo expone
	// SetColor, y este mismo archivo lo usa para el color base. Es un hueco por
	// hacer (#283), y lo que este test fija es lo mínimo exigible mientras
	// tanto: el token no se ve y el formato de adentro no se pierde.
	danger := docxRunContaining(t, xml, "combine")
	if !strings.Contains(danger, "<w:b") {
		t.Errorf("el run de una clase sin representación perdió la negrita de adentro:\n%s", danger)
	}
}

// El estilo BASE del contexto —la cursiva de una explicación de quiz, la
// negrita de una pregunta— tiene que seguir llegando a los runs que produce el
// token, y eso depende de que el estampado del token propague su `postRun`.
//
// Sin este test la propagación quedaba sin cubrir: quitarla dejaba TODA la
// suite en verde, y el daño es exactamente el caso que la descripción del PR
// dice haber verificado a mano. Medido con la propagación quitada, la
// explicación en cursiva pierde la cursiva justo en el tramo del token:
// `'Ctrl' [B, Consolas]` en vez de `[B, I, Consolas]`.
func TestDOCXGenerator_SpanTokenKeepsTheSurroundingBaseStyle(t *testing.T) {
	q := ast.NewQuizElement(diagnostics.NewPosition(1, 1))
	q.Question = "¿Sirve [**Ctrl**]{.kbd}?"
	q.Options = []string{"Con [**Ctrl**]{.kbd}", "No"}
	q.Answer = 0
	q.Explanation = "Explicación con [**Ctrl**]{.kbd} adentro."

	output := filepath.Join(t.TempDir(), "quiz-token.docx")
	if err := New(newTestLogger()).Generate(astWithElements(q), output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	xml := docxDocumentXML(t, output)

	if strings.Contains(xml, escapeForDocxText("**Ctrl**")) {
		t.Fatal("el token salió literal dentro del quiz")
	}

	// Tres runs con el mismo texto, uno por campo. El de la explicación es el
	// que lleva la cursiva base; los otros dos, la negrita de la pregunta y de
	// la opción correcta.
	var estilos []string
	for _, run := range docxRunsContaining(xml, "Ctrl") {
		var s string
		if strings.Contains(run, "<w:b") {
			s += "B"
		}
		if strings.Contains(run, "<w:i") {
			s += "I"
		}
		if strings.Contains(run, `w:ascii="Consolas"`) {
			s += "M"
		}
		estilos = append(estilos, s)
	}
	if len(estilos) != 3 {
		t.Fatalf("se esperaban 3 runs con el token, hay %d: %v", len(estilos), estilos)
	}
	// El de la explicación tiene que traer la cursiva del contexto además de
	// la negrita de adentro y la monoespaciada del token.
	var conCursiva int
	for _, s := range estilos {
		if strings.Contains(s, "I") {
			conCursiva++
		}
		if !strings.Contains(s, "B") || !strings.Contains(s, "M") {
			t.Errorf("un run del token perdió la negrita interna o la fuente del token: %q (todos: %v)", s, estilos)
		}
	}
	if conCursiva != 1 {
		t.Errorf("runs con la cursiva base de la explicación: %d, se esperaba 1 (estilos: %v)", conCursiva, estilos)
	}
}

// docxRunsContaining devuelve TODOS los <w:r>…</w:r> cuyo <w:t> es text.
// docxRunContaining (docx_quizpoll_inline_test.go) devuelve solo el primero,
// que no sirve cuando el mismo texto aparece en varios campos.
func docxRunsContaining(xml, text string) []string {
	var out []string
	needle := ">" + escapeForDocxText(text) + "<"
	for pos := 0; ; {
		start := strings.Index(xml[pos:], "<w:r>")
		if start < 0 {
			return out
		}
		start += pos
		end := strings.Index(xml[start:], "</w:r>")
		if end < 0 {
			return out
		}
		run := xml[start : start+end+len("</w:r>")]
		if strings.Contains(run, needle) {
			out = append(out, run)
		}
		pos = start + end
	}
}

// El estilo del token se estampa DESPUÉS del estilo del pattern interno y solo
// agrega: si pusiera fuente base, le pisaría la Consolas al patrón de code.
// Este test fija esa dirección con el caso donde chocan —un `code` adentro de
// un token que no toca la fuente.
//
// Efecto lateral de que el pattern de code ahora corra ahí adentro, que antes
// no pasaba: un “ `cod` “ dentro de un token también hereda el TAMAÑO y el
// COLOR del código inline, no solo la fuente. Es lo mismo que le pasa a un
// “ `cod` “ suelto en prosa, así que la salida queda más consistente, pero es
// un cambio observable.
func TestDOCXGenerator_SpanTokenStampDoesNotClobberInnerCodeFont(t *testing.T) {
	text := ast.NewTextElement(diagnostics.NewPosition(1, 1), "Un [`cod`]{.underline} suelto.")

	output := filepath.Join(t.TempDir(), "span-token-code.docx")
	if err := New(newTestLogger()).Generate(astWithElements(text), output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	run := docxRunContaining(t, docxDocumentXML(t, output), "cod")

	if !strings.Contains(run, `w:ascii="Consolas"`) {
		t.Errorf("el estampado del token le pisó la fuente al código de adentro:\n%s", run)
	}
	if !strings.Contains(run, "<w:u ") {
		t.Errorf("el run perdió el subrayado del token:\n%s", run)
	}
}
