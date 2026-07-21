// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/util"
)

func pos() diagnostics.Position { return diagnostics.Position{Line: 1, Column: 1} }

func TestPptxSplitInline(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []pptxInlineSegment
	}{
		{"plain", "hello world", []pptxInlineSegment{{text: "hello world"}}},
		{"bold", "a **bold** word", []pptxInlineSegment{
			{text: "a "}, {text: "bold", bold: true}, {text: " word"},
		}},
		{"italic", "a *italic* word", []pptxInlineSegment{
			{text: "a "}, {text: "italic", italic: true}, {text: " word"},
		}},
		{"code", "a `code` word", []pptxInlineSegment{
			{text: "a "}, {text: "code", code: true}, {text: " word"},
		}},
		{"code before bold", "`**not bold**`", []pptxInlineSegment{
			{text: "**not bold**", code: true},
		}},
		{"mixed", "**bold** and *italic* and `code`", []pptxInlineSegment{
			{text: "bold", bold: true}, {text: " and "}, {text: "italic", italic: true}, {text: " and "}, {text: "code", code: true},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pptxSplitInline(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("pptxSplitInline(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("segment %d: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPptxEstimateLines(t *testing.T) {
	short := pptxEstimateLines("short line")
	if short != 1 {
		t.Errorf("short text: got %d lines, want 1", short)
	}

	long := pptxEstimateLines(string(make([]byte, pptxCharsPerLine*3)))
	if long < 3 {
		t.Errorf("text 3x wrap width: got %d lines, want >= 3", long)
	}

	multiline := pptxEstimateLines("line one\nline two\nline three")
	if multiline < 3 {
		t.Errorf("3 explicit lines: got %d lines, want >= 3", multiline)
	}

	if pptxEstimateLines("") != 1 {
		t.Errorf("empty text should still estimate at least 1 line, got %d", pptxEstimateLines(""))
	}
}

// buildTestAST arma un *ast.AST mínimo con un bloque "title" y un bloque de
// contenido que ejercita los 4 tipos de elemento del alcance MVP v0: texto
// con markdown inline, puntos (con subitem), tabla, e imagen.
func buildTestAST(imageSource string) *ast.AST {
	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FrontMatter.Title = "Test Deck"

	titleBlock := ast.NewContentBlock(pos(), "title")
	titleBlock.Heading = "Test Deck"
	titleBlock.Subtitle = "A subtitle"
	doc.ContentBlocks = append(doc.ContentBlocks, *titleBlock)

	contentBlock := ast.NewContentBlock(pos(), "content")
	contentBlock.Title = "Content Slide"

	text := ast.NewTextElement(pos(), "Some **bold** and *italic* and `code` text.")
	contentBlock.Elements = append(contentBlock.Elements, text)

	points := ast.NewPointsElement(pos())
	points.ListType = "unordered"
	item := ast.NewPointItem(pos(), "First point with *emphasis*")
	item.SubPoints = append(item.SubPoints, *ast.NewPointItem(pos(), "Nested point"))
	points.Items = append(points.Items, *item)
	contentBlock.Elements = append(contentBlock.Elements, points)

	table := ast.NewTableElement(pos())
	table.Headers = []string{"A", "B"}
	table.Rows = [][]string{{"1", "2"}, {"3", "4"}}
	contentBlock.Elements = append(contentBlock.Elements, table)

	if imageSource != "" {
		img := ast.NewImageElementWithContext(pos(), imageSource, "alt text", ast.ImageContextTitle)
		contentBlock.Elements = append(contentBlock.Elements, img)
	}

	doc.ContentBlocks = append(doc.ContentBlocks, *contentBlock)
	return doc
}

// tinyPNG es un PNG 2x2 azul sólido válido, suficientemente pequeño para
// embeber inline en el test — evita depender de un asset externo en el repo.
var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02,
	0x08, 0x02, 0x00, 0x00, 0x00, 0xfd, 0xd4, 0x9a, 0x73, 0x00, 0x00, 0x00,
	0x15, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0x60, 0x60, 0xf8, 0xcf,
	0x80, 0x04, 0x8c, 0x8c, 0x0c, 0x0c, 0x00, 0x0b, 0x94, 0x02, 0xea, 0xa9,
	0x67, 0x0f, 0x5a, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

// zipEntryNames lista los nombres de entrada de un .pptx (que es un zip
// OOXML) — usado para verificar la estructura del paquete sin un lector
// OOXML completo.
func zipEntryNames(t *testing.T, path string) []string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("output is not a valid zip/pptx: %v", err)
	}
	defer func() { _ = r.Close() }()

	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return names
}

func hasEntry(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// TestGeneratePPTX_ProducesValidPackage es la prueba central: genera un
// .pptx real a partir de un AST con los 4 tipos de elemento del alcance MVP
// v0 (incluida una imagen local confinada a AssetRoot) y verifica que el
// resultado es un paquete OOXML válido con un slide por ContentBlock — no
// solo que "no hubo error", sino que el archivo es abrible como zip y trae
// la estructura esperada. El smoke-test E2E real (build binario +
// LibreOffice + inspección visual, ver la PR) encontró un bug real que este
// test unitario por sí solo no habría detectado (formato inline no
// aplicado dentro de items de viñeta) — este test cubre el caso que sí
// puede expresarse sin un renderer externo: la estructura del paquete.
func TestGeneratePPTX_ProducesValidPackage(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "logo.png")
	if err := os.WriteFile(imgPath, tinyPNG, 0644); err != nil {
		t.Fatalf("failed to write test image: %v", err)
	}

	doc := buildTestAST("logo.png")
	doc.FilePath = "test.slidelang"

	g := New(util.NewNoop())
	opts := GeneratorOptions{AssetRoot: dir}
	if err := g.generatePPTX(doc, dir, opts); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	outputPath := filepath.Join(dir, "test.pptx")
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("expected output file %s to exist: %v", outputPath, err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	names := zipEntryNames(t, outputPath)
	if !hasEntry(names, "ppt/presentation.xml") {
		t.Errorf("expected ppt/presentation.xml in package, got entries: %v", names)
	}

	slideCount := 0
	for _, n := range names {
		if filepath.Dir(n) == "ppt/slides" && filepath.Ext(n) == ".xml" {
			slideCount++
		}
	}
	if slideCount != len(doc.ContentBlocks) {
		t.Errorf("expected %d slides (one per ContentBlock), got %d", len(doc.ContentBlocks), slideCount)
	}

	mediaCount := 0
	for _, n := range names {
		if filepath.Dir(n) == "ppt/media" {
			mediaCount++
		}
	}
	if mediaCount != 1 {
		t.Errorf("expected 1 embedded media part (the confined logo.png), got %d", mediaCount)
	}
}

// TestGeneratePPTX_BlocksImageOutsideAssetRoot es una regresión de AL-4
// (docs/SECURITY_AUDIT_2026-07.md): una fuente de imagen que escapa
// AssetRoot vía ".." no debe leerse del disco ni abortar el build entero —
// se degrada a un placeholder de texto, igual que
// doclang/internal/generator/docx.go.
func TestGeneratePPTX_BlocksImageOutsideAssetRoot(t *testing.T) {
	outsideDir := t.TempDir()
	secretPath := filepath.Join(outsideDir, "secret.png")
	if err := os.WriteFile(secretPath, tinyPNG, 0644); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}

	assetRoot := t.TempDir()
	doc := buildTestAST("../" + filepath.Base(outsideDir) + "/secret.png")
	doc.FilePath = "test.slidelang"

	g := New(util.NewNoop())
	opts := GeneratorOptions{AssetRoot: assetRoot}
	if err := g.generatePPTX(doc, assetRoot, opts); err != nil {
		t.Fatalf("generatePPTX() should degrade gracefully, not error: %v", err)
	}

	outputPath := filepath.Join(assetRoot, "test.pptx")
	names := zipEntryNames(t, outputPath)
	for _, n := range names {
		if filepath.Dir(n) == "ppt/media" {
			t.Errorf("image outside AssetRoot must not be embedded, but found media part: %s", n)
		}
	}
}

// TestGeneratePPTX_EmptyBlocksSkipped confirma que un ContentBlock sin
// Elements no rompe el generador (título vacío, sin cuerpo) — caso límite
// real: el fixture usado en el smoke-test manual produce un slide "title"
// sin Elements.
func TestGeneratePPTX_EmptyBlocksSkipped(t *testing.T) {
	dir := t.TempDir()
	doc := ast.NewAST(pos())
	block := ast.NewContentBlock(pos(), "title")
	block.Heading = "Only a title"
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	doc.FilePath = "empty.slidelang"

	g := New(util.NewNoop())
	if err := g.generatePPTX(doc, dir, GeneratorOptions{}); err != nil {
		t.Fatalf("generatePPTX() error on title-only block = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "empty.pptx")); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
}
