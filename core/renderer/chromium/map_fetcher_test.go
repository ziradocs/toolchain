// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import (
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// TestGenerateMapHash_IncludesDimensions cubre el hallazgo aplicado por
// analogía desde PR #163 (chart_fetcher.go/cacheKeyInput): width/height
// afectan los bytes del PNG en ambos backends (el screenshot de chromedp y
// el ctx.SetSize del render nativo), así que dos llamadas con el mismo
// renderer.MapConfig pero distinto tamaño deben producir hashes distintos — si no,
// FetchAndSave serviría el PNG del tamaño equivocado desde el cache en
// disco (os.Stat) sin volver a renderizar.
func newTestMapConfig() renderer.MapConfig {
	return renderer.MapConfig{
		CenterLat: 40.7128,
		CenterLng: -74.0060,
		Zoom:      10,
		MapType:   "world",
		Markers: []renderer.MapMarker{
			{Lat: 40.7128, Lng: -74.0060, Label: "New York", Color: "red"},
			{Lat: 34.0522, Lng: -118.2437, Label: "Los Angeles", Color: "blue"},
		},
	}
}

func TestGenerateMapHash_IncludesDimensions(t *testing.T) {
	config := newTestMapConfig()

	h1 := generateMapHash(config, 800, 600)
	h2 := generateMapHash(config, 400, 300)

	if h1 == h2 {
		t.Fatal("generateMapHash produced the same hash for different width/height")
	}

	// Config distinto, mismas dimensiones: también debe diferir.
	other := config
	other.Zoom = 5
	h3 := generateMapHash(other, 800, 600)
	if h3 == h1 {
		t.Fatal("generateMapHash produced the same hash for different renderer.MapConfig")
	}
}
