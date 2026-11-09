// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// mapTileServer es un servidor de tiles propio para los mapas que se dibujan
// al compilar (el render nativo con go-staticmaps y el respaldo de Leaflet en
// Chromium), en lugar de OpenStreetMap y ArcGIS World Imagery. Lo fija el
// operador (una bandera del CLI), nunca el documento: mismo nivel de
// confianza que PlantUMLServer. El cero es "sin servidor propio".
type mapTileServer struct {
	// urlTemplate lleva {z}, {x} y {y}, y opcionalmente {s} (subdominios
	// a, b y c), la sintaxis de L.tileLayer.
	urlTemplate string
	// userAgent, si no está vacío, reemplaza el User-Agent de los pedidos
	// de tiles.
	userAgent string
	// attribution es el crédito del servidor propio que se dibuja en el
	// mapa (WithTileAttribution); vacío, ninguno.
	attribution string
}

// validateMapTileTemplate exige una URL http(s) absoluta con host y los tres
// marcadores {z}, {x} y {y}. Los únicos marcadores admitidos son esos y {s}:
// cualquier otro {…} quedaría literal en la URL de cada tile.
func validateMapTileTemplate(urlTemplate string) error {
	if strings.TrimSpace(urlTemplate) != urlTemplate || urlTemplate == "" {
		return fmt.Errorf("tile server template %q must be a non-empty URL without surrounding spaces", urlTemplate)
	}
	for _, p := range []string{"{z}", "{x}", "{y}"} {
		if !strings.Contains(urlTemplate, p) {
			return fmt.Errorf("tile server template %q lacks %s (expected e.g. https://tiles.example.com/{z}/{x}/{y}.png)", urlTemplate, p)
		}
	}
	rest := urlTemplate
	for _, p := range []string{"{s}", "{z}", "{x}", "{y}"} {
		rest = strings.ReplaceAll(rest, p, "0")
	}
	// Ni espacios ni caracteres de control, ni lo que pueda cortar un
	// string de JS, una cabecera o una directiva de CSP (; y ,).
	for _, r := range rest {
		if r <= 0x20 || r == 0x7f || strings.ContainsRune("{}'\"\\<>;,`", r) {
			return fmt.Errorf("tile server template %q has a space, control character, %q or a placeholder other than {s}, {z}, {x} and {y}", urlTemplate, r)
		}
	}
	u, err := url.Parse(rest)
	if err != nil {
		return fmt.Errorf("tile server template %q is not a valid URL: %w", urlTemplate, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Hostname() == "" {
		return fmt.Errorf("tile server template %q must be an absolute http(s) URL", urlTemplate)
	}
	if u.User != nil {
		return fmt.Errorf("tile server template %q must not carry user information", urlTemplate)
	}
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("tile server template %q has an invalid port %q", urlTemplate, p)
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return fmt.Errorf("tile server template %q has an empty port", urlTemplate)
	}
	// {s} es un subdominio (a, b o c): solo vale como una etiqueta completa
	// del host, como en https://{s}.tile.openstreetmap.org/….
	if strings.Contains(urlTemplate, "{s}") {
		host := urlTemplate[len(u.Scheme)+len("://"):]
		if i := strings.IndexAny(host, "/?#"); i >= 0 {
			if strings.Contains(host[i:], "{s}") {
				return fmt.Errorf("tile server template %q uses {s} outside the host; it stands for a subdomain", urlTemplate)
			}
			host = host[:i]
		}
		hostname := host
		if i := strings.LastIndex(hostname, ":"); i >= 0 {
			hostname = hostname[:i]
		}
		for _, label := range strings.Split(hostname, ".") {
			if strings.Contains(label, "{s}") && label != "{s}" {
				return fmt.Errorf("tile server template %q uses {s} inside a host label; it must be a whole label, as in https://{s}.tiles.example.com/…", urlTemplate)
			}
		}
	}
	return nil
}

// validateMapTileUserAgent rechaza saltos de línea y caracteres de control,
// que no caben en una cabecera HTTP.
func validateMapTileUserAgent(userAgent string) error {
	for _, r := range userAgent {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("tile server user agent has a control character")
		}
	}
	return nil
}

// newMapTileServer valida y arma un mapTileServer. Un template vacío con un
// user agent no vacío es un error: el user agent solo aplica a un servidor
// propio.
func newMapTileServer(urlTemplate, userAgent string) (mapTileServer, error) {
	if urlTemplate == "" {
		if userAgent != "" {
			return mapTileServer{}, fmt.Errorf("a tile server user agent needs a tile server template")
		}
		return mapTileServer{}, nil
	}
	if err := validateMapTileTemplate(urlTemplate); err != nil {
		return mapTileServer{}, err
	}
	if err := validateMapTileUserAgent(userAgent); err != nil {
		return mapTileServer{}, err
	}
	return mapTileServer{urlTemplate: urlTemplate, userAgent: userAgent}, nil
}

