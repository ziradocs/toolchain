// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package generator

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// pptx_watermark_test.go cubre issue #179: --format pptx no tenía ningún
// concepto de watermark. Dos divergencias deliberadas respecto a HTML/PDF
// se prueban acá — opacidad aproximada por pre-mezcla y "repeat: true"
// ignorado — junto con el z-order (primer shape del slide, detrás de
// título/contenido).

// buildPPTXWithWatermark genera un .pptx de un solo slide con el
// watermark dado en el front matter y devuelve el XML de slide1, más el
// logger spy usado (para inspeccionar warnings).
func buildPPTXWithWatermark(t *testing.T, watermark *ast.WatermarkConfig) (string, *directiveWarnSpy) {
	t.Helper()
	dir := t.TempDir()

	doc := ast.NewAST(pos())
	doc.FrontMatter = ast.NewFrontMatterNode(pos())
	doc.FrontMatter.Watermark = watermark
	doc.FilePath = "watermark.slidelang"

	block := ast.NewContentBlock(pos(), "content")
	block.Title = "Watermark Coverage"
	text := ast.NewTextElement(pos(), "content-marker")
	block.Elements = append(block.Elements, text)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	spy := &directiveWarnSpy{}
	g := New(spy)
	if err := g.generatePPTX(doc, dir, GeneratorOptions{AssetRoot: dir}); err != nil {
		t.Fatalf("generatePPTX() error = %v", err)
	}

	return zipEntryContent(t, filepath.Join(dir, "watermark.pptx"), "ppt/slides/slide1.xml"), spy
}

func TestPPTX_WatermarkIsFirstShapeBehindContent(t *testing.T) {
	xml, _ := buildPPTXWithWatermark(t, &ast.WatermarkConfig{Enabled: true, Text: "BORRADOR"})

	names := regexp.MustCompile(`name="([^"]+)"`).FindAllStringSubmatch(xml, -1)
	if len(names) < 2 {
		t.Fatalf("expected at least 2 shapes (watermark + title placeholder), got: %v", names)
	}
	if !strings.Contains(xml, "BORRADOR") {
		t.Fatalf("expected watermark text in slide XML, got:\n%s", xml)
	}

	watermarkPos := strings.Index(xml, "BORRADOR")
	titlePos := strings.Index(xml, "Watermark Coverage")
	if titlePos == -1 {
		t.Fatal("expected the slide title to also render")
	}
	if watermarkPos > titlePos {
		t.Errorf("watermark must be the FIRST shape (behind content) — found watermark text after the title in XML order")
	}
}

func TestPPTX_WatermarkRotationEncodedInEMU(t *testing.T) {
	rotation := -30.0
	xml, _ := buildPPTXWithWatermark(t, &ast.WatermarkConfig{Enabled: true, Text: "X", Rotation: &rotation})

	// rot is in 60,000ths of a degree (drawingml a:xfrm/@rot) — -30deg * 60000 = -1800000.
	if !strings.Contains(xml, `rot="-1800000"`) {
		t.Errorf("expected rot=\"-1800000\" (=-30deg) in watermark xfrm, got:\n%s", xml)
	}
}

func TestPPTX_WatermarkOpacityPreBlended(t *testing.T) {
	opacity := 0.15
	xml, _ := buildPPTXWithWatermark(t, &ast.WatermarkConfig{
		Enabled: true, Text: "X", Color: "#000000", Opacity: &opacity,
	})

	// black (#000000) at 15% opacity over a white background blends to
	// #D9D9D9 (255*0.85 rounded = 217 = 0xD9) — BlendOverOpaque's contract.
	if !strings.Contains(strings.ToUpper(xml), `SRGBCLR VAL="D9D9D9"`) {
		t.Errorf("expected the pre-blended color D9D9D9, got:\n%s", xml)
	}
}

func TestPPTX_WatermarkWordWrapDisabledAndCentered(t *testing.T) {
	xml, _ := buildPPTXWithWatermark(t, &ast.WatermarkConfig{Enabled: true, Text: "X"})

	if !strings.Contains(xml, `wrap="none"`) {
		t.Errorf("expected wrap=\"none\" on the watermark textbox, got:\n%s", xml)
	}
	if !strings.Contains(xml, `anchor="ctr"`) {
		t.Errorf("expected anchor=\"ctr\" on the watermark textbox, got:\n%s", xml)
	}
	if !strings.Contains(xml, `algn="ctr"`) {
		t.Errorf("expected algn=\"ctr\" on the watermark paragraph, got:\n%s", xml)
	}
}

func TestPPTX_WatermarkDisabledDrawsNothing(t *testing.T) {
	xml, _ := buildPPTXWithWatermark(t, &ast.WatermarkConfig{Enabled: false, Text: "BORRADOR"})

	if strings.Contains(xml, "BORRADOR") {
		t.Errorf("enabled:false must not draw the watermark, got:\n%s", xml)
	}
}

func TestPPTX_WatermarkAbsentDrawsNothing(t *testing.T) {
	xml, spy := buildPPTXWithWatermark(t, nil)

	// wrap="none" only ever appears on the watermark's own textbox (see
	// TestPPTX_WatermarkWordWrapDisabledAndCentered) — no other shape this
	// helper produces sets it, so its absence is the discriminating signal
	// that no watermark shape was added.
	if strings.Contains(xml, `wrap="none"`) {
		t.Errorf("nil Watermark must not add a watermark shape, got:\n%s", xml)
	}
	if spy.sawSubstring("watermark") {
		t.Errorf("nil Watermark must not warn about approximation, got: %v", spy.warnings)
	}
}

func TestPPTX_WatermarkWarnsAboutApproximation(t *testing.T) {
	_, spy := buildPPTXWithWatermark(t, &ast.WatermarkConfig{Enabled: true, Text: "X", Repeat: boolPtr(true)})

	if !spy.sawSubstring("opacidad aproximada") {
		t.Errorf("expected a warning about approximated opacity, got: %v", spy.warnings)
	}
	if !spy.sawSubstring("repeat") {
		t.Errorf("expected a warning naming 'repeat' as ignored, got: %v", spy.warnings)
	}
}

func boolPtr(v bool) *bool { return &v }
