// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package docpreview

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
)

func render(t *testing.T, src string) string {
	t.Helper()
	doc, diags, err := Parse(util.NewNoop(), src)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d)
		}
	}
	return HTML(doc, renderer.DocumentHTMLOptions{EmbedAssets: true})
}

// El código es literal, como en doclang build (issue #487): una {{variable}}
// en un bloque de código o en un code span queda como se escribió, y la prosa
// la sustituye. El playground usaba NewDefaultRenderContext() y daba
// <code>ACME</code> y v := "ACME".
func TestHTMLCodeIsLiteral(t *testing.T) {
	html := render(t, "---\ntitle: D\nvariables:\n  company: ACME\n---\n\n# A\n\n"+
		"Prosa de {{company}} con `{{company}}`.\n\n"+
		"```go\nv := \"{{company}}\"\n```\n")
	for _, want := range []string{"Prosa de ACME", "<code>{{company}}</code>", "v := &quot;{{company}}&quot;"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(html, "<code>ACME</code>") || strings.Contains(html, "v := &quot;ACME&quot;") {
		t.Error("a {{variable}} inside code was substituted")
	}
}

// El dialecto sale del `mode:` del frontmatter, como en doclang build: un
// documento strict tiene su sección y su párrafo. Antes se leía siempre como
// flex y salía un solo párrafo "SECTION "Uno" TEXT Hola desde strict.".
func TestParseStrictDocument(t *testing.T) {
	html := render(t, "---\ntitle: S\nmode: strict\n---\n\nSECTION \"Uno\"\n  TEXT\n    Hola desde strict.\n")
	if !strings.Contains(html, `<h1 id="uno">Uno</h1>`) || !strings.Contains(html, "<p>Hola desde strict.</p>") {
		t.Errorf("strict document not read as strict:\n%s", html[strings.Index(html, "<body"):])
	}
	if strings.Contains(html, "SECTION") {
		t.Error("the SECTION keyword was rendered as prose")
	}
}

// El transform de xref corre, como en doclang build: \ref{id} es un vínculo
// numerado a la tabla. Antes quedaba "\ref{tbl-windows}" en el texto.
func TestParseResolvesReferences(t *testing.T) {
	html := render(t, "---\ntitle: R\nmode: strict\nlang: en\n---\n\nSECTION \"Uno\"\n  TEXT\n    See \\ref{tbl-w}.\n\n"+
		"  TABLE\n    headers: [\"A\"]\n    rows: [[\"1\"]]\n    caption: \"Windows\"\n    label: tbl-w\n")
	if strings.Contains(html, `\ref{`) {
		t.Error(`\ref{tbl-w} was not resolved`)
	}
	if !strings.Contains(html, `<a href="#tbl-w">Table 1</a>`) {
		t.Errorf("missing the numbered reference to the table:\n%s", html[strings.Index(html, "<body"):])
	}
}

func previewOptions(*ast.AST) renderer.DocumentHTMLOptions {
	return renderer.DocumentHTMLOptions{EmbedAssets: true, TOC: true, Numbering: true}
}

// transformError es el error de xref.Transform sobre una lectura limpia de
// src: Parse corre sus dos fases por separado y tiene que dar el mismo mensaje.
func transformError(t *testing.T, src string) string {
	t.Helper()
	doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "")
	_, err := xref.Transform(doc)
	if err == nil {
		t.Fatal("xref.Transform did not fail")
	}
	return err.Error()
}

// Un \ref a un label que todavía no existe no deja la vista previa en blanco:
// sale el HTML con el documento numerado, el \ref que sí resuelve como vínculo
// y el otro como texto, junto con el error de xref y valid=false, como
// doclang build, que con ese error no genera nada. Antes doclangRenderHTML
// devolvía el error sin HTML y el playground vaciaba la vista previa.
func TestRenderUnresolvedRefKeepsHTML(t *testing.T) {
	src := "---\ntitle: R\nmode: strict\nlang: en\n---\n\nSECTION \"Uno\"\n  TEXT\n    See \\ref{tbl-w} and \\ref{tbl-nope}.\n\n" +
		"  TABLE\n    headers: [\"A\"]\n    rows: [[\"1\"]]\n    caption: \"Windows\"\n    label: tbl-w\n"
	res := Render(util.NewNoop(), src, previewOptions)
	if res.Valid {
		t.Error("valid = true, want false: doclang build fails on an unresolved \\ref")
	}
	if res.Err == nil || res.Err.Error() != transformError(t, src) {
		t.Errorf("error = %v, want xref.Transform's %q", res.Err, transformError(t, src))
	}
	for _, want := range []string{`<a href="#tbl-w">Table 1</a> and \ref{tbl-nope}.`, "Table 1: Windows"} {
		if !strings.Contains(res.HTML, want) {
			t.Errorf("missing %q in the preview HTML (%d bytes)", want, len(res.HTML))
		}
	}
}

