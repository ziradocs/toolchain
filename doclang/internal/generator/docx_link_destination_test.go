// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

var docxTextPattern = regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)

// docxVisibleText concatena el texto de todos los <w:t> del documento, que es
// lo que ve quien abre el .docx.
func docxVisibleText(xml string) string {
	var b strings.Builder
	for _, m := range docxTextPattern.FindAllStringSubmatch(xml, -1) {
		b.WriteString(m[1])
	}
	return b.String()
}

func generateDocxText(t *testing.T, content string) string {
	t.Helper()
	text := ast.NewTextElement(diagnostics.NewPosition(1, 1), content)
	output := filepath.Join(t.TempDir(), "link-destination.docx")
	if err := New(newTestLogger()).Generate(astWithElements(text), output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	return docxDocumentXML(t, output)
}

// El pattern de links de DOCX tenía su propia copia del regex,
// `\[([^\]]+)\]\(([^)]+)\)`, que cortaba el destino en el primer ")". El
// HTML ya lee el destino hasta el ")" que balancea los paréntesis (como
// CommonMark), así que el mismo documento salía distinto: en el .docx quedaba
// un ")" suelto después de la etiqueta. Ahora DOCX usa
// renderer.FindInlineLinkIndex y los dos reconocen los mismos links.
func TestDOCXGenerator_LinkDestinationWithBalancedParentheses(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		visible string
		link    string
	}{
		{"wiki con paréntesis", "Ver [Wiki](https://es.wikipedia.org/wiki/Foo_(bar)) fin.", "Ver Wiki fin.", "Wiki"},
		{"javascript con paréntesis", "Ver [Elegir](javascript:alert(1)) fin.", "Ver Elegir fin.", "Elegir"},
		{"paréntesis anidados", "Ver [a](https://x.com/(a(b))) fin.", "Ver a fin.", "a"},
		{"texto después del cierre se conserva", "Ver [a](https://x.com/f(1)) y más)", "Ver a y más)", "a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xml := generateDocxText(t, tc.input)
			if got := docxVisibleText(xml); !strings.Contains(got, tc.visible) {
				t.Errorf("texto visible = %q, quería que contuviera %q", got, tc.visible)
			}
			if run := docxRunContaining(t, xml, tc.link); !strings.Contains(run, "<w:u ") {
				t.Errorf("el run del link %q no lleva el subrayado de link:\n%s", tc.link, run)
			}
		})
	}
}

// Un destino sin ")" que lo cierre no es un link y queda literal, como en
// HTML (antes el regex lo tomaba como link con el destino "b (c"). Eso no
// impide reconocer un link bien formado más adelante en la misma línea.
func TestDOCXGenerator_UnbalancedLinkDestinationStaysLiteral(t *testing.T) {
	xml := generateDocxText(t, "Uno [a](b (c) y [d](https://e.com) fin.")
	if got := docxVisibleText(xml); !strings.Contains(got, "Uno [a](b (c) y d fin.") {
		t.Errorf("texto visible = %q", got)
	}
	if run := docxRunContaining(t, xml, "d"); !strings.Contains(run, "<w:u ") {
		t.Errorf("el link que sigue al desbalanceado no se reconoció:\n%s", run)
	}
}

// Dentro de negrita el link pasa por la recursión de énfasis
// (docxEmphasisInnerPatterns), que usa el mismo pattern: también ahí el
// destino con paréntesis sale completo y el run compone negrita y subrayado.
func TestDOCXGenerator_LinkWithParenthesesInsideEmphasis(t *testing.T) {
	xml := generateDocxText(t, "Uno **[x](https://x.com/f(1))** fin.")
	if got := docxVisibleText(xml); !strings.Contains(got, "Uno x fin.") {
		t.Errorf("texto visible = %q", got)
	}
	run := docxRunContaining(t, xml, "x")
	if !strings.Contains(run, "<w:b") {
		t.Errorf("el run del link perdió la negrita de afuera:\n%s", run)
	}
	if !strings.Contains(run, "<w:u ") {
		t.Errorf("el run del link perdió el subrayado:\n%s", run)
	}
}
