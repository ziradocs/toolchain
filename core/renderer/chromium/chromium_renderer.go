// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"go.ziradocs.com/core/v2/renderer"
)

// Content-Security-Policy aplicadas a las páginas temporales que chromedp
// carga para rasterizar mermaid/chart/mapas a PDF/DOCX (origen file://,
// contenido derivado del usuario). Es defensa en profundidad: aunque un
// breakout de HTML/JS lograra ejecutar código, connect-src/img-src 'none'
// bloquean tanto fetch/XHR como el patrón `new Image().src=...` usado para
// exfiltración ciega en el exploit de referencia.
// Ver docs/SECURITY_AUDIT_2026-07.md, CR-6 / issue #24.
const (
	// Leaflet, su hoja de estilos y los íconos van embebidos
	// (leaflet_embed.go) y los tiles los entrega Go desde
	// mapTileProxyOrigin (map_tile_proxy.go): la página no puede pedir nada
	// más.
	mapRenderCSP = "default-src 'none'; script-src 'unsafe-inline'; " +
		"style-src 'unsafe-inline'; " +
		"img-src " + mapTileProxyOrigin + " data:; " +
		"connect-src 'none'; object-src 'none'; base-uri 'none';"
)

// mermaidSVGContainerWidthPx es el ancho que se le da a .mermaid en
// buildMermaidSVGHTML antes de que mermaid.js dibuje (issue #204, misma
// familia que #173/#203 pero en el eje horizontal — ese commit clampó
// max-height en CSS, corriente abajo del render; este bug nace antes,
// dentro del propio SVG que mermaid entrega).
//
// Mermaid mide el ancho disponible para diagramas sensibles al contenedor
// (gantt confirmado; probablemente timeline/quadrantChart comparten el
// mecanismo) ANTES de dibujar, y si no encuentra un ancho definido cae a su
// default interno de 300px (confirmado empíricamente: un .mermaid sin width
// explícito produce viewBox="0 0 300 388" para un gantt de 4 secciones/12
// tareas). Con el timeline comprimido a 300px, las barras quedan demasiado
// angostas para su label y mermaid lo empuja afuera de la barra
// (.taskTextOutsideLeft, text-anchor:end) o centra el título — ambos casos
// posicionan texto en x negativo relativo al viewBox, que arranca en 0. Ese
// texto queda recortado DENTRO del SVG que Chromium captura vía OuterHTML:
// no es un problema de CSS/overflow corriente abajo (overflow-x en un
// wrapper no revela contenido que el propio viewBox del SVG ya excluye), así
// que ningún ajuste en mermaid.css puede arreglarlo.
//
// Es un hint de layout, no un tope: el <svg> resultante sigue trayendo
// width="100%" + un style="max-width:Npx" inline (donde N es el ancho
// NATURAL que mermaid calculó con este hint), así que corriente abajo sigue
// escalando de forma responsive al contenedor real (slide/página) vía CSS
// height:auto — un valor grande acá no fuerza un diagrama grande en la
// salida final, solo evita que mermaid trunque contenido a un ancho
// arbitrariamente chico antes de que la CSS responsive tenga oportunidad de
// actuar. Mismo motivo por el que buildMermaidPNGHTML (abajo) ya le da a
// #mermaidContainer un ancho fijo en vez de dejarlo sin definir — ese path
// nunca mostró este bug porque ya evitaba el fallback de 300px.
//
// Verificado que diagramas NO sensibles al contenedor (flowchart) producen
// el mismo viewBox sin importar este ancho — no es una regresión para ellos.
const mermaidSVGContainerWidthPx = 1200

// buildMermaidSVGHTML arma la página temporal usada para rasterizar un
// diagrama Mermaid a SVG. El contenido es dato del usuario: se HTML-escapa
// antes de insertarlo como texto del nodo ".mermaid" (Mermaid lee
// textContent, así que escapar no rompe el render) y se fija
// securityLevel:'strict' + htmlLabels:false para bloquear HTML/script
// embebido dentro del propio diagrama. Ver docs/SECURITY_AUDIT_2026-07.md,
// CR-6/AL-6 (issue #24).
func buildMermaidSVGHTML(mermaidCode string, themes ...renderer.DiagramThemeColors) string {
	theme := renderer.DiagramThemeColors{}
	if len(themes) > 0 {
		theme = themes[0]
	}
	// mermaidCode (vía BuildMermaidDiv) es dato del usuario: va como argumento
	// %s, nunca concatenado dentro del format string en sí — un '%' literal en
	// el diagrama del usuario (p. ej. un label "Growth +20%") no debe
	// reinterpretarse como un verbo de fmt (mismo patrón ya usado por
	// buildMermaidPNGHTML, abajo).
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="%s">
    %s
    <style>
        body { margin: 0; padding: 20px; background: white; }
        .mermaid { display: inline-block; width: %dpx; }
    </style>
</head>
<body>
    %s
    <script>
        mermaid.initialize(`+renderer.MermaidInitConfigJSWithTheme(true, theme)+`);
    </script>
</body>
</html>`, mermaidRenderCSP, embeddedMermaidScriptTag, mermaidSVGContainerWidthPx, renderer.BuildMermaidDiv(mermaidCode))
}

// mathThemeCSS aplica los colores del tema de diagramas a la página de math.
func mathThemeCSS(theme renderer.DiagramThemeColors) string {
	var declarations []string
	if color := theme.MermaidThemeVariables()["primaryTextColor"]; color != "" {
		declarations = append(declarations, "color: "+color+" !important")
	}
	if color := theme.MermaidThemeVariables()["primaryColor"]; color != "" {
		declarations = append(declarations, "background: "+color+" !important")
	}
	if len(declarations) == 0 {
		return ""
	}
	return "body { " + strings.Join(declarations, "; ") + "; }\n        "
}

// embedMathThemeInSVG makes the visual contract survive extraction from the
// temporary Chromium page. RenderMathToSVG returns only the <svg>, so body
// CSS cannot reach offline assets loaded later through <img>. MathJax paths
// use currentColor; setting color on the root SVG and adding a first-child
// rect keeps both foreground and surface self-contained.
func embedMathThemeInSVG(svg string, theme renderer.DiagramThemeColors) string {
	variables := theme.MermaidThemeVariables()
	foreground := variables["primaryTextColor"]
	background := variables["primaryColor"]
	if foreground == "" && background == "" {
		return svg
	}
	end := strings.IndexByte(svg, '>')
	if end < 0 {
		return svg
	}
	var attrs strings.Builder
	if foreground != "" {
		attrs.WriteString(` color="`)
		attrs.WriteString(foreground)
		attrs.WriteByte('"')
	}
	var content string
	if background != "" {
		content = `<rect width="100%" height="100%" fill="` + background + `"/>`
	}
	return svg[:end] + attrs.String() + svg[end:end+1] + content + svg[end+1:]
}

// buildMathSVGHTML arma la página temporal usada para rasterizar una
// ecuación LaTeX a SVG (issue #239-B), con MathJax inline y la CSP sin red
// de mathRenderCSP. El contenido es dato del usuario: BuildMathDiv
// (math_html.go) ya lo HTML-escapa antes de insertarlo (MathJax lee el
// textContent del nodo, igual razonamiento de seguridad que
// buildMermaidSVGHTML/CR-6/AL-6, issue #24).
func buildMathSVGHTML(latex string, themes ...renderer.DiagramThemeColors) string {
	theme := renderer.DiagramThemeColors{}
	if len(themes) > 0 {
		theme = themes[0]
	}
	return buildMathSVGPage(latex, theme)
}

// buildMathSVGPage es buildMathSVGHTML con el tema ya resuelto. MathJax va
// inline y lo que carga a demanda lo sirve serveMathAssets (math_embed.go),
// así que la página no pide nada a la red.
func buildMathSVGPage(latex string, theme renderer.DiagramThemeColors) string {
	return `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    ` + mathPageHead() + `
    <style>
        ` + mathThemeCSS(theme) + `
        body { margin: 0; padding: 20px; background: white; }
    </style>
</head>
<body>
    ` + renderer.BuildMathDiv(latex) + `
</body>
</html>`
}

// buildMathPNGHTML arma la página temporal usada para rasterizar una
// ecuación LaTeX a PNG (issue #239-B, DOCX) — mismo patrón que
// buildMermaidPNGHTML: contenedor de tamaño fijo. La captura espera a que
// MathJax deje el <svg> en el DOM (mathStateJS); renderComplete se conserva
// como marca de compatibilidad.
func buildMathPNGHTML(latex string, width, height int, themes ...renderer.DiagramThemeColors) string {
	theme := renderer.DiagramThemeColors{}
	if len(themes) > 0 {
		theme = themes[0]
	}
	return buildMathPNGPage(latex, width, height, theme)
}

func buildMathPNGPage(latex string, width, height int, theme renderer.DiagramThemeColors) string {
	return `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    ` + mathPageHead() + `
    <style>
        ` + mathThemeCSS(theme) + `
        body { margin: 0; padding: 0; background: white; display: flex; justify-content: center; align-items: center; }
        #mathContainer { width: ` + fmt.Sprint(width) + `px; height: ` + fmt.Sprint(height) + `px; display: flex; justify-content: center; align-items: center; }
    </style>
</head>
<body>
    <div id="mathContainer">
        ` + renderer.BuildMathDiv(latex) + `
    </div>
    <div id="renderComplete" style="display:none;">ready</div>
</body>
</html>`
}

