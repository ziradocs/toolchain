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
	"time"

	sm "github.com/flopp/go-staticmaps"
	"github.com/golang/geo/s2"
)

// native_map.go implementa la rasterización nativa de mapas (issue #130): en
// vez de levantar Chromium + Leaflet (generateLeafletHTML) para tomar un
// screenshot, se dibuja directamente con go-staticmaps (MIT), que hace su
// propio fetch de tiles OSM/ArcGIS por HTTP simple sin necesitar un browser.
// A diferencia de charts (que tienen IsJSONMode/Options sin equivalente
// nativo), MapConfig no tiene ningún campo sin mapeo aquí — Heatmap tampoco
// lo implementa hoy el pipeline chromedp (generateLeafletHTML nunca lo lee),
// así que no hay una condición de "no soportado, cae a Chromium por
// features": el render nativo siempre se intenta, y el único fallback en
// map_fetcher.go es un error real de red/render.

// nativeMapTileProvider elige el mismo proveedor de tiles que
// generateLeafletHTML: OpenStreetMap por defecto, ArcGIS World Imagery para
// MapType=="satellite" — únicas dos opciones fijas, nunca una URL controlada
// por el autor del documento (mismo par ya evaluado por SSRF en el análisis
// de pre-flight de este issue: docs/SECURITY_AUDIT_2026-07.md).
func nativeMapTileProvider(mapType string) *sm.TileProvider {
	if mapType == "satellite" {
		return sm.NewTileProviderArcgisWorldImagery()
	}
	return sm.NewTileProviderOpenStreetMaps()
}

// nativeMapMarkerColors mapea el mismo allowlist de 9 nombres que ya valida
// SanitizeLeafletMarkerColor (el sink real en el pipeline chromedp) a un
// color.RGBA sólido. go-staticmaps dibuja un pin relleno de un solo color en
// vez de un ícono PNG, así que no hay un archivo "marker-icon-2x-<color>"
// que reusar — pero mantener el mismo allowlist en vez de aceptar hex/CSS
// arbitrario evita que el mismo marker.Color produzca un color distinto
// según qué backend termine dibujándolo.
var nativeMapMarkerColors = map[string]color.RGBA{
	"black":  {R: 0x21, G: 0x21, B: 0x21, A: 0xff},
	"blue":   {R: 0x21, G: 0x96, B: 0xf3, A: 0xff},
	"gold":   {R: 0xff, G: 0xd7, B: 0x00, A: 0xff},
	"green":  {R: 0x4c, G: 0xaf, B: 0x50, A: 0xff},
	"grey":   {R: 0x9e, G: 0x9e, B: 0x9e, A: 0xff},
	"orange": {R: 0xff, G: 0x98, B: 0x00, A: 0xff},
	"red":    {R: 0xf4, G: 0x43, B: 0x36, A: 0xff},
	"violet": {R: 0x9c, G: 0x27, B: 0xb0, A: 0xff},
	"yellow": {R: 0xff, G: 0xeb, B: 0x3b, A: 0xff},
}

// nativeMapMarkerSize replica el tamaño fijo que usa generateLeafletHTML
// (iconSize [25, 41] para todos los marcadores, sin importar
// marker.Size) — marker.Size se ignora acá por la misma razón.
const nativeMapMarkerSize = 16.0

func nativeMapMarkerColor(name string) color.RGBA {
	return nativeMapMarkerColors[SanitizeLeafletMarkerColor(name)]
}

// nativeMapProbeTimeout acota el chequeo de conectividad de un único tile
// (probeTileProvider) antes de intentar el render completo.
const nativeMapProbeTimeout = 5 * time.Second

// nativeMapRenderTimeout replica los mismos umbrales por zoom que
// RenderMapToPNG (chromium_renderer.go): a mayor zoom, más tiles a
// descargar, más tiempo real de red hace falta.
func nativeMapRenderTimeout(zoom int) time.Duration {
	switch {
	case zoom >= 11:
		return 60 * time.Second
	case zoom >= 7:
		return 20 * time.Second
	default:
		return 15 * time.Second
	}
}

// probeTileProvider verifica que el proveedor de tiles responda antes de
// intentar el render completo (hallazgo de code review sobre PR #165):
// go-staticmaps traga en silencio cualquier error de fetch por tile
// (renderLayer en context.go de la librería solo hace log.Printf, nunca
// propaga el error — Render() devuelve err==nil incluso si CERO tiles se
// descargaron), así que sin este chequeo previo un servidor de tiles
// inalcanzable (firewall, DNS caído, red offline) produciría un PNG "válido"
// con solo los marcadores sobre fondo vacío, cacheado como si fuera un
// render exitoso, en vez de caer al pipeline chromedp. Se pide el tile
// 0/0/0 (todo proveedor slippy-map lo sirve, independiente del center/zoom
// real que se vaya a renderizar) reusando sm.TileFetcher directamente —
// mismo código de fetch/decode que usa go-staticmaps internamente, sin
// reimplementar su lógica de armado de URL. sm.NewTileProviderNone()
// (usado en los tests) no tiene URL configurada, así que se salta el
// chequeo en vez de intentar un GET a una URL vacía.
func probeTileProvider(ctx context.Context, provider *sm.TileProvider) error {
	if provider.IsNone() {
		return nil
	}
	fetcher := sm.NewTileFetcher(provider, nil, true)
	return runWithTimeout(ctx, nativeMapProbeTimeout, func() error {
		return fetcher.Fetch(&sm.Tile{Zoom: 0, X: 0, Y: 0})
	})
}

