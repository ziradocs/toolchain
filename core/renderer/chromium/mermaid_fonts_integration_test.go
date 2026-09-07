// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"go.ziradocs.com/core/v2/renderer"
)

// monoFontCandidates son fuentes MONOESPACIADAS con métricas muy distintas a
// cualquier fallback proporcional — es justamente ese contraste lo que hace
// medible si la fuente llegó a tiempo. Se prueba una lista porque el test
// corre tanto en macOS como en los runners de Linux.
var monoFontCandidates = []struct{ path, mime string }{
	{"/System/Library/Fonts/Supplemental/Andale Mono.ttf", "font/ttf"},
	{"/System/Library/Fonts/Supplemental/Courier New.ttf", "font/ttf"},
	{"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", "font/ttf"},
	{"/usr/share/fonts/dejavu/DejaVuSansMono.ttf", "font/ttf"},
	{"/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf", "font/ttf"},
}

func probeMonoFontDataURI(t *testing.T) string {
	t.Helper()
	for _, c := range monoFontCandidates {
		data, err := os.ReadFile(c.path)
		if err != nil {
			continue
		}
		return "data:" + c.mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	// Ruidoso a propósito: estos son los ÚNICOS tests que prueban que el
	// arreglo sirve (el resto verifica que la página emita lo correcto, que
	// es necesario pero no suficiente). Si se saltan, el verde de CI no dice
	// nada sobre las métricas.
	t.Skip("SIN COBERTURA EMPÍRICA: no hay ninguna fuente monoespaciada conocida en esta máquina, así que la única prueba de que la fuente llega al layout no corrió")
	return ""
}

var svgViewBoxRe = regexp.MustCompile(`viewBox="[-\d.]+ [-\d.]+ ([\d.]+) ([\d.]+)"`)

func svgViewBoxWidth(t *testing.T, svg string) float64 {
	t.Helper()
	m := svgViewBoxRe.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("el SVG no trae viewBox:\n%s", svg)
	}
	w, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("viewBox con ancho no numérico (%q): %v", m[1], err)
	}
	return w
}

// TestRenderMermaidToSVGWithTheme_FontChangesMeasuredLayout es la ÚNICA
// prueba que demuestra que el arreglo sirve. Que la página emita el
// @font-face y el document.fonts.load() correctos es necesario pero no
// suficiente: lo que estaba roto es que Mermaid MEDÍA con la fuente
// equivocada, y eso solo se ve en las métricas del SVG que produce.
//
// Las dos corridas declaran la MISMA familia. La única diferencia es si el
// recurso viaja o no:
//
//   - con recurso: la familia existe, Mermaid mide con métricas monoespaciadas
//   - sin recurso: la familia no resuelve a nada (es un nombre inventado) y
//     Mermaid cae al fallback proporcional — el comportamiento de antes de
//     este PR, aun pasándole el nombre vía themeVariables
//
// El label está hecho de 'i': el carácter donde más se separan un
// monoespaciado y un proporcional, para que la diferencia no dependa de qué
// fuente concreta encontró probeMonoFontDataURI.
func TestRenderMermaidToSVGWithTheme_FontChangesMeasuredLayout(t *testing.T) {
	src := probeMonoFontDataURI(t)
	r := newTestChromiumRenderer(t)

	const familia = "ZiradocsProbeMono"
	const code = "graph TD; A[iiiiiiiiiiiiiiiiiiiiiiiiiiiiii] --> B[iiiiiiiiiiiiiiiiiiiiiiiiiiiiii]"

	conRecurso := renderer.DiagramThemeColors{
		FontFamily: familia,
		Fonts:      []renderer.DiagramFontFace{{Family: familia, Src: src}},
	}
	soloNombre := renderer.DiagramThemeColors{FontFamily: familia}

	svgCon, err := r.RenderMermaidToSVGWithTheme(context.Background(), code, conRecurso)
	if err != nil {
		t.Fatalf("render con la fuente: %v", err)
	}
	svgSin, err := r.RenderMermaidToSVGWithTheme(context.Background(), code, soloNombre)
	if err != nil {
		t.Fatalf("render sin la fuente: %v", err)
	}

	anchoCon := svgViewBoxWidth(t, svgCon)
	anchoSin := svgViewBoxWidth(t, svgSin)
	t.Logf("ancho medido: con la fuente=%.2f, solo con el nombre=%.2f", anchoCon, anchoSin)

	if anchoCon == anchoSin {
		t.Errorf("Mermaid midió igual con y sin la fuente (%.2f): el recurso no llegó a tiempo al layout", anchoCon)
	}
	// Un monoespaciado da a 'i' el ancho completo, así que tiene que salir
	// MÁS ancho que el fallback proporcional. La desigualdad estricta
	// descarta que la diferencia venga de ruido de redondeo.
	if anchoCon <= anchoSin {
		t.Errorf("con la fuente monoespaciada el diagrama debía salir más ancho: con=%.2f sin=%.2f", anchoCon, anchoSin)
	}
}