// mermaidPNGDeviceScale es el deviceScaleFactor con el que se captura un
// diagrama Mermaid a PNG: 2 px por px CSS, para que el PNG siga nítido
// impreso al tamaño natural del diagrama (96 px CSS por pulgada → 192 dpi).
// doclang (docx.go) divide entre este mismo factor para recuperar el tamaño
// natural; TestMermaidPNGDeviceScale lo fija.
const mermaidPNGDeviceScale = 2

// buildMermaidPNGHTML arma la página temporal usada para rasterizar un
// diagrama Mermaid a PNG (ver buildMermaidSVGHTML para el razonamiento de
// seguridad, idéntico aquí). width es el ancho de layout que ve mermaid
// antes de dibujar (como mermaidSVGContainerWidthPx en el path SVG): un
// diagrama más ancho se encoge a ese ancho, uno más angosto queda a su
// tamaño natural. No hay alto fijo: el PNG es la caja del <svg>, sin el
// margen blanco de un contenedor de tamaño fijo.
func buildMermaidPNGHTML(mermaidCode string, width int, themes ...renderer.DiagramThemeColors) string {
	theme := renderer.DiagramThemeColors{}
	if len(themes) > 0 {
		theme = themes[0]
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="%s">
    %s
    <style>
        body { margin: 0; padding: 8px; background: white; }
        .mermaid { display: block; width: %dpx; }
        .mermaid svg { display: block; }
    </style>
</head>
<body>
    %s
    <div id="renderComplete" style="display:none;">ready</div>
    <script>
        mermaid.initialize(%s);
        // Esperar a que Mermaid termine de renderizar
        setTimeout(() => {
            document.getElementById('renderComplete').setAttribute('data-ready', 'true');
        }, 1500);
    </script>
</body>
</html>`, mermaidRenderCSP, embeddedMermaidScriptTag, width, renderer.BuildMermaidDiv(mermaidCode), renderer.MermaidInitConfigJSWithTheme(true, theme))
}

// buildChartHTML arma la página temporal usada para rasterizar un chart de
// Chart.js (PNG o WebP, ambos comparten el mismo HTML). chartConfig ya llega
// re-serializado con json.Marshal desde el llamador (html.go), por lo que
// `<`,`>`,`&` están escapados como \u00xx y no rompen el <script>.
// chartSurfaceCSS decide el valor de `background:` de la página temporal.
// surface es un token chart-surface de un tema EXTERNO, o sea contenido no
// confiable que aquí se interpola SIN escapar dentro de un <style> — el
// mismo riesgo que motivó renderer.SanitizeCSSCustomProperty (audit BA-11:
// un valor como `red; } </style><script>...` cierra la regla y el bloque).
// Se reusa esa función auditada en vez de escribir un segundo chequeo casi
// igual; si el valor no pasa, se cae a "white", que es el comportamiento
// histórico y siempre seguro, en vez de romper la página.
func chartSurfaceCSS(surface string) string {
	if surface == "" {
		return "white"
	}
	safe, ok := renderer.SanitizeCSSCustomProperty("--chart-surface", surface)
	if !ok {
		return "white"
	}
	return safe
}

func buildChartHTML(chartConfig string, width, height int, surface string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="%s">
    %s
    %s
    <style>
        body { margin: 0; padding: 0; background: %s; }
        #chartContainer { width: %dpx; height: %dpx; }
    </style>
</head>
<body>
    <div id="chartContainer">
        <canvas id="myChart"></canvas>
    </div>
    <div id="renderComplete" style="display:none;">ready</div>
    <script>
        try {
            const ctx = document.getElementById('myChart').getContext('2d');
            const config = %s;
            const chart = new Chart(ctx, config);

            // Esperar a que la animación termine completamente
            setTimeout(() => {
                document.getElementById('renderComplete').setAttribute('data-ready', 'true');
                window.__renderReady = true;
            }, 1500);
        } catch (e) {
            window.__renderError = 'Chart.js: ' + ((e && e.message) ? e.message : e);
        }
    </script>
</body>
</html>`, chartRenderCSP, renderErrorHookJS, embeddedChartScriptTags, chartSurfaceCSS(surface), width, height, chartConfig)
}

// setDocumentContentAction navega a about:blank e inyecta htmlContent
// directamente vía Page.setDocumentContent, evitando cualquier origen
// file:// o http(s):// contra el cual el contenido pudiera resolverse —
// cierra la misma clase de bypass de divulgación de archivos locales
// (AL-5, docs/SECURITY_AUDIT_2026-07.md, issue #28/#78) documentada para
// RenderHTMLToPDF (#77), para todos los Render* que arman su propia
// página HTML desechable.
func setDocumentContentAction(htmlContent string) chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		res, err := cdp.Call(ctx, t, page.GetFrameTree, cdp.Empty{})
		if err != nil {
			return fmt.Errorf("failed to get frame tree: %w", err)
		}
		if res.FrameTree == nil || res.FrameTree.Frame == nil {
			return fmt.Errorf("chromium returned an empty frame tree")
		}
		_, err = cdp.Call(ctx, t, page.SetDocumentContent, page.SetDocumentContentParams{
			FrameID: res.FrameTree.Frame.ID,
			HTML:    htmlContent,
		})
		return err
	})
}

// navigateAndSetContent es el par chromedp.Navigate("about:blank") +
// setDocumentContentAction, repetido en cada builder de este archivo — un
// único punto de reuso evita que un octavo builder futuro copie el patrón
// viejo de archivo temporal + file:// en su lugar. chromedp.Steps devuelve
// un Action[Void], así que esto es un reemplazo directo de las dos líneas
// anteriores en cada chromedp.Do(...).
func navigateAndSetContent(htmlContent string) chromedp.Action[chromedp.Void] {
	return chromedp.Steps(
		chromedp.Navigate("about:blank"),
		setDocumentContentAction(htmlContent),
	)
}

// bringTabToFront activa el tab antes de capturarlo. Cada render abre su
// propio tab (chromedp.NewContext) y, en headless, Page.captureScreenshot
// sobre un tab que no es el activo espera un frame del compositor que a
// veces nunca llega: el build se quedaba colgado para siempre en el segundo
// o tercer diagrama de un documento (reproducido con Edge, también en
// origin/main). Con el tab al frente la captura vuelve siempre.
func bringTabToFront() chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		_, err := cdp.Call(ctx, t, page.BringToFront, cdp.Empty{})
		return err
	})
}

// captureScreenshot corre Page.captureScreenshot con params y devuelve la
// imagen. FromSurface va en true como lo mandaba el constructor
// page.CaptureScreenshot() de cdproto antes de v0.157 (que es además el
// default del protocolo); los demás booleanos quedan en su default (false).
func captureScreenshot(params page.CaptureScreenshotParams) chromedp.Action[[]byte] {
	return func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
		params.FromSurface = new(true)
		res, err := cdp.Call(ctx, t, page.CaptureScreenshot, params)
		if err != nil {
			return nil, err
		}
		return res.Data, nil
	}
}

// webpQuality es el campo quality de Page.captureScreenshot. Desde cdproto
// v0.157.4 es un puntero y nil es "no mandar": una calidad 0 se sigue
// omitiendo (Chrome usa su default), como cuando el campo era un int64 con
// omitempty.
func webpQuality(quality int) *int64 {
	if quality == 0 {
		return nil
	}
	return new(int64(quality))
}

// captureBox es la caja (px CSS, coordenadas de página) que devuelve el
// getBoundingClientRect de una captura recortada.
type captureBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// setDeviceMetricsOverride es Emulation.setDeviceMetricsOverride sin emular
// un móvil, lo que mandaba emulation.SetDeviceMetricsOverride(w, h, scale,
// false) antes de cdproto v0.157.
func setDeviceMetricsOverride(width, height int64, deviceScaleFactor float64) chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		_, err := cdp.Call(ctx, t, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
			Width:             width,
			Height:            height,
			DeviceScaleFactor: deviceScaleFactor,
			Mobile:            false,
		})
		return err
	})
}

// setEmulatedMedia es Emulation.setEmulatedMedia; "" quita la emulación.
func setEmulatedMedia(ctx context.Context, t *chromedp.Target, media string) error {
	_, err := cdp.Call(ctx, t, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{Media: media})
	return err
}

// rgbaAlwaysAlpha es cdp.RGBA con el alfa siempre en el JSON. En cdproto
// v0.157.x cdp.RGBA.A es un float64 con omitempty: A: 0 no se manda y Chrome
// lo toma como 1 (su default), así que pedir transparencia con cdp.RGBA da un
// fondo negro opaco. Antes de v0.157 el campo no tenía omitempty.
type rgbaAlwaysAlpha struct {
	R int64   `json:"r"`
	G int64   `json:"g"`
	B int64   `json:"b"`
	A float64 `json:"a"`
}

// backgroundColorOverrideParams es emulation.SetDefaultBackgroundColorOverrideParams
// con rgbaAlwaysAlpha como color.
type backgroundColorOverrideParams struct {
	Color *rgbaAlwaysAlpha `json:"color,omitempty"`
}

// setDefaultBackgroundColorOverride es el mismo comando del protocolo que
// emulation.SetDefaultBackgroundColorOverride, con backgroundColorOverrideParams.
var setDefaultBackgroundColorOverride = cdp.Command[backgroundColorOverrideParams, cdp.Empty]{
	Method: emulation.CommandSetDefaultBackgroundColorOverride,
}

// transparentBackground pide a Chrome un fondo por defecto transparente
// (Emulation.setDefaultBackgroundColorOverride con alfa 0).
func transparentBackground() chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		_, err := cdp.Call(ctx, t, setDefaultBackgroundColorOverride, backgroundColorOverrideParams{
			Color: &rgbaAlwaysAlpha{R: 0, G: 0, B: 0, A: 0},
		})
		return err
	})
}

