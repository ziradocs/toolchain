// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

type headingRecord struct {
	Level        int
	Text, Anchor string
	NodeID       string
}

func typedHeadings(t *testing.T, doc *ast.AST) []headingRecord {
	t.Helper()
	var out []headingRecord
	_ = ast.Walk(doc, func(n ast.Node) error {
		if h, ok := n.(*ast.HeadingElement); ok {
			out = append(out, headingRecord{h.Level, h.Text, h.Anchor, h.NodeID})
		}
		return nil
	})
	return out
}

func mustParse(t *testing.T, src string, document bool) *ast.AST {
	t.Helper()
	p := parser.New(util.NewNoop())
	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if document {
		doc, diags = p.ParseDocument(src, "")
	} else {
		doc, diags = p.Parse(src, "")
	}
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v\n%s", d, src)
		}
	}
	return doc
}

const typedFlexDeck = `---
mode: flex
ast_capabilities: [typed-headings-v1]
---
# Deck

## Slide

<!-- node-id: ResultsA -->
### Results **now** and ` + "`code`" + `

Body.

#### Detail [texto]{lang=es}
`

// El texto autoral viaja exacto (énfasis, código inline, marca de idioma):
// flex → strict → parse devuelve los mismos encabezados tipados, con su
// nodeId, y el formatter es idempotente sobre su propia salida.
func TestTypedHeadingsStrictRoundTripIsLossless(t *testing.T) {
	doc := mustParse(t, typedFlexDeck, false)
	want := typedHeadings(t, doc)
	if len(want) != 2 || want[0].Text != "Results **now** and `code`" || want[0].NodeID != "ResultsA" {
		t.Fatalf("unexpected parsed headings: %+v", want)
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"ast_capabilities:", ast.TypedHeadingsCapability, "SECTION \"Results **now** and `code`\"", "<!-- node-id: ResultsA -->"} {
		if !strings.Contains(out, s) {
			t.Fatalf("output lacks %q:\n%s", s, out)
		}
	}
	reparsed := mustParse(t, out, false)
	if got := typedHeadings(t, reparsed); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip changed headings\n got: %+v\nwant: %+v\n%s", got, want, out)
	}
	again, err := FormatStrict(reparsed)
	if err != nil {
		t.Fatal(err)
	}
	if again != out {
		t.Fatalf("not idempotent:\n--- 1 ---\n%s\n--- 2 ---\n%s", out, again)
	}
}

// Reordenar dos encabezados de igual texto deja sus anchors fuera del orden
// de derivación; el formatter declara `id:` donde hace falta y el reparseo
// conserva anchor y nodeId de cada uno.
func TestTypedHeadingsReorderKeepsAnchorsAndIDs(t *testing.T) {
	src := "---\nmode: strict\nast_capabilities: [typed-headings-v1]\n---\nSLIDE content\n  title: \"S\"\n  <!-- node-id: First -->\n  SECTION \"Same\"\n    level: 3\n  <!-- node-id: Second -->\n  SECTION \"Same\"\n    level: 4\n"
	doc := mustParse(t, src, false)
	els := doc.ContentBlocks[0].Elements
	els[0], els[1] = els[1], els[0]
	want := typedHeadings(t, doc)
	if want[0].Anchor != "heading-same-2" || want[1].Anchor != "heading-same" {
		t.Fatalf("fixture anchors = %+v", want)
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "id: heading-same-2") {
		t.Fatalf("expected an explicit id for the moved heading:\n%s", out)
	}
	if got := typedHeadings(t, mustParse(t, out, false)); !reflect.DeepEqual(got, want) {
		t.Fatalf("reorder lost identity\n got: %+v\nwant: %+v\n%s", got, want, out)
	}
}

// Sin opt-in el encabezado legado ya no se pierde (issue #259): vuelve como
// encabezado RawHTML con el mismo nivel y anchor, y el documento no gana una
// declaración de capability.
func TestLegacyHeadingsStrictRoundTrip(t *testing.T) {
	src := "---\nmode: flex\n---\n# Deck\n\n## Slide\n\n### Quarterly Revenue\n\nBody.\n\n### Quarterly Revenue\n"
	doc := mustParse(t, src, false)
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "ast_capabilities") || strings.Contains(out, "###") {
		t.Fatalf("legacy output changed contract or kept Markdown:\n%s", out)
	}
	legacy := func(d *ast.AST) []string {
		var out []string
		_ = ast.Walk(d, func(n ast.Node) error {
			if te, ok := n.(*ast.TextElement); ok && te.IsRawHTML {
				out = append(out, te.Content)
			}
			return nil
		})
		return out
	}
	want := legacy(doc)
	got := legacy(mustParse(t, out, false))
	if !reflect.DeepEqual(got, want) || len(want) != 2 || want[1] != `<h3 id="heading-quarterly-revenue-2">Quarterly Revenue</h3>` {
		t.Fatalf("legacy headings changed\n got: %q\nwant: %q\n%s", got, want, out)
	}
}