// TestRenderMermaidToSVGWithTheme_UnusableFontStillRenders fija el
// degradado: una fuente que el navegador nunca podrá cargar no debe colgar ni
// romper el render. La cara se descarta en Go antes de llegar a la página
// (ver DiagramFontFace.valid), así que este caso recorre el camino de
// siempre.
func TestRenderMermaidToSVGWithTheme_UnusableFontStillRenders(t *testing.T) {
	r := newTestChromiumRenderer(t)

	svg, err := r.RenderMermaidToSVGWithTheme(context.Background(), "graph TD; A-->B", renderer.DiagramThemeColors{
		FontFamily: "Rota",
		Fonts:      []renderer.DiagramFontFace{{Family: "Rota", Src: "data:font/woff2;base64,no-es-base64!"}},
	})
	if err != nil {
		t.Fatalf("una fuente inutilizable no debe romper el render: %v", err)
	}
	if svgViewBoxWidth(t, svg) <= 0 {
		t.Errorf("el diagrama salió vacío:\n%s", svg)
	}
}

// TestRenderMermaidToPNGWithTheme_FontsDoNotBreakTheReadySignal es la SEGUNDA
// puerta, y la que más fácil se rompería en silencio: el camino PNG no espera
// al nodo svg como el SVG, espera a #renderComplete, cuyo setTimeout arranca
// al evaluar el script. Al diferir el render detrás de las fuentes, esa señal
// tuvo que moverse dentro del then() de mermaid.run(); si se hubiera quedado
// donde estaba, chromedp capturaría antes de que el diagrama exista y saldría
// un PNG en blanco — sin error, sin timeout, sin nada que lo delate.
//
// Que los dos PNG difieran es lo que prueba que el de la izquierda se dibujó
// CON la fuente: mismo diagrama, misma familia declarada, única diferencia el
// recurso.
func TestRenderMermaidToPNGWithTheme_FontsDoNotBreakTheReadySignal(t *testing.T) {
	src := probeMonoFontDataURI(t)
	r := newTestChromiumRenderer(t)

	const familia = "ZiradocsProbeMono"
	const code = "graph TD; A[iiiiiiiiiiiiiiiiiiiiiiiiiiiiii] --> B[iiiiiiiiiiiiiiiiiiiiiiiiiiiiii]"

	conRecurso, err := r.RenderMermaidToPNGWithTheme(context.Background(), code, 600, 400, renderer.DiagramThemeColors{
		FontFamily: familia,
		Fonts:      []renderer.DiagramFontFace{{Family: familia, Src: src}},
	})
	if err != nil {
		t.Fatalf("render PNG con la fuente: %v", err)
	}
	soloNombre, err := r.RenderMermaidToPNGWithTheme(context.Background(), code, 600, 400, renderer.DiagramThemeColors{FontFamily: familia})
	if err != nil {
		t.Fatalf("render PNG sin la fuente: %v", err)
	}

	if len(conRecurso) == 0 {
		t.Fatal("el PNG salió vacío: la señal de #renderComplete se adelantó al render")
	}
	t.Logf("PNG: con la fuente=%d bytes, solo con el nombre=%d bytes", len(conRecurso), len(soloNombre))
	if bytes.Equal(conRecurso, soloNombre) {
		t.Error("los dos PNG son idénticos: la fuente no llegó al layout del camino PNG")
	}
}

// ---------------------------------------------------------------------------
// Segunda ronda de revisión: 3 hallazgos más, todos con render real
// ---------------------------------------------------------------------------