// ChromiumRenderer usa Chromium para rendering avanzado
type ChromiumRenderer struct {
	manager *ChromiumManager
	ctx     context.Context
	cancel  context.CancelFunc
	logger  ChromiumLogger

	// browserStartOnce/browserReady/browserErr respaldan ensureBrowser (issue
	// #114): garantizan que el proceso de Chrome se aloque una sola vez,
	// contra r.ctx, mientras cada caller puede seguir abandonando su propia
	// espera vía su ctx puntual (ver ensureBrowser).
	browserStartOnce sync.Once
	browserReady     chan struct{}
	browserErr       error

	// parentCtx y chromiumPath permiten rearmar el contexto del navegador
	// antes de arrancarlo (RestrictNetwork); started dice si ya arrancó.
	parentCtx    context.Context
	chromiumPath string
	// startMu ordena el arranque (ensureBrowser) contra RestrictNetwork:
	// started y el cambio de r.ctx no se cruzan.
	startMu sync.Mutex
	started bool
}

// RestrictNetwork deja al navegador sin salida a la red. En Linux lo arranca
// en un espacio de nombres de red propio (networkIsolation), donde ninguna
// conexión sale de la máquina; además, en todo sistema, ningún nombre
// resuelve, toda conexión que no sea a loopback va a un proxy que no existe,
// WebRTC no manda UDP y los servicios de fondo (actualización de
// componentes, pings) quedan apagados. Las páginas de render no lo necesitan: los scripts van
// embebidos y los tiles de mapas los entrega Go. Lo usa --no-network de
// doclang. Tiene que llamarse antes del primer render: el proceso de Chrome
// arranca con él y sus banderas no se cambian después.
func (r *ChromiumRenderer) RestrictNetwork() error {
	r.startMu.Lock()
	defer r.startMu.Unlock()
	if r.started {
		return fmt.Errorf("RestrictNetwork must be called before the browser starts")
	}
	// Primero con el espacio de nombres de red propio. El navegador se
	// arranca acá mismo para saber si funciona: donde Chrome no puede armar
	// su sandbox dentro del espacio de nombres (p. ej. cae al sandbox SUID,
	// como en los runners de GitHub), no arranca, y se vuelve a las
	// banderas solas.
	base := append([]chromedp.ExecAllocatorOption{}, noNetworkAllocatorOptions...)
	if iso, err := networkIsolationFn(r.chromiumPath); err != nil {
		r.logger.Warn("CHROMIUM", "--no-network: could not isolate the browser's network (%v); relying on launch flags only", err)
	} else {
		ctx, cancel, err := r.manager.createContext(r.parentCtx, r.chromiumPath, false, append(append([]chromedp.ExecAllocatorOption{}, base...), iso)...)
		if err == nil {
			if err = chromedp.Do(ctx); err == nil {
				r.replaceContext(ctx, cancel)
				return nil
			}
			cancel()
		}
		r.logger.Warn("CHROMIUM", "--no-network: the browser did not start in an isolated network namespace (%v); relying on launch flags only", err)
	}
	// Sin aislamiento quedan las banderas: cortan los pedidos de las
	// páginas, WebRTC y los servicios de fondo, pero no la conexión
	// directa del navegador a su servidor de DNS sobre HTTPS.
	ctx, cancel, err := r.manager.createContext(r.parentCtx, r.chromiumPath, false, base...)
	if err != nil {
		return err
	}
	r.replaceContext(ctx, cancel)
	return nil
}

// networkIsolationFn es networkIsolation; una variable para que las
// pruebas simulen un sistema donde el navegador no arranca aislado.
var networkIsolationFn = networkIsolation

// replaceContext cambia el contexto del navegador (con startMu tomado).
func (r *ChromiumRenderer) replaceContext(ctx context.Context, cancel context.CancelFunc) {
	if r.cancel != nil {
		r.cancel()
	}
	r.ctx, r.cancel = ctx, cancel
}

// NewChromiumRenderer crea un nuevo renderer con Chromium, con el branding
// por defecto de doclang (issue #92). Ver NewChromiumRendererWithBrand para
// otros CLIs que comparten este pipeline (p. ej. slidelang).
func NewChromiumRenderer(ctx context.Context, customPath string, autoInstall bool, logger ChromiumLogger) (*ChromiumRenderer, error) {
	return NewChromiumRendererWithBrand(ctx, customPath, autoInstall, logger, DefaultChromiumBrand)
}

// NewChromiumRendererWithBrand es como NewChromiumRenderer pero permite
// escoger el ChromiumBrand (directorio local de instalación/cache + texto de
// ayuda cuando no se encuentra Chromium) en vez de heredar el de doclang.
// brand se inyecta por instancia, no vía estado global mutable (issue #92,
// hallazgo de code review sobre el diseño anterior con `var` a nivel de
// paquete — antipatrón sin thread-safety, sujeto a colisión si dos brands
// coexistieran en el mismo proceso).
//
// ctx gobierna tanto la descarga de Chromium (si autoInstall la dispara)
// como el ciclo de vida completo del browser: cancelarlo cierra el browser y
// cualquier render en curso (issue #134/G1d) — antes esto era siempre
// context.Background(), así que ni un caller de librería ni el propio CLI
// tenían forma de abortar un build colgado salvo matar el proceso entero.
func NewChromiumRendererWithBrand(ctx context.Context, customPath string, autoInstall bool, logger ChromiumLogger, brand ChromiumBrand) (*ChromiumRenderer, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	manager := newChromiumManager(customPath, autoInstall, logger, brand)

	// Garantizar que Chromium esté disponible
	chromiumPath, err := manager.EnsureChromium(ctx)
	if err != nil {
		return nil, err
	}

	// Mostrar versión
	version := manager.GetVersion(chromiumPath)
	logger.Info("CHROMIUM", "Using: %s", version)

	// Crear contexto
	browserCtx, cancel, err := manager.CreateContext(ctx, chromiumPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create chromium context: %w", err)
	}

	return &ChromiumRenderer{
		manager:      manager,
		ctx:          browserCtx,
		cancel:       cancel,
		logger:       logger,
		browserReady: make(chan struct{}),
		parentCtx:    ctx,
		chromiumPath: chromiumPath,
	}, nil
}

// withCallerCancel deriva de base un context que además se cancela si
// callerCtx termina primero (issue #134/G1d). base gobierna el tab/timeout
// interno de cada Render*; callerCtx es la cancelación/timeout puntual que
// pide quien invoca ese método (p. ej. un backend que aborta el render de un
// request cuyo cliente se desconectó). chromedp no ofrece un merge nativo de
// dos context.Context independientes, así que un goroutine watcher los
// combina; se cierra a sí mismo en cuanto la llamada termina (vía el
// CancelFunc retornado), para no dejar el goroutine vivo más allá de esta
// invocación.
func withCallerCancel(base context.Context, callerCtx context.Context) (context.Context, context.CancelFunc) {
	derived, cancel := context.WithCancel(base)
	if callerCtx == nil {
		return derived, cancel
	}
	done := make(chan struct{})
	var closeOnce sync.Once
	stop := func() { closeOnce.Do(func() { close(done) }) }
	go func() {
		select {
		case <-callerCtx.Done():
			cancel()
		case <-done:
		}
	}()
	// El CancelFunc retornado debe ser idempotente, como cualquier
	// context.CancelFunc (contrato del stdlib) — un caller que además
	// cancele explícito en un branch de salida rápida y conserve el
	// `defer cancel()` como red de seguridad (patrón común en Go) llamaría
	// esta función dos veces; sync.Once evita un doble close(done) (panic:
	// close of closed channel) en ese caso (hallazgo de code review).
	return derived, func() {
		stop()
		cancel()
	}
}

// ensureBrowser aloca el proceso de Chrome contra r.ctx — el contexto de
// vida larga que ya es dueño de Close() — en vez de dejar que lo aloque
// implícitamente el primer chromedp.Do(runCtx) de algún Render* (issue
// #114). chromedp ata exec.CommandContext al ctx de ESE primer Run; si ese
// ctx es el runCtx puntual de un Render*, el `defer cancel()` de ese método
// mata el proceso de Chrome al retornar, y vía el watchdog LostConnection de
// chromedp (allocate.go) eso cancela r.ctx mismo — envenenando todo Render*
// posterior sobre este renderer con "context canceled". Alocando siempre
// contra r.ctx, la alocación y el primer target quedan atados a él, así que
// cada Render* subsecuente simplemente abre un tab
// (chromedp.NewContext(r.ctx)) sobre un browser ya vivo.
//
// callerCtx es el ctx puntual de quien llama (issue #134/G1d, hallazgo de
// code review sobre la primera versión de este método): la alocación real
// corre en un goroutine de fondo, contra r.ctx, disparada una sola vez
// (browserStartOnce) sin importar cuántos callers pasen por acá; cada
// caller espera esa alocación con un select sobre su propio callerCtx, así
// que uno que cancela mientras Chrome sigue iniciando retorna de inmediato en
// vez de bloquear ese tiempo entero. Desde chromedp 0.21 el ExecAllocator
// habla con Chrome por un pipe (--remote-debugging-pipe) y no por websocket:
// el plazo de 20s de wsURLReadTimeout ya no aplica, y un Chrome que arranca
// pero nunca contesta deja esta espera abierta hasta que el proceso muere o
// se cancela r.ctx; solo el callerCtx la acota — el contrato documentado de que ctx
// acota cada Render* puntual, ya respetado en todo el resto del archivo vía
// withCallerCancel, ahora también cubre esta primera espera. El arranque en
// sí no se aborta por eso: sigue en curso contra r.ctx y lo reusa quien
// llame después.
func (r *ChromiumRenderer) ensureBrowser(callerCtx context.Context) error {
	r.browserStartOnce.Do(func() {
		r.startMu.Lock()
		r.started = true
		browserCtx := r.ctx
		r.startMu.Unlock()
		go func() {
			r.browserErr = chromedp.Do(browserCtx)
			close(r.browserReady)
		}()
	})

	if callerCtx == nil {
		<-r.browserReady
		return r.browserErr
	}

	select {
	case <-r.browserReady:
		return r.browserErr
	case <-callerCtx.Done():
		return callerCtx.Err()
	}
}

