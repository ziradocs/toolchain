// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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
	// mermaidAndChartRenderCSP cubre tanto mermaid como chart: ambos cargan
	// su script únicamente desde jsdelivr y no necesitan imágenes externas.
	mermaidAndChartRenderCSP = "default-src 'none'; script-src https://cdn.jsdelivr.net 'unsafe-inline'; " +
		"style-src 'unsafe-inline'; img-src data:; connect-src 'none'; object-src 'none'; base-uri 'none';"
	mapRenderCSP = "default-src 'none'; script-src https://unpkg.com 'unsafe-inline'; " +
		"style-src https://unpkg.com 'unsafe-inline'; " +
		"img-src https://*.tile.openstreetmap.org https://server.arcgisonline.com " +
		"https://raw.githubusercontent.com https://cdnjs.cloudflare.com data:; " +
		"connect-src 'none'; object-src 'none'; base-uri 'none';"
)

// buildMermaidSVGHTML arma la página temporal usada para rasterizar un
// diagrama Mermaid a SVG. El contenido es dato del usuario: se HTML-escapa
// antes de insertarlo como texto del nodo ".mermaid" (Mermaid lee
// textContent, así que escapar no rompe el render) y se fija
// securityLevel:'strict' + htmlLabels:false para bloquear HTML/script
// embebido dentro del propio diagrama. Ver docs/SECURITY_AUDIT_2026-07.md,
// CR-6/AL-6 (issue #24).
func buildMermaidSVGHTML(mermaidCode string) string {
	return `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="` + mermaidAndChartRenderCSP + `">
    ` + renderer.MermaidCDNScriptTag + `
    <style>
        body { margin: 0; padding: 20px; background: white; }
        .mermaid { display: inline-block; }
    </style>
</head>
<body>
    ` + renderer.BuildMermaidDiv(mermaidCode) + `
    <script>
        mermaid.initialize(` + renderer.MermaidInitConfigJS(true) + `);
    </script>
</body>
</html>`
}

// buildMathSVGHTML arma la página temporal usada para rasterizar una
// ecuación LaTeX a SVG (issue #239-B). Reutiliza mermaidAndChartRenderCSP
// tal cual: MathJax también carga únicamente desde jsdelivr y no necesita
// imágenes externas, mismo perfil que mermaid/chart. El contenido es dato
// del usuario: BuildMathDiv (math_html.go) ya lo HTML-escapa antes de
// insertarlo (MathJax lee el textContent del nodo, igual razonamiento de
// seguridad que buildMermaidSVGHTML/CR-6/AL-6, issue #24).
func buildMathSVGHTML(latex string) string {
	return `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="` + mermaidAndChartRenderCSP + `">
    ` + renderer.MathCDNScriptTag + `
    <style>
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
// buildMermaidPNGHTML: contenedor de tamaño fijo + señal "renderComplete"
// vía setTimeout heurístico (MathJax no expone un callback tan simple como
// mermaid.initialize() para "ya tipografié"; se usa el mismo heurístico de
// tiempo fijo que ya usa el path PNG de Mermaid, no un rigor menor).
func buildMathPNGHTML(latex string, width, height int) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="%s">
    `+renderer.MathCDNScriptTag+`
    <style>
        body { margin: 0; padding: 0; background: white; display: flex; justify-content: center; align-items: center; }
        #mathContainer { width: %dpx; height: %dpx; display: flex; justify-content: center; align-items: center; }
    </style>
</head>
<body>
    <div id="mathContainer">
        %s
    </div>
    <div id="renderComplete" style="display:none;">ready</div>
    <script>
        setTimeout(() => {
            document.getElementById('renderComplete').setAttribute('data-ready', 'true');
        }, 1500);
    </script>
</body>
</html>`, mermaidAndChartRenderCSP, width, height, renderer.BuildMathDiv(latex))
}

// buildMermaidPNGHTML arma la página temporal usada para rasterizar un
// diagrama Mermaid a PNG (ver buildMermaidSVGHTML para el razonamiento de
// seguridad, idéntico aquí).
func buildMermaidPNGHTML(mermaidCode string, width, height int) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="%s">
    `+renderer.MermaidCDNScriptTag+`
    <style>
        body { margin: 0; padding: 0; background: white; display: flex; justify-content: center; align-items: center; }
        #mermaidContainer { width: %dpx; height: %dpx; display: flex; justify-content: center; align-items: center; }
        .mermaid { display: inline-block; }
    </style>
