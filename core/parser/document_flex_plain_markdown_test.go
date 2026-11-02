// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// Markdown común de proyecto convertido con `doclang build` (modo flex):
// sin frontmatter, solo con `##`, o con texto antes del primer `#`. Antes el
// parser saltaba en silencio todo lo previo al primer `#`; un archivo sin
// `#` quedaba con cero secciones. Lo previo al primer `#` es ahora una
// sección inicial sin encabezado y nada se descarta.

func parsePlainMarkdown(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, diags := New(util.NewNoop()).ParseDocument(src, "")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("diagnóstico de error inesperado: %+v", d)
		}
	}
	if doc == nil {
		t.Fatal("ParseDocument devolvió un AST nil")
	}
	return doc
}

// astContains busca s en el AST serializado: la forma más directa de
// probar que un texto sobrevivió sin atarse a qué elemento lo lleva.
func astContains(t *testing.T, doc *ast.AST, s string) bool {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return strings.Contains(string(b), s)
}

func headingLevels(block ast.ContentBlock) []int {
	var levels []int
	for _, el := range block.Elements {
		if te, ok := el.(*ast.TextElement); ok && te.Level > 0 {
			levels = append(levels, te.Level)
		}
	}
	return levels
}

func TestParseDocument_PlainMarkdown_NoFrontMatter(t *testing.T) {
	doc := parsePlainMarkdown(t, "# Mi título\n\nPárrafo SENTINEL_A.\n\n## Sub\n\nTexto SENTINEL_B.\n")

	if doc.FrontMatter != nil {
		t.Errorf("no se debe sintetizar un frontmatter en el AST, got %+v", doc.FrontMatter)
	}
	if len(doc.ContentBlocks) != 1 {
		t.Fatalf("want 1 sección, got %d", len(doc.ContentBlocks))
	}
	b := doc.ContentBlocks[0]
	if b.BlockType != "title" || b.Heading != "Mi título" {
		t.Errorf("primer `#` debe seguir siendo el bloque title con Heading, got type=%q heading=%q", b.BlockType, b.Heading)
	}
	for _, s := range []string{"SENTINEL_A", "SENTINEL_B"} {
		if !astContains(t, doc, s) {
			t.Errorf("se perdió %s", s)
		}
	}
}

func TestParseDocument_PlainMarkdown_OnlyH2(t *testing.T) {
	doc := parsePlainMarkdown(t, "## Solo dos\n\nTexto SENTINEL_C.\n\n### Tres\n\nMás SENTINEL_D.\n")

	if len(doc.ContentBlocks) != 1 {
		t.Fatalf("want 1 sección inicial sin encabezado, got %d", len(doc.ContentBlocks))
	}
	b := doc.ContentBlocks[0]
	if b.BlockType != "content" || b.Title != "" || b.Heading != "" {
		t.Errorf("la sección inicial no lleva encabezado, got type=%q title=%q heading=%q", b.BlockType, b.Title, b.Heading)
	}
	if got := headingLevels(b); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Errorf("`##`/`###` deben conservar su nivel (sin promoverse), got %v", got)
	}
	for _, s := range []string{"Solo dos", "SENTINEL_C", "Tres", "SENTINEL_D"} {
		if !astContains(t, doc, s) {
			t.Errorf("se perdió %q", s)
		}
	}
}