func TestTypedHeadingsDocLangRoundTrips(t *testing.T) {
	src := "---\nmode: strict\nast_capabilities: [typed-headings-v1]\n---\nSECTION \"Doc\"\n\nSECTION \"Scope *now*\"\n  level: 2\n  TEXT\n    Text.\n\nSECTION \"Other\"\n  level: 3\n  id: custom-anchor\n"
	doc := mustParse(t, src, true)
	want := typedHeadings(t, doc)
	strictOut, err := FormatDocumentStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got := typedHeadings(t, mustParse(t, strictOut, true)); !reflect.DeepEqual(got, want) {
		t.Fatalf("strict round trip\n got: %+v\nwant: %+v\n%s", got, want, strictOut)
	}
	// El flex de DocLang no puede declarar un anchor: se rechaza en vez de
	// cambiar el destino de una referencia.
	if _, err := FormatDocument(doc); err == nil || !strings.Contains(err.Error(), "custom-anchor") {
		t.Fatalf("flex accepted a non-derivable anchor: %v", err)
	}
	flexSrc := "---\nast_capabilities: [typed-headings-v1]\n---\n# Doc\n\n## Scope *now*\n\nText.\n"
	flexDoc := mustParse(t, flexSrc, true)
	flexOut, err := FormatDocument(flexDoc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(flexOut, "## Scope *now*") {
		t.Fatalf("flex lost authored emphasis:\n%s", flexOut)
	}
	if got, want := typedHeadings(t, mustParse(t, flexOut, true)), typedHeadings(t, flexDoc); !reflect.DeepEqual(got, want) {
		t.Fatalf("flex round trip\n got: %+v\nwant: %+v", got, want)
	}
}

func TestTypedHeadingFormatterRejectsUnrepresentableHeadings(t *testing.T) {
	doc := mustParse(t, typedFlexDeck, false)
	var h *ast.HeadingElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if e, ok := n.(*ast.HeadingElement); ok && h == nil {
			h = e
		}
		return nil
	})
	h.Level = 2
	if _, err := FormatStrict(doc); err == nil || !strings.Contains(err.Error(), "level 2") {
		t.Fatalf("level 2 inside a slide accepted: %v", err)
	}
	h.Level = 3
	h.Text = "a\nb"
	if _, err := FormatStrict(doc); err == nil {
		t.Fatal("multiline heading accepted")
	}
}

// Un encabezado tipado anidado en un bloque especial no tiene sintaxis strict
// (el cuerpo del bloque se re-emite crudo y strict no reconoce `###` ahí): los
// formatters strict lo rechazan en vez de convertirlo en prosa, y el flex de
// DocLang, que sí lo reparsea como encabezado, lo conserva.
func TestNestedTypedHeadingsFailClosedInStrict(t *testing.T) {
	slides := mustParse(t, "---\nmode: flex\nast_capabilities: [typed-headings-v1]\n---\n# Deck\n\n## Slide\n\n::: note\n### Inside\nBody.\n:::\n", false)
	if len(typedHeadings(t, slides)) != 1 {
		t.Fatal("fixture has no nested typed heading")
	}
	if _, err := FormatStrict(slides); err == nil || !strings.Contains(err.Error(), ":::note block") {
		t.Fatalf("slide strict formatter degraded a nested heading: %v", err)
	}
	doc := mustParse(t, "---\nast_capabilities: [typed-headings-v1]\n---\n# Doc\n\n::: note\n### Inside\nBody.\n:::\n", true)
	want := typedHeadings(t, doc)
	if len(want) != 1 {
		t.Fatalf("doc fixture headings = %+v", want)
	}
	if _, err := FormatDocumentStrict(doc); err == nil || !strings.Contains(err.Error(), ":::note block") {
		t.Fatalf("document strict formatter degraded a nested heading: %v", err)
	}
	flexOut, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("document flex formatter: %v", err)
	}
	if got := typedHeadings(t, mustParse(t, flexOut, true)); !reflect.DeepEqual(got, want) {
		t.Fatalf("flex lost the nested heading\n got: %+v\nwant: %+v\n%s", got, want, flexOut)
	}
}

// Una columna de grid con elementos tipados ya se rechaza en strict; el
// encabezado tipado no abre una excepción.
func TestTypedHeadingInGridColumnFailsClosed(t *testing.T) {
	doc := mustParse(t, typedFlexDeck, false)
	pos := doc.ContentBlocks[0].Position
	grid := ast.NewGridElement(pos)
	col := ast.NewColumnElement(pos, "")
	col.Elements = []ast.Element{ast.NewHeadingElement(pos, 3, "Col", "heading-col")}
	grid.Columns = []ast.ColumnElement{*col}
	last := len(doc.ContentBlocks) - 1
	doc.ContentBlocks[last].Elements = append(doc.ContentBlocks[last].Elements, grid)
	if _, err := FormatStrict(doc); err == nil {
		t.Fatal("heading inside a grid column was formatted")
	}
}
