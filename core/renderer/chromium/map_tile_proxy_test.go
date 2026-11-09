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
	"time"

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

// TestRenderMapToPNG_WaitsForSlowTiles (Chromium real): un servidor que da
// un tile al instante y el resto tarde. Antes el plazo de la página
// (5 s a zoom bajo, menos que el timeout de un tile) daba el mapa por listo,
// se capturaba con un solo tile y err() era nil porque los demás seguían en
// vuelo, ni bien ni mal. Ahora se espera a que no quede ninguno en vuelo y
// el mapa sale completo. El plazo de la página se acorta para que la prueba
// no tarde; el caso es el mismo.
func TestRenderMapToPNG_WaitsForSlowTiles(t *testing.T) {
	r := newTestChromiumRenderer(t)
	prevFallback := mapPageFallback
	mapPageFallback = func(int) time.Duration { return 300 * time.Millisecond }
	t.Cleanup(func() { mapPageFallback = prevFallback })

	srv, seen := tileTestServer(t, func(p string) bool {
		if p != "/t/4/3/6.png" {
			time.Sleep(1500 * time.Millisecond)
		}
		return true
	})
	data, err := r.RenderMapToPNG(context.Background(), tileServerMap(t, srv.URL, ""), 400, 300)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	paths, _ := seen()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	// La esquina inferior derecha la ocupa el crédito de Leaflet.
	for _, p := range [][2]int{{3, 3}, {396, 3}, {3, 296}, {396, 150}, {200, 150}} {
		if r, g, b, _ := img.At(p[0], p[1]).RGBA(); r>>8 != 0x20 || g>>8 != 0xa0 || b>>8 != 0x40 {
			t.Errorf("pixel %v = %d,%d,%d: a slow tile was not in the capture (requests %d)", p, r>>8, g>>8, b>>8, len(paths))
		}
	}
}

// TestMapTileStats_InFlightCountAsFailed: lo que sigue en vuelo al capturar
// cuenta como fallido, así que un tile listo y cinco en camino no pasan.
func TestMapTileStats_InFlightCountAsFailed(t *testing.T) {
	var s mapTileStats
	s.started.Store(6)
	s.ok.Store(1)
	s.snapshot()
	if err := s.err(); err == nil || !strings.Contains(err.Error(), "only 1 of 6") {
		t.Fatalf("err = %v, want only 1 of 6", err)
	}
	var settled mapTileStats
	settled.started.Store(6)
	settled.ok.Store(5)
	settled.failed.Store(1)
	if err := settled.err(); err != nil {
		t.Fatalf("five of six tiles: %v", err)
	}
	if mapPageFallback(1) <= mapTileTimeout {
		t.Errorf("page fallback %v is not longer than a tile timeout %v", mapPageFallback(1), mapTileTimeout)
	}
}

// TestFetchMapTile_ContentTypeAndRedirects: un tile como
// application/octet-stream vale si es PNG o JPEG por su firma (como en el
// render nativo); una redirección al mismo host se sigue y a otro host no.
func TestFetchMapTile_ContentTypeAndRedirects(t *testing.T) {
	pngTile := []byte("\x89PNG\r\n\x1a\nrest")
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngTile)
	}))
	t.Cleanup(other.Close)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/octet-png":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(pngTile)
		case "/octet-jpeg":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xe0, 1, 2})
		case "/octet-html":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("<html>"))
		case "/same-host":
			http.Redirect(w, r, "/octet-png", http.StatusFound)
		case "/other-host":
			http.Redirect(w, r, other.URL+"/x.png", http.StatusFound)
		}
	}))
	t.Cleanup(srv.Close)
	client := newMapTileClient()
	cases := []struct {
		path, wantType string
		wantErr        bool
	}{
		{"/octet-png", "image/png", false},
		{"/octet-jpeg", "image/jpeg", false},
		{"/octet-html", "", true},
		{"/same-host", "image/png", false},
		{"/other-host", "", true},
	}
	for _, c := range cases {
		_, ctype, err := fetchMapTile(context.Background(), client, srv.URL+c.path, "ua")
		if (err != nil) != c.wantErr || ctype != c.wantType {
			t.Errorf("%s: type %q err %v", c.path, ctype, err)
		}
	}
}

// TestMapTileURL_Normalized y TestCheckMapSize: las coordenadas se vuelven a
// escribir como enteros y un mapa del respaldo tiene un tope de lado.
func TestMapTileURL_Normalized(t *testing.T) {
	got, ok := mapTileURL(mapTileProxyOrigin+"/a/04/+3/0005", "https://t.example.com/{z}/{x}/{y}.png")
	if !ok || got != "https://t.example.com/4/3/5.png" {
		t.Fatalf("url = %q (%v)", got, ok)
	}
	if checkMapSize(800, 600) != nil || checkMapSize(5000, 600) == nil || checkMapSize(0, 600) == nil {
		t.Fatal("checkMapSize does not enforce 1..4096")
	}
}