// var _ renderer.PDFBackend asserts that *ChromiumRenderer satisfies the
// backend seam structurally, sin declararlo explícitamente (mismo patrón que
// las interfaces de fetcher en core/renderer/fetchers.go, para evitar el
// ciclo de import renderer -> chromium -> renderer).
var _ renderer.PDFBackend = (*ChromiumRenderer)(nil)

// RenderHTMLToPDF convierte HTML a PDF. ctx acota/cancela esta llamada
// puntual (issue #134/G1d) sin afectar el ciclo de vida del browser, que
// sigue gobernado por el ctx pasado al constructor.
func (r *ChromiumRenderer) RenderHTMLToPDF(ctx context.Context, htmlContent string, outputPath string, opts PDFOptions) error {
	r.logger.Info("PDF", "Rendering HTML to PDF...")

	// Validar antes de arrancar el navegador: un Scale fuera de rango falla
	// igual, pero acá el mensaje dice cuál es el rango y no cuesta un tab.
	if err := ValidatePDFScale(opts.Scale); err != nil {
		return err
	}

	r.logger.Info("PDF", "Loading HTML document...")

	// Buffer para el PDF
	var pdfBuf []byte

	if err := r.ensureBrowser(ctx); err != nil {
		return fmt.Errorf("chromium rendering failed: %w", err)
	}

	// Plazo propio (pdfPrintTimeout): las esperas de abajo ya estaban
	// acotadas una por una (fuentes 3 s), pero el conjunto no.
	runCtx, cancel := r.newCaptureContext(ctx, pdfPrintTimeout)
	defer cancel()

	// Ejecutar chromedp. htmlContent es el documento completo del usuario
	// (con <img>/<a> potencialmente controlados por el atacante): en vez de
	// escribirlo a un archivo temporal y navegar a file://, se inyecta vía
	// Page.setDocumentContent sobre about:blank. Esto es la corrección de
	// docs/SECURITY_AUDIT_2026-07.md, AL-5 — SanitizeURL bloquea el scheme
	// file: pero permite rutas absolutas con scheme vacío ("/etc/hostname"),
	// que sobre un documento file:// resuelven a file:///etc/hostname y
	// Chrome las rasteriza en el PDF. Sobre about:blank no hay origen
	// file:// contra el cual resolver esas rutas, así que el bypass deja de
	// funcionar sin necesidad de tocar SanitizeURL.
	err := chromedp.Do(runCtx,
		navigateAndSetContent(htmlContent),
		chromedp.WaitReady(chromedp.CSS("body")),
		r.waitForFontsReady(),
		r.checkPageOverflow(opts.OverflowSelector),
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			r.logger.Info("PDF", "Generating PDF...")

			printParams := buildPrintToPDFParams(opts)

			// Generar PDF
			res, err := cdp.Call(ctx, t, page.PrintToPDF, printParams)
			if err != nil {
				return err
			}
			pdfBuf = res.Data
			return nil
		}),
	)

	if err != nil {
		return captureError("PDF", pdfPrintTimeout, ctx, err)
	}

	// Guardar PDF
	if err := os.WriteFile(outputPath, pdfBuf, 0644); err != nil {
		return fmt.Errorf("failed to write PDF: %w", err)
	}

	r.logger.Info("PDF", "✅ PDF generated: %s (%.2f KB)", outputPath, float64(len(pdfBuf))/1024)
	return nil
}

// waitForFontsReady reemplaza el sleep fijo de 500ms que RenderHTMLToPDF
// tenía antes (issue "quitar Chrome del pipeline", Fase 3: el propio código
// documentaba, en slidelang/internal/generator/pdf.go, que ese margen NO
// alcanzaba para que un script CDN cargara+inicializara+dibujara — el
// diagrama quedaba en blanco). La solución real a esa carrera es que PDF ya
// no depende de ningún CDN (ver doclang/internal/generator/pdf.go, que
// ahora fuerza offline-inline); lo único que le queda esperar acá es una
// condición real y acotada: que el navegador termine de cargar cualquier
// @font-face que el CSS de la página declare. Un deck cuyo tema no declara
// fuentes (el caso de los temas embebidos) no tiene ningún @font-face en el
// CSS, así que document.fonts.size===0 ya es verdadera en el primer poll y
// la espera real converge casi de inmediato — pero un tema externo SÍ puede
// declarar fuentes auto-hospedadas (motor-temas-v2.md §2.3, ver
// slidelang/internal/generator/css/themes/fonts.go), en cuyo caso esta
// espera es la que evita que el PDF imprima con la fuente todavía sin
// cargar. font-src 'self' data: ya está abierto en core/renderer/csp.go
// para permitirlo — no hace falta ninguna otra fuente externa. Acotado a 3s
// (mismo criterio de timeout que RenderMapToPNG) para que un caso
// patológico no cuelgue el build entero; si el poll falla o agota el
// timeout, se loguea y se sigue directo a imprimir — nunca bloquea la
// generación del PDF por esto, que era justamente el problema con el sleep
// fijo original: una espera que no puede fallar tampoco puede tener una
// condición real detrás.
func (r *ChromiumRenderer) waitForFontsReady() chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		_, err := chromedp.Poll[chromedp.Void](
			`document.fonts.status === "loaded" || document.fonts.size === 0`,
			chromedp.WithPollingInterval(50*time.Millisecond),
		)(timeoutCtx, t)
		if err != nil {
			r.logger.Info("PDF", "font loading wait ended early (%v), continuing to print anyway", err)
		}
		return nil
	})
}

// checkPageOverflow mide, bajo emulación de print, qué elementos que hacen
// match con selector se desbordan de su propia caja (scrollHeight/Width >
// clientHeight/Width) y avisa por cada uno — issue #175: --format pdf
// (desde #168) trata cada elemento de página como un lienzo de tamaño fijo
// con overflow:hidden en vez de dejar que el contenido fluya, así que un
// desborde hoy se recorta EN SILENCIO. selector vacío = no-op inmediato
// (doclang no lo setea: su PDF fluye entre páginas, no recorta nada que
// medir).
//
// La emulación de print es obligatoria para que la medición signifique
// algo: las reglas que fijan el tamaño de la caja y activan overflow:hidden
// viven en @media print (p. ej. responsive.css de slidelang) — medir en
// modo pantalla mide una caja completamente distinta. Se restaura a "" al
// salir por higiene, aunque printToPDF (más abajo en RenderHTMLToPDF)
// fuerza su propia emulación de print de todos modos, independiente de
// cualquier override previo.
//
// Nunca falla el build: cualquier error de la medición se degrada a un log
// informativo y la función retorna nil, mismo patrón que
// waitForFontsReady — un WARNING perdido es aceptable, un PDF que no se
// genera no lo es.
//
// Límite de precisión confirmado empíricamente (no solo teórico): esta
// medición corre en el DOM en vivo bajo emulación de print
// (Emulation.setEmulatedMedia), un paso DISTINTO del pipeline interno que
// Page.printToPDF usa para el rasterizado final — ambos deberían coincidir,
// y coinciden con margen amplio (confirmado con un desborde de ~2x la
// altura de página: la medición lo detecta y el WARNING sale), pero un
// desborde de apenas una fracción de línea justo en el borde de la página
// puede recortarse en el PDF final sin que esta medición lo capture,
// porque las dos pasadas de layout no son bit-a-bit idénticas a esa
// escala. La tolerancia de 2px de abajo ya asume esto — no es un bug de
// este código, es el techo de fidelidad de medir "por fuera" del pipeline
// real de impresión en vez de inspeccionar el PDF ya generado.
func (r *ChromiumRenderer) checkPageOverflow(selector string) chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		if selector == "" {
			return nil
		}

		if err := setEmulatedMedia(ctx, t, "print"); err != nil {
			r.logger.Info("PDF", "overflow check: no se pudo activar emulación de print (%v), se omite la medición", err)
			return nil
		}
		defer func() {
			_ = setEmulatedMedia(ctx, t, "")
		}()

		selectorJSON, err := json.Marshal(selector)
		if err != nil {
			r.logger.Info("PDF", "overflow check: selector inválido (%v), se omite la medición", err)
			return nil
		}

		type overflowEntry struct {
			Index int    `json:"index"`
			Title string `json:"title"`
		}
		// Tolerancia de 2px: un elemento exactamente del alto de su caja
		// puede reportar una diferencia sub-pixel por redondeo (mismo
		// fenómeno que #163 documentó al calibrar el alto de página), que
		// no es un desborde real de contenido.
		js := fmt.Sprintf(`Array.from(document.querySelectorAll(%s)).reduce(function(acc, el, i) {
			if (el.scrollHeight - el.clientHeight > 2 || el.scrollWidth - el.clientWidth > 2) {
				var idx = el.hasAttribute('data-slide') ? parseInt(el.getAttribute('data-slide'), 10) : i;
				acc.push({index: idx, title: el.getAttribute('data-slide-title') || ''});
			}
			return acc;
		}, [])`, selectorJSON)

		overflowing, err := chromedp.Evaluate[[]overflowEntry](js)(ctx, t)
		if err != nil {
			r.logger.Info("PDF", "overflow check: no se pudo medir (%v), se omite", err)
			return nil
		}

		for _, o := range overflowing {
			// El título va como ARG, nunca interpolado en el format string
			// — sanitizeLogArgs (util/logger.go) neutraliza \r\n/ESC/U+2028
			// solo en los args, y un título de slide puede venir de
			// contenido del autor del deck.
			r.logger.Warn("PDF", "slide %d (%q) content overflows the page and was clipped", o.Index+1, o.Title)
		}

		return nil
	})
}

