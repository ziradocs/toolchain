// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/util"
)

// TestDocumentFlexParser_NoInfiniteLoopOnAsteriskAfterAngleBracket es una
// regresión para AL-3 (docs/SECURITY_AUDIT_2026-07.md): un subtítulo con un
// "*" precedido por "<" colgaba el proceso indefinidamente (202% CPU
// reproducido con un archivo de 3 líneas). El input corre en una goroutine
// con deadline: si el parser no termina en tiempo lineal, el test falla en
// vez de colgarse.
func TestDocumentFlexParser_NoInfiniteLoopOnAsteriskAfterAngleBracket(t *testing.T) {
	inputs := []string{
		"# T\n\n### <*\n\nx\n",          // exploit de la auditoría
		"# T\n\n## Using <*ptr>\n\nx\n", // input benigno que también colgaba
	}

	for _, input := range inputs {
		input := input
		done := make(chan struct{})
		go func() {
			log := util.NewConsoleLogger(util.LevelError, false)
			parser := NewDocumentFlexParser(input, log)
			parser.Parse()
			close(done)
		}()

		select {
		case <-done:
			// ok, terminó en tiempo razonable
		case <-time.After(2 * time.Second):
			t.Fatalf("Parse() no terminó en 2s para input %q (regresión del DoS AL-3)", input)
		}
	}
}