// runWithTimeout acota una operación que la librería no permite cancelar
// (go-staticmaps no acepta un context.Context ni expone un timeout propio,
// y usa http.DefaultClient sin Timeout — ver tile_fetcher.go de la
// librería). Si fn no termina dentro de timeout, O si ctx se cancela antes
// (issue #134/G1d), la goroutine sigue corriendo en background (fuga
// acotada, no memoria creciente sin límite — sigue atada al timeout fijo
// aunque ctx se haya cancelado, porque go-staticmaps no expone forma de
// abortar el fetch HTTP en curso) pero la llamada retorna igual, para no
// colgar el build entero indefinidamente (hallazgo de code review sobre PR
// #165). ctx puede ser nil (equivalente a solo esperar el timeout fijo).
func runWithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic: %v", r)
			}
		}()
		done <- fn()
	}()
	var ctxDone <-chan struct{}
	if ctx != nil {
		ctxDone = ctx.Done()
	}
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timed out after %s", timeout)
	case <-ctxDone:
		return fmt.Errorf("canceled: %w", ctx.Err())
	}
}

// RenderMapNativePNG rasteriza config a PNG vía go-staticmaps. Popups
// (marker.Label/Details) no se dibujan: el pipeline chromedp tampoco los
// muestra en el screenshot final (nunca llama a .openPopup() antes de
// capturar, ver generateLeafletHTML) — omitirlos preserva la paridad
// existente en vez de agregar contenido visible que el otro backend no
// produce.
func RenderMapNativePNG(ctx context.Context, config MapConfig, width, height int) ([]byte, error) {
	return renderMapNativePNG(ctx, config, width, height, nativeMapTileProvider(config.MapType))
}

// renderMapNativePNG hace el trabajo real, con el TileProvider inyectado:
// separado de RenderMapNativePNG para que los tests puedan pasar
// sm.NewTileProviderNone() y rasterizar de forma determinística sin red
// (los tiles reales de OSM/ArcGIS no son aptos para un test — flakiness de
// red y las políticas de uso de esos servidores desaconsejan bombardearlos
// desde una suite de CI).
func renderMapNativePNG(ctx context.Context, config MapConfig, width, height int, provider *sm.TileProvider) ([]byte, error) {
	if err := probeTileProvider(ctx, provider); err != nil {
		return nil, fmt.Errorf("map tile provider unreachable: %w", err)
	}

	smCtx := sm.NewContext()
	smCtx.SetSize(width, height)
	// go-staticmaps cachea tiles en os.UserCacheDir() por defecto (ver
	// sm.NewContext) — un side effect silencioso fuera del contrato de
	// cache explícito que ya tiene el resto del pipeline offline
	// (BaseFetcher, dentro de outputDir/assets). Se desactiva acá.
	smCtx.SetCache(nil)
	smCtx.SetTileProvider(provider)
	smCtx.SetCenter(s2.LatLngFromDegrees(config.CenterLat, config.CenterLng))
	smCtx.SetZoom(config.Zoom)

	for _, marker := range config.Markers {
		col := nativeMapMarkerColor(marker.Color)
		smCtx.AddObject(sm.NewMarker(s2.LatLngFromDegrees(marker.Lat, marker.Lng), col, nativeMapMarkerSize))
	}

	// Acotado con timeout + recover (hallazgos de code review y security
	// review sobre PR #165): go-staticmaps no acepta un context.Context, así
	// que sin esto un servidor de tiles colgado bloquea el build entero sin
	// límite, y un panic dentro de Render() (p. ej. de una asignación
	// image.NewRGBA extrema si width/height son enormes) tumbaría todo el
	// proceso en vez de caer al fallback de chromedp como cualquier otro
	// error de render nativo. ctx (el caller context, issue #134/G1d) acota
	// además la ESPERA por cancelación externa — no el fetch HTTP interno de
	// go-staticmaps en sí, que sigue sin forma de abortarse (ver
	// runWithTimeout/renderWithTimeout).
	img, err := renderWithTimeout(ctx, smCtx, nativeMapRenderTimeout(config.Zoom))
	if err != nil {
		return nil, fmt.Errorf("native map render failed: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("native map encode failed: %w", err)
	}
	return buf.Bytes(), nil
}

// renderWithTimeout es la variante de runWithTimeout para Context.Render,
// que devuelve (image.Image, error) en vez de solo error. ctx (issue
// #134/G1d) puede ser nil (equivalente a solo esperar el timeout fijo).
func renderWithTimeout(ctx context.Context, smCtx *sm.Context, timeout time.Duration) (image.Image, error) {
	type result struct {
		img image.Image
		err error
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- result{nil, fmt.Errorf("panic: %v", r)}
			}
		}()
		img, err := smCtx.Render()
		done <- result{img, err}
	}()
	var ctxDone <-chan struct{}
	if ctx != nil {
		ctxDone = ctx.Done()
	}
	select {
	case r := <-done:
		return r.img, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timed out after %s", timeout)
	case <-ctxDone:
		return nil, fmt.Errorf("canceled: %w", ctx.Err())
	}
}