// TestRenderMermaidToSVGWithTheme_ApostropheInFontFamilyIsMeasured es el
// hallazgo "Mermaid descarta los font-main reales que contienen apóstrofes o
// guiones". Mismo diagrama, misma fuente monoespaciada, dos temas: uno con un
// nombre de familia SEGURO y otro con el MISMO archivo bajo un nombre con
// apóstrofe. Si el arreglo funciona, los dos deben medir igual de anchos —
// antes del arreglo, el segundo caía al fallback proporcional aunque la
// fuente ya estuviera cargada (document.fonts.load la carga bien; lo que
// fallaba es que Mermaid nunca llegaba a APLICARLA, porque el nombre crudo
// rompía la CSS interna que arma con theme.fontFamily).
func TestRenderMermaidToSVGWithTheme_ApostropheInFontFamilyIsMeasured(t *testing.T) {
	src := probeMonoFontDataURI(t)
	r := newTestChromiumRenderer(t)
	const code = "graph TD; A[iiiiiiiiiiiiiiiiiiiiiiiiiiiiii] --> B[iiiiiiiiiiiiiiiiiiiiiiiiiiiiii]"

	const familiaSegura = "ZiradocsApostropheProbeSafe"
	svgSeguro, err := r.RenderMermaidToSVGWithTheme(context.Background(), code, renderer.DiagramThemeColors{
		FontFamily: familiaSegura,
		Fonts:      []renderer.DiagramFontFace{{Family: familiaSegura, Src: src}},
	})
	if err != nil {
		t.Fatalf("render con nombre seguro: %v", err)
	}

	const familiaConApostrofe = "Zira's Probe Mono"
	svgApostrofe, err := r.RenderMermaidToSVGWithTheme(context.Background(), code, renderer.DiagramThemeColors{
		FontFamily: familiaConApostrofe,
		Fonts:      []renderer.DiagramFontFace{{Family: familiaConApostrofe, Src: src}},
	})
	if err != nil {
		t.Fatalf("render con nombre con apóstrofe: %v", err)
	}

	anchoSeguro := svgViewBoxWidth(t, svgSeguro)
	anchoApostrofe := svgViewBoxWidth(t, svgApostrofe)
	t.Logf("ancho medido: nombre seguro=%.2f, nombre con apóstrofe=%.2f", anchoSeguro, anchoApostrofe)

	// Tolerancia pequeña: mismo diagrama y misma fuente, deberían coincidir
	// exactamente, pero un margen de redondeo no invalida la prueba.
	if diff := anchoSeguro - anchoApostrofe; diff > 1 || diff < -1 {
		t.Errorf("el nombre con apóstrofe midió distinto al seguro (seguro=%.2f apóstrofe=%.2f): Mermaid no aplicó la fuente", anchoSeguro, anchoApostrofe)
	}
}

// TestSanitizeFontFamilyList_IsWhatMakesTheApostropheCaseWork es el control
// negativo del test anterior: quitando el saneo (dejando pasar el valor
// crudo, que es justo lo que hacía este código antes del arreglo), el nombre
// con apóstrofe SÍ debe medir distinto — o esta suite no estaría detectando
// nada.
func TestSanitizeFontFamilyList_IsWhatMakesTheApostropheCaseWork(t *testing.T) {
	crudo := "Zira's Broken Probe"
	// El saneo en sí (sanitizeFontFamilyList) vive en package renderer y
	// tiene su propia cobertura ahí (diagram_theme_test.go); acá se confirma
	// desde ESTE lado —vía la API pública que de verdad llega a
	// mermaid.initialize()— que el valor NO viaja crudo.
	vars := renderer.DiagramThemeColors{FontFamily: crudo}.MermaidThemeVariables()
	if vars["fontFamily"] == crudo {
		t.Fatalf("MermaidThemeVariables dejó pasar el valor crudo: %q", vars["fontFamily"])
	}
}

