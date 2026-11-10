// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"embed"
	"encoding/base64"
	"strings"
)

// Leaflet 1.9.4 (leaflet.js y leaflet.css, los mismos builds que carga el
// HTML en modo browser: renderer.LeafletCDNScriptTag/LeafletCDNCSSTag,
// TestEmbeddedLeaflet_MatchesCDNIntegrity exige que su sha384 sea el hash
// SRI de esos tags), la sombra de marcador de Leaflet y los nueve íconos de
// pointhi/leaflet-color-markers, todos sin modificar y BSD-2-Clause. Origen
// exacto y sha256 en third_party/leaflet/SOURCE y
// third_party/leaflet-color-markers/SOURCE; licencias en sus LICENSE y en
// NOTICE.
//
// Antes la página del respaldo de mapas (generateLeafletHTML, cuando el
// render nativo falla) cargaba Leaflet de unpkg.com, los íconos de
// raw.githubusercontent.com y la sombra de cdnjs.cloudflare.com en cada
// build: tres terceros que no tienen nada que ver con el servidor de tiles
// (issue #472). Ahora la página solo pide tiles.
//
//go:embed third_party/leaflet/leaflet.js
var embeddedLeafletJS string

//go:embed third_party/leaflet/leaflet.css
var embeddedLeafletCSS string

//go:embed third_party/leaflet/images/marker-shadow.png
var embeddedLeafletShadowPNG []byte

//go:embed third_party/leaflet-color-markers/img/*.png
var embeddedLeafletMarkers embed.FS

// leafletAssetTags inyecta la hoja de estilos y el script inline.
var leafletAssetTags = "<style>" + strings.ReplaceAll(embeddedLeafletCSS, "</style", `<\/style`) + "</style>\n    " +
	inlineScriptTag(embeddedLeafletJS)

// leafletShadowDataURI es la sombra de los marcadores como data: URI.
var leafletShadowDataURI = pngDataURI(embeddedLeafletShadowPNG)

// leafletMarkerDataURI es el ícono de un color ya validado por
// renderer.SanitizeLeafletMarkerColor, como data: URI. Un color sin archivo
// (no debería pasar: el allowlist y los archivos son los mismos nueve, ver
// TestEmbeddedLeafletMarkers_CoverAllowlist) cae al azul, el fallback del
// allowlist.
func leafletMarkerDataURI(color string) string {
	data, err := embeddedLeafletMarkers.ReadFile("third_party/leaflet-color-markers/img/marker-icon-2x-" + color + ".png")
	if err != nil {
		data, _ = embeddedLeafletMarkers.ReadFile("third_party/leaflet-color-markers/img/marker-icon-2x-blue.png")
	}
	return pngDataURI(data)
}

func pngDataURI(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}
