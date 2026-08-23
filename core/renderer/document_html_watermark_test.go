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

	// The structural CSS rule (.doclang-watermark-fixed) is always present
	// in the stylesheet — an unused selector is inert. Only the actual div
	// markup must be absent.
	if strings.Contains(html, `class="doclang-watermark-fixed"`) {
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

// TestGenerateDocumentHTML_WatermarkPageView covers page-view mode: the
// same single fixed-position watermark used by flow mode, not one per
// .document-page. A per-page div would miss the static TOC (rendered
// outside any .document-page) and any block that overflows a page-view
// .document-page onto more than one physical printed sheet — the
// position:fixed mechanism, repeated by Chromium on every printed page,
// covers both without extra markup.
func TestGenerateDocumentHTML_WatermarkPageView(t *testing.T) {
	doc := simpleDoc(
		ast.ContentBlock{BlockType: "title", Heading: "Doc"},
		ast.ContentBlock{BlockType: "content", Title: "Section 2"},
	)

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		PageBreaks: true,
		TOC:        true,
		Watermark:  &ast.WatermarkConfig{Enabled: true, Text: "BORRADOR"},
		HeaderFooter: &ast.HeaderFooterConfig{
			Header: &ast.HeaderConfig{Enabled: true, Text: &ast.HeaderFooterText{Center: "x"}},
		},
	}, nil)

	if !strings.Contains(html, `class="doclang-watermark-fixed"`) {
		t.Errorf("expected the fixed-position watermark in page-view mode too, got:\n%s", html)
	}
	if strings.Count(html, `class="doclang-watermark-fixed"`) != 1 {
		t.Errorf("expected exactly one watermark div regardless of .document-page count, got:\n%s", html)
	}
	if !strings.Contains(html, `class="toc"`) {
		t.Fatal("test setup expected a TOC to be present")
	}
	// The single fixed div must be emitted before the TOC in document
	// order — irrelevant for print (position:fixed ignores DOM order) but
	// confirms it isn't nested inside a .document-page that the TOC falls
	// outside of.
	if strings.Index(html, `class="doclang-watermark-fixed"`) > strings.Index(html, `class="toc"`) {
		t.Errorf("expected the watermark div before the TOC in document order, got:\n%s", html)
	}
}

// TestGenerateDocumentHTML_WatermarkDisabled covers an explicit
// enabled:false — no markup at all, same as nil.
func TestGenerateDocumentHTML_WatermarkDisabled(t *testing.T) {
	doc := simpleDoc(ast.ContentBlock{BlockType: "title", Heading: "Doc"})

	html := GenerateDocumentHTML(doc, DocumentHTMLOptions{
		Watermark: &ast.WatermarkConfig{Enabled: false, Text: "BORRADOR"},
	}, nil)

	if strings.Contains(html, `class="doclang-watermark-fixed"`) {
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