// TestRenderMermaidToSVGWithTheme_SavedSVGEmbedsFontFace es el hallazgo "el
// SVG guardado no incluye el recurso @font-face". No basta con que la página
// TEMPORAL vea la fuente — el archivo que se guarda a disco (offline-assets)
// o se referencia luego (offline-inline) tiene que traerla consigo, porque
// un <img> no hereda ningún <style> de ningún documento padre.
func TestRenderMermaidToSVGWithTheme_SavedSVGEmbedsFontFace(t *testing.T) {
	src := probeMonoFontDataURI(t)
	r := newTestChromiumRenderer(t)

	const familia = "ZiradocsEmbedProbe"
	svg, err := r.RenderMermaidToSVGWithTheme(context.Background(), "graph TD; A-->B", renderer.DiagramThemeColors{
		FontFamily: familia,
		Fonts:      []renderer.DiagramFontFace{{Family: familia, Src: src}},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(svg, "@font-face") {
		t.Fatalf("el SVG guardado no trae @font-face:\n%s", svg[:min(len(svg), 500)])
	}
	if !strings.Contains(svg, src) {
		t.Fatalf("el SVG guardado no trae el data: URI de la fuente")
	}
	// Tiene que ser el PRIMER hijo del <svg>, antes que cualquier otro
	// contenido — en particular, antes del <style> que el propio Mermaid
	// arma con font-family:<nombre>, para que ambos coexistan sin depender
	// de orden.
	svgOpenEnd := strings.IndexByte(svg, '>')
	styleIdx := strings.Index(svg, "@font-face")
	if svgOpenEnd < 0 || styleIdx < svgOpenEnd {
		t.Fatalf("@font-face no quedó DESPUÉS de la etiqueta <svg> de apertura")
	}
	firstMermaidStyle := strings.Index(svg[svgOpenEnd:], "font-family:"+familia)
	if firstMermaidStyle >= 0 && styleIdx > svgOpenEnd+firstMermaidStyle {
		t.Errorf("@font-face quedó después del <style> propio de Mermaid, no antes")
	}
}

// TestRenderMermaidToSVGWithTheme_SavedSVGWorksStandaloneViaImg es la prueba
// de verdad del hallazgo anterior: confirma que la fuente que
// embedFontFacesInSVG embebe SÍ se aplica cuando el archivo se consume tal
// como offline-assets lo consume de verdad —vía <img>, en una página nueva
// sin relación con la que generó el SVG—.
//
// A propósito NO reutiliza el SVG completo que devuelve
// RenderMermaidToSVGWithTheme para la comparación de píxeles: ese SVG trae
// la GEOMETRÍA (viewBox, cajas de nodo, posiciones de texto) ya fijada por
// Mermaid durante el render original —con la fuente correcta, que es lo que
// prueban los otros tests de esta suite—, así que deshabilitar el
// @font-face DESPUÉS no cambia esa geometría, solo cómo se pintan los
// glifos dentro de cajas que ya estaban bien dimensionadas; con texto
// centrado y una fuente de reemplazo de ancho similar, un proxy de "columna
// de píxel más a la derecha" puede no mostrar diferencia alguna (verificado:
// no la mostró). Por eso acá se arma un SVG mínimo, con texto alineado a la
// izquierda y tamaño absoluto, que SÍ aísla la pregunta real: ¿el navegador
// aplica el recurso que embedFontFacesInSVG puso en el archivo, o no? El CSS
// en sí sale de la función de producción (theme.FontFaceCSS()), no de un
// string armado a mano en el test.
func TestRenderMermaidToSVGWithTheme_SavedSVGWorksStandaloneViaImg(t *testing.T) {
	src := probeMonoFontDataURI(t)
	const familia = "ZiradocsImgProbe"
	theme := renderer.DiagramThemeColors{Fonts: []renderer.DiagramFontFace{{Family: familia, Src: src}}}

	label := "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"
	buildSVG := func(css string) string {
		return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="600" height="100"><style>%s
text { font-family: %q; font-size: 24px; }</style><text x="5" y="50">%s</text></svg>`, css, familia, label)
	}

	r := newTestChromiumRenderer(t)
	conFuente := buildSVG(theme.FontFaceCSS())
	sinFuente := buildSVG("") // sin @font-face: "familia" no resuelve a nada, cae al fallback del navegador

	anchoConFuente := darkPixelExtentOfSVGAsImg(t, r, conFuente)
	anchoSinFuente := darkPixelExtentOfSVGAsImg(t, r, sinFuente)
	t.Logf("ancho de píxeles oscuros vía <img>: con @font-face=%d, sin @font-face=%d", anchoConFuente, anchoSinFuente)

	if anchoConFuente <= anchoSinFuente {
		t.Errorf("el <img> no aplicó la fuente embebida: con=%d sin=%d", anchoConFuente, anchoSinFuente)
	}
}

// darkPixelExtentOfSVGAsImg carga svgMarkup como <img src="data:image/svg+xml...">
// en una página nueva (sin relación con la que generó el SVG, igual que
// offline-assets lo serviría) y devuelve la columna de píxel más a la derecha
// que no es blanca — proxy del ancho de texto realmente pintado.
func darkPixelExtentOfSVGAsImg(t *testing.T, r *ChromiumRenderer, svgMarkup string) int {
	t.Helper()
	dataURI := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svgMarkup))
	dataURIJSON, err := json.Marshal(dataURI)
	if err != nil {
		t.Fatalf("marshal data URI: %v", err)
	}

	html := fmt.Sprintf(`<!DOCTYPE html><html><body>
<img id="probe" src=%s>
<canvas id="c" width="900" height="300"></canvas>
</body></html>`, string(dataURIJSON))

	var maxX int
	if err := r.ensureBrowser(context.Background()); err != nil {
		t.Fatalf("ensureBrowser: %v", err)
	}
	tabCtx, cancel := chromedp.NewContext(r.ctx)
	defer cancel()

	js := `(function(){
		var img = document.getElementById('probe');
		var canvas = document.getElementById('c');
		var ctx = canvas.getContext('2d');
		ctx.fillStyle = 'white';
		ctx.fillRect(0, 0, canvas.width, canvas.height);
		ctx.drawImage(img, 0, 0);
		var data = ctx.getImageData(0, 0, canvas.width, canvas.height).data;
		var maxX = 0;
		for (var y = 0; y < canvas.height; y++) {
			for (var x = 0; x < canvas.width; x++) {
				var idx = (y * canvas.width + x) * 4;
				if (data[idx] < 250 || data[idx+1] < 250 || data[idx+2] < 250) {
					if (x > maxX) maxX = x;
				}
			}
		}
		return maxX;
	})()`

	err = chromedp.Run(tabCtx,
		navigateAndSetContent(html),
		chromedp.WaitReady("#probe", chromedp.ByID),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.Evaluate(js, &maxX),
	)
	if err != nil {
		t.Fatalf("medición vía <img>: %v", err)
	}
	return maxX
}

// TestMermaidBootstrapJS_OneRejectedFontDoesNotStarveOthers es el hallazgo
// "el rechazo de una sola fuente adelanta draw() sin esperar las demás".
//
// No se prueba con fuentes reales: se verificó por separado (Chromium real,
// vía herramienta de navegador, cinco corridas) que con fuentes data: URI el
// decode de Chromium resuelve tan rápido que la ventana de la carrera nunca
// se observa en la práctica — lo cual NO significa que Promise.all sea
// correcto ahí, solo que ese entorno concreto no expone el bug. La garantía
// tiene que venir de la LÓGICA, no de la suerte del temporizador. Por eso
// este test controla el temporizado exacto reemplazando document.fonts.load
// por un mock: una fuente "lenta" que resuelve a los 300ms y una "que falla
// rápido" que rechaza de inmediato. Si el bootstrap espera a la lenta antes
// de llamar mermaid.run(), el bug está arreglado.
func TestMermaidBootstrapJS_OneRejectedFontDoesNotStarveOthers(t *testing.T) {
	theme := renderer.DiagramThemeColors{
		FontFamily: "SlowProbe",
		Fonts: []renderer.DiagramFontFace{
			{Family: "SlowProbe", Src: "data:font/woff2;base64,AAAA"},
			{Family: "FastFailProbe", Src: "data:font/woff2;base64,BBBB"},
		},
	}
	bootstrapJS := mermaidBootstrapJS(theme, "")
	if !strings.Contains(bootstrapJS, "Promise.allSettled(") {
		t.Fatalf("el bootstrap no usa Promise.allSettled:\n%s", bootstrapJS)
	}

	r := newTestChromiumRenderer(t)
	ranAfterMs := runBootstrapMockAndMeasure(t, r, bootstrapJS)
	t.Logf("mermaid.run() disparó a los %dms (la lenta resuelve a los 300ms)", ranAfterMs)
	if ranAfterMs < 250 {
		t.Errorf("mermaid.run() se adelantó a la fuente lenta: disparó a los %dms", ranAfterMs)
	}
}

// TestMermaidBootstrapJS_NegativeControl_PromiseAllStarvesTheSlowFont es el
// control negativo del test anterior: reconstruye a mano el bootstrap
// PRE-ARREGLO (Promise.all + swallow del rechazo) a partir del texto real que
// genera mermaidBootstrapJS, y confirma que ESE sí dispara temprano. Si esta
// prueba no fallara sin el arreglo, la de arriba no probaría nada.
func TestMermaidBootstrapJS_NegativeControl_PromiseAllStarvesTheSlowFont(t *testing.T) {
	theme := renderer.DiagramThemeColors{
		FontFamily: "SlowProbe",
		Fonts: []renderer.DiagramFontFace{
			{Family: "SlowProbe", Src: "data:font/woff2;base64,AAAA"},
			{Family: "FastFailProbe", Src: "data:font/woff2;base64,BBBB"},
		},
	}
	fixed := mermaidBootstrapJS(theme, "")
	broken := strings.Replace(fixed, "Promise.allSettled(", "Promise.all(", 1)
	broken = strings.Replace(broken, "Promise.race([loaded, deadline]).then(draw);", "Promise.race([loaded, deadline]).catch(function () {}).then(draw);", 1)
	if broken == fixed {
		t.Fatal("la reconstrucción de la versión rota no cambió nada — el patrón esperado ya no está en el bootstrap actual")
	}

	r := newTestChromiumRenderer(t)
	ranAfterMs := runBootstrapMockAndMeasure(t, r, broken)
	t.Logf("(versión pre-arreglo) mermaid.run() disparó a los %dms", ranAfterMs)
	if ranAfterMs >= 250 {
		t.Errorf("la reconstrucción de la versión rota NO se adelantó (disparó a los %dms): el control negativo no detecta nada", ranAfterMs)
	}
}

// runBootstrapMockAndMeasure inyecta bootstrapJS en una página que reemplaza
// document.fonts.load y mermaid.run por dobles controlados en el tiempo, y
// devuelve cuántos milisegundos pasaron desde la carga hasta que
// mermaid.run() se invocó.
func runBootstrapMockAndMeasure(t *testing.T, r *ChromiumRenderer, bootstrapJS string) int {
	t.Helper()
	html := `<!DOCTYPE html><html><body><script>
window.__t0 = performance.now();
window.__ranAfterMs = null;
var mermaid = {
	initialize: function () {},
	run: function () {
		window.__ranAfterMs = Math.round(performance.now() - window.__t0);
		return Promise.resolve();
	}
};
// document.fonts es un getter de solo lectura sobre Document.prototype:
// reasignarlo ("document.fonts = {...}") falla en silencio fuera de modo
// estricto y deja el objeto REAL intacto. Hay que mutar su método .load en
// vez de reemplazar el objeto entero.
document.fonts.load = function (spec) {
	if (spec.indexOf('SlowProbe') !== -1) {
		return new Promise(function (res) { setTimeout(function () { res([]); }, 300); });
	}
	if (spec.indexOf('FastFailProbe') !== -1) {
		return Promise.reject(new Error('mock: fuente rota'));
	}
	return Promise.resolve([]);
};
</script>
<script>
` + bootstrapJS + `
</script>
</body></html>`

	if err := r.ensureBrowser(context.Background()); err != nil {
		t.Fatalf("ensureBrowser: %v", err)
	}
	tabCtx, cancel := chromedp.NewContext(r.ctx)
	defer cancel()

	var ranAfterMs *int
	err := chromedp.Run(tabCtx,
		navigateAndSetContent(html),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate("window.__ranAfterMs", &ranAfterMs),
	)
	if err != nil {
		t.Fatalf("medición del bootstrap: %v", err)
	}
	if ranAfterMs == nil {
		t.Fatal("mermaid.run() nunca se llamó dentro del tiempo de espera")
	}
	return *ranAfterMs
}

// ---------------------------------------------------------------------------
// Tercera ronda: el CSS embebido no se serializaba como XML
// ---------------------------------------------------------------------------

// TestEmbedFontFacesInSVG_AmpersandIsWellFormedXML es el hallazgo "el CSS
// embebido no se serializa como XML". Un nombre de familia con `&` es CSS
// perfectamente válido —cssEscapeString no lo toca porque no hace falta
// escaparlo para CSS— pero insertado crudo dentro de un <style> de un
// documento XML de verdad (el SVG extraído, a diferencia del <style> HTML de
// la página temporal) deja de ser XML bien formado.
//
// Se verifica con encoding/xml, que decodifica el documento completo token
// por token: si CUALQUIER parte no es XML bien formado, Decode falla. No es
// una aproximación — un decoder XML real, no una heurística de string.
func TestEmbedFontFacesInSVG_AmpersandIsWellFormedXML(t *testing.T) {
	theme := renderer.DiagramThemeColors{Fonts: []renderer.DiagramFontFace{
		{Family: `AT&T Sans <Special> "Edition"`, Src: "data:font/woff2;base64,QUJD"},
	}}
	svg := embedFontFacesInSVG(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"></svg>`, theme)

	if !strings.Contains(svg, "AT&T Sans") {
		t.Fatalf("el nombre de familia no llegó al SVG:\n%s", svg)
	}

	dec := xml.NewDecoder(strings.NewReader(svg))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("el SVG no es XML bien formado: %v\n%s", err, svg)
		}
	}
}

