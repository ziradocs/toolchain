// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"bytes"
	"context"
	"image"
	_ "image/png"
	"strings"
	"testing"
	"time"

	sm "github.com/flopp/go-staticmaps"
)

func newTestMapConfig() MapConfig {
	return MapConfig{
		CenterLat: 40.7128,
		CenterLng: -74.0060,
		Zoom:      10,
		MapType:   "world",
		Markers: []MapMarker{
			{Lat: 40.7128, Lng: -74.0060, Label: "New York", Color: "red"},
			{Lat: 34.0522, Lng: -118.2437, Label: "Los Angeles", Color: "blue"},
		},
	}
}

// decodeMapPNGAndCheck aplica el golden laxo para rasterización nativa
// (issue #130): decode + dimensiones + ratio de píxeles OPACOS, no
// comparación byte-exacta. A diferencia de decodePNGAndCheck (charts, que
// arrancan con fondo blanco), los tests de mapa usan
// sm.NewTileProviderNone() para evitar red — sin tiles ni SetBackground, el
// fondo queda completamente transparente (alpha=0) y solo los marcadores
// quedan opacos, así que el canal alpha es la señal confiable de "se dibujó
// algo" (un chequeo de "no-blanco" sería inútil acá: un fondo transparente
// también es "no blanco").
func decodeMapPNGAndCheck(t *testing.T, data []byte, wantWidth, wantHeight int, wantOpaquePixels bool) {
	t.Helper()

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}
	if format != "png" {
		t.Errorf("format = %q, want \"png\"", format)
	}

	bounds := img.Bounds()
	if bounds.Dx() != wantWidth || bounds.Dy() != wantHeight {
		t.Errorf("dimensions = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), wantWidth, wantHeight)
	}

	opaque := 0
	total := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
			total++
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0 {
				opaque++
			}
		}
	}
	if total == 0 {
		t.Fatal("no pixels sampled")
	}

	hasOpaque := opaque > 0
	if hasOpaque != wantOpaquePixels {
		t.Errorf("has opaque pixels = %v (%d/%d sampled), want %v", hasOpaque, opaque, total, wantOpaquePixels)
	}
}

func TestRenderMapNativePNG_DrawsMarkers(t *testing.T) {
	config := newTestMapConfig()

	data, err := renderMapNativePNG(context.Background(), config, 640, 480, sm.NewTileProviderNone())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decodeMapPNGAndCheck(t, data, 640, 480, true)
}

// TestRenderMapNativePNG_NoMarkersStillRenders cubre el caso donde
// MapConfig no trae marcadores: determineZoomCenter (go-staticmaps) exige
// center+zoom explícitos O al menos un objeto para no devolver error — como
// renderMapNativePNG siempre llama SetCenter/SetZoom explícitamente (igual
// que generateLeafletHTML siempre pasa center/zoom a setView), no debería
// fallar aunque no haya marcadores.
func TestRenderMapNativePNG_NoMarkersStillRenders(t *testing.T) {
	config := MapConfig{CenterLat: 0, CenterLng: 0, Zoom: 0, MapType: "world"}

	data, err := renderMapNativePNG(context.Background(), config, 320, 240, sm.NewTileProviderNone())
	if err != nil {
		t.Fatalf("unexpected error with zero markers and zoom=0: %v", err)
	}

	decodeMapPNGAndCheck(t, data, 320, 240, false)
}

func TestNativeMapTileProvider(t *testing.T) {
	tests := []struct {
		mapType  string
		wantName string
	}{
		{"satellite", "arcgis-worldimagery"},
		{"world", "osm"},
		{"city", "osm"},
		{"country", "osm"},
		{"region", "osm"},
		{"", "osm"},
	}
	for _, tt := range tests {
		t.Run(tt.mapType, func(t *testing.T) {
			if got := nativeMapTileProvider(tt.mapType).Name; got != tt.wantName {
				t.Errorf("nativeMapTileProvider(%q).Name = %q, want %q", tt.mapType, got, tt.wantName)
			}
		})
	}
}

// TestNativeMapMarkerColor verifica que el mapeo de color nativo comparta el
// mismo allowlist que SanitizeLeafletMarkerColor (el sink real del pipeline
// chromedp): un nombre soportado da un color propio y distinto del
// fallback; cualquier valor no reconocido (incluido un hex, que el pipeline
// chromedp tampoco respeta) cae al mismo azul de fallback.
func TestNativeMapMarkerColor(t *testing.T) {
	blue := nativeMapMarkerColor("blue")
	red := nativeMapMarkerColor("red")
	if red == blue {
		t.Fatal("nativeMapMarkerColor(\"red\") should differ from nativeMapMarkerColor(\"blue\")")
	}

	tests := []string{"", "not-a-color", "#ff0000", "coral"}
	for _, name := range tests {
		if got := nativeMapMarkerColor(name); got != blue {
			t.Errorf("nativeMapMarkerColor(%q) = %v, want fallback blue %v", name, got, blue)
		}
	}
}

// TestNativeMapMarkerColors_MatchesLeafletAllowlist cubre un hallazgo de
// code-review: nativeMapMarkerColors (native_map.go) duplica a mano el mismo
// conjunto de 9 nombres que validLeafletMarkerColors (chromium_renderer.go,
// el allowlist real que valida SanitizeLeafletMarkerColor). Si alguien
// extiende uno sin el otro, el nombre nuevo cae silenciosamente al
// color.RGBA{} cero (negro transparente) en el backend nativo en vez de
// fallar — este test convierte ese drift en un fallo de CI en vez de un bug
// visual silencioso.
func TestNativeMapMarkerColors_MatchesLeafletAllowlist(t *testing.T) {
	for name := range validLeafletMarkerColors {
		if _, ok := nativeMapMarkerColors[name]; !ok {
			t.Errorf("validLeafletMarkerColors has %q but nativeMapMarkerColors does not", name)
		}
	}
	for name := range nativeMapMarkerColors {
		if !validLeafletMarkerColors[name] {
			t.Errorf("nativeMapMarkerColors has %q but validLeafletMarkerColors does not", name)
		}
	}
}

// TestRunWithTimeout_ReturnsPromptlyOnCallerCancellation es la prueba de
// cancelación que exige issue #134/G1d para el único caso de este archivo
// que NO puede cancelar de verdad la operación subyacente (go-staticmaps no
// acepta context.Context — ver el comentario de runWithTimeout): cancelar
// ctx debe hacer que la llamada retorne casi de inmediato, mucho antes del
// timeout fijo, aunque fn siga bloqueada en segundo plano hasta que termine
// por su cuenta.
func TestRunWithTimeout_ReturnsPromptlyOnCallerCancellation(t *testing.T) {
	fnStarted := make(chan struct{})
	fnDone := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		close(fnStarted)
		cancel()
	}()

	err := runWithTimeout(ctx, 5*time.Second, func() error {
		<-fnStarted
		// Simula una fn que sigue bloqueada más allá de la cancelación del
		// caller (p. ej. un fetch HTTP real, no abortable por go-staticmaps).
		time.Sleep(200 * time.Millisecond)
		close(fnDone)
		return nil
	})

	if err == nil {
		t.Fatal("expected an error when ctx is canceled before fn completes")
	}
	if !strings.Contains(err.Error(), "canceled") {
		t.Errorf("expected a cancellation error, got: %v", err)
	}

	select {
	case <-fnDone:
		t.Fatal("runWithTimeout returned only after fn finished — cancellation is not being honored promptly")
	default:
		// fn todavía corriendo en background: exactamente el trade-off
		// documentado (bounded wait, no true abort).
	}
}