// RenderMermaidToSVG renderiza un diagrama Mermaid a SVG. ctx acota/cancela
// esta llamada puntual (issue #134/G1d).
func (r *ChromiumRenderer) RenderMermaidToSVG(ctx context.Context, mermaidCode string) (string, error) {
	return r.RenderMermaidToSVGWithTheme(ctx, mermaidCode, renderer.DiagramThemeColors{})
}

// RenderMermaidToSVGWithTheme renders Mermaid with resolved literal theme
// values. It is intentionally separate from the legacy method so zero-value
// callers retain byte-compatible output.
func (r *ChromiumRenderer) RenderMermaidToSVGWithTheme(ctx context.Context, mermaidCode string, theme renderer.DiagramThemeColors) (string, error) {
	r.logger.Info("MERMAID", "Rendering diagram to SVG...")

	html := buildMermaidSVGHTML(mermaidCode, theme)

	var svgContent string

	if err := r.ensureBrowser(ctx); err != nil {
		return "", fmt.Errorf("mermaid rendering failed: %w", err)
	}

	runCtx, cancel := r.newCaptureContext(ctx, captureTimeout)
	defer cancel()

	// Renderizar con Chromium. Se inyecta el HTML vía about:blank +
	// Page.setDocumentContent en vez de escribir un archivo temporal y
	// navegar a file:// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	err := chromedp.Do(runCtx,
		navigateAndSetContent(html),
		chromedp.WaitVisible(chromedp.CSS(".mermaid svg")),
		chromedp.Sleep(200*time.Millisecond), // Esperar renderizado completo
		store(&svgContent, chromedp.OuterHTML(chromedp.CSS(".mermaid svg"))),
	)

	if err != nil {
		return "", captureError("mermaid", captureTimeout, ctx, err)
	}

	r.logger.Info("MERMAID", "✅ SVG rendered (%.2f KB)", float64(len(svgContent))/1024)
	return svgContent, nil
}

// RenderMathToSVG renderiza una ecuación LaTeX a SVG vía MathJax (issue
// #239-B). ctx acota/cancela esta llamada puntual (mismo patrón que
// RenderMermaidToSVG). MathJax tipografía \[...\] de forma automática al
// cargar (comportamiento default del bundle tex-svg combinado, sin config
// adicional) y reemplaza el nodo por un <mjx-container><svg>...</svg>
// </mjx-container> — estructura de salida estable de MathJax v3+ para el
// renderer SVG.
func (r *ChromiumRenderer) RenderMathToSVG(ctx context.Context, latex string) (string, error) {
	return r.RenderMathToSVGWithTheme(ctx, latex, renderer.DiagramThemeColors{})
}

// RenderMathToSVGWithTheme renders MathJax with the diagram surface/text
// colors. Formulas intentionally share the neutral visual contract so both
// DSLs use one set of resolved values in offline output.
func (r *ChromiumRenderer) RenderMathToSVGWithTheme(ctx context.Context, latex string, theme renderer.DiagramThemeColors) (string, error) {
	r.logger.Info("MATH", "Rendering equation to SVG...")

	if err := r.ensureBrowser(ctx); err != nil {
		return "", fmt.Errorf("math rendering failed: %w", err)
	}

	var svgContent string
	err := r.renderMath(ctx,
		// Se inyecta el HTML vía about:blank + Page.setDocumentContent en
		// vez de escribir un archivo temporal y navegar a file://
		// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
		navigateAndSetContent(buildMathSVGPage(latex, theme)),
		waitRenderState(mathStateJS),
		chromedp.Sleep(200*time.Millisecond), // Esperar tipografiado completo
		store(&svgContent, chromedp.OuterHTML(chromedp.CSS("mjx-container svg"))),
	)
	if err != nil {
		return "", err
	}
	svgContent = embedMathThemeInSVG(svgContent, theme)

	r.logger.Info("MATH", "✅ SVG rendered (%.2f KB)", float64(len(svgContent))/1024)
	return svgContent, nil
}

// renderMath corre una captura de math en su propio tab, con su plazo y con
// los archivos de MathJax servidos desde el binario (serveMathAssets).
func (r *ChromiumRenderer) renderMath(ctx context.Context, steps ...chromedp.Action[chromedp.Void]) error {
	runCtx, cancel := r.newCaptureContext(ctx, captureTimeout)
	defer cancel()
	if err := chromedp.Do(runCtx, append([]chromedp.Action[chromedp.Void]{serveMathAssets()}, steps...)...); err != nil {
		return captureError("math", captureTimeout, ctx, err)
	}
	return nil
}

// RenderMathToPNG renderiza una ecuación LaTeX a PNG (issue #239-B, uso
// principal: rasterizar para DOCX, que no puede embeber SVG). ctx acota/
// cancela esta llamada puntual — mismo patrón que RenderMermaidToPNG.
func (r *ChromiumRenderer) RenderMathToPNG(ctx context.Context, latex string, width, height int) ([]byte, error) {
	return r.RenderMathToPNGWithTheme(ctx, latex, width, height, renderer.DiagramThemeColors{})
}

// RenderMathToPNGWithTheme is the PNG counterpart of
// RenderMathToSVGWithTheme.
func (r *ChromiumRenderer) RenderMathToPNGWithTheme(ctx context.Context, latex string, width, height int, theme renderer.DiagramThemeColors) ([]byte, error) {
	r.logger.Info("MATH", "Rendering equation to PNG...")

	if err := r.ensureBrowser(ctx); err != nil {
		return nil, fmt.Errorf("math rendering failed: %w", err)
	}

	var pngData []byte
	err := r.renderMath(ctx,
		navigateAndSetContent(buildMathPNGPage(latex, width, height, theme)),
		waitRenderState(mathStateJS),
		chromedp.Sleep(200*time.Millisecond),
		bringTabToFront(),
		store(&pngData, chromedp.Screenshot(chromedp.ID("#mathContainer"))),
	)
	if err != nil {
		return nil, err
	}

	r.logger.Info("MATH", "✅ PNG rendered (%.2f KB)", float64(len(pngData))/1024)
	return pngData, nil
}

// MathPNGDeviceScale es el deviceScaleFactor de RenderMathToFittedPNGWithTheme:
// 2 px por px CSS, como mermaidPNGDeviceScale. El tamaño natural de la
// ecuación es el del PNG entre este factor (a 96 px CSS por pulgada).
const MathPNGDeviceScale = 2

// mathPNGPaddingPx es el margen (px CSS) que se deja alrededor de la caja
// del <svg> al recortar, para que los trazos que sobresalen un poco de la
// caja (subíndices, el gancho de una integral) no queden cortados.
const mathPNGPaddingPx = 4

