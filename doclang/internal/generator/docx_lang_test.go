// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// docxPartXML unzips a .docx and returns the given part (e.g.
// "word/styles.xml") as a string.
func docxPartXML(t *testing.T, docxPath, part string) string {
	t.Helper()
	r, err := zip.OpenReader(docxPath)
	if err != nil {
		t.Fatalf("failed to open generated docx: %v", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.Name != part {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open %s in docx: %v", part, err)
		}
		defer func() { _ = rc.Close() }()
		buf, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("failed to read %s: %v", part, err)
		}
		return string(buf)
	}
	t.Fatalf("%s not found in generated docx", part)
	return ""
}

// TestDOCXGenerator_SetsDocumentLanguage cubre el prerrequisito de los
// issues #62/#63: un .doclang con `lang: fr` en el frontmatter debe producir
// un .docx con ese idioma declarado (docxgo.SetLanguage, disponible desde
// v2.6.0), no un documento sin idioma de revisión — antes de este cambio,
// FrontMatter.Lang ni siquiera existía, así que este dato se perdía por
// completo camino a Word.
func TestDOCXGenerator_SetsDocumentLanguage(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	front := ast.NewFrontMatterNode(pos)
	front.Title = "Fixture"
	front.Lang = "fr"
	doc.FrontMatter = front
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Elements = append(block.Elements, ast.NewTextElement(pos, "Contenu."))
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	logger := newTestLogger()
	gen := New(logger)
	output := filepath.Join(t.TempDir(), "lang.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	styles := docxPartXML(t, output, "word/styles.xml")
	// Val/EastAsia/Bidi los tres al mismo tag (code review): dejar
	// EastAsia/Bidi sin poner significaba que un documento en un script
	// CJK o RTL con `lang:` declarado seguía usando el idioma del sistema
	// para esos runs — Word solo consulta w:val para script latino.
	for _, attr := range []string{`w:val="fr"`, `w:eastAsia="fr"`, `w:bidi="fr"`} {
		if !strings.Contains(styles, attr) {
			t.Errorf("expected word/styles.xml to declare lang %s, got:\n%s", attr, styles)
		}
	}
}

// TestDOCXGenerator_NoLangDeclared_LeavesDocumentUnaffected confirma que un
// documento sin `lang:` no rompe la generación ni declara un w:lang vacío.
// Regresión de code review: la versión anterior de este test solo
// verificaba que Generate() no fallara, sin inspeccionar la salida —
// habría seguido en verde si un futuro refactor tumbara el guard
// `if astDoc.FrontMatter.Lang != ""` y emitiera w:val="" explícito.
func TestDOCXGenerator_NoLangDeclared_LeavesDocumentUnaffected(t *testing.T) {
	doc := astWithElements(ast.NewTextElement(diagnostics.NewPosition(1, 1), "Contenido."))

	logger := newTestLogger()
	gen := New(logger)
	output := filepath.Join(t.TempDir(), "nolang.docx")
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	styles := docxPartXML(t, output, "word/styles.xml")
	if strings.Contains(styles, "w:lang") {
		t.Errorf("expected no w:lang element when FrontMatter.Lang is unset, got:\n%s", styles)
	}
}
