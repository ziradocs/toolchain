// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"image/color"
	"image/png"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func TestResolveWatermark(t *testing.T) {
	trueVal := true
	falseVal := false
	half := 0.5
	rot := 30.0

	tests := []struct {
		name      string
		cfg       *ast.WatermarkConfig
		variables map[string]interface{}
		wantOK    bool
		want      ResolvedWatermark
	}{
		{
			name:   "nil config",
			cfg:    nil,
			wantOK: false,
		},
		{
			name:   "disabled",
			cfg:    &ast.WatermarkConfig{Enabled: false, Text: "DRAFT"},
			wantOK: false,
		},
		{
			name:   "empty text",
			cfg:    &ast.WatermarkConfig{Enabled: true, Text: ""},
			wantOK: false,
		},
		{
			name:   "blank text",
			cfg:    &ast.WatermarkConfig{Enabled: true, Text: "   "},
			wantOK: false,
		},
		{
			name:   "defaults fill in when unset",
			cfg:    &ast.WatermarkConfig{Enabled: true, Text: "DRAFT"},
			wantOK: true,
			want: ResolvedWatermark{
				Text: "DRAFT", Color: DefaultWatermarkColor, Opacity: DefaultWatermarkOpacity,
				Rotation: DefaultWatermarkRotation, FontSize: DefaultWatermarkFontSize, Repeat: true,
			},
		},
		{
			name: "explicit values override defaults",
			cfg: &ast.WatermarkConfig{
				Enabled: true, Text: "CONFIDENTIAL", Color: "#ff0000",
				Opacity: &half, Rotation: &rot, FontSize: "24pt", Repeat: &falseVal,
			},
			wantOK: true,
			want: ResolvedWatermark{
				Text: "CONFIDENTIAL", Color: "#ff0000", Opacity: 0.5,
				Rotation: 30, FontSize: "24pt", Repeat: false,
			},
		},
		{
			name:      "text substitutes {{variable}}",
			cfg:       &ast.WatermarkConfig{Enabled: true, Text: "{{title}} DRAFT"},
			variables: map[string]interface{}{"title": "Q3 Report"},
			wantOK:    true,
			want: ResolvedWatermark{
				Text: "Q3 Report DRAFT", Color: DefaultWatermarkColor, Opacity: DefaultWatermarkOpacity,
				Rotation: DefaultWatermarkRotation, FontSize: DefaultWatermarkFontSize, Repeat: true,
			},
		},
		{
			name: "out-of-range opacity clamps defensively",
			cfg: &ast.WatermarkConfig{
				Enabled: true, Text: "DRAFT", Opacity: ptrFloat(5),
			},
			wantOK: true,
			want: ResolvedWatermark{
				Text: "DRAFT", Color: DefaultWatermarkColor, Opacity: 1,
				Rotation: DefaultWatermarkRotation, FontSize: DefaultWatermarkFontSize, Repeat: true,
			},
		},
		{
			name: "large rotation normalizes defensively",
			cfg: &ast.WatermarkConfig{
				Enabled: true, Text: "DRAFT", Rotation: ptrFloat(730),
			},
			wantOK: true,
			want: ResolvedWatermark{
				Text: "DRAFT", Color: DefaultWatermarkColor, Opacity: DefaultWatermarkOpacity,
				Rotation: 10, FontSize: DefaultWatermarkFontSize, Repeat: true,
			},
		},
		{
			name:   "unused pointer helpers keep vet happy",
			cfg:    &ast.WatermarkConfig{Enabled: true, Text: "X", Repeat: &trueVal},
			wantOK: true,
			want: ResolvedWatermark{
				Text: "X", Color: DefaultWatermarkColor, Opacity: DefaultWatermarkOpacity,
				Rotation: DefaultWatermarkRotation, FontSize: DefaultWatermarkFontSize, Repeat: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ResolveWatermark(tt.cfg, tt.variables)
			if ok != tt.wantOK {
				t.Fatalf("ResolveWatermark() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Errorf("ResolveWatermark() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func ptrFloat(v float64) *float64 { return &v }

func TestWatermarkHTML(t *testing.T) {
	rw := ResolvedWatermark{
		Text: "DRAFT & CONFIDENTIAL", Color: "#000000", Opacity: 0.08,
		Rotation: -45, FontSize: "72pt", Repeat: true,
	}
	html := WatermarkHTML(rw, "slidelang-watermark")

	if !strings.Contains(html, `class="slidelang-watermark"`) {
		t.Errorf("missing outer class, got: %s", html)
	}
	if !strings.Contains(html, `class="slidelang-watermark-rotator"`) {
		t.Errorf("missing rotator class, got: %s", html)
	}
	if !strings.Contains(html, "rotate(-45deg)") {
		t.Errorf("missing rotation transform, got: %s", html)
	}
	if !strings.Contains(html, "DRAFT &amp; CONFIDENTIAL") {
		t.Errorf("text should be HTML-escaped, got: %s", html)
	}
	if strings.Contains(html, "DRAFT & CONFIDENTIAL") {
		t.Errorf("raw unescaped text must not appear, got: %s", html)
	}
	if got := strings.Count(html, "<span"); got != watermarkRepeatCount {
		t.Errorf("expected %d repeated spans, got %d", watermarkRepeatCount, got)
	}
	if !strings.Contains(html, `aria-hidden="true"`) {
		t.Errorf("watermark must be aria-hidden (decorative, not content), got: %s", html)
	}
}

func TestWatermarkHTML_NoRepeatDrawsOnce(t *testing.T) {
	rw := ResolvedWatermark{Text: "DRAFT", Color: "#000000", Opacity: 0.08, Rotation: 0, FontSize: "72pt", Repeat: false}
	html := WatermarkHTML(rw, "doclang-watermark")
	if got := strings.Count(html, "<span"); got != 1 {
		t.Errorf("expected exactly 1 span when Repeat is false, got %d", got)
	}
}

func TestWatermarkHTML_RejectsUnsafeColor(t *testing.T) {
	// A color/font_size containing characters sanitizeStyleValue rejects
	// (";", quotes) must fall back to the default, never reach the
	// attribute unescaped — this is the guard the doc comment on
	// WatermarkHTML calls out explicitly.
	rw := ResolvedWatermark{Text: "X", Color: `red; } body { display:none`, Opacity: 0.5, Rotation: 0, FontSize: "72pt", Repeat: false}
	html := WatermarkHTML(rw, "watermark")
	if strings.Contains(html, "display:none") {
		t.Errorf("unsafe color value leaked into output: %s", html)
	}
	if !strings.Contains(html, DefaultWatermarkColor) {
		t.Errorf("expected fallback to DefaultWatermarkColor, got: %s", html)
	}
}

func TestBlendOverOpaque(t *testing.T) {
	tests := []struct {
		name    string
		fg, bg  color.RGBA
		opacity float64
		want    color.RGBA
	}{
		{
			name: "fully opaque fg wins entirely", fg: color.RGBA{R: 255, A: 255}, bg: color.RGBA{R: 0, G: 0, B: 0, A: 255},
			opacity: 1, want: color.RGBA{R: 255, G: 0, B: 0, A: 255},
		},
		{
			name: "zero opacity leaves bg untouched", fg: color.RGBA{R: 255, A: 255}, bg: color.RGBA{R: 0, G: 0, B: 0, A: 255},
			opacity: 0, want: color.RGBA{R: 0, G: 0, B: 0, A: 255},
		},
		{
			name: "half opacity averages black over white", fg: color.RGBA{R: 0, G: 0, B: 0, A: 255}, bg: color.RGBA{R: 255, G: 255, B: 255, A: 255},
			opacity: 0.5, want: color.RGBA{R: 128, G: 128, B: 128, A: 255},
		},
		{
			name: "out-of-range opacity clamps", fg: color.RGBA{R: 255, A: 255}, bg: color.RGBA{R: 0, G: 0, B: 0, A: 255},
			opacity: 2.0, want: color.RGBA{R: 255, G: 0, B: 0, A: 255},
		},
		{
			name: "result is always fully opaque", fg: color.RGBA{R: 10, G: 20, B: 30, A: 0}, bg: color.RGBA{R: 200, G: 200, B: 200, A: 128},
			opacity: 0.5, want: color.RGBA{R: 105, G: 110, B: 115, A: 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BlendOverOpaque(tt.fg, tt.bg, tt.opacity)
			if got != tt.want {
				t.Errorf("BlendOverOpaque(%+v, %+v, %v) = %+v, want %+v", tt.fg, tt.bg, tt.opacity, got, tt.want)
			}
		})
	}
}

func TestRenderWatermarkPNG(t *testing.T) {
	rw := ResolvedWatermark{Text: "DRAFT", Color: "#ff0000", Opacity: 0.3, Rotation: -45, FontSize: "24pt", Repeat: true}
	data, err := RenderWatermarkPNG(rw, 400, 300, 150)
	if err != nil {
		t.Fatalf("RenderWatermarkPNG() error = %v", err)
	}
	img, err := png.Decode(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 400 || bounds.Dy() != 300 {
		t.Errorf("PNG dimensions = %dx%d, want 400x300", bounds.Dx(), bounds.Dy())
	}

	// At least one pixel must carry a partial alpha channel (the
	// translucent watermark text) — a fully-opaque or fully-transparent
	// image would mean nothing got drawn, or drew at the wrong opacity.
	foundPartialAlpha := false
	for y := bounds.Min.Y; y < bounds.Max.Y && !foundPartialAlpha; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0 && a < 0xffff {
				foundPartialAlpha = true
				break
			}
		}
	}
	if !foundPartialAlpha {
		t.Error("expected at least one pixel with partial alpha (translucent watermark text), found none")
	}
}

func TestRenderWatermarkPNG_InvalidSize(t *testing.T) {
	rw := ResolvedWatermark{Text: "DRAFT", Color: "#000000", Opacity: 0.5, FontSize: "24pt"}
	if _, err := RenderWatermarkPNG(rw, 0, 100, 150); err == nil {
		t.Error("expected an error for a zero-width canvas, got nil")
	}
}

func TestRenderWatermarkPNG_InvalidDPI(t *testing.T) {
	rw := ResolvedWatermark{Text: "DRAFT", Color: "#000000", Opacity: 0.5, FontSize: "24pt"}
	if _, err := RenderWatermarkPNG(rw, 400, 300, 0); err == nil {
		t.Error("expected an error for a zero DPI, got nil")
	}
}

// TestRenderWatermarkPNG_DPIScalesGlyphSize verifies dpi actually governs
// how large the rasterized text is, not just the canvas dimensions: a
// fixed FontSize rendered at a higher DPI on the same canvas must occupy
// visibly more of it. Without wiring dpi into truetype.Options (it
// defaults to 72), a "72pt" glyph would render at the same pixel size
// whether the canvas were built for 72 or 150 DPI — a mismatch between
// the physical size the caller intended and what actually lands on the
// page.
func TestRenderWatermarkPNG_DPIScalesGlyphSize(t *testing.T) {
	rw := ResolvedWatermark{Text: "M", Color: "#000000", Opacity: 1.0, FontSize: "72pt", Repeat: false}

	countOpaquePixels := func(dpi int) int {
		data, err := RenderWatermarkPNG(rw, 300, 300, dpi)
		if err != nil {
			t.Fatalf("RenderWatermarkPNG(dpi=%d) error = %v", dpi, err)
		}
		img, err := png.Decode(strings.NewReader(string(data)))
		if err != nil {
			t.Fatalf("output is not a valid PNG: %v", err)
		}
		bounds := img.Bounds()
		count := 0
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
					count++
				}
			}
		}
		return count
	}

	low := countOpaquePixels(72)
	high := countOpaquePixels(150)
	if high <= low {
		t.Errorf("expected more opaque pixels at 150 DPI than at 72 DPI for the same FontSize, got %d (72 DPI) vs %d (150 DPI)", low, high)
	}
}
