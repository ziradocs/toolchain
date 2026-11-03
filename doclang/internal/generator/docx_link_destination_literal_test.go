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

// El destino y el título de una imagen o un enlace son literales, como en el
// HTML: un "*" ahí adentro no cierra un énfasis abierto antes. El walker del
// DOCX se queda con el match que empieza primero, así que en
// `*x ![a](https://e.com/a*b*.png) y*` ganaba la cursiva `*x ![a](https://e.com/a*`
// y la imagen salía como su Markdown partido; en `**t [x](https://e.com/u**v)`
// la negrita se comía el enlace.
func TestDOCXGenerator_EmphasisDoesNotCloseInsideDestination(t *testing.T) {
	text := ast.NewTextElement(diagnostics.NewPosition(1, 1),
		"Hola *x ![alt](https://e.com/a*b*.png \"un *gran* gato\") y* fin, **t [enl](https://e.com/u**v) y [otro](https://e.com/a_b_ \"t _y_\") _z_.")
	output := filepath.Join(t.TempDir(), "dest.docx")
	if err := New(newTestLogger()).Generate(astWithElements(text), output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	xml := docxDocumentXML(t, output)

	for _, literal := range []string{"![alt]", "](https://", "png", "u**v", "[enl]"} {
		if strings.Contains(xml, escapeForDocxText(literal)) {
			t.Errorf("%q salió literal al .docx", literal)
		}
	}
	// La imagen remota sale como su alt, dentro de la cursiva.
	if alt := docxRunContaining(t, xml, "alt"); !strings.Contains(alt, "<w:i") {
		t.Errorf("el alt de la imagen no quedó dentro de la cursiva:\n%s", alt)
	}
	// La negrita no abre: "**t " queda literal y el enlace es un enlace.
	if !strings.Contains(xml, escapeForDocxText("**t ")) {
		t.Error(`"**t " debía quedar literal: su cierre está dentro del destino`)
	}
	if enl := docxRunContaining(t, xml, "enl"); strings.Contains(enl, "<w:b") || !strings.Contains(enl, "<w:u ") {
		t.Errorf("el enlace salió en negrita o sin subrayado:\n%s", enl)
	}
	// "_" no es énfasis en el DOCX.
	if !strings.Contains(xml, escapeForDocxText("_z_")) {
		t.Error(`"_z_" debía quedar literal`)
	}
}

// Un code span se resuelve antes que los enlaces, como en el HTML: el
// "[0](" de adentro no abre un enlace que se trague la cursiva de después.
func TestDOCXGenerator_CodeSpanDoesNotOpenLink(t *testing.T) {
	text := ast.NewTextElement(diagnostics.NewPosition(1, 1), "L4 `f[0](` y *importante* texto) fin")
	output := filepath.Join(t.TempDir(), "code.docx")
	if err := New(newTestLogger()).Generate(astWithElements(text), output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	xml := docxDocumentXML(t, output)
	if run := docxRunContaining(t, xml, "importante"); !strings.Contains(run, "<w:i") {
		t.Errorf("perdió la cursiva:\n%s", run)
	}
}
