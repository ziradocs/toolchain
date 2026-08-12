// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
)

// TestMarkdownGeneratorGenerate_HeaderFooterRoundTrips cubre issue #117
// para el backend Markdown: header:/footer:/layout_defaults: se perdían en
// silencio en el round-trip — un .doclang con `header:` producía un .md sin
// rastro de esa config. Este test genera el .md y lo re-parsea con el
// FrontMatterParser real de core (no solo revisa substrings) para
// confirmar que la config sobrevive completa, no solo parcialmente.
func TestMarkdownGeneratorGenerate_HeaderFooterRoundTrips(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)
	doc := newTestAST()
	doc.FrontMatter.HeaderFooter = &ast.HeaderFooterConfig{
		Header: &ast.HeaderConfig{
			Enabled: true,
			Height:  "20mm",
			Text:    &ast.HeaderFooterText{Left: "Izquierda", Center: "Centro \"con comillas\"", Right: "Derecha"},
			Logo:    &ast.LogoConfig{Source: "logo.png", Alt: "Logo", Position: "left"},
			Border:  &ast.BorderConfig{Enabled: true, Color: "#333", Position: "bottom"},
		},
		Footer: &ast.FooterConfig{
			Enabled: true,
			PageNumbers: &ast.PageNumbersConfig{
				Enabled:            true,
				Format:             "{{current}} / {{total}}",
				Position:           "right",
				ExcludeTitleSlides: true,
				StartFrom:          2,
			},
		},
		LayoutDefaults: map[string]*ast.LayoutHeaderFooterConfig{
			"title": {
				Header: &ast.HeaderConfig{Enabled: false},
			},
		},
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
	if node == nil || node.HeaderFooter == nil {
		t.Fatalf("expected HeaderFooter to survive the round-trip, got nil (content:\n%s)", content)
	}

	hf := node.HeaderFooter
	if hf.Header == nil || !hf.Header.Enabled || hf.Header.Height != "20mm" {
		t.Fatalf("header round-trip mismatch: %+v", hf.Header)
	}
	if hf.Header.Text == nil || hf.Header.Text.Left != "Izquierda" || hf.Header.Text.Center != `Centro "con comillas"` || hf.Header.Text.Right != "Derecha" {
		t.Fatalf("header.text round-trip mismatch: %+v", hf.Header.Text)
	}
	if hf.Header.Logo == nil || hf.Header.Logo.Source != "logo.png" || hf.Header.Logo.Position != "left" {
		t.Fatalf("header.logo round-trip mismatch: %+v", hf.Header.Logo)
	}
	if hf.Header.Border == nil || !hf.Header.Border.Enabled || hf.Header.Border.Color != "#333" || hf.Header.Border.Position != "bottom" {
		t.Fatalf("header.border round-trip mismatch: %+v", hf.Header.Border)
	}
	if hf.Footer == nil || !hf.Footer.Enabled {
		t.Fatalf("footer round-trip mismatch: %+v", hf.Footer)
	}
	pn := hf.Footer.PageNumbers
	if pn == nil || !pn.Enabled || pn.Format != "{{current}} / {{total}}" || pn.Position != "right" || !pn.ExcludeTitleSlides || pn.StartFrom != 2 {
		t.Fatalf("footer.page_numbers round-trip mismatch: %+v", pn)
	}
	titleLayout, ok := hf.LayoutDefaults["title"]
	if !ok || titleLayout.Header == nil || titleLayout.Header.Enabled {
		t.Fatalf("layout_defaults.title round-trip mismatch: %+v", hf.LayoutDefaults)
	}
}

// TestMarkdownGeneratorGenerate_HeaderFooterRoundTrips_MultilineValue cubre
// un hallazgo de code review sobre este PR: un valor con un salto de línea
// LITERAL (no las dos letras "\n") sobrevive dentro de las comillas dobles
// YAML sin escapar — pero YAML pliega ("folding") ese salto de línea en un
// espacio al re-parsear, así que "línea uno\nlínea dos" volvía como
// "línea uno línea dos" en el round-trip. yamlScalarEscaper ahora escapa
// \n/\r explícitamente (como \n/\r de dos caracteres) para que el
// re-parseo preserve el valor exacto.
func TestMarkdownGeneratorGenerate_HeaderFooterRoundTrips_MultilineValue(t *testing.T) {
	logger := newTestLogger()
	gen := NewMarkdownGenerator(logger)
	doc := newTestAST()
	doc.FrontMatter.HeaderFooter = &ast.HeaderFooterConfig{
		Header: &ast.HeaderConfig{
			Enabled: true,
			Text:    &ast.HeaderFooterText{Center: "line one\nline two"},
		},
	}

	output := filepath.Join(t.TempDir(), "document.md")
	if err := gen.Generate(doc, output, GeneratorOptions{}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	fmParser := &parser.FrontMatterParser{}
	node, _, diags := fmParser.Parse(string(data))
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error-severity diagnostic re-parsing generated Markdown: %v\ncontent:\n%s", d, data)
		}
	}
	if node == nil || node.HeaderFooter == nil || node.HeaderFooter.Header == nil || node.HeaderFooter.Header.Text == nil {
		t.Fatalf("expected header.text to survive the round-trip, got nil (content:\n%s)", data)
	}
	if got := node.HeaderFooter.Header.Text.Center; got != "line one\nline two" {
		t.Errorf("text.center round-trip mismatch: got %q, want %q (content:\n%s)", got, "line one\nline two", data)
	}
}

func TestMarkdownGeneratorGenerate_NoHeaderFooter_OmitsKeys(t *testing.T) {
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
	content := string(data)

	for _, key := range []string{"header:", "footer:", "layout_defaults:"} {
		if strings.Contains(content, key) {
			t.Errorf("expected no %q key when FrontMatter.HeaderFooter is nil, got:\n%s", key, content)
		}
	}
}
