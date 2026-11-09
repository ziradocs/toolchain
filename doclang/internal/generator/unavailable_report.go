// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/renderer"
)

// UnavailableContent es un contenido que la salida dejó como marcador de no
// disponible: una imagen que falta (o queda fuera de --asset-root, o es
// remota y no se pidió) o un mapa, chart, diagrama, ecuación o medio que no
// se pudo dibujar.
type UnavailableContent struct {
	// Kind es "image", "map", "chart", "diagram", "equation" o "media".
	Kind string
	// Line es la línea fuente del elemento; 0 si no viene de uno (el logo
	// del encabezado).
	Line int
	// Detail es la fuente de una imagen tal como la escribió el autor;
	// vacío para lo demás.
	Detail string
}

// UnavailableError es el error de GeneratorOptions.FailOnUnavailable: lo
// que quedó como marcador, en el orden del documento. La salida no se
// escribe.
type UnavailableError struct {
	Items []UnavailableContent
}

func (e *UnavailableError) Error() string {
	var b strings.Builder
	n := len(e.Items)
	if n == 1 {
		b.WriteString("1 item could not be included and would be left as a \"not available\" marker (--fail-on-unavailable); no output was written:")
	} else {
		fmt.Fprintf(&b, "%d items could not be included and would be left as \"not available\" markers (--fail-on-unavailable); no output was written:", n)
	}
	for _, it := range e.Items {
		b.WriteString("\n  ")
		if it.Line > 0 {
			fmt.Fprintf(&b, "line %d: ", it.Line)
		} else {
			b.WriteString("front matter: ")
		}
		b.WriteString(it.Kind)
		if it.Detail != "" {
			fmt.Fprintf(&b, " %q", it.Detail)
		}
	}
	return b.String()
}

// unavailableCollector junta los UnavailableContent de una generación, sin
// repetir uno idéntico (el mismo logo de encabezado, p. ej., se arma una vez
// por variante de encabezado).
type unavailableCollector struct {
	items []UnavailableContent
	seen  map[UnavailableContent]bool
}

func (c *unavailableCollector) add(kind string, line int, detail string) {
	it := UnavailableContent{Kind: kind, Line: line, Detail: detail}
	if c.seen == nil {
		c.seen = map[UnavailableContent]bool{}
	}
	if c.seen[it] {
		return
	}
	c.seen[it] = true
	c.items = append(c.items, it)
}

// err es el UnavailableError de lo juntado, o nil si no hubo nada.
func (c *unavailableCollector) err() error {
	if c == nil || len(c.items) == 0 {
		return nil
	}
	return &UnavailableError{Items: c.items}
}

// unavailableReporterSetter es RenderContext.SetUnavailableReporter de
// core. Se pide por interfaz porque este módulo compila también contra el
// core publicado, que puede no tenerlo.
type unavailableReporterSetter interface {
	SetUnavailableReporter(func(kind string, line int, detail string))
}

// errUnavailableNeedsCore: el core con el que se compiló no reporta lo que
// el HTML/PDF deja como marcador, así que --fail-on-unavailable no podría
// cumplir lo que promete en esos formatos.
var errUnavailableNeedsCore = errors.New("--fail-on-unavailable for html and pdf needs a core with RenderContext.SetUnavailableReporter (this build has an older one); docx is checked regardless")

// watchUnavailable conecta un collector al contexto de render si
// opts.FailOnUnavailable lo pide. Devuelve nil (sin collector) si no.
func watchUnavailable(ctx any, opts GeneratorOptions) (*unavailableCollector, error) {
	if !opts.FailOnUnavailable {
		return nil, nil
	}
	s, ok := ctx.(unavailableReporterSetter)
	if !ok {
		return nil, errUnavailableNeedsCore
	}
	c := &unavailableCollector{}
	s.SetUnavailableReporter(c.add)
	return c, nil
}

// mapTileServerSetter es RenderContext.SetMapTileServer de core, y
// mapTileServerConfig MapConfig.WithTileServer; por interfaz por lo mismo
// que unavailableReporterSetter.
type mapTileServerSetter interface {
	SetMapTileServer(urlTemplate, userAgent string) error
}

type mapTileServerConfig interface {
	WithTileServer(urlTemplate, userAgent string) (renderer.MapConfig, error)
}

// mapTileAttributionSetter y mapTileAttributionConfig son
// RenderContext.SetMapTileAttribution y MapConfig.WithTileAttribution.
type mapTileAttributionSetter interface {
	SetMapTileAttribution(text string) error
}

type mapTileAttributionConfig interface {
	WithTileAttribution(text string) (renderer.MapConfig, error)
}

// errTileAttributionNeedsCore: el core no sabe dibujar el crédito.
var errTileAttributionNeedsCore = errors.New("--tile-attribution needs a core with MapConfig.WithTileAttribution (this build has an older one)")

// networkRestricter es ChromiumRenderer.RestrictNetwork de core.
type networkRestricter interface{ RestrictNetwork() error }

// errNoNetworkNeedsCore: el core no sabe arrancar Chromium sin red, así que
// --no-network no podría prometer que el navegador no sale a la red.
var errNoNetworkNeedsCore = errors.New("--no-network with Chromium (pdf, mermaid, equations, svg images, offline html) needs a core with ChromiumRenderer.RestrictNetwork (this build has an older one)")