// RenderMathToFittedPNGWithTheme rasteriza una ecuación a PNG recortado a la
// caja de la ecuación, a MathPNGDeviceScale px por px CSS, con el mismo
// tamaño de letra que la salida HTML. RenderMathToPNGWithTheme, en cambio,
// captura un contenedor fijo de width×height con la ecuación chica en el
// medio; se conserva tal cual porque slidelang (PPTX) la escala al ancho de
// la lámina.
func (r *ChromiumRenderer) RenderMathToFittedPNGWithTheme(ctx context.Context, latex string, theme renderer.DiagramThemeColors) ([]byte, error) {
	r.logger.Info("MATH", "Rendering equation to PNG...")

	if err := r.ensureBrowser(ctx); err != nil {
		return nil, fmt.Errorf("math rendering failed: %w", err)
	}

	var pngData []byte
	var box captureBox
	err := r.renderMath(ctx,
		setDeviceMetricsOverride(1200, 600, MathPNGDeviceScale),
		navigateAndSetContent(buildMathSVGPage(latex, theme)),
		waitRenderState(mathStateJS),
		chromedp.Sleep(200*time.Millisecond),
		store(&box, chromedp.Evaluate[captureBox](fmt.Sprintf(`(() => { const r = document.querySelector('mjx-container svg').getBoundingClientRect(); const p = %d; return {x: r.left + window.scrollX - p, y: r.top + window.scrollY - p, width: r.width + 2*p, height: r.height + 2*p}; })()`, mathPNGPaddingPx))),
		bringTabToFront(),
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			if box.Width <= 2*mathPNGPaddingPx || box.Height <= 2*mathPNGPaddingPx {
				return fmt.Errorf("MathJax produced an empty equation (%.0fx%.0f)", box.Width, box.Height)
			}
			buf, err := captureScreenshot(page.CaptureScreenshotParams{
				Format:                page.CaptureScreenshotFormatPng,
				CaptureBeyondViewport: new(true),
				Clip: &page.Viewport{
					X:      math.Max(0, math.Floor(box.X)),
					Y:      math.Max(0, math.Floor(box.Y)),
					Width:  math.Ceil(box.Width),
					Height: math.Ceil(box.Height),
					Scale:  1,
				},
			})(ctx, t)
			if err != nil {
				return err
			}
			pngData = buf
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	r.logger.Info("MATH", "✅ PNG rendered (%.2f KB)", float64(len(pngData))/1024)
	return pngData, nil
}

// RenderMermaidToPNG renderiza un diagrama Mermaid a PNG. ctx acota/cancela
// esta llamada puntual (issue #134/G1d).
func (r *ChromiumRenderer) RenderMermaidToPNG(ctx context.Context, mermaidCode string, width, height int) ([]byte, error) {
	return r.RenderMermaidToPNGWithTheme(ctx, mermaidCode, width, height, renderer.DiagramThemeColors{})
}

// RenderMermaidToPNGWithTheme is the PNG counterpart of
// RenderMermaidToSVGWithTheme.
//
// width es el ancho de layout (px CSS) que ve mermaid antes de dibujar; height
// solo es el alto inicial de la ventana. El PNG es la caja real del <svg>
// (sin el margen de un contenedor fijo) a mermaidPNGDeviceScale px por px
// CSS, así que su tamaño natural es ancho/alto del PNG entre ese factor.
// Antes se capturaba un #mermaidContainer fijo de width×height: un diagrama
// chico quedaba como una isla en un PNG mayormente blanco, y uno más alto
// que el contenedor se recortaba.
func (r *ChromiumRenderer) RenderMermaidToPNGWithTheme(ctx context.Context, mermaidCode string, width, height int, theme renderer.DiagramThemeColors) ([]byte, error) {
	r.logger.Info("MERMAID", "Rendering diagram to PNG...")

	html := buildMermaidPNGHTML(mermaidCode, width, theme)

	var pngData []byte

	if err := r.ensureBrowser(ctx); err != nil {
		return nil, fmt.Errorf("mermaid rendering failed: %w", err)
	}

	runCtx, cancel := r.newCaptureContext(ctx, captureTimeout)
	defer cancel()

	// Renderizar con Chromium - esperar a que el renderizado esté completo.
	// Se inyecta el HTML vía about:blank + Page.setDocumentContent en vez
	// de escribir un archivo temporal y navegar a file://
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	viewportH := int64(height)
	if viewportH < 600 {
		viewportH = 600
	}
	var box captureBox
	err := chromedp.Do(runCtx,
		setDeviceMetricsOverride(int64(width+16), viewportH, mermaidPNGDeviceScale),
		navigateAndSetContent(html),
		chromedp.WaitVisible(chromedp.CSS(".mermaid svg")),
		chromedp.WaitReady(chromedp.CSS("#renderComplete[data-ready='true']")),
		chromedp.Sleep(200*time.Millisecond), // Buffer adicional
		store(&box, chromedp.Evaluate[captureBox](`(() => { const r = document.querySelector('.mermaid svg').getBoundingClientRect(); return {x: r.left + window.scrollX, y: r.top + window.scrollY, width: r.width, height: r.height}; })()`)),
		bringTabToFront(),
		// Captura de la caja del <svg> en coordenadas de página, con
		// captureBeyondViewport: un diagrama más alto que la ventana sale
		// entero, sin redimensionar la ventana.
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			if box.Width <= 0 || box.Height <= 0 {
				return fmt.Errorf("mermaid produced an empty diagram (%.0fx%.0f)", box.Width, box.Height)
			}
			buf, err := captureScreenshot(page.CaptureScreenshotParams{
				Format:                page.CaptureScreenshotFormatPng,
				CaptureBeyondViewport: new(true),
				Clip: &page.Viewport{
					X:      math.Floor(box.X),
					Y:      math.Floor(box.Y),
					Width:  math.Ceil(box.Width),
					Height: math.Ceil(box.Height),
					Scale:  1,
				},
			})(ctx, t)
			if err != nil {
				return err
			}
			pngData = buf
			return nil
		}),
	)

	if err != nil {
		return nil, captureError("mermaid", captureTimeout, ctx, err)
	}

	r.logger.Info("MERMAID", "✅ PNG rendered (%.2f KB)", float64(len(pngData))/1024)
	return pngData, nil
}

// RenderChartToPNG renderiza un gráfico Chart.js a PNG. ctx acota/cancela
// esta llamada puntual (issue #134/G1d).
func (r *ChromiumRenderer) RenderChartToPNG(ctx context.Context, chartConfig string, width, height int) ([]byte, error) {
	return r.RenderChartToPNGWithSurface(ctx, chartConfig, width, height, "")
}

// RenderChartToPNGWithSurface es RenderChartToPNG con el token chart-surface
// de motor-temas-v2.md §2.2. Entrada NUEVA en vez de un parámetro más:
// RenderChartToPNG la llama doclang (docx.go) por nombre y CI corre
// workspace-integration contra el core del árbol. "" reproduce el fondo
// blanco de siempre.
//
// Solo recibe la superficie y no el ChartThemeColors completo porque es lo
// único que esta capa puede honrar: grid/axis/label son opciones de Chart.js
// y viajan DENTRO de chartConfig (ver applyScaleThemeColors), no por acá. El
// fondo, en cambio, no es una opción de Chart.js — el canvas es transparente
// y lo que se ve es el body de la página, así que tiene que entrar por el
// HTML.
func (r *ChromiumRenderer) RenderChartToPNGWithSurface(ctx context.Context, chartConfig string, width, height int, surface string) ([]byte, error) {
	r.logger.Info("CHART", "Rendering chart to PNG...")

	html := buildChartHTML(chartConfig, width, height, surface)

	var pngData []byte

	if err := r.ensureBrowser(ctx); err != nil {
		return nil, fmt.Errorf("chart rendering failed: %w", err)
	}

	// Tab propio con plazo (newCaptureContext): una página que no avisa
	// "listo" ni "error" termina en timeout, nunca en una espera sin fin.
	runCtx, cancel := r.newCaptureContext(ctx, captureTimeout)
	defer cancel()

	// Renderizar con Chromium - esperar a que el renderizado esté completo.
	// Se inyecta el HTML vía about:blank + Page.setDocumentContent en vez
	// de escribir un archivo temporal y navegar a file://
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	err := chromedp.Do(runCtx,
		navigateAndSetContent(html),
		waitRenderState(renderStateJS),
		chromedp.Sleep(200*time.Millisecond), // Buffer adicional
		bringTabToFront(),
		// La captura es el <canvas>, no el contenedor: un config JSON sin
		// maintainAspectRatio: false deja que Chart.js dibuje a su
		// proporción por defecto (2:1), así que en el contenedor de 800x600
		// el canvas medía 800x400 y la imagen llevaba 200 px en blanco
		// debajo de la gráfica. Con maintainAspectRatio: false (los configs
		// que arma GenerateChartConfig) el canvas llena el contenedor y la
		// captura es la misma de antes.
		store(&pngData, chromedp.Screenshot(chromedp.ID("#myChart"))),
	)

	if err != nil {
		return nil, captureError("chart", captureTimeout, ctx, err)
	}

	r.logger.Info("CHART", "✅ PNG rendered (%.2f KB)", float64(len(pngData))/1024)
	return pngData, nil
}

// RenderMapToPNG renderiza un mapa Leaflet a PNG. ctx acota/cancela esta
// llamada puntual (issue #134/G1d), además del timeout ajustado por zoom.
func (r *ChromiumRenderer) RenderMapToPNG(ctx context.Context, mapConfig renderer.MapConfig, width, height int) ([]byte, error) {
	r.logger.Info("MAP", "Rendering map to PNG...")
	if err := checkMapSize(width, height); err != nil {
		return nil, err
	}

	// Generar HTML con Leaflet + flag de renderizado completo
	html := r.generateLeafletHTML(mapConfig, width, height)

	var pngData []byte

	if err := r.ensureBrowser(ctx); err != nil {
		return nil, fmt.Errorf("map rendering failed: %w", err)
	}

	// IMPORTANTE: Crear un nuevo tab context para esta renderización
	// Esto evita conflictos cuando se renderizan múltiples mapas
	tabCtx, tabCancel := chromedp.NewContext(r.ctx)
	defer tabCancel()

	// El plazo cubre el de la página (mapPageFallback), la espera de los
	// tiles en vuelo y la captura (mapCaptureTimeout).
	timeout := mapCaptureTimeout(mapConfig.Zoom)

	timeoutCtx, timeoutCancel := context.WithTimeout(tabCtx, timeout)
	defer timeoutCancel()

	runCtx, cancel := withCallerCancel(timeoutCtx, ctx)
	defer cancel()

	// Renderizar con Chromium - esperar a que tiles y marcadores estén cargados.
	// Se inyecta el HTML vía about:blank + Page.setDocumentContent en vez
	// de escribir un archivo temporal y navegar a file://
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	var tiles mapTileStats
	err := chromedp.Do(runCtx,
		serveMapTiles(mapConfig, &tiles),
		navigateAndSetContent(html),
		// Esperar a que window.mapReady sea true (establecido por el
		// JavaScript), o a que la página reporte un error: sin red, Leaflet
		// no carga, `L.map` lanza y eso corta la espera de inmediato en vez
		// de agotar el plazo por zoom.
		waitRenderState(mapStateJS),
		// Ningún tile en vuelo al capturar (ver waitTilesSettled).
		waitTilesSettled(&tiles),
		bringTabToFront(),
		store(&pngData, chromedp.Screenshot(chromedp.ID("#mapContainer"))),
	)

	if err != nil {
		return nil, captureError("map", timeout, ctx, err)
	}
	if err := tiles.err(); err != nil {
		return nil, fmt.Errorf("map rendering failed: %w", err)
	}

	r.logger.Info("MAP", "✅ PNG rendered (%.2f KB)", float64(len(pngData))/1024)
	return pngData, nil
}

