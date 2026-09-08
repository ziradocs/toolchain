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

	// Una clase que DOCX no sabe representar —`danger` no tiene color por run
	// en docxgo v2.12.0— conserva el texto y el formato interno, y no imprime
	// la sintaxis. Perder el color es aceptable; mostrar el token no.
	danger := docxRunContaining(t, xml, "combine")
	if !strings.Contains(danger, "<w:b") {
		t.Errorf("el run de una clase sin representación perdió la negrita de adentro:\n%s", danger)
	}
}

// El estilo del token se estampa DESPUÉS del estilo del pattern interno y solo
// agrega: si pusiera fuente base, le pisaría la Consolas al patrón de code.
// Este test fija esa dirección con el caso donde chocan —un `code` adentro de
// un token que no toca la fuente.
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
