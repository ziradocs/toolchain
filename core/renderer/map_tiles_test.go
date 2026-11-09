// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package renderer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestWithTileServer_Validation(t *testing.T) {
	ok := []string{
		"https://tiles.example.com/{z}/{x}/{y}.png",
		"http://localhost:8080/osm/{z}/{x}/{y}.png",
		"https://{s}.tiles.example.com/{z}/{x}/{y}.png?key=abc",
	}
	for _, tmpl := range ok {
		if _, err := (MapConfig{}).WithTileServer(tmpl, "ua/1"); err != nil {
			t.Errorf("%q rejected: %v", tmpl, err)
		}
	}
	bad := []string{
		"tiles.example.com/{z}/{x}/{y}.png",              // sin esquema
		"file:///tiles/{z}/{x}/{y}.png",                  // esquema no http(s)
		"javascript:alert(1)//{z}/{x}/{y}",               // esquema peligroso
		"https://tiles.example.com/{z}/{x}.png",          // falta {y}
		"https://tiles.example.com/{z}/{x}/{y}/{r}.png",  // marcador desconocido
		"https://tiles.example.com/{z}/{x}/{y}.png'+x+'", // rompería el string JS
		" https://tiles.example.com/{z}/{x}/{y}.png",
		"https:///{z}/{x}/{y}.png", // sin host
	}
	for _, tmpl := range bad {
		if _, err := (MapConfig{}).WithTileServer(tmpl, ""); err == nil {
			t.Errorf("%q accepted", tmpl)
		}
	}
	if _, err := (MapConfig{}).WithTileServer("https://t.example.com/{z}/{x}/{y}.png", "a\r\nX-Evil: 1"); err == nil {
		t.Error("user agent with CRLF accepted")
	}
	if _, err := (MapConfig{}).WithTileServer("", "ua/1"); err == nil {
		t.Error("user agent without template accepted")
	}
	if c, err := (MapConfig{}).WithTileServer("", ""); err != nil || c.tiles != (mapTileServer{}) {
		t.Errorf("empty template: %v %+v", err, c.tiles)
	}
}

func TestGoStaticMapsPattern(t *testing.T) {
	got := goStaticMapsPattern("https://{s}.t.example.com/a%20b/{z}/{x}/{y}.png")
	if want := "https://%[1]s.t.example.com/a%%20b/%[2]d/%[3]d/%[4]d.png"; got != want {
		t.Fatalf("pattern = %q, want %q", got, want)
	}
	provider := nativeMapTileProviderFor(MapConfig{tiles: mapTileServer{urlTemplate: "https://{s}.t.example.com/{z}/{x}/{y}.png"}})
	if u := fmt.Sprintf(provider.URLPattern, "b", 3, 4, 5, ""); u != "https://b.t.example.com/3/4/5.png" {
		t.Fatalf("url = %q", u)
	}
	if len(provider.Shards) != 3 {
		t.Fatalf("shards = %v", provider.Shards)
	}
}

// tileServer sirve un tile PNG gris y anota la ruta y el User-Agent de cada
// pedido.
func tileServer(t *testing.T) (*httptest.Server, func() (paths, agents []string)) {
	t.Helper()
	var mu sync.Mutex
	var paths, agents []string
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			img.Set(x, y, color.RGBA{0x80, 0x80, 0x80, 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		agents = append(agents, r.UserAgent())
		mu.Unlock()
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

// TestRenderMapNativePNG_UsesTileServer: con un servidor propio, el render
// nativo (también de un mapa satelital) le pide los tiles a él, con el
// User-Agent configurado, y el mapa sale con los tiles servidos.
func TestRenderMapNativePNG_UsesTileServer(t *testing.T) {
	srv, seen := tileServer(t)
	config := MapConfig{CenterLat: 19.43, CenterLng: -99.13, Zoom: 5, MapType: "satellite"}
	config, err := config.WithTileServer(srv.URL+"/tiles/{z}/{x}/{y}.png", "doclang-test/1.0")
	if err != nil {
		t.Fatal(err)
	}
	data, err := RenderMapNativePNG(context.Background(), config, 320, 240)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	paths, agents := seen()
	if len(paths) < 2 { // el sondeo 0/0/0 y al menos un tile del mapa
		t.Fatalf("tile server got %d requests: %v", len(paths), paths)
	}
	for i, p := range paths {
		if !strings.HasPrefix(p, "/tiles/") {
			t.Errorf("unexpected path %q", p)
		}
		if agents[i] != "doclang-test/1.0" {
			t.Errorf("request %s with user agent %q", p, agents[i])
		}
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if r, g, b, _ := img.At(5, 5).RGBA(); r>>8 != 0x80 || g>>8 != 0x80 || b>>8 != 0x80 {
		t.Errorf("corner pixel = %d,%d,%d, want the served gray tile", r>>8, g>>8, b>>8)
	}
}

// fakeMapFetcher anota el MapConfig que recibe.
type fakeMapFetcher struct{ got []MapConfig }

func (f *fakeMapFetcher) FetchAndSave(_ context.Context, c MapConfig, _ string, _, _ int) (string, error) {
	f.got = append(f.got, c)
	return "maps/x.png", nil
}

func (f *fakeMapFetcher) FetchInline(_ context.Context, c MapConfig, _, _ int) ([]byte, error) {
	f.got = append(f.got, c)
	return []byte("PNG"), nil
}

func (f *fakeMapFetcher) GetImageFormat() string { return "png" }

// TestRenderContext_SetMapTileServer: el servidor del contexto llega al
// MapConfig que recibe el fetcher en los dos modos offline; un template
// inválido es un error y no cambia el contexto.
func TestRenderContext_SetMapTileServer(t *testing.T) {
	const tmpl = "https://tiles.example.com/{z}/{x}/{y}.png"
	for _, mode := range []string{"offline-inline", "offline-assets"} {
		ctx := NewDefaultRenderContext()
		ctx.MapMode = mode
		fetcher := &fakeMapFetcher{}
		ctx.MapFetcher = fetcher
		if err := ctx.SetMapTileServer("ftp://bad/{z}/{x}/{y}", ""); err == nil {
			t.Fatal("invalid template accepted")
		}
		if err := ctx.SetMapTileServer(tmpl, "ua/2"); err != nil {
			t.Fatal(err)
		}
		RenderElementToHTML(ast.NewMapElement(diagnostics.NewPosition(1, 1), "default"), nil, ctx)
		if len(fetcher.got) != 1 {
			t.Fatalf("%s: fetcher called %d times", mode, len(fetcher.got))
		}
		if got, ua := fetcher.got[0].TileServer(); got != tmpl || ua != "ua/2" {
			t.Errorf("%s: fetcher got tile server %q/%q", mode, got, ua)
		}
	}
	// Sin SetMapTileServer, el MapConfig no trae servidor propio.
	ctx := NewDefaultRenderContext()
	ctx.MapMode = "offline-inline"
	fetcher := &fakeMapFetcher{}
	ctx.MapFetcher = fetcher
	RenderElementToHTML(ast.NewMapElement(diagnostics.NewPosition(1, 1), "default"), nil, ctx)
	if got, _ := fetcher.got[0].TileServer(); got != "" {
		t.Errorf("default context carried tile server %q", got)
	}
}