func TestParseDocument_PlainMarkdown_TextBeforeH1(t *testing.T) {
	src := "> [!CAUTION]\n> Aviso SENTINEL_E.\n\nIntro SENTINEL_F.\n\n## Antes\n\n# Título\n\nCuerpo SENTINEL_G.\n\n# Otra\n\nMás SENTINEL_H.\n"
	doc := parsePlainMarkdown(t, src)

	if len(doc.ContentBlocks) != 3 {
		t.Fatalf("want preámbulo + 2 secciones, got %d", len(doc.ContentBlocks))
	}
	pre, first, second := doc.ContentBlocks[0], doc.ContentBlocks[1], doc.ContentBlocks[2]
	if pre.BlockType != "content" || pre.Title != "" || pre.Heading != "" {
		t.Errorf("preámbulo sin encabezado, got type=%q title=%q heading=%q", pre.BlockType, pre.Title, pre.Heading)
	}
	if first.BlockType != "title" || first.Heading != "Título" {
		t.Errorf("el primer `#` sigue siendo title aunque haya preámbulo, got type=%q heading=%q", first.BlockType, first.Heading)
	}
	if second.BlockType != "content" || second.Title != "Otra" {
		t.Errorf("segundo `#`, got type=%q title=%q", second.BlockType, second.Title)
	}
	if pre.Position.Line != 1 {
		t.Errorf("el preámbulo debe anclarse en la línea 1, got %d", pre.Position.Line)
	}
	for _, s := range []string{"SENTINEL_E", "SENTINEL_F", "Antes", "SENTINEL_G", "SENTINEL_H"} {
		if !astContains(t, doc, s) {
			t.Errorf("se perdió %q", s)
		}
	}
}

// Un preámbulo que no produce elementos (solo `---`, que flex ignora como
// separador; la línea en blanco inicial evita que se lea como frontmatter)
// no abre sección, y el `#` que sigue tiene que abrir la suya en vez de
// quedar adentro del preámbulo como texto.
func TestParseDocument_PlainMarkdown_EmptyPreambleDoesNotSwallowH1(t *testing.T) {
	doc := parsePlainMarkdown(t, "\n---\n\n# Título\n\nCuerpo.\n")
	if len(doc.ContentBlocks) != 1 {
		t.Fatalf("want 1 sección, got %d", len(doc.ContentBlocks))
	}
	b := doc.ContentBlocks[0]
	if b.BlockType != "title" || b.Heading != "Título" {
		t.Errorf("el `#` debe abrir su propia sección, got type=%q heading=%q", b.BlockType, b.Heading)
	}
}

func TestParseDocument_PlainMarkdown_FrontMatterAndH1Unchanged(t *testing.T) {
	doc := parsePlainMarkdown(t, "---\nmode: flex\ntitle: Con FM\n---\n\n# Con FM\n\nCuerpo SENTINEL_H.\n")
	if doc.FrontMatter == nil || doc.FrontMatter.Title != "Con FM" {
		t.Fatalf("frontmatter, got %+v", doc.FrontMatter)
	}
	if len(doc.ContentBlocks) != 1 {
		t.Fatalf("want 1 sección, got %d", len(doc.ContentBlocks))
	}
	b := doc.ContentBlocks[0]
	if b.BlockType != "title" || b.Heading != "Con FM" || b.Position.Line != 6 {
		t.Errorf("got type=%q heading=%q line=%d", b.BlockType, b.Heading, b.Position.Line)
	}
}

func TestParseDocument_PlainMarkdown_EmptyDocumentHasNoBlocks(t *testing.T) {
	for _, src := range []string{"", "\n\n", "---\nmode: flex\n---\n\n"} {
		doc := parsePlainMarkdown(t, src)
		if len(doc.ContentBlocks) != 0 {
			t.Errorf("%q: want 0 secciones, got %d", src, len(doc.ContentBlocks))
		}
	}
}

// strict no pasa por DocumentFlexParser: la prosa fuera de un SECTION
// sigue siendo un error, no una sección inicial.
func TestParseDocument_PlainMarkdown_StrictUnchanged(t *testing.T) {
	src := "---\nmode: strict\n---\n\nTexto suelto.\n\nSECTION \"Uno\"\n  TEXT \"Hola\"\n"
	_, diags := New(util.NewNoop()).ParseDocument(src, "")
	hasErr := false
	for _, d := range diags {
		if d.IsError() {
			hasErr = true
		}
	}
	if !hasErr {
		t.Errorf("strict debe seguir rechazando prosa fuera de un SECTION, diags=%+v", diags)
	}
}
