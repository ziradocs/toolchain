// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"image/png"
	"testing"

	docx "github.com/mmonterroca/docxgo/v2"
	"github.com/mmonterroca/docxgo/v2/domain"
	"go.ziradocs.com/core/v2/ast"
)

// Estos tests cubren issue #179 para el backend DOCX: docxgo no expone
// w:pict/VML (el mecanismo nativo de Word para un watermark de texto), así
// que el watermark se rasteriza a PNG y se ancla como imagen flotante
// BehindText en el header — este archivo verifica esa cadena completa.

// renderWatermarkOnDoc es un wrapper para llamar
// (*DOCXGenerator).renderWatermark sin construir un DOCXGenerator
// completo, mismo patrón que renderHeaderFooterOnDoc en
// docx_headerfooter_test.go.
func renderWatermarkOnDoc(doc domain.Document, cfg *ast.WatermarkConfig, variables map[string]interface{}) error {
	g := &DOCXGenerator{}
	return g.renderWatermark(doc, cfg, variables)
}

func TestRenderWatermark_NilConfig_WritesNothing(t *testing.T) {
	doc := docx.NewDocument()

	if err := renderWatermarkOnDoc(doc, nil, nil); err != nil {
		t.Fatalf("renderWatermark failed: %v", err)
	}

	section, _ := doc.DefaultSection()
	header, _ := section.Header(domain.HeaderDefault)
	if len(header.Paragraphs()) != 0 {
		t.Errorf("expected no header paragraphs for nil config, got %d", len(header.Paragraphs()))
	}
}

func TestRenderWatermark_DisabledWritesNothing(t *testing.T) {
	doc := docx.NewDocument()
	cfg := &ast.WatermarkConfig{Enabled: false, Text: "BORRADOR"}

	if err := renderWatermarkOnDoc(doc, cfg, nil); err != nil {
		t.Fatalf("renderWatermark failed: %v", err)
	}

	section, _ := doc.DefaultSection()
	header, _ := section.Header(domain.HeaderDefault)
	if len(header.Paragraphs()) != 0 {
		t.Errorf("expected enabled:false to write nothing, got %d paragraphs", len(header.Paragraphs()))
	}
}

func TestRenderWatermark_EmbedsFloatingBehindTextImage(t *testing.T) {
	doc := docx.NewDocument()
	cfg := &ast.WatermarkConfig{Enabled: true, Text: "BORRADOR"}

	if err := renderWatermarkOnDoc(doc, cfg, nil); err != nil {
		t.Fatalf("renderWatermark failed: %v", err)
	}

	section, _ := doc.DefaultSection()
	header, _ := section.Header(domain.HeaderDefault)
	paras := header.Paragraphs()
	if len(paras) != 1 {
		t.Fatalf("expected exactly 1 header paragraph, got %d", len(paras))
	}

	images := paras[0].Images()
	if len(images) != 1 {
		t.Fatalf("expected exactly 1 image, got %d", len(images))
	}

	pos := images[0].Position()
	if pos.Type != domain.ImagePositionFloating {
		t.Errorf("Position.Type = %v, want ImagePositionFloating", pos.Type)
	}
	if !pos.BehindText {
		t.Error("Position.BehindText = false, want true — a watermark on top would obscure the document text")
	}
	if pos.WrapText != domain.WrapNone {
		t.Errorf("Position.WrapText = %v, want WrapNone", pos.WrapText)
	}
}

func TestRenderWatermark_ImageSizeMatchesPageSize(t *testing.T) {
	doc := docx.NewDocument()
	cfg := &ast.WatermarkConfig{Enabled: true, Text: "X"}

	if err := renderWatermarkOnDoc(doc, cfg, nil); err != nil {
		t.Fatalf("renderWatermark failed: %v", err)
	}

	section, _ := doc.DefaultSection()
	pageSize := section.PageSize()

	header, _ := section.Header(domain.HeaderDefault)
	images := header.Paragraphs()[0].Images()
	pos := images[0]

	// Image() has no direct Size() getter besides Position()'s offsets;
	// decode the actual PNG dimensions instead and cross-check against the
	// resolution RenderWatermarkPNG was asked for (150 DPI over the page's
	// real inch dimensions) — this is what proves the watermark spans the
	// FULL page, not some hardcoded fallback size.
	img, err := png.Decode(bytes.NewReader(pos.Data()))
	if err != nil {
		t.Fatalf("embedded image is not a valid PNG: %v", err)
	}
	const dpi = 150
	const twipsPerInch = 1440
	wantW := pageSize.Width * dpi / twipsPerInch
	wantH := pageSize.Height * dpi / twipsPerInch
	if img.Bounds().Dx() != wantW || img.Bounds().Dy() != wantH {
		t.Errorf("PNG dimensions = %dx%d, want %dx%d (page size at 150 DPI)", img.Bounds().Dx(), img.Bounds().Dy(), wantW, wantH)
	}
}

func TestRenderWatermark_TextSubstitutesVariables(t *testing.T) {
	doc := docx.NewDocument()
	cfg := &ast.WatermarkConfig{Enabled: true, Text: "{{title}} DRAFT"}
	variables := map[string]interface{}{"title": "Q3 Report"}

	if err := renderWatermarkOnDoc(doc, cfg, variables); err != nil {
		t.Fatalf("renderWatermark failed: %v", err)
	}

	section, _ := doc.DefaultSection()
	header, _ := section.Header(domain.HeaderDefault)
	if len(header.Paragraphs()[0].Images()) != 1 {
		t.Fatal("expected the watermark image to render even with variable substitution")
	}
	// The substituted text is baked into pixels, not recoverable from the
	// PNG without OCR — TestResolveWatermark (core) already covers
	// substitution correctness; this test only proves the DOCX path
	// doesn't error or skip when Text contains a placeholder.
}