// restrictNetwork aplica GeneratorOptions.NoNetwork a un ChromiumRenderer
// recién creado, antes de su primer render: Chrome y Edge hablan solos con
// sus servidores (actualizaciones, telemetría) aunque la página no pida
// nada.
func restrictNetwork(r any, opts GeneratorOptions) error {
	if !opts.NoNetwork {
		return nil
	}
	s, ok := r.(networkRestricter)
	if !ok {
		return errNoNetworkNeedsCore
	}
	return s.RestrictNetwork()
}

// errTileServerNeedsCore: el core con el que se compiló no sabe pedirle los
// tiles a otro servidor.
var errTileServerNeedsCore = errors.New("--tile-server needs a core with MapConfig.WithTileServer (this build has an older one)")

// ValidateTileServer revisa --tile-server y --tile-user-agent antes de
// generar: el template (con {z}, {x}, {y} y opcionalmente {s}) y que este
// build sepa usarlo.
func ValidateTileServer(urlTemplate, userAgent, attribution string) error {
	if urlTemplate == "" && (userAgent != "" || attribution != "") {
		return errors.New("--tile-user-agent and --tile-attribution need --tile-server")
	}
	_, err := withTileServer(renderer.MapConfig{}, urlTemplate, userAgent, attribution)
	return err
}

// withTileServer aplica el servidor de tiles (y su crédito) a un MapConfig
// del DOCX.
func withTileServer(cfg renderer.MapConfig, urlTemplate, userAgent, attribution string) (renderer.MapConfig, error) {
	if urlTemplate == "" && userAgent == "" && attribution == "" {
		return cfg, nil
	}
	c, ok := any(cfg).(mapTileServerConfig)
	if !ok {
		return cfg, errTileServerNeedsCore
	}
	cfg, err := c.WithTileServer(urlTemplate, userAgent)
	if err != nil || attribution == "" {
		return cfg, err
	}
	a, ok := any(cfg).(mapTileAttributionConfig)
	if !ok {
		return cfg, errTileAttributionNeedsCore
	}
	return a.WithTileAttribution(attribution)
}

// setTileServer aplica el servidor de tiles al contexto de render del
// HTML/PDF.
func setTileServer(ctx any, opts GeneratorOptions) error {
	if opts.TileServer == "" && opts.TileUserAgent == "" && opts.TileAttribution == "" {
		return nil
	}
	s, ok := ctx.(mapTileServerSetter)
	if !ok {
		return errTileServerNeedsCore
	}
	if err := s.SetMapTileServer(opts.TileServer, opts.TileUserAgent); err != nil || opts.TileAttribution == "" {
		return err
	}
	a, ok := ctx.(mapTileAttributionSetter)
	if !ok {
		return errTileAttributionNeedsCore
	}
	return a.SetMapTileAttribution(opts.TileAttribution)
}

// errNoNetwork es el error de los fetchers de GeneratorOptions.NoNetwork:
// core lo trata como cualquier fallo de un fetcher y deja el marcador.
var errNoNetwork = errors.New("not fetched: --no-network forbids network requests during the build")

// noNetworkPlantUML, noNetworkMermaid y noNetworkMap reemplazan a los
// fetchers del contexto de render que salen a la red (el servidor de
// PlantUML o Kroki y los tiles de mapas) cuando NoNetwork lo pide: no hacen
// ningún pedido y devuelven errNoNetwork. Un fetcher nil no sirve para esto:
// con PlantUML, core cae al modo browser y deja un <img> que apunta al
// servidor, que Chromium pediría al imprimir el PDF.
type noNetworkPlantUML struct{}

func (noNetworkPlantUML) FetchDiagramToAssets(context.Context, string) (string, error) {
	return "", errNoNetwork
}

func (noNetworkPlantUML) FetchDiagramInline(context.Context, string) (string, error) {
	return "", errNoNetwork
}

type noNetworkMermaid struct{}

func (noNetworkMermaid) FetchAndSave(context.Context, string, string) (string, error) {
	return "", errNoNetwork
}

func (noNetworkMermaid) FetchInline(context.Context, string) (string, error) {
	return "", errNoNetwork
}

type noNetworkMap struct{ format string }

func (noNetworkMap) FetchAndSave(context.Context, renderer.MapConfig, string, int, int) (string, error) {
	return "", errNoNetwork
}

func (noNetworkMap) FetchInline(context.Context, renderer.MapConfig, int, int) ([]byte, error) {
	return nil, errNoNetwork
}

func (f noNetworkMap) GetImageFormat() string { return f.format }

// cutNetwork aplica GeneratorOptions.NoNetwork al contexto de render del
// HTML/PDF: PlantUML (servidor de PlantUML o Kroki), mermaid por Kroki y los
// mapas quedan como marcador sin hacer ningún pedido. Mermaid por Chromium,
// los charts y las ecuaciones se dibujan en esta máquina y no cambian.
func cutNetwork(ctx *renderer.RenderContext, opts GeneratorOptions) {
	if !opts.NoNetwork {
		return
	}
	if ctx.Fetcher != nil {
		ctx.Fetcher = noNetworkPlantUML{}
	}
	if opts.DiagramBackend == "kroki" && ctx.MermaidFetcher != nil {
		ctx.MermaidFetcher = noNetworkMermaid{}
	}
	if ctx.MapFetcher != nil {
		ctx.MapFetcher = noNetworkMap{format: ctx.MapFetcher.GetImageFormat()}
	}
}
