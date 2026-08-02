// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestMarkdownGenerator_EmitsLangInFrontMatter cubre el prerrequisito de los
// issues #62/#63: el export a Markdown debe reemitir `lang:` en el
// frontmatter cuando el AST lo declara, para que reingerir el .md no pierda
// el idioma del documento.
func TestMarkdownGenerator_EmitsLangInFrontMatter(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)

	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	front := ast.NewFrontMatterNode(pos)
	front.Title = "Doc"
	front.Lang = "pt-BR"
	doc.FrontMatter = front
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Elements = append(block.Elements, ast.NewTextElement(pos, "Conteúdo."))
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	output := filepath.Join(t.TempDir(), "out.md")
	if err := gen.Generate(doc, output, GeneratorOptions{}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read generated markdown: %v", err)
	}
	if !strings.Contains(string(content), "lang: pt-BR\n") {
		t.Errorf("expected frontmatter to contain %q, got:\n%s", "lang: pt-BR", content)
	}
}

func TestMarkdownGenerator_NoLangDeclared_OmitsLangKey(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)
	doc := newTestAST()

	output := filepath.Join(t.TempDir(), "out.md")
	if err := gen.Generate(doc, output, GeneratorOptions{}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read generated markdown: %v", err)
	}
	if strings.Contains(string(content), "lang:") {
		t.Errorf("expected no lang: key when FrontMatter.Lang is empty, got:\n%s", content)
	}
}
