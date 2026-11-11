// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
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

// La etiqueta de una pestaña de code-group no es código: es un campo de una
// línea, como una leyenda. El DOCX la escribe con las {{variables}}
// sustituidas fuera de sus code spans, igual que el labelHTML de get_ast.
func TestDOCXCodeGroupLabelLikeLabelHTML(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	vars := map[string]interface{}{"company": "ACME"}
	labels := []string{"tab {{company}}", "otra `{{company}}`", "{{falta}}"}
	group := ast.NewCodeGroupElement(pos)
	for _, l := range labels {
		group.CodeBlocks = append(group.CodeBlocks, ast.CodeBlock{Language: "bash", Label: l, Content: "echo {{company}}"})
	}
	doc := newTestAST()
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Variables = vars
	doc.ContentBlocks[0].Elements = []ast.Element{group}
	documentXML := html.UnescapeString(docxZipPart(t, buildDOCXFile(t, doc), "word/document.xml"))

	oracle := ast.NewCodeGroupElement(pos)
	oracle.CodeBlocks = append(oracle.CodeBlocks, group.CodeBlocks...)
	astDoc := newTestAST()
	astDoc.ContentBlocks[0].Elements = []ast.Element{oracle}
	renderer.PopulateInlineHTMLWithOptions(astDoc, vars, renderer.InlineHTMLOptions{LiteralCode: true})
	for i, want := range []string{"tab ACME", "otra `{{company}}`", "{{falta}}"} {
		if got := html.UnescapeString(oracle.CodeBlocks[i].LabelHTML); got != want {
			t.Fatalf("labelHTML(%q) = %q, want %q", labels[i], got, want)
		}
		if !strings.Contains(documentXML, "▸ "+want+"<") {
			t.Errorf("label %q: the DOCX does not have %q", labels[i], "▸ "+want)
		}
	}
	if !strings.Contains(documentXML, "echo {{company}}") {
		t.Errorf("the code body must stay literal")
	}
}

// El marcador de una imagen que no se insertó lleva el alt con sus
// {{variables}} sustituidas fuera de sus code spans, como el alt de una imagen
// insertada y como el HTML (alt="Logo de ACME y `{{company}}`"). Antes decía
// "[Imagen no disponible: Logo de {{company}} y `{{company}}`]". Vale para la
// imagen de bloque, la inline de un párrafo y la de un encabezado, que pasan
// por el mismo marcador. Sin alt, el marcador muestra la fuente como se
// escribió.
func TestDOCXUnavailableImageAltSubstitutesVariables(t *testing.T) {
	src := "---\ntitle: D\nvariables:\n  company: ACME\n---\n\n# A\n\n" +
		"![Logo de {{company}} y `{{company}}`](falta.png)\n\n" +
		"En línea ![icono de {{company}}](falta2.png) fin.\n\n" +
		"## Sección ![marca de {{company}}](falta3.png)\n\n" +
		"![]({{company}}.png)\n"
	doc := parseDocLang(t, src)
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "d.docx")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: dir, SourceDir: dir}); err != nil {
		t.Fatal(err)
	}
	text := docxVisibleText(docxDocumentXML(t, out))
	for _, want := range []string{
		"[Imagen no disponible: Logo de ACME y `{{company}}`]",
		"[Imagen no disponible: icono de ACME]",
		"[Imagen no disponible: marca de ACME]",
		"[Imagen no disponible: {{company}}.png]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
}

// El marcador de una imagen que no se insertó lleva el mismo alt que el HTML,
// también con un code span en el alt: en una imagen de bloque, en una inline
// de párrafo y en una de encabezado. El alt del <img> de un encabezado ya
// perdió los backticks, y el marcador volvía a sustituir sobre él: con w:
// VALOR daba "hdr VALOR y VALOR" y el HTML "hdr VALOR y {{w}}". El de párrafo
// conservaba los backticks y los marcadores de formato que el HTML quita (en
// un párrafo, un ítem y una celda). Una variable vacía como único alt deja la
// fuente, como un alt vacío.
func TestDOCXUnavailableImageAltMatchesHTML(t *testing.T) {
	src := "---\ntitle: D\nvariables:\n  v: VALOR\n  w: VALOR\n  e: \"\"\n---\n\n# A\n\n" +
		"![blk {{v}} y `{{w}}`](falta1.png)\n\n" +
		"Texto ![par {{v}} y `{{w}}`](falta2.png) fin.\n\n" +
		"## Encabezado ![hdr {{v}} y `{{w}}`](falta3.png)\n\n" +
		"![{{e}}](falta4.png)\n\n" +
		"Con formato ![**neg** y *cur* & <x> \"q\"](falta5.png) fin.\n\n" +
		"- Ítem ![**neg** en ítem](falta6.png)\n\n" +
		"| Col |\n|---|\n| ![*cur* en celda & \"q\"](falta7.png) |\n"
	dir := t.TempDir()
	opts := GeneratorOptions{AssetRoot: dir, SourceDir: dir}

	htmlOut := filepath.Join(t.TempDir(), "d.html")
	opts.Format = "html"
	if err := New(newTestLogger()).Generate(parseDocLang(t, src), htmlOut, opts); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(htmlOut)
	if err != nil {
		t.Fatal(err)
	}
	htmlAlt := map[string]string{}
	for _, m := range regexp.MustCompile(`<img src="(falta\d\.png)" alt="([^"]*)"`).FindAllStringSubmatch(string(page), -1) {
		htmlAlt[m[1]] = html.UnescapeString(m[2])
	}

	docxOut := filepath.Join(t.TempDir(), "d.docx")
	opts.Format = "docx"
	if err := New(newTestLogger()).Generate(parseDocLang(t, src), docxOut, opts); err != nil {
		t.Fatal(err)
	}
	// docxVisibleText deja las entidades del XML (&amp;, &#34;).
	text := html.UnescapeString(docxVisibleText(docxDocumentXML(t, docxOut)))

	for _, c := range []struct{ src, wantHTML string }{
		{"falta1.png", "blk VALOR y `{{w}}`"},
		{"falta2.png", "par VALOR y {{w}}"},
		{"falta3.png", "hdr VALOR y {{w}}"},
		// El HTML escribe el alt de una imagen inline en texto plano, sin los
		// marcadores de formato; el marcador del DOCX también.
		{"falta5.png", `neg y cur & <x> "q"`},
		{"falta6.png", "neg en ítem"},
		{"falta7.png", `cur en celda & "q"`},
	} {
		alt, ok := htmlAlt[c.src]
		if !ok {
			t.Fatalf("no <img> for %s in the HTML", c.src)
		}
		// Fija lo que dice el HTML, para que la comparación no pase si los
		// dos cambian juntos.
		if alt != c.wantHTML {
			t.Errorf("HTML alt of %s = %q, want %q", c.src, alt, c.wantHTML)
		}
		if want := "[Imagen no disponible: " + alt + "]"; !strings.Contains(text, want) {
			t.Errorf("%s: missing %q (the HTML alt) in %q", c.src, want, text)
		}
	}
	if alt := htmlAlt["falta4.png"]; alt != "" {
		t.Errorf(`HTML alt of {{e}} with e: "" = %q, want empty`, alt)
	}
	if want := "[Imagen no disponible: falta4.png]"; !strings.Contains(text, want) {
		t.Errorf("missing %q in %q", want, text)
	}
}