</head>
<body>
    <div id="mermaidContainer">
        %s
    </div>
    <div id="renderComplete" style="display:none;">ready</div>
    <script>
        mermaid.initialize(%s);
        // Esperar a que Mermaid termine de renderizar
        setTimeout(() => {
            document.getElementById('renderComplete').setAttribute('data-ready', 'true');
        }, 1500);
    </script>
</body>
</html>`, mermaidAndChartRenderCSP, width, height, renderer.BuildMermaidDiv(mermaidCode), renderer.MermaidInitConfigJS(true))
}

// buildChartHTML arma la página temporal usada para rasterizar un chart de
// Chart.js (PNG o WebP, ambos comparten el mismo HTML). chartConfig ya llega
// re-serializado con json.Marshal desde el llamador (html.go), por lo que
// `<`,`>`,`&` están escapados como \u00xx y no rompen el <script>.
func buildChartHTML(chartConfig string, width, height int) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="%s">
    `+renderer.ChartJSCDNScriptTag+`
    <style>
        body { margin: 0; padding: 0; background: white; }
        #chartContainer { width: %dpx; height: %dpx; }
    </style>
</head>
<body>
    <div id="chartContainer">
        <canvas id="myChart"></canvas>
    </div>
    <div id="renderComplete" style="display:none;">ready</div>
    <script>
        const ctx = document.getElementById('myChart').getContext('2d');
        const config = %s;
        const chart = new Chart(ctx, config);

        // Esperar a que la animación termine completamente
        setTimeout(() => {
            document.getElementById('renderComplete').setAttribute('data-ready', 'true');
        }, 1500);
    </script>
</body>
</html>`, mermaidAndChartRenderCSP, width, height, chartConfig)
}

// setDocumentContentAction navega a about:blank e inyecta htmlContent
// directamente vía Page.setDocumentContent, evitando cualquier origen
// file:// o http(s):// contra el cual el contenido pudiera resolverse —
// cierra la misma clase de bypass de divulgación de archivos locales
// (AL-5, docs/SECURITY_AUDIT_2026-07.md, issue #28/#78) documentada para
// RenderHTMLToPDF (#77), para todos los Render* que arman su propia
// página HTML desechable.
func setDocumentContentAction(htmlContent string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		frameTree, err := page.GetFrameTree().Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to get frame tree: %w", err)
		}
		if frameTree == nil || frameTree.Frame == nil {
			return fmt.Errorf("chromium returned an empty frame tree")
		}
		return page.SetDocumentContent(frameTree.Frame.ID, htmlContent).Do(ctx)
	}
}

// navigateAndSetContent es el par chromedp.Navigate("about:blank") +
// setDocumentContentAction, repetido en cada builder de este archivo — un
// único punto de reuso evita que un octavo builder futuro copie el patrón
// viejo de archivo temporal + file:// en su lugar. chromedp.Tasks
// implementa Action, así que esto es un reemplazo directo de las dos
// líneas anteriores en cada chromedp.Run(...).
func navigateAndSetContent(htmlContent string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate("about:blank"),
		setDocumentContentAction(htmlContent),
	}
}

