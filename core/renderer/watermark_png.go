// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import (
	"bytes"
	"fmt"
	"math"
	"sync"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/util"
)

// watermarkPNGMaxTiles bounds the tiled-draw grid RenderWatermarkPNG walks
// when Repeat is true: a pathological combination (tiny font_size on a
// large page) would otherwise produce an unbounded number of DrawString
// calls. 60 per axis (121x121 max) is far more than any real page needs to
// look fully tiled and keeps render time bounded regardless of input.
const watermarkPNGMaxTiles = 60

// watermarkFont parses the embedded Go regular TTF once — truetype.Parse
// is pure CPU work over a fixed byte slice, safe to share; only the
// font.Face derived from it per call (via truetype.NewFace) is NOT
// goroutine-safe, per gg.LoadFontFace's own doc comment, which is why a
// fresh Face is built inside RenderWatermarkPNG rather than cached here.
var watermarkFont = sync.OnceValue(func() *truetype.Font {
	f, err := truetype.Parse(goregular.TTF)
	if err != nil {
		// goregular.TTF is a fixed embedded asset from golang.org/x/image;
		// a parse failure here means that dependency itself is broken, not
		// anything a caller's input could trigger.
		panic("renderer: embedded gofont/goregular failed to parse: " + err.Error())
	}
	return f
})

// RenderWatermarkPNG rasterizes rw onto a transparent widthPx x heightPx
// canvas — the path doclang's DOCX backend uses (issue #179), since
// docxgo has no w:pict/VML API for a native vector text watermark, only
// AddImageFromBytesWithPosition(..., BehindText: true) for a floating
// image. Text is baked into the bitmap using the embedded Go Regular
// font, not the document's own theme font — docxgo exposes no path to a
// vector watermark, so this is a deliberate fidelity tradeoff, documented
// in llm-kit/reference/frontmatter.md, not an oversight.
//
// dpi is the resolution the caller rasterized widthPx/heightPx at (e.g.
// 150 for doclang's DOCX backend, ParseLengthInches(pageSize)*dpi). It
// must match: rw.FontSize is a physical unit (points/inches/cm/...), and
// truetype.Options.DPI is what converts that physical size into the
// correct number of PIXELS on THIS canvas — passing a mismatched or zero
// DPI (truetype defaults to 72) would rasterize text at the wrong
// fraction of the canvas, sized as if the canvas were a different
// resolution than it actually is.
func RenderWatermarkPNG(rw ResolvedWatermark, widthPx, heightPx, dpi int) ([]byte, error) {
	if widthPx <= 0 || heightPx <= 0 {
		return nil, fmt.Errorf("renderer: invalid watermark canvas size %dx%d", widthPx, heightPx)
	}
	if dpi <= 0 {
		return nil, fmt.Errorf("renderer: invalid watermark DPI %d", dpi)
	}

	points := 72.0
	if inches, err := util.ParseLengthInches(rw.FontSize); err == nil {
		points = inches * 72
	}
	face := truetype.NewFace(watermarkFont(), &truetype.Options{Size: points, DPI: float64(dpi)})

	dc := gg.NewContext(widthPx, heightPx)
	dc.SetFontFace(face)

	r, g, b, ok := a11y.ParseColor(rw.Color)
	if !ok {
		r, g, b = 0, 0, 0
	}
	alpha := int(math.Round(math.Min(1, math.Max(0, rw.Opacity)) * 255))
	dc.SetRGBA255(int(r), int(g), int(b), alpha)

	cx, cy := float64(widthPx)/2, float64(heightPx)/2
	angle := math.Mod(rw.Rotation, 360) * math.Pi / 180

	dc.Push()
	dc.RotateAbout(angle, cx, cy)

	if !rw.Repeat {
		dc.DrawStringAnchored(rw.Text, cx, cy, 0.5, 0.5)
	} else {
		textW, textH := dc.MeasureString(rw.Text)
		spacingX := textW * 1.8
		if spacingX <= 0 {
			spacingX = points * 4
		}
		spacingY := textH * 4
		if spacingY <= 0 {
			spacingY = points * 2
		}
		diag := math.Hypot(float64(widthPx), float64(heightPx))
		cols := min(int(diag/spacingX)+2, watermarkPNGMaxTiles)
		rows := min(int(diag/spacingY)+2, watermarkPNGMaxTiles)

		for row := -rows; row <= rows; row++ {
			for col := -cols; col <= cols; col++ {
				x := cx + float64(col)*spacingX
				y := cy + float64(row)*spacingY
				dc.DrawStringAnchored(rw.Text, x, y, 0.5, 0.5)
			}
		}
	}
	dc.Pop()

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("renderer: encoding watermark PNG: %w", err)
	}
	return buf.Bytes(), nil
}
