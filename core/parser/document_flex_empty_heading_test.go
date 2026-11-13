// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// Un `#` que sigue a una sección todavía vacía abre su propia sección. Antes
// el bucle del cuerpo solo cortaba en `#` si la sección ya tenía elementos,
// así que `# A` + `# B` dejaba "# B" como párrafo dentro de A (y sin línea en
// blanco en medio, pegado al texto siguiente: "# B Texto.").

// sectionTitle devuelve el texto del encabezado de un bloque, esté en
// Heading (bloque "title") o en Title (bloques "content").
func sectionTitle(b ast.ContentBlock) string {
	if b.Heading != "" {
		return b.Heading
	}
	return b.Title
}

// assertNoHeadingAsText falla si algún elemento de texto conserva el `#`
// literal de un encabezado que debió abrir sección.
func assertNoHeadingAsText(t *testing.T, doc *ast.AST, heading string) {
	t.Helper()
	for i, b := range doc.ContentBlocks {
		for _, el := range b.Elements {
			if te, ok := el.(*ast.TextElement); ok && strings.Contains(te.Content, heading) {
				t.Errorf("bloque %d: %q quedó como texto: %q", i, heading, te.Content)
			}
		}
	}
}

// assertSectionWithParagraph busca la sección titulada title y comprueba que
// su único elemento sea el párrafo para.
func assertSectionWithParagraph(t *testing.T, doc *ast.AST, title, para string) {
	t.Helper()
	for _, b := range doc.ContentBlocks {
		if sectionTitle(b) != title {
			continue
		}
		if len(b.Elements) != 1 {
			t.Fatalf("sección %q: se esperaba 1 elemento, hay %d: %+v", title, len(b.Elements), b.Elements)
		}
		te, ok := b.Elements[0].(*ast.TextElement)
		if !ok || te.Content != para || te.Level != 0 {
			t.Fatalf("sección %q: se esperaba el párrafo %q, got %+v", title, para, b.Elements[0])
		}
		return
	}
	t.Fatalf("no hay sección %q; bloques: %+v", title, doc.ContentBlocks)
}

func TestDocumentFlex_HeadingAfterEmptyTitleSection(t *testing.T) {
	cases := map[string]string{
		"LF":                     "# A\n\n# B\n\nTexto.\n",
		"CRLF":                   "# A\r\n\r\n# B\r\n\r\nTexto.\r\n",
		"sin líneas en blanco":   "# A\n# B\nTexto.\n",
		"con frontmatter flex":   "---\ntitle: Doc\nmode: flex\n---\n\n# A\n\n# B\n\nTexto.\n",
		"con node-id en las dos": "<!-- node-id: SecA -->\n# A\n\n<!-- node-id: SecB -->\n# B\n\nTexto.\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			doc := parsePlainMarkdown(t, src)
			if len(doc.ContentBlocks) != 2 {
				t.Fatalf("se esperaban 2 secciones, hay %d: %+v", len(doc.ContentBlocks), doc.ContentBlocks)
			}
			a, b := doc.ContentBlocks[0], doc.ContentBlocks[1]
			if a.BlockType != "title" || a.Heading != "A" || len(a.Elements) != 0 {
				t.Errorf("primera sección: se esperaba title \"A\" vacía, got %+v", a)
			}
			if b.BlockType != "content" || b.Title != "B" {
				t.Errorf("segunda sección: se esperaba content \"B\", got %+v", b)
			}
			assertSectionWithParagraph(t, doc, "B", "Texto.")
			assertNoHeadingAsText(t, doc, "# B")
		})
	}
}

// La misma situación con la sección vacía en medio del documento. Lo que se
// comprueba es que B abre sección; no se fija si la A vacía se conserva o se
// descarta (core/spec/language-specification.md describe hoy que flex
// descarta un `#` sin contenido).
func TestDocumentFlex_HeadingAfterEmptyContentSection(t *testing.T) {
	for name, src := range map[string]string{
		"LF":   "# T\n\nx\n\n# A\n\n# B\n\nTexto.\n",
		"CRLF": "# T\r\n\r\nx\r\n\r\n# A\r\n\r\n# B\r\n\r\nTexto.\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			doc := parsePlainMarkdown(t, src)
			assertSectionWithParagraph(t, doc, "T", "x")
			assertSectionWithParagraph(t, doc, "B", "Texto.")
			assertNoHeadingAsText(t, doc, "# B")
		})
	}
}

// `##` sigue siendo subsección dentro de la sección vacía, no sección propia,
// y un `#` dentro de un bloque de código pegado al encabezado sigue siendo
// código.
func TestDocumentFlex_EmptySectionKeepsSubsectionAndCode(t *testing.T) {
	doc := parsePlainMarkdown(t, "# A\n\n## Sub\n\nTexto.\n")
	if len(doc.ContentBlocks) != 1 {
		t.Fatalf("se esperaba 1 sección, hay %d: %+v", len(doc.ContentBlocks), doc.ContentBlocks)
	}
	if got := headingLevels(doc.ContentBlocks[0]); len(got) != 1 || got[0] != 2 {
		t.Errorf("se esperaba un h2 dentro de A, niveles: %v", got)
	}

	doc = parsePlainMarkdown(t, "# A\n\n```sh\n# comentario\n```\n\n# B\n\nTexto.\n")
	if len(doc.ContentBlocks) != 2 {
		t.Fatalf("se esperaban 2 secciones, hay %d: %+v", len(doc.ContentBlocks), doc.ContentBlocks)
	}
	code, ok := doc.ContentBlocks[0].Elements[0].(*ast.CodeElement)
	if !ok || code.Content != "# comentario" {
		t.Errorf("el bloque de código de A cambió: %+v", doc.ContentBlocks[0].Elements)
	}
	assertSectionWithParagraph(t, doc, "B", "Texto.")
}
