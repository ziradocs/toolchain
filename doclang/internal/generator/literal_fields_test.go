// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

// literalFieldCases son campos de una línea con {{variables}} dentro y fuera
// de code spans, con los emparejamientos que el regex viejo del DOCX hacía
// mal (backtick escapado, span de dos backticks, backtick sin cierre).
var literalFieldCases = []struct{ in, want string }{
	{"Ventas de {{company}}", "Ventas de ACME"},
	{"Ventas de {{company}} en `{{company}}`", "Ventas de ACME en `{{company}}`"},
	{"``a ` {{company}}`` y {{company}}", "``a ` {{company}}`` y ACME"},
	// El backtick escapado no abre: el span es "` y `" y las dos se sustituyen.
	{"\\`{{company}}` y `{{company}}`", "\\`ACME` y `ACME`"},
	{"\\`x\\` `{{company}}`", "\\`x\\` `{{company}}`"},
	{"`sin cierre {{company}}", "`sin cierre ACME"},
	{"`{{falta}}` y {{falta}}", "`{{falta}}` y {{falta}}"},
	{"sin variables `x`", "sin variables `x`"},
}

func TestFieldVariables(t *testing.T) {
	vars := map[string]interface{}{"company": "ACME"}
	for _, c := range literalFieldCases {
		if got := fieldVariables(c.in, vars); got != c.want {
			t.Errorf("fieldVariables(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Oráculo: el DOCX sustituye un campo igual que el HTML de core con código
// literal (el CaptionHTML de una imagen, que core escapa sin Markdown).
func TestFieldVariablesMatchesCoreHTML(t *testing.T) {
	vars := map[string]interface{}{"company": "ACME"}
	for _, c := range literalFieldCases {
		pos := diagnostics.NewPosition(1, 1)
		img := ast.NewImageElement(pos, "x.png", "x")
		img.Caption = c.in
		doc := newTestAST()
		doc.ContentBlocks[0].Elements = []ast.Element{img}
		renderer.PopulateInlineHTMLWithOptions(doc, vars, renderer.InlineHTMLOptions{LiteralCode: true})
		if core, ours := html.UnescapeString(img.CaptionHTML), fieldVariables(c.in, vars); core != ours {
			t.Errorf("%q: core HTML %q, DOCX %q", c.in, core, ours)
		}
	}
}

// Issue #487 en el DOCX: lo que va entre backticks en una leyenda es código y
// queda literal; fuera de los backticks las {{variables}} se sustituyen, como
// en el HTML.
func TestDOCXCaptionsKeepCodeSpansLiteral(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Headers = []string{"A"}
	table.Rows = [][]string{{"1"}}
	table.Caption = "Ventas de {{company}} con `{{company}}`"
	img := ast.NewImageElement(pos, "missing.png", "logo")
	img.Caption = "Logo de {{company}} y `{{company}}`"
	doc := newTestAST()
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Variables = map[string]interface{}{"company": "ACME"}
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, table, img)
	documentXML := html.UnescapeString(docxZipPart(t, buildDOCXFile(t, doc), "word/document.xml"))
	for _, want := range []string{"Ventas de ACME con `{{company}}`", "Logo de ACME y `{{company}}`"} {
		if !strings.Contains(documentXML, want) {
			t.Errorf("caption %q missing from the DOCX", want)
		}
	}
}

// issue487Source es el caso del issue #487: {{variables}} escritas en un
// bloque de código, en un code span y en una cerca dentro de una cita.
const issue487Source = "---\ntitle: D\nvariables:\n  company: ACME\n---\n\n# A\n\n" +
	"Prosa de {{company}} con `{{company}}`.\n\n" +
	"```go\nv := \"{{company}}\"\n```\n\n" +
	"> ```\n> cita {{company}}\n> ```\n"

// Issue #487: el código es literal en el HTML, la vista previa, el DOCX y el
// Markdown; la prosa fuera del código sigue sustituyendo en el HTML.
func TestIssue487CodeIsLiteralInEveryFormat(t *testing.T) {
	doc := parseDocLang(t, issue487Source)
	dir := t.TempDir()
	read := func(format, name string) string {
		t.Helper()
		out := filepath.Join(dir, name)
		if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: format}); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if format == "docx" {
			return html.UnescapeString(docxZipPart(t, out, "word/document.xml"))
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return html.UnescapeString(string(data))
	}
	preview := html.UnescapeString(New(newTestLogger()).RenderHTMLPreview(doc, ""))
	for _, w := range []string{"Prosa de ACME", "<code>{{company}}</code>", `v := "{{company}}"`, "cita {{company}}"} {
		if !strings.Contains(preview, w) {
			t.Errorf("preview: missing %q", w)
		}
	}
	for _, c := range []struct {
		format, name string
		want         []string
	}{
		{"html", "d.html", []string{"Prosa de ACME", "<code>{{company}}</code>", `v := "{{company}}"`, "cita {{company}}"}},
		{"docx", "d.docx", []string{`v := "{{company}}"`, ">{{company}}<", "cita {{company}}"}},
		{"markdown", "d.md", []string{"`{{company}}`", "v := \"{{company}}\"", "cita {{company}}"}},
	} {
		got := read(c.format, c.name)
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: missing %q", c.format, w)
			}
		}
		if strings.Contains(got, `v := "ACME"`) || strings.Contains(got, "cita ACME") || strings.Contains(got, "<code>ACME</code>") {
			t.Errorf("%s: a {{variable}} inside code was substituted", c.format)
		}
	}
}
