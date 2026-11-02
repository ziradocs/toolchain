// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// astSingleSection es la forma que deja un .md con "# Proyecto" y todo lo
// demás debajo (o, con el cambio de T1, un .md sin front matter cuyo título
// se sintetiza del primer H1): un solo ContentBlock cuyo Title coincide con
// FrontMatter.Title.
func astSingleSection(fmTitle, sectionTitle string) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	front := ast.NewFrontMatterNode(pos)
	front.Title = fmTitle
	front.Author = "Ana"
	doc.FrontMatter = front

	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = sectionTitle
	block.Elements = append(block.Elements,
		ast.NewTextElement(diagnostics.NewPosition(3, 1), "Cuerpo del documento"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

func buildDOCXXML(t *testing.T, doc *ast.AST, opts GeneratorOptions) string {
	t.Helper()
	opts.Format = "docx"
	output := filepath.Join(t.TempDir(), "out.docx")
	if err := New(newTestLogger()).Generate(doc, output, opts); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	return docxDocumentXML(t, output)
}

// TestDOCXGenerator_TitleMatchingFirstHeading_RendersOnce: el título del
// front matter y el primer H1 iguales salen una sola vez, como Heading1, y
// arriba del autor (en el lugar del título).
func TestDOCXGenerator_TitleMatchingFirstHeading_RendersOnce(t *testing.T) {
	xml := buildDOCXXML(t, astSingleSection("Mi  Proyecto", "mi proyecto"), GeneratorOptions{TOC: true})

	if n := strings.Count(strings.ToLower(xml), "mi proyecto"); n != 2 {
		// Una vez como Heading1 y otra en el TOC estático.
		t.Fatalf("want the title twice (heading + static TOC entry), got %d:\n%s", n, xml)
	}
	if n := strings.Count(xml, `<w:pStyle w:val="Heading1">`); n != 1 {
		t.Errorf("want exactly one Heading1, got %d", n)
	}
	heading := strings.Index(xml, `<w:pStyle w:val="Heading1">`)
	author := strings.Index(xml, "Por: Ana")
	toc := strings.Index(xml, "TOC ")
	if heading < 0 || author < 0 || toc < 0 || !(heading < author && author < toc) {
		t.Errorf("want Heading1 < author < TOC, got %d, %d, %d", heading, author, toc)
	}
}

// TestDOCXGenerator_TitleMatchingFirstHeading_KeepsNumbering: con
// --numbering el encabezado conserva el número que le da HTML.
func TestDOCXGenerator_TitleMatchingFirstHeading_KeepsNumbering(t *testing.T) {
	xml := buildDOCXXML(t, astSingleSection("Proyecto", "Proyecto"), GeneratorOptions{Numbering: true})
	if !strings.Contains(xml, "<w:t>1. Proyecto</w:t>") {
		t.Errorf("numbered heading missing:\n%s", xml)
	}
	if strings.Count(xml, "Proyecto") != 1 {
		t.Errorf("title rendered more than once:\n%s", xml)
	}
}

// TestDOCXGenerator_TitleMatchingLaterHeading_OmitsCenteredTitle: si hay
// contenido antes del encabezado, éste se queda en su lugar y solo se omite
// el título centrado.
func TestDOCXGenerator_TitleMatchingLaterHeading_OmitsCenteredTitle(t *testing.T) {
	doc := astSingleSection("Proyecto", "Proyecto")
	pre := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	pre.Elements = append(pre.Elements, ast.NewTextElement(diagnostics.NewPosition(2, 1), "Preámbulo"))
	doc.ContentBlocks = append([]ast.ContentBlock{*pre}, doc.ContentBlocks...)

	xml := buildDOCXXML(t, doc, GeneratorOptions{})
	if n := strings.Count(xml, "<w:t>Proyecto</w:t>"); n != 1 {
		t.Fatalf("want the title once, got %d:\n%s", n, xml)
	}
	if strings.Index(xml, "Preámbulo") > strings.Index(xml, "<w:t>Proyecto</w:t>") {
		t.Error("heading moved above the content that precedes it")
	}
}

// TestDOCXGenerator_DifferentTitle_KeepsBoth: títulos distintos no se tocan.
func TestDOCXGenerator_DifferentTitle_KeepsBoth(t *testing.T) {
	xml := buildDOCXXML(t, astSingleSection("Informe anual", "Introducción"), GeneratorOptions{})
	if !strings.Contains(xml, "<w:t>Informe anual</w:t>") || !strings.Contains(xml, "<w:t>Introducción</w:t>") {
		t.Errorf("different title and heading must both render:\n%s", xml)
	}
}

// TestDOCXGenerator_ThemeOption_OverridesFrontMatter: --theme (que el CLI
// resuelve en opts.Theme) gana sobre el theme del front matter, igual que
// en HTML y PDF. academic usa Georgia; professional, Segoe UI.
func TestDOCXGenerator_ThemeOption_OverridesFrontMatter(t *testing.T) {
	doc := newTestAST()
	doc.FrontMatter.Theme = "professional"

	xml := buildDOCXXML(t, doc, GeneratorOptions{Theme: "academic"})
	if !strings.Contains(xml, "Georgia") || strings.Contains(xml, "Segoe UI") {
		t.Errorf("opts.Theme=academic not applied over front matter professional")
	}

	xml = buildDOCXXML(t, doc, GeneratorOptions{})
	if !strings.Contains(xml, "Segoe UI") {
		t.Errorf("front matter theme not used when opts.Theme is empty")
	}
}

func TestResolveDOCXThemeName(t *testing.T) {
	doc := newTestAST()
	if got := resolveDOCXThemeName(doc, GeneratorOptions{}); got != "professional" {
		t.Errorf("default = %q, want professional", got)
	}
	doc.FrontMatter.Theme = "technical"
	if got := resolveDOCXThemeName(doc, GeneratorOptions{}); got != "technical" {
		t.Errorf("front matter = %q, want technical", got)
	}
	if got := resolveDOCXThemeName(doc, GeneratorOptions{Theme: "page-view"}); got != "page-view" {
		t.Errorf("opts.Theme = %q, want page-view", got)
	}
}

// TestDOCXGenerator_NoFrontMatter_LeadingSectionAndTitleOnce cubre la forma
// que deja el parser flex de la rama de Markdown sin front matter (T1):
// FrontMatter nil, una sección inicial sin encabezado con el texto previo al
// primer `#`, y luego el bloque del H1. El texto inicial sale primero y sin
// encabezado, el H1 sale una sola vez, y las propiedades del .docx toman
// ese H1 como título.
func TestDOCXGenerator_NoFrontMatter_LeadingSectionAndTitleOnce(t *testing.T) {
	doc := astSingleSection("", "Proyecto")
	doc.FrontMatter = nil
	lead := ast.NewContentBlock(diagnostics.NewPosition(1, 1), "content")
	lead.Elements = append(lead.Elements, ast.NewTextElement(diagnostics.NewPosition(1, 1), "Texto antes del título"))
	doc.ContentBlocks = append([]ast.ContentBlock{*lead}, doc.ContentBlocks...)

	output := filepath.Join(t.TempDir(), "out.docx")
	if err := New(newTestLogger()).Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	xml := docxDocumentXML(t, output)
	if n := strings.Count(xml, "<w:t>Proyecto</w:t>"); n != 1 {
		t.Fatalf("want the H1 once, got %d:\n%s", n, xml)
	}
	if n := strings.Count(xml, `<w:pStyle w:val="Heading1">`); n != 1 {
		t.Errorf("leading section must not get a heading; Heading1 count = %d", n)
	}
	if strings.Index(xml, "Texto antes del título") > strings.Index(xml, "<w:t>Proyecto</w:t>") {
		t.Error("leading content must come before the H1")
	}
	if got := docxCoreTitle(t, output); got != "Proyecto" {
		t.Errorf("docProps title = %q, want Proyecto", got)
	}
}

func docxCoreTitle(t *testing.T, path string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	for _, f := range r.File {
		if f.Name != "docProps/core.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rc.Close() }()
		data, _ := io.ReadAll(rc)
		m := regexp.MustCompile(`<dc:title>([^<]*)</dc:title>`).FindStringSubmatch(string(data))
		if m == nil {
			return ""
		}
		return m[1]
	}
	return ""
}