// WithTileServer devuelve config con un servidor de tiles propio para
// dibujarlo al compilar: el render nativo (RenderMapNativePNG) y el respaldo
// de Leaflet en Chromium piden los tiles a urlTemplate ({z}, {x}, {y} y, si
// está, {s} con los subdominios a, b y c) en vez de a OpenStreetMap o, para
// un mapa satelital, a ArcGIS World Imagery; userAgent, si no está vacío,
// reemplaza el User-Agent de esos pedidos. Un template vacío deja config
// como estaba (con un userAgent no vacío es un error). Es un método, no un
// campo exportado, para que un CLI que compila contra un core publicado sin
// él lo pida por interfaz sin dejar de compilar.
func (c MapConfig) WithTileServer(urlTemplate, userAgent string) (MapConfig, error) {
	tiles, err := newMapTileServer(urlTemplate, userAgent)
	if err != nil {
		return c, err
	}
	tiles.attribution = c.tiles.attribution
	c.tiles = tiles
	return c, nil
}

// TileServer devuelve el template y el user agent fijados con
// WithTileServer (vacíos si no hay servidor propio).
func (c MapConfig) TileServer() (urlTemplate, userAgent string) {
	return c.tiles.urlTemplate, c.tiles.userAgent
}

// maxMapTileAttribution acota el texto de atribución.
const maxMapTileAttribution = 200

// validateMapTileAttribution rechaza caracteres de control y textos largos.
func validateMapTileAttribution(text string) error {
	if len(text) > maxMapTileAttribution {
		return fmt.Errorf("tile attribution is longer than %d bytes", maxMapTileAttribution)
	}
	for _, r := range text {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("tile attribution has a control character")
		}
	}
	return nil
}

// WithTileAttribution devuelve config con el crédito que se dibuja en la
// esquina del mapa cuando los tiles salen de un servidor propio
// (WithTileServer): el render nativo y el respaldo de Leaflet lo ponen igual.
// Sin servidor propio no tiene efecto (OpenStreetMap y ArcGIS llevan el
// suyo). Es texto plano: no se interpreta como HTML.
func (c MapConfig) WithTileAttribution(text string) (MapConfig, error) {
	if err := validateMapTileAttribution(text); err != nil {
		return c, err
	}
	c.tiles.attribution = text
	return c, nil
}

// TileAttribution devuelve el crédito de WithTileAttribution, si hay
// servidor propio.
func (c MapConfig) TileAttribution() string {
	if c.tiles.urlTemplate == "" {
		return ""
	}
	return c.tiles.attribution
}

// SetMapTileAttribution es WithTileAttribution para los mapas que este
// contexto dibuja al compilar.
func (c *RenderContext) SetMapTileAttribution(text string) error {
	if err := validateMapTileAttribution(text); err != nil {
		return err
	}
	c.mapTiles.attribution = text
	return nil
}

// SetMapTileServer hace que los mapas que este contexto dibuja al compilar
// (MapMode offline-assets u offline-inline: el PDF y el HTML offline) usen
// un servidor de tiles propio; ver MapConfig.WithTileServer. No cambia el
// HTML de modo browser, cuyo mapa lo dibuja el navegador del lector. Un
// template inválido es un error y deja el contexto como estaba.
func (c *RenderContext) SetMapTileServer(urlTemplate, userAgent string) error {
	tiles, err := newMapTileServer(urlTemplate, userAgent)
	if err != nil {
		return err
	}
	tiles.attribution = c.mapTiles.attribution
	c.mapTiles = tiles
	return nil
}

// goStaticMapsPattern traduce el template de L.tileLayer al URLPattern de
// go-staticmaps (%[1]s subdominio, %[2]d zoom, %[3]d x, %[4]d y). Un % del
// template se duplica para que fmt lo deje literal.
func goStaticMapsPattern(urlTemplate string) string {
	p := strings.ReplaceAll(urlTemplate, "%", "%%")
	return strings.NewReplacer("{s}", "%[1]s", "{z}", "%[2]d", "{x}", "%[3]d", "{y}", "%[4]d").Replace(p)
}

// mapTileShards son los subdominios de {s}, los mismos que Leaflet usa por
// defecto.
var mapTileShards = []string{"a", "b", "c"}
