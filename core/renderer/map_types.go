// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "strings"

// MapConfig configura un mapa Leaflet
type MapConfig struct {
	CenterLat float64
	CenterLng float64
	Zoom      int
	MapType   string // "default", "satellite", etc.
	Markers   []MapMarker
	Heatmap   bool
}

// MapMarker representa un marcador en el mapa
type MapMarker struct {
	Lat     float64
	Lng     float64
	Label   string
	Details string
	Color   string
	Value   float64
}

// validLeafletMarkerColors son los únicos nombres soportados por el set de
// iconos pointhi/leaflet-color-markers (marker-icon-2x-<color>.png), que es
// el sink real de sanitizeLeafletMarkerColor. El allowlist genérico
// SanitizeColor (hex/nombre CSS) no sirve aquí: el valor se usa como nombre
// de archivo de icono, no como color CSS, así que un hex o un nombre CSS
// válido pero no soportado (p. ej. "coral") produciría un icono roto (404)
// en vez de bloquear el ataque. Ver docs/SECURITY_AUDIT_2026-07.md, CR-6.
//
// Vive en renderer/ (no en renderer/chromium) porque nativeMapMarkerColor
// (native_map.go, backend de rasterización nativa) también lo consulta —
// ambos backends deben resolver los mismos nombres de color al mismo
// fallback (ver TestNativeMapMarkerColors_MatchesLeafletAllowlist).
var validLeafletMarkerColors = map[string]bool{
	"black": true, "blue": true, "gold": true, "green": true, "grey": true,
	"orange": true, "red": true, "violet": true, "yellow": true,
}

// SanitizeLeafletMarkerColor valida el color de un marcador de mapa contra
// el allowlist de iconos disponibles; cualquier valor no reconocido
// (incluido vacío) cae al fallback "blue". Exportada porque
// renderer/chromium (generateLeafletHTML) la llama desde otro paquete.
func SanitizeLeafletMarkerColor(color string) string {
	const fallback = "blue"
	color = strings.ToLower(strings.TrimSpace(color))
	if validLeafletMarkerColors[color] {
		return color
	}
	return fallback
}
