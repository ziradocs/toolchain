// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// SVGPNGDeviceScale son los px del PNG por cada px CSS en RenderSVGImageToPNG:
// una imagen que se muestra a su tamaño natural queda a 192 DPI.
const SVGPNGDeviceScale = 2

// svgImageMaxSidePx acota el lado de la captura (px CSS). El caller pide el
// tamaño al que la imagen va en la página, que nunca pasa de una caja de
// texto; esto solo frena un tamaño absurdo que llegue de un SVG mal formado.
const svgImageMaxSidePx = 4096

// svgImageCSP deja cargar solo el data: URI de la imagen y los handlers
// inline de esta página. Un SVG que se muestra como <img> no ejecuta scripts
// ni pide recursos externos, así que el contenido del archivo (no confiable)
// no puede salir de la página.
const svgImageCSP = "default-src 'none'; img-src data:; script-src 'unsafe-inline'; style-src 'unsafe-inline'"

// RenderSVGImageToPNG rasteriza un archivo SVG a PNG de widthPx x heightPx px
// CSS, a SVGPNGDeviceScale px por px CSS y con fondo transparente. Es la ruta
// de las imágenes SVG en DOCX: docxgo no escribe la extensión svgBlip de Word,
// así que el SVG entra como PNG. El SVG se carga como <img> desde un data:
// URI, no como <svg> en el DOM, por la razón de svgImageCSP.
func (r *ChromiumRenderer) RenderSVGImageToPNG(ctx context.Context, svg []byte, widthPx, heightPx float64) ([]byte, error) {
	if widthPx <= 0 || heightPx <= 0 {
		return nil, fmt.Errorf("svg image rendering failed: invalid size %.1fx%.1f", widthPx, heightPx)
	}
	if s := math.Max(widthPx, heightPx); s > svgImageMaxSidePx {
		widthPx *= svgImageMaxSidePx / s
		heightPx *= svgImageMaxSidePx / s
	}
	w, h := int64(math.Ceil(widthPx)), int64(math.Ceil(heightPx))

	r.logger.Info("IMAGE", "Rendering SVG image to PNG (%dx%d px)...", w, h)
	if err := r.ensureBrowser(ctx); err != nil {
		return nil, fmt.Errorf("svg image rendering failed: %w", err)
	}

	html := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8">`+
		`<meta http-equiv="Content-Security-Policy" content="%s">`+
		`<style>html,body{margin:0;padding:0;background:transparent;overflow:hidden}img{display:block;width:%dpx;height:%dpx}</style>`+
		`</head><body><img id="svg" alt="" onload="window.__renderReady=true" onerror="window.__renderError='the SVG image could not be decoded'" src="data:image/svg+xml;base64,%s"></body></html>`,
		svgImageCSP, w, h, base64.StdEncoding.EncodeToString(svg))

	runCtx, cancel := r.newCaptureContext(ctx, captureTimeout)
	defer cancel()

	var pngData []byte
	err := chromedp.Run(runCtx,
		emulation.SetDeviceMetricsOverride(w, h, SVGPNGDeviceScale, false),
		emulation.SetDefaultBackgroundColorOverride().WithColor(&cdp.RGBA{R: 0, G: 0, B: 0, A: 0}),
		navigateAndSetContent(html),
		waitRenderState(renderStateJS),
		bringTabToFront(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, err := page.CaptureScreenshot().
				WithFormat(page.CaptureScreenshotFormatPng).
				WithClip(&page.Viewport{X: 0, Y: 0, Width: float64(w), Height: float64(h), Scale: 1}).
				Do(ctx)
			if err != nil {
				return err
			}
			pngData = buf
			return nil
		}),
	)
	if err != nil {
		return nil, captureError("svg image", captureTimeout, ctx, err)
	}

	r.logger.Info("IMAGE", "✅ SVG image rendered (%.2f KB)", float64(len(pngData))/1024)
	return pngData, nil
}
