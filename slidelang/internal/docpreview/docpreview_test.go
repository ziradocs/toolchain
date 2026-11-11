// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package docpreview

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
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
