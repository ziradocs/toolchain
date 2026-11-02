// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"bytes"
	"context"
	"image/png"
	"math"
	"testing"
)

// TestRenderSVGImageToPNG: el SVG sale a SVGPNGDeviceScale px por px CSS del
// tamaño pedido, con el color que dibuja, y un SVG que no decodifica falla
// con error en vez de dar un PNG vacío.
func TestRenderSVGImageToPNG(t *testing.T) {
	r := newTestChromiumRenderer(t)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="300" height="150" viewBox="0 0 300 150"><rect x="0" y="0" width="300" height="150" fill="#ff0000"/></svg>`)

	data, err := r.RenderSVGImageToPNG(context.Background(), svg, 300, 150)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 300*SVGPNGDeviceScale || b.Dy() != 150*SVGPNGDeviceScale {
		t.Errorf("PNG is %dx%d, want %dx%d", b.Dx(), b.Dy(), 300*SVGPNGDeviceScale, 150*SVGPNGDeviceScale)
	}
	cr, cg, cb, ca := img.At(img.Bounds().Dx()/2, img.Bounds().Dy()/2).RGBA()
	if cr>>8 < 240 || cg>>8 > 15 || cb>>8 > 15 || ca>>8 < 240 {
		t.Errorf("center pixel = %d,%d,%d,%d, want opaque red", cr>>8, cg>>8, cb>>8, ca>>8)
	}

	if _, err := r.RenderSVGImageToPNG(context.Background(), []byte("not an svg"), 100, 100); err == nil {
		t.Error("an undecodable SVG must fail")
	}
	if _, err := r.RenderSVGImageToPNG(context.Background(), svg, 0, 10); err == nil {
		t.Error("a zero size must fail")
	}
}

// TestRenderSVGImageToPNG_NoScriptsNoNetwork: un script dentro del SVG no
// corre (una <img> no ejecuta el SVG) y una referencia externa no impide la
// captura.
func TestRenderSVGImageToPNG_NoScriptsNoNetwork(t *testing.T) {
	r := newTestChromiumRenderer(t)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="40" height="40"><script>window.top.__renderError='script ran'</script><image xlink:href="https://example.invalid/x.png" width="40" height="40"/><rect width="40" height="40" fill="#00ff00"/></svg>`)
	if _, err := r.RenderSVGImageToPNG(context.Background(), svg, 40, 40); err != nil {
		t.Fatal(err)
	}
}

// TestRenderSVGImageToPNG_NonFiniteSize: NaN o Inf fallan antes de tocar el
// browser (no hace falta Chromium para este test).
func TestRenderSVGImageToPNG_NonFiniteSize(t *testing.T) {
	r := &ChromiumRenderer{}
	for _, wh := range [][2]float64{{math.NaN(), 10}, {10, math.Inf(1)}, {math.Inf(-1), 10}, {0, 0}} {
		if _, err := r.RenderSVGImageToPNG(context.Background(), []byte("<svg/>"), wh[0], wh[1]); err == nil {
			t.Errorf("size %v must fail", wh)
		}
	}
}
