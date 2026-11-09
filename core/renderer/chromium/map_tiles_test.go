// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// TestGenerateLeafletHTML_TileServer: la página de Leaflet pide los tiles
// solo a mapTileProxyOrigin, con o sin servidor propio, y serveMapTiles los
// traduce al servidor real: el propio si hay uno, si no OpenStreetMap o
// ArcGIS según el tipo de mapa.
func TestGenerateLeafletHTML_TileServer(t *testing.T) {
	r := &ChromiumRenderer{}
	base := renderer.MapConfig{Zoom: 5, MapType: "satellite"}
	config, err := base.WithTileServer("https://{s}.tiles.example.com:8443/{z}/{x}/{y}.png", "ua/1")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []renderer.MapConfig{base, config} {
		html := r.generateLeafletHTML(c, 800, 600)
		if !strings.Contains(html, "L.tileLayer('"+mapTileProxyTemplate+"'") || !strings.Contains(html, mapRenderCSP) {
			t.Error("map page does not request tiles from the proxy origin")
		}
		for _, host := range []string{"tile.openstreetmap.org", "arcgisonline.com", "tiles.example.com"} {
			if strings.Contains(html, host) {
				t.Errorf("page references %s directly", host)
			}
		}
	}
	cases := []struct {
		config renderer.MapConfig
		want   string
	}{
		{renderer.MapConfig{}, "https://b.tile.openstreetmap.org/3/4/5.png"},
		{base, "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/3/5/4"},
		{config, "https://b.tiles.example.com:8443/3/4/5.png"},
	}
	for _, c := range cases {
		got, ok := mapTileURL(mapTileProxyOrigin+"/b/3/4/5", realMapTileTemplate(c.config))
		if !ok || got != c.want {
			t.Errorf("tile url = %q (%v), want %q", got, ok, c.want)
		}
	}
	for _, bad := range []string{
		"https://map-tiles.offline.invalid/d/3/4/5",   // subdominio fuera de a-c
		"https://map-tiles.offline.invalid/a/3/8/5",   // x fuera del zoom
		"https://map-tiles.offline.invalid/a/-1/0/0",  // zoom negativo
		"https://map-tiles.offline.invalid/a/3/4/5/6", // segmento de más
		"https://map-tiles.offline.invalid/a/3/4/x",
		"https://example.com/a/3/4/5",
	} {
		if got, ok := mapTileURL(bad, realMapTileTemplate(config)); ok {
			t.Errorf("%q accepted as %q", bad, got)
		}
	}
}

// TestGenerateMapHash_TileServer: el mismo mapa con otro servidor de tiles
// no comparte archivo de caché, y sin servidor propio el hash no cambia.
func TestGenerateMapHash_TileServer(t *testing.T) {
	base := renderer.MapConfig{Zoom: 5}
	withTiles, err := base.WithTileServer("https://t.example.com/{z}/{x}/{y}.png", "")
	if err != nil {
		t.Fatal(err)
	}
	if generateMapHash(base, 800, 600) == generateMapHash(withTiles, 800, 600) {
		t.Fatal("tile server does not change the cache hash")
	}
	if got, want := generateMapHash(base, 800, 600), GenerateContentHash(`{"CenterLat":0,"CenterLng":0,"Zoom":5,"MapType":"","Markers":null,"Heatmap":false}|800x600`); got != want {
		t.Fatalf("hash without tile server changed: %s != %s", got, want)
	}
}