// ChromiumRenderer usa Chromium para rendering avanzado
type ChromiumRenderer struct {
	manager *ChromiumManager
	ctx     context.Context
	cancel  context.CancelFunc
	logger  ChromiumLogger
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
		manager: manager,
		ctx:     browserCtx,
		cancel:  cancel,
		logger:  logger,
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

// RenderHTMLToPDF convierte HTML a PDF. ctx acota/cancela esta llamada
// puntual (issue #134/G1d) sin afectar el ciclo de vida del browser, que
// sigue gobernado por el ctx pasado al constructor.
func (r *ChromiumRenderer) RenderHTMLToPDF(ctx context.Context, htmlContent string, outputPath string, opts PDFOptions) error {
	r.logger.Info("PDF", "Rendering HTML to PDF...")

	r.logger.Info("PDF", "Loading HTML document...")

	// Buffer para el PDF
	var pdfBuf []byte

	runCtx, cancel := withCallerCancel(r.ctx, ctx)
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
	err := chromedp.Run(runCtx,
		navigateAndSetContent(htmlContent),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Dar tiempo a CSS/JS
		chromedp.ActionFunc(func(ctx context.Context) error {
			r.logger.Info("PDF", "Generating PDF...")

			// Configurar opciones de impresión
			printParams := page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true)

			// Margins
			if opts.MarginTop > 0 {
				printParams = printParams.WithMarginTop(opts.MarginTop)
			}
			if opts.MarginBottom > 0 {
				printParams = printParams.WithMarginBottom(opts.MarginBottom)
			}
			if opts.MarginLeft > 0 {
				printParams = printParams.WithMarginLeft(opts.MarginLeft)
			}
			if opts.MarginRight > 0 {
				printParams = printParams.WithMarginRight(opts.MarginRight)
			}

			// Paper size
			if opts.PaperWidth > 0 && opts.PaperHeight > 0 {
				printParams = printParams.
					WithPaperWidth(opts.PaperWidth).
					WithPaperHeight(opts.PaperHeight)
			}

			// Landscape
			if opts.Landscape {
				printParams = printParams.WithLandscape(true)
			}

			// Header/Footer template
			if opts.DisplayHeaderFooter {
				printParams = printParams.
					WithDisplayHeaderFooter(true).
					WithHeaderTemplate(opts.HeaderTemplate).
					WithFooterTemplate(opts.FooterTemplate)
			}

			// Generar PDF
			buf, _, err := printParams.Do(ctx)
			if err != nil {
				return err
			}
			pdfBuf = buf
			return nil
		}),
	)

	if err != nil {
		return fmt.Errorf("chromium rendering failed: %w", err)
	}

	// Guardar PDF
	if err := os.WriteFile(outputPath, pdfBuf, 0644); err != nil {
		return fmt.Errorf("failed to write PDF: %w", err)
	}

	r.logger.Info("PDF", "✅ PDF generated: %s (%.2f KB)", outputPath, float64(len(pdfBuf))/1024)
	return nil
}

// RenderMermaidToSVG renderiza un diagrama Mermaid a SVG. ctx acota/cancela
// esta llamada puntual (issue #134/G1d).
func (r *ChromiumRenderer) RenderMermaidToSVG(ctx context.Context, mermaidCode string) (string, error) {
	r.logger.Info("MERMAID", "Rendering diagram to SVG...")

	html := buildMermaidSVGHTML(mermaidCode)

	var svgContent string

	runCtx, cancel := withCallerCancel(r.ctx, ctx)
	defer cancel()

	// Renderizar con Chromium. Se inyecta el HTML vía about:blank +
	// Page.setDocumentContent en vez de escribir un archivo temporal y
	// navegar a file:// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	err := chromedp.Run(runCtx,
		navigateAndSetContent(html),
		chromedp.WaitVisible(".mermaid svg", chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond), // Esperar renderizado completo
		chromedp.OuterHTML(".mermaid svg", &svgContent, chromedp.ByQuery),
	)

	if err != nil {
		return "", fmt.Errorf("mermaid rendering failed: %w", err)
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
	r.logger.Info("MATH", "Rendering equation to SVG...")

	html := buildMathSVGHTML(latex)

	var svgContent string

	runCtx, cancel := withCallerCancel(r.ctx, ctx)
	defer cancel()

	// Renderizar con Chromium. Se inyecta el HTML vía about:blank +
	// Page.setDocumentContent en vez de escribir un archivo temporal y
	// navegar a file:// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	err := chromedp.Run(runCtx,
		navigateAndSetContent(html),
		chromedp.WaitVisible("mjx-container svg", chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond), // Esperar tipografiado completo
		chromedp.OuterHTML("mjx-container svg", &svgContent, chromedp.ByQuery),
	)

	if err != nil {
		return "", fmt.Errorf("math rendering failed: %w", err)
	}

	r.logger.Info("MATH", "✅ SVG rendered (%.2f KB)", float64(len(svgContent))/1024)
	return svgContent, nil
}