// RenderChartToWebP renderiza un gráfico Chart.js directamente a WebP. ctx
// acota/cancela esta llamada puntual (issue #134/G1d).
func (r *ChromiumRenderer) RenderChartToWebP(ctx context.Context, chartConfig string, width, height int, quality int) ([]byte, error) {
	return r.RenderChartToWebPWithSurface(ctx, chartConfig, width, height, quality, "")
}

// RenderChartToWebPWithSurface — ver RenderChartToPNGWithSurface. Existe para
// que el fondo no se pierda cuando el fetcher está configurado en WebP: sin
// esto, el mismo tema daría un PNG con fondo temátizado y un WebP blanco.
func (r *ChromiumRenderer) RenderChartToWebPWithSurface(ctx context.Context, chartConfig string, width, height int, quality int, surface string) ([]byte, error) {
	r.logger.Info("CHART", "Rendering chart to WebP (quality: %d)...", quality)

	html := buildChartHTML(chartConfig, width, height, surface)

	var webpData []byte

	if err := r.ensureBrowser(ctx); err != nil {
		return nil, fmt.Errorf("chart rendering to WebP failed: %w", err)
	}

	runCtx, cancel := r.newCaptureContext(ctx, captureTimeout)
	defer cancel()

	var canvasHeight float64
	// Renderizar con Chromium usando CDP para captura WebP - esperar renderizado completo.
	// Se inyecta el HTML vía about:blank + Page.setDocumentContent en vez
	// de escribir un archivo temporal y navegar a file://
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78). bringTabToFront: mismo
	// cuelgue de Page.captureScreenshot en un tab inactivo que #450 arregló
	// en el camino PNG.
	err := chromedp.Do(runCtx,
		navigateAndSetContent(html),
		waitRenderState(renderStateJS),
		chromedp.Sleep(200*time.Millisecond), // Buffer adicional
		bringTabToFront(),
		// Alto del <canvas> (ver el camino PNG: Chart.js puede dibujar más
		// bajo que el contenedor si conserva su proporción).
		store(&canvasHeight, chromedp.Evaluate[float64](`document.getElementById('myChart').getBoundingClientRect().height`)),
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			clipHeight := float64(height)
			if canvasHeight > 0 && canvasHeight < clipHeight {
				clipHeight = math.Ceil(canvasHeight)
			}
			// Usar page.CaptureScreenshot con formato WebP
			buf, err := captureScreenshot(page.CaptureScreenshotParams{
				Format:  page.CaptureScreenshotFormatWebp,
				Quality: webpQuality(quality),
				Clip: &page.Viewport{
					X:      0,
					Y:      0,
					Width:  float64(width),
					Height: clipHeight,
					Scale:  1,
				},
			})(ctx, t)
			if err != nil {
				return err
			}
			webpData = buf
			return nil
		}),
	)

	if err != nil {
		return nil, captureError("chart (WebP)", captureTimeout, ctx, err)
	}

	r.logger.Info("CHART", "✅ WebP rendered (%.2f KB)", float64(len(webpData))/1024)
	return webpData, nil
}

// RenderMapToWebP renderiza un mapa Leaflet directamente a WebP. ctx
// acota/cancela esta llamada puntual (issue #134/G1d).
func (r *ChromiumRenderer) RenderMapToWebP(ctx context.Context, mapConfig renderer.MapConfig, width, height int, quality int) ([]byte, error) {
	if err := checkMapSize(width, height); err != nil {
		return nil, err
	}
	r.logger.Info("MAP", "Rendering map to WebP (quality: %d)...", quality)

	// Generar HTML con Leaflet + flag de renderizado completo
	html := r.generateLeafletHTML(mapConfig, width, height)

	var webpData []byte

	if err := r.ensureBrowser(ctx); err != nil {
		return nil, fmt.Errorf("map rendering to WebP failed: %w", err)
	}

	// IMPORTANTE: Crear un nuevo tab context para esta renderización
	// Esto evita conflictos cuando se renderizan múltiples mapas
	tabCtx, tabCancel := chromedp.NewContext(r.ctx)
	defer tabCancel()

	// Mismo plazo que RenderMapToPNG (mapCaptureTimeout).
	timeout := mapCaptureTimeout(mapConfig.Zoom)
	timeoutCtx, timeoutCancel := context.WithTimeout(tabCtx, timeout)
	defer timeoutCancel()

	runCtx, cancel := withCallerCancel(timeoutCtx, ctx)
	defer cancel()

	// Renderizar con Chromium usando CDP para captura WebP - esperar renderizado completo.
	// Se inyecta el HTML vía about:blank + Page.setDocumentContent en vez
	// de escribir un archivo temporal y navegar a file://
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	var tiles mapTileStats
	err := chromedp.Do(runCtx,
		serveMapTiles(mapConfig, &tiles),
		navigateAndSetContent(html),
		// generateLeafletHTML publica window.mapReady, no un
		// #renderComplete (que este camino esperaba y la página nunca
		// creó: todo mapa WebP agotaba los 30 s).
		waitRenderState(mapStateJS),
		waitTilesSettled(&tiles),
		bringTabToFront(),
		// Usar page.CaptureScreenshot con formato WebP
		store(&webpData, captureScreenshot(page.CaptureScreenshotParams{
			Format:  page.CaptureScreenshotFormatWebp,
			Quality: webpQuality(quality),
			Clip: &page.Viewport{
				X:      0,
				Y:      0,
				Width:  float64(width),
				Height: float64(height),
				Scale:  1,
			},
		})),
	)

	if err != nil {
		return nil, captureError("map (WebP)", timeout, ctx, err)
	}
	if err := tiles.err(); err != nil {
		return nil, fmt.Errorf("map rendering to WebP failed: %w", err)
	}

	r.logger.Info("MAP", "✅ WebP rendered (%.2f KB)", float64(len(webpData))/1024)
	return webpData, nil
}

// leafletAttribution es el crédito del mapa como literal de string JS
// entre comillas simples: el de OpenStreetMap, o con un servidor propio el
// de MapConfig.WithTileAttribution (vacío, ninguno), escapado como HTML
// (Leaflet lo inserta como HTML) y como JS.
func leafletAttribution(config renderer.MapConfig) string {
	text := "© OpenStreetMap contributors"
	if tmpl, _ := config.TileServer(); tmpl != "" {
		text = config.TileAttribution()
	}
	return jsSingleQuoted(renderer.EscapeHTML(text))
}

// jsSingleQuoted escapa s para un string JS entre comillas simples dentro
// de un <script>: barra invertida, comilla, saltos y "/" (para que nunca
// aparezca "</script").
func jsSingleQuoted(s string) string {
	return strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\n", `\n`, "\r", `\r`, "\t", `\t`, "/", `\/`, "\u2028", `\u2028`, "\u2029", `\u2029`).Replace(s)
}