// TestEmbedFontFacesInSVG_NegativeControl_RawAmpersandBreaksXML es el control
// negativo: sin CDATA (la versión previa a este arreglo), el mismo tema
// SÍ debe romper el XML — o el test de arriba no estaría detectando nada.
func TestEmbedFontFacesInSVG_NegativeControl_RawAmpersandBreaksXML(t *testing.T) {
	theme := renderer.DiagramThemeColors{Fonts: []renderer.DiagramFontFace{
		{Family: "AT&T Sans", Src: "data:font/woff2;base64,QUJD"},
	}}
	css := theme.FontFaceCSS()
	svgRoto := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><style>` + css + `</style></svg>`

	dec := xml.NewDecoder(strings.NewReader(svgRoto))
	var sawError bool
	for {
		_, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			sawError = true
			break
		}
	}
	if !sawError {
		t.Fatal("la reconstrucción sin CDATA debía romper el XML, y no lo hizo — el control negativo no detecta nada")
	}
}

// TestRenderMermaidToSVGWithTheme_AmpersandFamilyStillLoadsAsImg es la
// consecuencia real y grave del hallazgo, verificada en Chromium de verdad:
// antes del arreglo, un <img> cuyo SVG trae un & crudo NO CARGA — se pierde
// el diagrama completo, no solo la fuente (comprobado fuera de esta suite,
// vía herramienta de navegador, con DOMParser e <img> reales). Con CDATA
// tiene que cargar Y aplicar la fuente, igual que los casos sin `&`.
func TestRenderMermaidToSVGWithTheme_AmpersandFamilyStillLoadsAsImg(t *testing.T) {
	src := probeMonoFontDataURI(t)
	const familia = "Zira & Probe Mono"
	theme := renderer.DiagramThemeColors{Fonts: []renderer.DiagramFontFace{{Family: familia, Src: src}}}
	const label = "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"

	// La regla que aplica la familia al <text> también trae un '&' crudo, y
	// se envuelve en su PROPIO CDATA — a propósito, para que un fallo de
	// well-formedness en el harness del test no se confunda con el hallazgo
	// bajo prueba, que es específicamente sobre embedFontFacesInSVG.
	textStyle := `<style><![CDATA[text { font-family: "` + familia + `"; font-size: 24px; }]]></style>`
	base := `<svg xmlns="http://www.w3.org/2000/svg" width="600" height="100">` + textStyle +
		`<text x="5" y="50">` + label + `</text></svg>`

	r := newTestChromiumRenderer(t)
	// El @font-face lo arma FontFaceCSS() y lo inserta embedFontFacesInSVG
	// —las dos funciones de producción bajo prueba— no un string a mano.
	conFuente := embedFontFacesInSVG(base, theme)
	sinFuente := embedFontFacesInSVG(base, renderer.DiagramThemeColors{}) // zero value: no-op, byte por byte igual a base

	anchoConFuente := darkPixelExtentOfSVGAsImg(t, r, conFuente)
	anchoSinFuente := darkPixelExtentOfSVGAsImg(t, r, sinFuente)
	t.Logf("familia con '&': ancho con @font-face=%d, sin @font-face=%d", anchoConFuente, anchoSinFuente)

	if anchoConFuente == 0 {
		t.Fatal("el <img> no cargó en absoluto: el & crudo debe haber roto el XML")
	}
	if anchoConFuente <= anchoSinFuente {
		t.Errorf("el <img> cargó pero no aplicó la fuente: con=%d sin=%d", anchoConFuente, anchoSinFuente)
	}
}
