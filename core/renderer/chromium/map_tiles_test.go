// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// TestGenerateLeafletHTML_TileServer: con un servidor de tiles propio, la
// página de Leaflet le pide los tiles a él y su CSP solo admite ese origen
// en lugar de OpenStreetMap y ArcGIS; sin él, la página es la de siempre.
func TestGenerateLeafletHTML_TileServer(t *testing.T) {
	r := &ChromiumRenderer{}
	base := renderer.MapConfig{Zoom: 5, MapType: "satellite"}
	if html := r.generateLeafletHTML(base, 800, 600); !strings.Contains(html, mapRenderCSP) || !strings.Contains(html, "server.arcgisonline.com/ArcGIS") {
		t.Fatal("default map page changed")
	}

	config, err := base.WithTileServer("https://{s}.tiles.example.com:8443/{z}/{x}/{y}.png", "ua/1")
	if err != nil {
		t.Fatal(err)
	}
	html := r.generateLeafletHTML(config, 800, 600)
	if !strings.Contains(html, "L.tileLayer('https://{s}.tiles.example.com:8443/{z}/{x}/{y}.png'") {
		t.Error("tile layer does not use the tile server")
	}
	if !strings.Contains(html, "img-src https://*.tiles.example.com:8443 https://raw.githubusercontent.com") {
		t.Errorf("CSP img-src not limited to the tile server origin: %s", mapCSPFor(config))
	}
	for _, host := range []string{"tile.openstreetmap.org", "server.arcgisonline.com"} {
		if strings.Contains(html, host) {
			t.Errorf("page still references %s", host)
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