// generateLeafletHTML genera HTML con mapa Leaflet
func (r *ChromiumRenderer) generateLeafletHTML(config renderer.MapConfig, width, height int) string {
	markersJS := ""
	for i, marker := range config.Markers {
		color := renderer.SanitizeLeafletMarkerColor(marker.Color)

		// Escapar el label para evitar problemas con caracteres especiales en JavaScript
		// IMPORTANTE: Escapar backslash PRIMERO antes de otros escapes
		label := strings.ReplaceAll(marker.Label, "\\", "\\\\")
		label = strings.ReplaceAll(label, "'", "\\'")  // Single quote para el string JS
		label = strings.ReplaceAll(label, "\n", "\\n") // Newlines
		label = strings.ReplaceAll(label, "\r", "\\r") // Carriage returns
		label = strings.ReplaceAll(label, "\t", "\\t") // Tabs
		// Escapar "/" como "\/" (JS válido, decodifica igual a "/"): un
		// label con literal "</script>" cerraría el <script> del parser
		// HTML antes de que corra el JS, inyectando un <script> nuevo — el
		// tokenizer de HTML busca la secuencia de bytes "</script"
		// case-insensitive, sin decodificar JS/entidades, así que romper
		// cualquier "/" lo neutraliza. Ver docs/SECURITY_AUDIT_2026-07.md,
		// CR-6 (issue #24).
		label = strings.ReplaceAll(label, "/", "\\/")

		markersJS += fmt.Sprintf(`
        L.marker([%f, %f], {
            icon: L.icon({
                iconUrl: '%s',
                shadowUrl: '%s',
                iconSize: [25, 41],
                iconAnchor: [12, 41],
                popupAnchor: [1, -34],
                shadowSize: [41, 41]
            })
        }).addTo(map).bindPopup('%s');
        `, marker.Lat, marker.Lng, leafletMarkerDataURI(color), leafletShadowDataURI, label)

		if i < len(config.Markers)-1 {
			markersJS += "\n"
		}
	}

	// Los tiles los entrega Go (serveMapTiles) desde el servidor real de
	// config: el de siempre o el de MapConfig.WithTileServer.
	tileLayer := mapTileProxyTemplate

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="%s">
    `+renderErrorHookJS+`
    %s
    <style>
        body { margin: 0; padding: 0; }
        #mapContainer { width: %dpx; height: %dpx; }
        #map { width: 100%%; height: 100%%; }
    </style>
</head>
<body>
    <div id="mapContainer">
        <div id="map"></div>
    </div>
    <script>
        if (typeof L === 'undefined') {
            window.__renderError = 'Leaflet did not load';
        }
    </script>
    <script>
        // Sin animación de entrada: un tile recién llegado se captura entero.
        const map = L.map('map', {fadeAnimation: false}).setView([%f, %f], %d);
        const tileLayer = L.tileLayer('%s', {
            attribution: '%s',
            maxZoom: 19
        }).addTo(map);
        %s
        
        // Estrategia mejorada: combinar evento 'load' con conteo de tiles
        let isReady = false;
        let tilesLoading = 0;
        let tilesLoaded = 0;
        let loadCheckInterval;
        
        // Rastrear tiles que se están cargando
        tileLayer.on('tileloadstart', () => {
            tilesLoading++;
        });
        
        tileLayer.on('tileload', () => {
            tilesLoaded++;
        });
        
        tileLayer.on('tileerror', () => {
            tilesLoaded++;  // Contar errores como "completados" para no esperar indefinidamente
        });
        
        // Evento 'load' oficial de Leaflet (todos los tiles visibles cargados)
        tileLayer.on('load', () => {
            if (!isReady) {
                isReady = true;
                clearInterval(loadCheckInterval);
                setTimeout(() => {
                    window.mapReady = true;
                }, 100);
            }
        });
        
        // Verificar periódicamente si todos los tiles iniciados han terminado
        loadCheckInterval = setInterval(() => {
            if (!isReady && tilesLoading > 0 && tilesLoaded >= tilesLoading) {
                isReady = true;
                clearInterval(loadCheckInterval);
                console.log('All tiles loaded: ' + tilesLoaded + '/' + tilesLoading);
                setTimeout(() => {
                    window.mapReady = true;
                }, 100);
            }
        }, 200);
        
        // Plazo de la página (mapPageFallback en Go): al menos un timeout
        // de tile, para no capturar con tiles todavía en camino.
        const fallbackTimeout = %d;
        
        setTimeout(() => {
            if (!isReady) {
                isReady = true;
                clearInterval(loadCheckInterval);
                console.log('Fallback timeout after ' + (fallbackTimeout/1000) + 's (tiles: ' + tilesLoaded + '/' + tilesLoading + ')');
                window.mapReady = true;
            }
        }, fallbackTimeout);
        
        // Inicializar el tamaño del mapa correctamente
        setTimeout(() => {
            map.invalidateSize();
        }, 100);
    </script>
</body>
</html>`, mapRenderCSP, leafletAssetTags, width, height, config.CenterLat, config.CenterLng, config.Zoom, tileLayer, leafletAttribution(config), markersJS, mapPageFallback(config.Zoom).Milliseconds())
}

// Close cierra el renderer y libera recursos
func (r *ChromiumRenderer) Close() {
	if r.cancel != nil {
		r.cancel()
	}
}

// PDFOptions es un alias de renderer.PDFOptions (issue "quitar Chrome del
// pipeline"): el tipo se movió a core/renderer porque describe una página,
// no un navegador, y este alias evita romper callers existentes que siguen
// escribiendo chromium.PDFOptions. Código nuevo debería usar
// renderer.PDFOptions directamente.
type PDFOptions = renderer.PDFOptions

// DefaultPDFOptions retorna opciones por defecto (A4, portrait). Delega en
// renderer.DefaultPDFOptions — ver el alias PDFOptions arriba.
func DefaultPDFOptions() PDFOptions {
	return renderer.DefaultPDFOptions()
}

// nonNegativeMargin clampea un margen negativo a 0. CDP rechaza un margen
// negativo con un error duro ("top margin is negative", -32602) en vez de
// ignorarlo — a diferencia de los demás campos de PDFOptions, que solo
// necesitan protegerse contra el cero (ver el comentario de Scale más
// abajo), un margen negativo no tiene ninguna interpretación válida que
// preservar, así que clampear es más seguro que dejarlo pasar.
func nonNegativeMargin(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}

// buildPrintToPDFParams arma los parámetros de Page.printToPDF a partir de
// PDFOptions. Extraído de RenderHTMLToPDF (issue #62 prerequisito) para que
// GenerateTaggedPDF y el resto de opciones sean verificables sin un Chromium
// real: PrintToPDF() inicializaba GenerateTaggedPDF en false explícitamente
// (cdproto page.go), así que sin este override ningún PDF de este toolchain
// llevaba árbol de tags, ni /Lang, ni orden de lectura verificable por una
// regla o un lector de pantalla. Activarlo hace ALCANZABLES las cláusulas
// PDF/UA que el crosswalk cita — no las verifica: un árbol de tags mal
// formado sigue siendo posible, eso es trabajo de PDF/UA aparte.
//
// Desde cdproto v0.157 (chromedp 0.21) los booleanos y los márgenes son
// punteros con omitempty: nil es "no mandar" y Chrome usa su default. Para
// que el JSON siga siendo el de antes, todos los booleanos y los cuatro
// márgenes van siempre con valor: el viejo PrintToPDF() mandaba
// landscape/displayHeaderFooter/generateDocumentOutline en false y los
// márgenes sin omitempty. Un margen nil no es lo mismo que 0: Chrome le
// pondría su default de 1 cm.
func buildPrintToPDFParams(opts PDFOptions) page.PrintToPDFParams {
	printParams := page.PrintToPDFParams{
		Landscape:               new(false),
		DisplayHeaderFooter:     new(false),
		PrintBackground:         new(true),
		PreferCSSPageSize:       new(true),
		GenerateTaggedPDF:       new(true),
		GenerateDocumentOutline: new(false),
	}

	// Margins. Issue #165: este bloque tenía un guard `> 0` que, a
	// diferencia del de Scale más abajo, resultó ser un no-op para
	// CUALQUIER valor no-negativo — cdproto declaraba (antes de v0.157)
	// MarginTop/Bottom/Left/Right como float64 SIN `omitempty`, y su constructor
	// PrintToPDF() no les siembra ningún default distinto de cero, así que
	// el JSON que sale hacia Chromium lleva "marginTop":0 (y equivalentes)
	// exista o no la llamada a WithMarginTop — llamarlo con 0 y no
	// llamarlo producen el mismo struct. El guard no cambiaba una sola
	// margen en ningún PDF que este toolchain haya generado nunca
	// (confirmado: el único caller de producción es slidesPDFOptions(),
	// cuyos Margin* ya estaban en su cero-valor).
	//
	// Pero el guard SÍ hacía algo para un valor negativo: lo descartaba en
	// silencio, dejando el margen en su cero-valor de cdproto. Quitarlo sin
	// más deja pasar un negativo tal cual hacia CDP, que lo rechaza con un
	// error duro ("top margin is negative") — un doclang con
	// `page.margins.top: "-1in"` en su front matter pasaba de generar un
	// PDF (con margen 0) a tronar el build entero (hallazgo de code
	// review). nonNegativeMargin conserva la propiedad que hacía cero-riesgo
	// a este cambio — un valor negativo se clampea a 0 en vez de
	// propagarse — sin resucitar el guard `> 0` que también descartaba
	// valores positivos legítimos vía `omitempty` inexistente.
	printParams.MarginTop = new(nonNegativeMargin(opts.MarginTop))
	printParams.MarginBottom = new(nonNegativeMargin(opts.MarginBottom))
	printParams.MarginLeft = new(nonNegativeMargin(opts.MarginLeft))
	printParams.MarginRight = new(nonNegativeMargin(opts.MarginRight))

	// Paper size
	if opts.PaperWidth > 0 && opts.PaperHeight > 0 {
		printParams.PaperWidth = opts.PaperWidth
		printParams.PaperHeight = opts.PaperHeight
	}

	// Landscape
	if opts.Landscape {
		printParams.Landscape = new(true)
	}

	// Header/Footer template
	if opts.DisplayHeaderFooter {
		printParams.DisplayHeaderFooter = new(true)
		printParams.HeaderTemplate = opts.HeaderTemplate
		printParams.FooterTemplate = opts.FooterTemplate
	}

	// Scale. Hasta este punto PDFOptions.Scale era un campo público INERTE:
	// DefaultPDFOptions() lo inicializaba en 1.0 y slidesPDFOptions() lo
	// seteaba explícitamente, pero esta función nunca llamaba WithScale, así
	// que un caller que pidiera Scale: 0.5 obtenía un PDF a tamaño completo
	// sin ningún aviso.
	//
	// El contrato es: SOLO el cero significa "no especificado". Un
	// PDFOptions{} literal deja el default de cdproto en paz en vez de mandar
	// un 0 explícito, que CDP rechaza. Cualquier OTRO valor se manda tal
	// cual, incluido uno negativo o fuera del rango [0.1, 2] — quien valida
	// el rango y produce un error legible es ValidatePDFScale, llamado por
	// RenderHTMLToPDF antes de llegar acá. Un `> 0` en su lugar volvería a
	// descartar Scale: -1 en silencio, que es justo el bug que este bloque
	// vino a arreglar.
	if opts.Scale != 0 {
		printParams.Scale = opts.Scale
	}

	return printParams
}

// Rango que CDP acepta en Page.printToPDF.scale; fuera de él la llamada falla
// con un error del protocolo, no con un PDF raro.
const (
	minPDFScale = 0.1
	maxPDFScale = 2.0
)

// ValidatePDFScale rechaza un Scale fuera del rango que acepta CDP, con un
// mensaje que dice el rango — en vez de dejar que el caller reciba el error
// crudo del protocolo, que no lo menciona. El cero pasa: significa "no
// especificado" (ver buildPrintToPDFParams).
func ValidatePDFScale(scale float64) error {
	if scale == 0 {
		return nil
	}
	if scale < minPDFScale || scale > maxPDFScale {
		return fmt.Errorf("invalid PDF scale %v: must be 0 (unset) or between %v and %v", scale, minPDFScale, maxPDFScale)
	}
	return nil
}
