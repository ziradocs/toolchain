// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"go.ziradocs.com/core/v2/renderer"
)

// La página de Leaflet no pide los tiles por su cuenta: los pide a
// mapTileProxyOrigin, un origen que no existe, y serveMapTiles los descarga
// desde Go del servidor real y se los entrega a la página. Así:
//
//   - un servidor de tiles en loopback o en una red privada (--tile-server
//     http://127.0.0.1:…) funciona. Chromium bloquea los pedidos de la
//     página (about:blank) a la red local por Local Network Access
//     (CORS "InsecureLocalNetwork", net::ERR_FAILED) y el respaldo dibujaba
//     un mapa vacío sin hacer ni un pedido;
//   - se sabe cuántos tiles llegaron y cuántos fallaron, y el render falla
//     si faltan (ver mapTileStats.err): antes un tile que fallaba contaba
//     como cargado y un servidor que daba 404 a todo producía un PNG gris
//     sin error;
//   - el User-Agent de los tiles es el mismo que el del render nativo, sin
//     tocar el del navegador.
const mapTileProxyOrigin = "https://map-tiles.offline.invalid"

// mapTileProxyTemplate es el template que recibe L.tileLayer.
const mapTileProxyTemplate = mapTileProxyOrigin + "/{s}/{z}/{x}/{y}"

// defaultMapTileUserAgent es el User-Agent con el que go-staticmaps pide los
// tiles en el render nativo; el respaldo se identifica igual.
const defaultMapTileUserAgent = "Mozilla/5.0+(compatible; go-staticmaps/0.1; https://github.com/flopp/go-staticmaps)"

// mapTileMaxBytes acota cada tile descargado.
const mapTileMaxBytes = 4 << 20

// mapTileTimeout acota cada tile descargado.
const mapTileTimeout = 15 * time.Second

// mapTileStats cuenta los tiles que la página pidió.
type mapTileStats struct {
	ok, failed atomic.Int32
}

// err es la regla del respaldo: el mapa no se entrega si no llegó ningún
// tile o si fallaron más de la mitad. Un mapa con la mayoría del fondo vacío
// pasa por bueno sin serlo (el lector no ve que falta algo) y es mejor dejar
// el marcador de no disponible, que --fail-on-unavailable sí detecta. Uno o
// dos tiles perdidos en los bordes de un mapa grande no lo descartan.
func (s *mapTileStats) err() error {
	ok, failed := int(s.ok.Load()), int(s.failed.Load())
	switch {
	case ok == 0:
		return fmt.Errorf("no map tile could be downloaded (%d failed)", failed)
	case failed > ok:
		return fmt.Errorf("only %d of %d map tiles could be downloaded", ok, ok+failed)
	}
	return nil
}

// realMapTileTemplate es el servidor de tiles real de config: el propio de
// MapConfig.WithTileServer o el de siempre según MapType.
func realMapTileTemplate(config renderer.MapConfig) string {
	if tmpl, _ := config.TileServer(); tmpl != "" {
		return tmpl
	}
	if config.MapType == "satellite" {
		return "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}"
	}
	return "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
}

// mapTileURL traduce un pedido de la página (mapTileProxyTemplate) a la URL
// real. Solo acepta un subdominio a, b o c y coordenadas enteras dentro del
// rango del zoom: la página es contenido derivado del documento y no puede
// armar otra URL.
func mapTileURL(pageURL, realTemplate string) (string, bool) {
	rest, ok := strings.CutPrefix(pageURL, mapTileProxyOrigin+"/")
	if !ok {
		return "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 4 || (parts[0] != "a" && parts[0] != "b" && parts[0] != "c") {
		return "", false
	}
	z, errZ := strconv.Atoi(parts[1])
	x, errX := strconv.Atoi(parts[2])
	y, errY := strconv.Atoi(parts[3])
	if errZ != nil || errX != nil || errY != nil || z < 0 || z > 22 || x < 0 || y < 0 || x >= 1<<z || y >= 1<<z {
		return "", false
	}
	return strings.NewReplacer("{s}", parts[0], "{z}", parts[1], "{x}", parts[2], "{y}", parts[3]).Replace(realTemplate), true
}

// fetchMapTile descarga un tile y devuelve sus bytes y su Content-Type.
func fetchMapTile(ctx context.Context, client *http.Client, url, userAgent string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("tile server answered %s", resp.Status)
	}
	ctype := resp.Header.Get("Content-Type")
	if mt, _, err := mime.ParseMediaType(ctype); err != nil || !strings.HasPrefix(mt, "image/") {
		return nil, "", fmt.Errorf("tile server answered %q, not an image", ctype)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, mapTileMaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > mapTileMaxBytes {
		return nil, "", fmt.Errorf("tile larger than %d bytes", mapTileMaxBytes)
	}
	return data, ctype, nil
}

// serveMapTiles activa la intercepción de pedidos del tab del mapa: un
// pedido a mapTileProxyOrigin se contesta con el tile real, descargado desde
// Go, y cualquier otro pedido se rechaza (la página no tiene nada más que
// pedir: Leaflet y los íconos van embebidos). stats cuenta los resultados.
func serveMapTiles(config renderer.MapConfig, stats *mapTileStats) chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		if t == nil {
			return fmt.Errorf("map tile interception needs a chromedp target")
		}
		realTemplate := realMapTileTemplate(config)
		userAgent := defaultMapTileUserAgent
		if _, ua := config.TileServer(); ua != "" {
			userAgent = ua
		}
		client := &http.Client{Timeout: mapTileTimeout}
		paused := cdp.Events(ctx, t, fetch.RequestPaused)
		go func() {
			for e, err := range paused {
				if err != nil {
					return
				}
				go func(e fetch.EventRequestPaused) {
					if e.Request != nil {
						if url, ok := mapTileURL(e.Request.URL, realTemplate); ok {
							data, ctype, err := fetchMapTile(ctx, client, url, userAgent)
							if err == nil {
								stats.ok.Add(1)
								_, _ = cdp.Call(ctx, t, fetch.FulfillRequest, fetch.FulfillRequestParams{
									RequestID:       e.RequestID,
									ResponseCode:    200,
									ResponseHeaders: []*fetch.HeaderEntry{{Name: "Content-Type", Value: ctype}},
									Body:            data,
								})
								return
							}
							stats.failed.Add(1)
						}
					}
					_, _ = cdp.Call(ctx, t, fetch.FailRequest, fetch.FailRequestParams{
						RequestID:   e.RequestID,
						ErrorReason: network.ErrorReasonBlockedByClient,
					})
				}(e)
			}
		}()
		_, err := cdp.Call(ctx, t, fetch.Enable, fetch.EnableParams{
			Patterns: []*fetch.RequestPattern{{URLPattern: "*"}},
		})
		return err
	})
}
