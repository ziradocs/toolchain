// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Issue #194 trajo encabezados de subsección al dialecto flex de SlideLang.
// Hasta el issue #259 el dialecto strict no podía escribirlos y el formatter
// re-emitía la línea Markdown dentro de un TEXT, que al reparsear se volvía
// prosa. Ahora sale como `SECTION "Texto"` con `level:`, y `id:` solo cuando
// el anchor no es el que el parser derivaría en esa posición.
func TestFormatStrict_SubsectionHeadingEmitsSection(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = append(block.Elements,
		ast.NewRawHTMLTextElement(pos, `<h3 id="resultados">Resultados</h3>`),
		ast.NewTextElement(pos, "Texto normal."),
		ast.NewRawHTMLTextElement(pos, `<h5 id="heading-detalle">Detalle</h5>`),
	)
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}

	for _, want := range []string{
		"  SECTION \"Resultados\"\n    level: 3\n    id: resultados\n",
		"Texto normal.",
		"  SECTION \"Detalle\"\n    level: 5\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("la salida no contiene %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "id: heading-detalle") {
		t.Errorf("emitió un id re-derivable:\n%s", out)
	}
	if strings.Contains(out, "<h3") || strings.Contains(out, "<h5") || strings.Contains(out, "###") {
		t.Errorf("se filtró HTML crudo o Markdown a la salida strict:\n%s", out)
	}
}

// El formatter es idempotente en bytes sobre su propia salida reparseada, y
// el reparseo conserva el encabezado (nivel, anchor y texto) en vez de
// convertirlo en prosa.
func TestFormatStrict_SubsectionHeadingIsByteIdempotent(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Slide"
	block.Elements = append(block.Elements,
		ast.NewRawHTMLTextElement(pos, `<h3 id="foo">Foo</h3>`),
		ast.NewRawHTMLTextElement(pos, `<h4 id="heading-bar">Bar</h4>`))
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	first, err := FormatStrict(doc)
	if err != nil {
		t.Fatalf("FormatStrict: %v", err)
	}
	reparsed, diags := parser.NewStrictParser(first, util.NewNoop()).Parse()
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("la salida no re-parsea: %v\n%s", d, first)
		}
	}
	got := reparsed.ContentBlocks[0].Elements
	if len(got) != 2 {
		t.Fatalf("elementos reparseados = %d, want 2:\n%s", len(got), first)
	}
	for i, want := range []string{`<h3 id="foo">Foo</h3>`, `<h4 id="heading-bar">Bar</h4>`} {
		te, ok := got[i].(*ast.TextElement)
		if !ok || !te.IsRawHTML || te.Content != want {
			t.Fatalf("elemento %d reparseado = %#v, want encabezado %s", i, got[i], want)
		}
	}
	second, err := FormatStrict(reparsed)
	if err != nil {
		t.Fatalf("FormatStrict (2a pasada): %v", err)
	}
	if first != second {
		t.Errorf("no idempotente:\n--- 1a ---\n%s\n--- 2a ---\n%s", first, second)
	}
}