// Con un label duplicado, AssignNumbers corta a la mitad y deja numerado solo
// lo de antes. La vista previa se dibuja sin numerar y con cada \ref como
// texto, no con "Table 1" en la primera tabla y nada en las demás.
func TestRenderDuplicateLabelKeepsHTMLWithoutNumbering(t *testing.T) {
	src := "---\ntitle: D\nmode: strict\nlang: en\n---\n\nSECTION \"Uno\"\n  TEXT\n    See \\ref{tbl-b}.\n\n" +
		"  TABLE\n    headers: [\"A\"]\n    rows: [[\"1\"]]\n    caption: \"Primera\"\n    label: tbl-a\n\n" +
		"  TABLE\n    headers: [\"A\"]\n    rows: [[\"2\"]]\n    caption: \"Segunda\"\n    label: tbl-a\n\n" +
		"  TABLE\n    headers: [\"A\"]\n    rows: [[\"3\"]]\n    caption: \"Tercera\"\n    label: tbl-b\n"
	res := Render(util.NewNoop(), src, previewOptions)
	if res.Valid {
		t.Error("valid = true, want false: doclang build fails on a duplicate label")
	}
	if res.Err == nil || res.Err.Error() != transformError(t, src) {
		t.Errorf("error = %v, want xref.Transform's %q", res.Err, transformError(t, src))
	}
	for _, want := range []string{`See \ref{tbl-b}.`, `<p class="table-caption">Primera</p>`, `<p class="table-caption">Tercera</p>`} {
		if !strings.Contains(res.HTML, want) {
			t.Errorf("missing %q in the preview HTML (%d bytes)", want, len(res.HTML))
		}
	}
	if strings.Contains(res.HTML, "Table 1") {
		t.Error("the preview numbered the tables before the duplicate label")
	}
}

// Con errores de parseo no hay HTML, como antes.
func TestRenderParseErrorHasNoHTML(t *testing.T) {
	res := Render(util.NewNoop(), "---\ntitle: P\nast_capabilities: [no-such-capability]\n---\n\n# A\n\nTexto.\n", previewOptions)
	if res.Valid || res.HTML != "" {
		t.Errorf("valid = %v, %d bytes of HTML; want invalid with no HTML", res.Valid, len(res.HTML))
	}
	if !hasError(res.Diagnostics) {
		t.Error("expected a parse error diagnostic")
	}
}

// Un documento con typed-headings-v1 dibuja sus encabezados sin bajarlos
// antes: HTML no llama a LowerTypedHeadings (ver su comentario) porque
// GenerateDocumentHTML dibuja cada HeadingElement por su forma legada.
func TestHTMLRendersTypedHeadings(t *testing.T) {
	doc, diags, err := Parse(util.NewNoop(), "---\ntitle: T\nmode: flex\nast_capabilities: [typed-headings-v1]\n---\n\n# A\n\n## Sub uno\n\nTexto.\n")
	if err != nil || hasError(diags) {
		t.Fatalf("parse: %v %v", err, diags)
	}
	if !ast.UsesTypedHeadings(doc) {
		t.Fatal("the document has no typed headings")
	}
	if html := HTML(doc, renderer.DocumentHTMLOptions{EmbedAssets: true, TOC: true, Numbering: true}); !strings.Contains(html, `<h2 id="sub-uno">Sub uno</h2>`) {
		t.Errorf("missing the typed heading:\n%s", html[strings.Index(html, "<body"):])
	}
}

// Parse corre después de xref las mismas validaciones que
// transform.RunBuiltins en doclang build. Desde la fuente no sale un AST que
// pase el parser y falle aquí, así que la prueba las fuerza sobre el AST: dos
// nodos con el mismo nodeId, y un título de imagen (image-title-v1) que el
// contrato declarado del documento no incluye.
func TestValidateLikeBuiltins(t *testing.T) {
	doc, diags, err := Parse(util.NewNoop(), "---\ntitle: V\nmode: flex\n---\n\n# A\n\nUno.\n\nDos.\n")
	if err != nil || hasError(diags) {
		t.Fatalf("parse: %v %v", err, diags)
	}
	var texts []*ast.TextElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if te, ok := n.(*ast.TextElement); ok {
			texts = append(texts, te)
		}
		return nil
	})
	if len(texts) < 2 {
		t.Fatalf("want two paragraphs, got %d", len(texts))
	}
	texts[0].NodeID, texts[1].NodeID = "p-1", "p-1"
	if err := validateLikeBuiltins(doc); err == nil || !strings.Contains(err.Error(), `duplicate nodeId "p-1"`) {
		t.Errorf("duplicate nodeId: error = %v", err)
	}

	doc, diags, err = Parse(util.NewNoop(), "---\ntitle: V\nmode: flex\nast_capabilities: [list-start-v1]\n---\n\n# A\n\n3. uno\n4. dos\n\n![Logo](logo.png)\n")
	if err != nil || hasError(diags) {
		t.Fatalf("parse: %v %v", err, diags)
	}
	if !ast.DeclaresExtendedContract(doc) {
		t.Fatal("the document does not declare the extended contract")
	}
	_ = ast.Walk(doc, func(n ast.Node) error {
		if img, ok := n.(*ast.ImageElement); ok {
			img.Title = "Título"
		}
		return nil
	})
	want := ast.ValidateTableContract(doc)
	if want == nil {
		t.Fatal("ValidateTableContract accepted the mutated AST")
	}
	if err := validateLikeBuiltins(doc); err == nil || err.Error() != want.Error() {
		t.Errorf("table contract: error = %v, want %v", err, want)
	}
}
