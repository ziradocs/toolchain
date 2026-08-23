// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
)

// TestMarkdownGeneratorGenerate_WatermarkRoundTrips cubre issue #179 para
// el backend Markdown: Markdown no tiene concepto de página, así que
// watermark: se preserva por passthrough puro (no se renderiza nada), pero
// no debe perderse en el round-trip. Igual que su análogo de header/footer,
// re-parsea el .md generado con el FrontMatterParser real de core en vez
// de solo revisar substrings.
func TestMarkdownGeneratorGenerate_WatermarkRoundTrips(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)
	doc := newTestAST()
	opacity := 0.2
	rotation := 30.0
	repeat := false
	doc.FrontMatter.Watermark = &ast.WatermarkConfig{
		Enabled:  true,
		Text:     `Confidencial "2026"`,
		Color:    "#ff0000",
		Opacity:  &opacity,
		Rotation: &rotation,
		FontSize: "24pt",
		Repeat:   &repeat,
	}

	output := filepath.Join(t.TempDir(), "document.md")
	if err := gen.Generate(doc, output, GeneratorOptions{}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	content := string(data)

	fmParser := &parser.FrontMatterParser{}
	node, _, diags := fmParser.Parse(content)
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error-severity diagnostic re-parsing generated Markdown: %v\ncontent:\n%s", d, content)
		}
	}
	if node == nil || node.Watermark == nil {
		t.Fatalf("expected Watermark to survive the round-trip, got nil (content:\n%s)", content)
	}

	w := node.Watermark
	if !w.Enabled {
		t.Error("Enabled should round-trip as true")
	}
	if w.Text != `Confidencial "2026"` {
		t.Errorf("Text = %q, want %q", w.Text, `Confidencial "2026"`)
	}
	if w.Color != "#ff0000" {
		t.Errorf("Color = %q, want %q", w.Color, "#ff0000")
	}
	if w.Opacity == nil || *w.Opacity != 0.2 {
		t.Errorf("Opacity = %v, want 0.2", w.Opacity)
	}
	if w.Rotation == nil || *w.Rotation != 30 {
		t.Errorf("Rotation = %v, want 30", w.Rotation)
	}
	if w.FontSize != "24pt" {
		t.Errorf("FontSize = %q, want %q", w.FontSize, "24pt")
	}
	if w.Repeat == nil || *w.Repeat != false {
		t.Errorf("Repeat = %v, want false", w.Repeat)
	}
}

// TestMarkdownGeneratorGenerate_NilWatermark_NoBlock verifica que un
// documento sin watermark: no agrega el bloque al frontmatter generado.
func TestMarkdownGeneratorGenerate_NilWatermark_NoBlock(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)
	doc := newTestAST()

	output := filepath.Join(t.TempDir(), "document.md")
	if err := gen.Generate(doc, output, GeneratorOptions{}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	fmParser := &parser.FrontMatterParser{}
	node, _, _ := fmParser.Parse(string(data))
	if node != nil && node.Watermark != nil {
		t.Errorf("expected no Watermark block when unset, got: %+v", node.Watermark)
	}
}
