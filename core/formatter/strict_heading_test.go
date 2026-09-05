// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Issue #194 trajo encabezados de subsección al dialecto flex de SlideLang.
// El dialecto strict no tiene forma de escribirlos (issue #259), así que el
// formatter re-emite la línea Markdown dentro de un TEXT — que es
// exactamente lo que emitía antes de #194, cuando esa línea todavía era un
// TextElement común.
func TestFormatStrict_SubsectionHeadingRoundTripsAsMarkdown(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = append(block.Elements,
		ast.NewRawHTMLTextElement(pos, `<h3 id="resultados">Resultados</h3>`),
		ast.NewTextElement(pos, "Texto normal."),
		ast.NewRawHTMLTextElement(pos, `<h5 id="detalle">Detalle</h5>`),
	)
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}

	for _, want := range []string{"### Resultados", "##### Detalle", "Texto normal."} {
		if !strings.Contains(out, want) {
			t.Errorf("la salida no contiene %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "<h3") || strings.Contains(out, "<h5") {
		t.Errorf("se filtró HTML crudo a la salida strict:\n%s", out)
	}
}

// El formatter es idempotente en bytes: formatear dos veces da lo mismo.
// Con el heading eso se cumple porque la segunda pasada ya no ve un
// RawHTMLTextElement sino el texto "### Resultados" — y formatStrictText lo
// emite igual.
func TestFormatStrict_SubsectionHeadingIsByteIdempotent(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = append(block.Elements,
		ast.NewRawHTMLTextElement(pos, `<h3 id="foo">Foo</h3>`))
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	first, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}

	// Segunda pasada sobre el AST equivalente que produciría re-parsear
	// esa salida: el heading ya es texto plano.
	block2 := ast.NewContentBlock(pos, "content")
	block2.Title = "Slide"
	block2.Elements = append(block2.Elements, ast.NewTextElement(pos, "### Foo"))
	second, err := FormatStrict(&ast.AST{ContentBlocks: []ast.ContentBlock{*block2}})
	if err != nil {
		t.Fatalf("FormatStrict (2a pasada): %v", err)
	}

	if first != second {
		t.Errorf("no idempotente:\n--- 1a ---\n%s\n--- 2a ---\n%s", first, second)
	}
}