// RenderMathToPNG renderiza una ecuación LaTeX a PNG (issue #239-B, uso
// principal: rasterizar para DOCX, que no puede embeber SVG). ctx acota/
// cancela esta llamada puntual — mismo patrón que RenderMermaidToPNG.
func (r *ChromiumRenderer) RenderMathToPNG(ctx context.Context, latex string, width, height int) ([]byte, error) {
	r.logger.Info("MATH", "Rendering equation to PNG...")

	html := buildMathPNGHTML(latex, width, height)

	var pngData []byte

	tabCtx, tabCancel := chromedp.NewContext(r.ctx)
	defer tabCancel()

	runCtx, cancel := withCallerCancel(tabCtx, ctx)
	defer cancel()

	err := chromedp.Run(runCtx,
		navigateAndSetContent(html),
		chromedp.WaitVisible("mjx-container svg", chromedp.ByQuery),
		chromedp.WaitReady("#renderComplete[data-ready='true']", chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Screenshot("#mathContainer", &pngData, chromedp.ByID),
	)

	if err != nil {
		return nil, fmt.Errorf("math rendering failed: %w", err)
	}

	r.logger.Info("MATH", "✅ PNG rendered (%.2f KB)", float64(len(pngData))/1024)
	return pngData, nil
}

// RenderMermaidToPNG renderiza un diagrama Mermaid a PNG. ctx acota/cancela
// esta llamada puntual (issue #134/G1d).
func (r *ChromiumRenderer) RenderMermaidToPNG(ctx context.Context, mermaidCode string, width, height int) ([]byte, error) {
	r.logger.Info("MERMAID", "Rendering diagram to PNG...")

	html := buildMermaidPNGHTML(mermaidCode, width, height)

	var pngData []byte

	// Crear un nuevo tab context para esta renderización
	tabCtx, tabCancel := chromedp.NewContext(r.ctx)
	defer tabCancel()

	runCtx, cancel := withCallerCancel(tabCtx, ctx)
	defer cancel()

	// Renderizar con Chromium - esperar a que el renderizado esté completo.
	// Se inyecta el HTML vía about:blank + Page.setDocumentContent en vez
	// de escribir un archivo temporal y navegar a file://
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	err := chromedp.Run(runCtx,
		navigateAndSetContent(html),
		chromedp.WaitVisible(".mermaid svg", chromedp.ByQuery),
		chromedp.WaitReady("#renderComplete[data-ready='true']", chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond), // Buffer adicional
		chromedp.Screenshot("#mermaidContainer", &pngData, chromedp.ByID),
	)

	if err != nil {
		return nil, fmt.Errorf("mermaid rendering failed: %w", err)
	}

	r.logger.Info("MERMAID", "✅ PNG rendered (%.2f KB)", float64(len(pngData))/1024)
	return pngData, nil
}

// RenderChartToPNG renderiza un gráfico Chart.js a PNG. ctx acota/cancela
// esta llamada puntual (issue #134/G1d).
func (r *ChromiumRenderer) RenderChartToPNG(ctx context.Context, chartConfig string, width, height int) ([]byte, error) {
	r.logger.Info("CHART", "Rendering chart to PNG...")

	html := buildChartHTML(chartConfig, width, height)

	var pngData []byte

	// IMPORTANTE: Crear un nuevo tab context para esta renderización
	tabCtx, tabCancel := chromedp.NewContext(r.ctx)
	defer tabCancel()

	runCtx, cancel := withCallerCancel(tabCtx, ctx)
	defer cancel()

	// Renderizar con Chromium - esperar a que el renderizado esté completo.
	// Se inyecta el HTML vía about:blank + Page.setDocumentContent en vez
	// de escribir un archivo temporal y navegar a file://
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	err := chromedp.Run(runCtx,
		navigateAndSetContent(html),
		chromedp.WaitVisible("#myChart", chromedp.ByID),
		chromedp.WaitReady("#renderComplete[data-ready='true']", chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond), // Buffer adicional
		chromedp.Screenshot("#chartContainer", &pngData, chromedp.ByID),
	)

	if err != nil {
		return nil, fmt.Errorf("chart rendering failed: %w", err)
	}

	r.logger.Info("CHART", "✅ PNG rendered (%.2f KB)", float64(len(pngData))/1024)
	return pngData, nil
}

// RenderMapToPNG renderiza un mapa Leaflet a PNG. ctx acota/cancela esta
// llamada puntual (issue #134/G1d), además del timeout ajustado por zoom.
func (r *ChromiumRenderer) RenderMapToPNG(ctx context.Context, mapConfig renderer.MapConfig, width, height int) ([]byte, error) {
	r.logger.Info("MAP", "Rendering map to PNG...")

	// Generar HTML con Leaflet + flag de renderizado completo
	html := r.generateLeafletHTML(mapConfig, width, height)

	var pngData []byte

	// IMPORTANTE: Crear un nuevo tab context para esta renderización
	// Esto evita conflictos cuando se renderizan múltiples mapas
	tabCtx, tabCancel := chromedp.NewContext(r.ctx)
	defer tabCancel()

	// Crear contexto con timeout ajustado según zoom
	// El timeout debe ser MUCHO mayor que el fallback de JavaScript para dar tiempo al rendering
	// JS fallback: 5s (zoom <7), 10s (zoom 7-10), 30s (zoom 11+)
	// Go timeout: 15s (zoom <7), 20s (zoom 7-10), 60s (zoom 11+)
	timeout := 15 * time.Second
	if mapConfig.Zoom >= 11 {
		timeout = 60 * time.Second // Zoom muy alto: 60 segundos (JS fallback = 30s + buffer)
		r.logger.Info("MAP", "High zoom level (%d), using extended timeout (60s)...", mapConfig.Zoom)
	} else if mapConfig.Zoom >= 7 {
		timeout = 20 * time.Second // Zoom medio: 20 segundos (JS fallback = 10s + buffer)
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(tabCtx, timeout)
	defer timeoutCancel()

	runCtx, cancel := withCallerCancel(timeoutCtx, ctx)
	defer cancel()

	// Renderizar con Chromium - esperar a que tiles y marcadores estén cargados.
	// Se inyecta el HTML vía about:blank + Page.setDocumentContent en vez
	// de escribir un archivo temporal y navegar a file://
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	err := chromedp.Run(runCtx,
		navigateAndSetContent(html),
		chromedp.WaitVisible("#map", chromedp.ByID),
		// Esperar a que window.mapReady sea true (establecido por el JavaScript)
		chromedp.Poll(`window.mapReady === true`, nil, chromedp.WithPollingInterval(200*time.Millisecond)),
		chromedp.Sleep(200*time.Millisecond), // Pequeño buffer para asegurar rendering completo
		chromedp.Screenshot("#mapContainer", &pngData, chromedp.ByID),
	)

	if err != nil {
		return nil, fmt.Errorf("map rendering failed: %w", err)
	}

	r.logger.Info("MAP", "✅ PNG rendered (%.2f KB)", float64(len(pngData))/1024)
	return pngData, nil
}

// RenderChartToWebP renderiza un gráfico Chart.js directamente a WebP. ctx
// acota/cancela esta llamada puntual (issue #134/G1d).
func (r *ChromiumRenderer) RenderChartToWebP(ctx context.Context, chartConfig string, width, height int, quality int) ([]byte, error) {
	r.logger.Info("CHART", "Rendering chart to WebP (quality: %d)...", quality)

	html := buildChartHTML(chartConfig, width, height)

	var webpData []byte

	// IMPORTANTE: Crear un nuevo tab context para esta renderización
	tabCtx, tabCancel := chromedp.NewContext(r.ctx)
	defer tabCancel()

	runCtx, cancel := withCallerCancel(tabCtx, ctx)
	defer cancel()

	// Renderizar con Chromium usando CDP para captura WebP - esperar renderizado completo.
	// Se inyecta el HTML vía about:blank + Page.setDocumentContent en vez
	// de escribir un archivo temporal y navegar a file://
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	err := chromedp.Run(runCtx,
		navigateAndSetContent(html),
		chromedp.WaitVisible("#myChart", chromedp.ByID),
		chromedp.WaitReady("#renderComplete[data-ready='true']", chromedp.ByQuery),
		chromedp.Sleep(200*time.Millisecond), // Buffer adicional
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Usar page.CaptureScreenshot con formato WebP
			buf, err := page.CaptureScreenshot().
				WithFormat(page.CaptureScreenshotFormatWebp).
				WithQuality(int64(quality)).
				WithClip(&page.Viewport{
					X:      0,
					Y:      0,
					Width:  float64(width),
					Height: float64(height),
					Scale:  1,
				}).
				Do(ctx)
			if err != nil {
				return err
			}
			webpData = buf
			return nil
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("chart rendering to WebP failed: %w", err)
	}

	r.logger.Info("CHART", "✅ WebP rendered (%.2f KB)", float64(len(webpData))/1024)
	return webpData, nil
}

// RenderMapToWebP renderiza un mapa Leaflet directamente a WebP. ctx
// acota/cancela esta llamada puntual (issue #134/G1d).
func (r *ChromiumRenderer) RenderMapToWebP(ctx context.Context, mapConfig renderer.MapConfig, width, height int, quality int) ([]byte, error) {
	r.logger.Info("MAP", "Rendering map to WebP (quality: %d)...", quality)

	// Generar HTML con Leaflet + flag de renderizado completo
	html := r.generateLeafletHTML(mapConfig, width, height)

	var webpData []byte

	// IMPORTANTE: Crear un nuevo tab context para esta renderización
	// Esto evita conflictos cuando se renderizan múltiples mapas
	tabCtx, tabCancel := chromedp.NewContext(r.ctx)
	defer tabCancel()

	// Crear contexto con timeout para evitar espera infinita
	timeoutCtx, timeoutCancel := context.WithTimeout(tabCtx, 30*time.Second)
	defer timeoutCancel()

	runCtx, cancel := withCallerCancel(timeoutCtx, ctx)
	defer cancel()

	// Renderizar con Chromium usando CDP para captura WebP - esperar renderizado completo.
	// Se inyecta el HTML vía about:blank + Page.setDocumentContent en vez
	// de escribir un archivo temporal y navegar a file://
	// (docs/SECURITY_AUDIT_2026-07.md, AL-5/#78).
	err := chromedp.Run(runCtx,
		navigateAndSetContent(html),
		chromedp.WaitVisible("#map", chromedp.ByID),
		// Esperar a que el atributo data-ready='true' exista (el elemento tiene display:none)
		chromedp.WaitReady("#renderComplete[data-ready='true']", chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond), // Buffer adicional
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Usar page.CaptureScreenshot con formato WebP
			buf, err := page.CaptureScreenshot().
				WithFormat(page.CaptureScreenshotFormatWebp).
				WithQuality(int64(quality)).
				WithClip(&page.Viewport{
					X:      0,
					Y:      0,
					Width:  float64(width),
					Height: float64(height),
					Scale:  1,
				}).
				Do(ctx)
			if err != nil {
				return err
			}
			webpData = buf
			return nil
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("map rendering to WebP failed: %w", err)
	}

	r.logger.Info("MAP", "✅ WebP rendered (%.2f KB)", float64(len(webpData))/1024)
	return webpData, nil
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
                iconUrl: 'https://raw.githubusercontent.com/pointhi/leaflet-color-markers/master/img/marker-icon-2x-%s.png',
                shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.9.4/images/marker-shadow.png',
                iconSize: [25, 41],
                iconAnchor: [12, 41],
                popupAnchor: [1, -34],
                shadowSize: [41, 41]
            })
        }).addTo(map).bindPopup('%s');
        `, marker.Lat, marker.Lng, color, label)

		if i < len(config.Markers)-1 {
			markersJS += "\n"
		}
	}

	tileLayer := `https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png`
	if config.MapType == "satellite" {
		tileLayer = `https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}`
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="%s">
    `+renderer.LeafletCDNCSSTag+`
    `+renderer.LeafletCDNScriptTag+`
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
        const map = L.map('map').setView([%f, %f], %d);
        const tileLayer = L.tileLayer('%s', {
            attribution: '© OpenStreetMap contributors',
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
        
        // Fallback dinámico basado en zoom level
        const zoom = %d;
        let fallbackTimeout = 5000;
        if (zoom >= 11) {
            fallbackTimeout = 30000;  // 30 segundos para zoom muy alto
        } else if (zoom >= 7) {
            fallbackTimeout = 10000;  // 10 segundos para zoom medio
        }
        
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
</html>`, mapRenderCSP, width, height, config.CenterLat, config.CenterLng, config.Zoom, tileLayer, markersJS, config.Zoom)
}

// Close cierra el renderer y libera recursos
func (r *ChromiumRenderer) Close() {
	if r.cancel != nil {
		r.cancel()
	}
}

// PDFOptions configura la generación de PDF
type PDFOptions struct {
	// Paper size (inches)
	PaperWidth  float64
	PaperHeight float64
	Landscape   bool

	// Margins (inches)
	MarginTop    float64
	MarginBottom float64
	MarginLeft   float64
	MarginRight  float64

	// Header/Footer
	DisplayHeaderFooter bool
	HeaderTemplate      string
	FooterTemplate      string

	// Scale
	Scale float64
}

// DefaultPDFOptions retorna opciones por defecto (A4, portrait)
func DefaultPDFOptions() PDFOptions {
	return PDFOptions{
		PaperWidth:   8.27,  // A4 width in inches
		PaperHeight:  11.69, // A4 height in inches
		Landscape:    false,
		MarginTop:    0.4,
		MarginBottom: 0.4,
		MarginLeft:   0.4,
		MarginRight:  0.4,
		Scale:        1.0,
	}
}