// TestDocumentFlexParser_SubsectionHeaderEscapesHTML es una regresión para
// CR-2: el texto de subsección se emitía como HTML crudo sin escapar. Ahora
// pasa por renderer.ProcessInlineMarkdownSecureLine, que escapa antes de
// aplicar markdown.
func TestDocumentFlexParser_SubsectionHeaderEscapesHTML(t *testing.T) {
	input := "# T\n\n## <img src=x onerror=alert(1)>\n\nx\n"

	log := util.NewConsoleLogger(util.LevelError, false)
	parser := NewDocumentFlexParser(input, log)
	astNode, diags := parser.Parse()
	if len(diags) > 0 {
		t.Errorf("Expected no diagnostics, got %d: %v", len(diags), diags)
	}

	found := false
	for _, block := range astNode.ContentBlocks {
		for _, elem := range block.Elements {
			if textElem, ok := elem.(*ast.TextElement); ok && textElem.IsRawHTML {
				if strings.Contains(textElem.Content, "<img") {
					t.Errorf("subsection heading was not escaped, raw <img> leaked into output: %q", textElem.Content)
				}
				if strings.Contains(textElem.Content, "&lt;img") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("expected to find an escaped subsection heading (&lt;img), found none")
	}
}

// TestDocumentFlexParser_SubsectionHeaderNotTreatedAsList es una regresión
// encontrada en code-review de esta misma PR: renderer.ProcessInlineMarkdownSecure
// interpreta un "- " inicial como viñeta de lista y envuelve el contenido en
// <ul><li>...</li></ul>, lo que produce HTML inválido (una lista de bloque
// dentro de un <hN>) si se aplica tal cual al texto de un header de una sola
// línea. parseSubsectionHeader debe usar ProcessInlineMarkdownSecureLine, que
// aplica los mismos formatos inline sin la lógica de listas/multi-línea.
func TestDocumentFlexParser_SubsectionHeaderNotTreatedAsList(t *testing.T) {
	input := "# T\n\n## - Legacy features\n\nx\n"

	log := util.NewConsoleLogger(util.LevelError, false)
	parser := NewDocumentFlexParser(input, log)
	astNode, diags := parser.Parse()
	if len(diags) > 0 {
		t.Errorf("Expected no diagnostics, got %d: %v", len(diags), diags)
	}

	found := false
	for _, block := range astNode.ContentBlocks {
		for _, elem := range block.Elements {
			if textElem, ok := elem.(*ast.TextElement); ok && textElem.IsRawHTML {
				if strings.Contains(textElem.Content, "<h2") {
					if strings.Contains(textElem.Content, "<ul>") || strings.Contains(textElem.Content, "<li>") {
						t.Errorf("subsection heading starting with \"- \" was wrapped as a list, want plain heading text: %q", textElem.Content)
					}
					if !strings.Contains(textElem.Content, "- Legacy features") {
						t.Errorf("expected literal \"- Legacy features\" text in heading, got: %q", textElem.Content)
					}
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("expected to find the rendered subsection heading, found none")
	}
}

func TestDocumentFlexParser_BasicStructure(t *testing.T) {
	input := `# Main Title

This is intro text.

## Subsection 1

Content of subsection 1.

## Subsection 2

Content of subsection 2.
`

	log := util.NewConsoleLogger(util.LevelError, false)
	parser := NewDocumentFlexParser(input, log)
	ast, diags := parser.Parse()

	// Verificar sin errores
	if len(diags) > 0 {
		t.Errorf("Expected no diagnostics, got %d", len(diags))
		for _, d := range diags {
			t.Logf("Diagnostic: %s", d.Message)
		}
	}

	// Verificar que solo hay 1 slide (1 sección del documento)
	if len(ast.ContentBlocks) != 1 {
		t.Errorf("Expected 1 section (slide), got %d", len(ast.ContentBlocks))
		for i, slide := range ast.ContentBlocks {
			t.Logf("Slide %d: Title=%s, Elements=%d", i, slide.Title, len(slide.Elements))
		}
		return
	}

	slide := ast.ContentBlocks[0]

	// Verificar título
	if slide.Heading != "Main Title" {
		t.Errorf("Expected heading 'Main Title', got '%s'", slide.Heading)
	}

	// Verificar que tiene elementos (intro text + 2 subsecciones + sus contenidos)
	if len(slide.Elements) < 4 {
		t.Errorf("Expected at least 4 elements, got %d", len(slide.Elements))
		for i, elem := range slide.Elements {
			t.Logf("Element %d: %T", i, elem)
		}
	}
}

func TestDocumentFlexParser_MultipleH1Sections(t *testing.T) {
	input := `# Section 1

Content 1.

# Section 2

Content 2.

# Section 3

Content 3.
`

	log := util.NewConsoleLogger(util.LevelError, false)
	parser := NewDocumentFlexParser(input, log)
	ast, diags := parser.Parse()

	if len(diags) > 0 {
		t.Errorf("Expected no diagnostics, got %d", len(diags))
	}

	// Verificar que hay 3 slides (3 secciones del documento)
	if len(ast.ContentBlocks) != 3 {
		t.Errorf("Expected 3 sections, got %d", len(ast.ContentBlocks))
		return
	}

	// Verificar títulos (primer H1 va en Heading, el resto en Title)
	expectedTitles := []string{"Section 1", "Section 2", "Section 3"}
	for i, expected := range expectedTitles {
		var actual string
		if i == 0 {
			actual = ast.ContentBlocks[i].Heading // Primer slide usa Heading
		} else {
			actual = ast.ContentBlocks[i].Title // Los demás usan Title
		}
		if actual != expected {
			t.Errorf("Section %d: expected title '%s', got '%s'", i, expected, actual)
		}
	}
}

func TestDocumentFlexParser_SubsectionHeaders(t *testing.T) {
	input := `# Main Title

## H2 Subsection

### H3 Sub-subsection

#### H4 Header

Content here.
`

	log := util.NewConsoleLogger(util.LevelError, false)
	parser := NewDocumentFlexParser(input, log)
	ast, diags := parser.Parse()

	if len(diags) > 0 {
		t.Errorf("Expected no diagnostics, got %d", len(diags))
	}

	if len(ast.ContentBlocks) != 1 {
		t.Errorf("Expected 1 section, got %d", len(ast.ContentBlocks))
		return
	}

	slide := ast.ContentBlocks[0]

	// Verificar que las subsecciones se convirtieron en elementos
	if len(slide.Elements) < 3 {
		t.Errorf("Expected at least 3 elements (h2, h3, h4), got %d", len(slide.Elements))
	}

	// NOTE: Para validar el contenido completo, necesitaríamos renderizar a HTML
	// Por ahora, verificamos que se crearon los elementos
	t.Logf("Section has %d elements (includes h2, h3, h4, text)", len(slide.Elements))
}

func TestDocumentFlexParser_WithFrontmatter(t *testing.T) {
	input := `---
title: "Test Document"
mode: flex
doctype: document
---

# Introduction

Document content here.
`

	log := util.NewConsoleLogger(util.LevelError, false)
	parser := NewDocumentFlexParser(input, log)
	ast, diags := parser.Parse()

	if len(diags) > 0 {
		t.Errorf("Expected no diagnostics, got %d", len(diags))
	}

	// Verificar frontmatter
	if ast.FrontMatter == nil {
		t.Error("Expected frontmatter to be parsed")
		return
	}

	if !strings.Contains(ast.FrontMatter.Raw, "title:") {
		t.Error("Expected frontmatter to contain 'title:'")
	}

	// Verificar que la sección se parseó
	if len(ast.ContentBlocks) != 1 {
		t.Errorf("Expected 1 section, got %d", len(ast.ContentBlocks))
	}
}

func TestDocumentFlexParser_MermaidDiagram(t *testing.T) {
	input := `# Architecture

## System Diagram

` + "```mermaid" + `
graph TD
    A --> B
    B --> C
` + "```" + `

Text after diagram.
`

	log := util.NewConsoleLogger(util.LevelError, false)
	parser := NewDocumentFlexParser(input, log)
	ast, diags := parser.Parse()

	if len(diags) > 0 {
		t.Errorf("Expected no diagnostics, got %d", len(diags))
	}

	if len(ast.ContentBlocks) != 1 {
		t.Errorf("Expected 1 section, got %d", len(ast.ContentBlocks))
		return
	}

	slide := ast.ContentBlocks[0]

	// Verificar que hay elementos (h2 + mermaid + texto)
	if len(slide.Elements) < 3 {
		t.Errorf("Expected at least 3 elements, got %d", len(slide.Elements))
		for i, elem := range slide.Elements {
			t.Logf("Element %d: %T", i, elem)
		}
	}

	// NOTE: Para validar que Mermaid se parseó, necesitaríamos type assertions
	// Por ahora, verificamos que hay suficientes elementos
	t.Logf("Section has %d elements (should include mermaid diagram)", len(slide.Elements))
}

func TestDocumentFlexParser_RealWorldExample(t *testing.T) {
	input := `# 🎯 Project Overview

## 📖 Introduction

This is a complex document with:
- Multiple sections
- Subsections with emojis
- Lists and content

## 🏗️ Architecture

### Components

- **Component A**: Does something
- **Component B**: Does something else

### Deployment

Instructions here.

# Conclusion

Final thoughts.
`

	log := util.NewConsoleLogger(util.LevelError, false)
	parser := NewDocumentFlexParser(input, log)
	ast, diags := parser.Parse()

	if len(diags) > 0 {
		t.Errorf("Expected no diagnostics, got %d", len(diags))
		for _, d := range diags {
			t.Logf("Diagnostic: %s", d.Message)
		}
	}

	// Verificar que hay 2 secciones principales (2 H1)
	if len(ast.ContentBlocks) != 2 {
		t.Errorf("Expected 2 sections (2 H1), got %d", len(ast.ContentBlocks))
		for i, slide := range ast.ContentBlocks {
			if slide.Heading != "" {
				t.Logf("Section %d: Heading=%s", i, slide.Heading)
			}
			if slide.Title != "" {
				t.Logf("Section %d: Title=%s", i, slide.Title)
			}
		}
		return
	}

	// Primera sección debería tener subsecciones y contenido
	firstSection := ast.ContentBlocks[0]
	if len(firstSection.Elements) < 4 {
		t.Errorf("First section: expected at least 4 elements, got %d", len(firstSection.Elements))
	}

	// NOTE: Para validar headers específicos, necesitaríamos type assertions más complejas
	// Por ahora, verificamos que se creó la estructura correcta
	t.Logf("First section has %d elements (should include h2, h3 headers)", len(firstSection.Elements))
}
