// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// tileTestServer sirve un tile verde de 256x256 a los pedidos que ok
// aprueba y 404 al resto, y anota ruta y User-Agent de cada pedido.
func tileTestServer(t *testing.T, ok func(path string) bool) (*httptest.Server, func() (paths, agents []string)) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			img.Set(x, y, color.RGBA{0x20, 0xa0, 0x40, 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var paths, agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		agents = append(agents, r.UserAgent())
		mu.Unlock()
		if !ok(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv, func() ([]string, []string) {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), paths...), append([]string(nil), agents...)
	}
}

func tileServerMap(t *testing.T, url, ua string) renderer.MapConfig {
	t.Helper()
	config, err := renderer.MapConfig{CenterLat: 19.43, CenterLng: -99.13, Zoom: 4}.WithTileServer(url+"/t/{z}/{x}/{y}.png", ua)
	if err != nil {
		t.Fatal(err)
	}
	return config
}

// TestRenderMapToPNG_LoopbackTileServer (Chromium real): el respaldo de
// Leaflet carga los tiles de un servidor en 127.0.0.1, con el User-Agent
// configurado, y el PNG los muestra. Antes Chromium no le hacía ni un pedido
// (Local Network Access: la página es about:blank, CORS
// "InsecureLocalNetwork") y salía un mapa gris.
func TestRenderMapToPNG_LoopbackTileServer(t *testing.T) {
	r := newTestChromiumRenderer(t)
	srv, seen := tileTestServer(t, func(string) bool { return true })
	data, err := r.RenderMapToPNG(context.Background(), tileServerMap(t, srv.URL, "doclang-test/2"), 400, 300)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	paths, agents := seen()
	if len(paths) == 0 {
		t.Fatal("the loopback tile server got no request")
	}
	for i, p := range paths {
		if !strings.HasPrefix(p, "/t/4/") || agents[i] != "doclang-test/2" {
			t.Errorf("request %q with user agent %q", p, agents[i])
		}
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if r, g, b, _ := img.At(5, 5).RGBA(); r>>8 != 0x20 || g>>8 != 0xa0 || b>>8 != 0x40 {
		t.Errorf("corner pixel %d,%d,%d is not the served tile", r>>8, g>>8, b>>8)
	}
}

// TestRenderMapToPNG_FailsWithoutTiles (Chromium real): si no llegó ningún
// tile, o fallaron más de la mitad, el respaldo devuelve error y el mapa
// queda como no disponible. Antes un tile que fallaba contaba como cargado
// y salía un PNG gris sin error. Con uno solo perdido, el mapa sale.
func TestRenderMapToPNG_FailsWithoutTiles(t *testing.T) {
	r := newTestChromiumRenderer(t)
	cases := []struct {
		name    string
		ok      func(path string) bool
		wantErr string
	}{
		{"all 404", func(string) bool { return false }, "no map tile could be downloaded"},
		{"one of many", func(p string) bool { return p == "/t/4/3/6.png" }, "map tiles could be downloaded"},
		{"one missing", func(p string) bool { return p != "/t/4/3/6.png" }, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, seen := tileTestServer(t, c.ok)
			_, err := r.RenderMapToPNG(context.Background(), tileServerMap(t, srv.URL, ""), 400, 300)
			paths, _ := seen()
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("render with one tile missing failed: %v (requests %v)", err, paths)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want %q (requests %v)", err, c.wantErr, paths)
			}
		})
	}
}
