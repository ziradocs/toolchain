// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// Estos tests cubren issue #179: doclang parsea `watermark:` a
// ast.WatermarkConfig, pero antes de este cambio ningún renderer lo leía.

// TestGenerateDocumentHTML_NilWatermark_Unchanged verifica que
// opts.Watermark == nil no agrega ningún marcado nuevo.
func TestGenerateDocumentHTML_NilWatermark_Unchanged(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{Title: "Reporte"}, nil)

	// The structural CSS rules (.doclang-watermark, .doclang-watermark-fixed)
	// are always present in the stylesheet — unused selectors are inert.
	// Only the actual div markup must be absent.
	if strings.Contains(html, `class="doclang-watermark"`) || strings.Contains(html, `class="doclang-watermark-fixed"`) {
		t.Errorf("nil Watermark must not emit any watermark div, got:\n%s", html)
	}
}

// TestGenerateDocumentHTML_WatermarkFlowMode covers standard (non-page-view)
// mode: the watermark draws once, as a fixed-position overlay on <body>.
func TestGenerateDocumentHTML_WatermarkFlowMode(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		Watermark: &ast.WatermarkConfig{Enabled: true, Text: "BORRADOR"},
	}, nil)

	if !strings.Contains(html, `class="doclang-watermark-fixed"`) {
		t.Errorf("expected a fixed-position watermark in flow mode, got:\n%s", html)
	}
	if !strings.Contains(html, "BORRADOR") {
		t.Errorf("expected watermark text, got:\n%s", html)
	}
	if strings.Count(html, `class="doclang-watermark-fixed"`) != 1 {
		t.Errorf("expected exactly one fixed watermark div in flow mode, got:\n%s", html)
	}
}

// TestGenerateDocumentHTML_WatermarkPageView covers page-view mode: one
// watermark div per .document-page, not a single fixed one.
func TestGenerateDocumentHTML_WatermarkPageView(t *testing.T) {
	doc := simpleDoc(
		ast.ContentBlock{BlockType: "title", Heading: "Doc"},
		ast.ContentBlock{BlockType: "content", Title: "Section 2"},
	)

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		PageBreaks: true,
		Watermark:  &ast.WatermarkConfig{Enabled: true, Text: "BORRADOR"},
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{Enabled: true, Text: &ast.HeaderFooterText{Center: "x"}},
		},
	}, nil)

	if strings.Contains(html, `class="doclang-watermark-fixed"`) {
		t.Errorf("page-view mode must not also emit the flow-mode fixed watermark, got:\n%s", html)
	}
	gotDocPages := strings.Count(html, `class="document-page"`) + strings.Count(html, `class="document-page" style=`)
	gotWatermarks := strings.Count(html, `class="doclang-watermark"`)
	if gotDocPages == 0 {
		t.Fatal("expected at least one .document-page in page-view mode")
	}
	if gotWatermarks != gotDocPages {
		t.Errorf("expected one watermark per .document-page (%d pages), got %d watermark divs:\n%s", gotDocPages, gotWatermarks, html)
	}
}

// TestGenerateDocumentHTML_WatermarkDisabled covers an explicit
// enabled:false — no markup at all, same as nil.
func TestGenerateDocumentHTML_WatermarkDisabled(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		Watermark: &ast.WatermarkConfig{Enabled: false, Text: "BORRADOR"},
	}, nil)

	if strings.Contains(html, `class="doclang-watermark"`) || strings.Contains(html, `class="doclang-watermark-fixed"`) {
		t.Errorf("enabled:false must not emit any watermark div, got:\n%s", html)
	}
}

// TestGenerateDocumentHTML_WatermarkTextEscaped verifies the watermark
// text goes through EscapeHTML — this file builds HTML with fmt.Sprintf,
// not html/template, which would escape it on its own.
func TestGenerateDocumentHTML_WatermarkTextEscaped(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		Watermark: &ast.WatermarkConfig{Enabled: true, Text: `<script>alert(1)</script>`},
	}, nil)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Errorf("watermark text must be HTML-escaped, got:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("expected escaped watermark text, got:\n%s", html)
	}
}

// TestGenerateDocumentHTML_WatermarkSubstitutesVariables verifies
// {{title}} in watermark text resolves through the document's front
// matter variables, same as header/footer text does.
func TestGenerateDocumentHTML_WatermarkSubstitutesVariables(t *testing.T) {
	doc := &ast.AST{
		FrontMatter:   &ast.FrontMatterNode{Title: "Q3 Report"},
		ContentBlocks: []ast.ContentBlock{{BlockType: "title", Heading: "Doc"}},
	}

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		Watermark: &ast.WatermarkConfig{Enabled: true, Text: "{{title}} DRAFT"},
	}, nil)

	if !strings.Contains(html, "Q3 Report DRAFT") {
		t.Errorf("expected {{title}} to substitute, got:\n%s", html)
	}
}
