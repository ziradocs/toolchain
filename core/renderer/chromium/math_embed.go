// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"embed"
	"fmt"
	"path"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// embeddedMathJaxJS es el mismo bundle que carga el HTML en modo browser
// (renderer.MathCDNScriptTag: mathjax@4.1.3/tex-svg.js), copiado tal cual de
// jsDelivr; TestEmbeddedMathJax_MatchesCDNIntegrity fija que su sha384 sea el
// hash SRI de ese tag. Licencia Apache-2.0: third_party/mathjax/LICENSE y
// NOTICE (los glifos de la fuente que trae adentro: FONT-LICENSE).
//
//go:embed third_party/mathjax/tex-svg.js
var embeddedMathJaxJS string

var embeddedMathJaxScriptTag = inlineScriptTag(embeddedMathJaxJS)

// mathJaxAssets es lo que MathJax 4 carga a demanda y tex-svg.js no trae:
//
//   - las extensiones de TeX de mathjax@4.1.3 que no vienen precargadas
//     (\cancel, \boldsymbol, \color, \ce, ...), menos html y texhtml, que
//     meten HTML/enlaces del autor en la salida. Las que se cargan por
//     autoload no necesitan nada; las que no (cases para numcases, colortbl
//     para \rowcolor) piden \require{cases} o \require{colortbl} en la
//     fórmula, igual que en el modo browser con el CDN;
//   - ui/safe de mathjax@4.1.3, que filtra los estilos, clases, ids y URLs
//     que el autor pone con \bbox y compañía, para que el SVG que termina en
//     el HTML offline o en el PDF no pida nada a la red;
//   - los 40 archivos dinámicos de la fuente @mathjax/mathjax-newcm-font
//     4.1.3 (la versión de la que depende mathjax@4.1.3, su package.json pide
//     ^4.1.3): glifos de \mathbb, \mathfrak, \mathscr, \mathsf, acentos y
//     alfabetos no latinos;
//   - svg.js de @mathjax/mathjax-mhchem-font-extension 4.1.3 (14 KB), los
//     glifos que pide la extensión mhchem (\ce). Las otras extensiones con
//     fuente propia (bbm, bboldx, dsfont) no se embeben: la licencia de sus
//     glifos no está identificada, así que \require{bbm} y compañía siguen
//     necesitando red y sin ella fallan con un aviso.
//
// No se inyectan en la página: MathJax los pide como siempre, por URL, a
// mathJaxAssetOrigin, y serveMathAssets contesta cada pedido desde acá
// interceptándolo en Chromium (Fetch domain). Así el loader de MathJax
// resuelve solo las dependencias entre extensiones (cancel necesita
// enclose, cases necesita empheq) y cada fórmula carga únicamente lo que
// usa, como en el modo browser. Lo que no está acá se rechaza sin salir a la
// red y la fórmula falla con un aviso.
//
//go:embed third_party/mathjax/input/tex/extensions/*.js
//go:embed third_party/mathjax/ui/safe.js
//go:embed third_party/mathjax-newcm-font/svg/dynamic/*.js
//go:embed third_party/mathjax-mhchem-font-extension/svg.js
var mathJaxAssets embed.FS

// mathJaxAssetOrigin es el origen ficticio desde el que MathJax pide sus
// archivos. .invalid es un TLD reservado (RFC 2606) que nunca resuelve: si la
// intercepción fallara, el pedido tampoco saldría a ningún servidor.
const mathJaxAssetOrigin = "https://mathjax.offline.invalid"

// mathJaxAssetPath traduce la URL que pide MathJax a la ruta en
// mathJaxAssets, y false si no es un archivo embebido.
func mathJaxAssetPath(url string) (string, bool) {
	rest, ok := strings.CutPrefix(url, mathJaxAssetOrigin+"/")
	if !ok {
		return "", false
	}
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}
	var p string
	switch {
	case strings.HasPrefix(rest, "mathjax/"):
		p = "third_party/" + rest
	case strings.HasPrefix(rest, "fonts/mathjax-newcm-font/"),
		strings.HasPrefix(rest, "fonts/mathjax-mhchem-font-extension/"):
		p = "third_party/" + strings.TrimPrefix(rest, "fonts/")
	default:
		return "", false
	}
	if path.Clean(p) != p || !strings.HasSuffix(p, ".js") {
		return "", false
	}
	if _, err := mathJaxAssets.Open(p); err != nil {
		return "", false
	}
	return p, true
}

// serveMathAssets activa la intercepción de pedidos del tab: un pedido a un
// archivo embebido se contesta con ese archivo y cualquier otro se rechaza.
// La página no tiene ningún otro origen permitido en su CSP, así que esto
// cubre todo lo que podría pedir.
func serveMathAssets() chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		if t == nil {
			// Sin target no hay de dónde escuchar los pedidos pausados:
			// activar Fetch acá dejaba cada pedido de MathJax colgado hasta el
			// plazo de la captura, sin nadie que lo contestara.
			return fmt.Errorf("math asset interception needs a chromedp target")
		}
		// La suscripción empieza al volver de cdp.Events, antes de
		// Fetch.enable, así que no se pierde ningún pedido. El iterador
		// guarda los eventos sin límite y termina cuando termina ctx (el tab
		// de la captura); contestar desde el loop no frena la lectura de
		// eventos de chromedp como lo hacía contestar dentro del viejo
		// listener de ListenTarget.
		paused := cdp.Events(ctx, t, fetch.RequestPaused)
		go func() {
			for e, err := range paused {
				if err != nil {
					return
				}
				if e.Request != nil {
					if p, ok := mathJaxAssetPath(e.Request.URL); ok {
						if data, err := mathJaxAssets.ReadFile(p); err == nil {
							_, _ = cdp.Call(ctx, t, fetch.FulfillRequest, fetch.FulfillRequestParams{
								RequestID:       e.RequestID,
								ResponseCode:    200,
								ResponseHeaders: []*fetch.HeaderEntry{{Name: "Content-Type", Value: "text/javascript; charset=utf-8"}},
								// Body es []byte: cdproto lo codifica en base64.
								Body: data,
							})
							continue
						}
					}
				}
				_, _ = cdp.Call(ctx, t, fetch.FailRequest, fetch.FailRequestParams{
					RequestID:   e.RequestID,
					ErrorReason: network.ErrorReasonBlockedByClient,
				})
			}
		}()
		_, err := cdp.Call(ctx, t, fetch.Enable, fetch.EnableParams{
			Patterns: []*fetch.RequestPattern{{URLPattern: "*"}},
		})
		return err
	})
}

// mathJaxConfigScript configura MathJax antes de que cargue:
//
//   - loader.paths apunta mathjax y fonts a mathJaxAssetOrigin (ver
//     serveMathAssets). Además, inline MathJax no tiene de dónde sacar su
//     raíz y la página es about:blank: sin una raíz absoluta,
//     `new URL(raíz, location)` lanzaba "Invalid URL" y MathJax no arrancaba.
//   - loader.load incluye ui/safe, así startup espera a que cargue antes de
//     armar el documento (si se registrara después, no filtraría nada).
//   - si el typeset se rechaza (un archivo de fuente que no cargó), o termina
//     sin ninguna ecuación (un "{" sin cerrar se come el \] de cierre y
//     MathJax no ve nada que tipografiar), el motivo queda en
//     window.__renderError y la espera termina de inmediato. El
//     éxito no se señala desde acá: la promesa de MathJax puede quedar
//     pendiente del worker de accesibilidad (que la CSP bloquea), así que
//     mathStateJS mira el DOM.
const mathJaxConfigScript = `<script>window.MathJax = { loader: { load: ['ui/safe'], paths: { mathjax: '` + mathJaxAssetOrigin + `/mathjax', fonts: '` + mathJaxAssetOrigin + `/fonts' } }, startup: { pageReady: function () {
  return MathJax.startup.defaultPageReady().then(function () {
    if (!document.querySelector('mjx-container')) {
      window.__renderError = 'MathJax found no equation to typeset (unbalanced braces?)';
    }
  }, function (e) {
    window.__renderError = 'MathJax: ' + ((e && e.message) ? e.message : e);
  });
} } };</script>`

// mathStateJS es renderStateJS para math: listo cuando hay un
// mjx-container con su <svg>; error si la página lo reportó o si MathJax
// dibujó un error dentro de la fórmula. Antes esos errores en rojo terminaban
// en el documento sin ningún aviso. Son dos formas:
//
//   - data-mjx-error: LaTeX inválido, un entorno desconocido, una extensión
//     que no cargó;
//   - un macro no definido: la extensión noundefined lo dibuja como su nombre
//     en rojo (un mtext rojo cuyo data-latex es el macro pelado, "\foo"), sin
//     data-mjx-error.
const mathStateJS = `(() => {
  if (window.__renderError) { return {error: String(window.__renderError)}; }
  const c = document.querySelector('mjx-container');
  if (!c) { return null; }
  const e = c.querySelector('[data-mjx-error]');
  if (e) { return {error: 'MathJax: ' + e.getAttribute('data-mjx-error')}; }
  for (const t of c.querySelectorAll('[data-mml-node="mtext"]')) {
    const latex = t.getAttribute('data-latex') || '';
    if (/^\\[A-Za-z]+$/.test(latex) && (t.getAttribute('fill') === 'red' || t.getAttribute('color') === 'red')) {
      return {error: 'MathJax: undefined control sequence ' + latex};
    }
  }
  if (c.querySelector('svg')) { return {ready: true}; }
  return null;
})()`

// mathRenderCSP es la CSP de las páginas de math: el bundle va inline y lo
// demás solo puede venir de mathJaxAssetOrigin, que nunca sale a la red
// (serveMathAssets).
const mathRenderCSP = "default-src 'none'; script-src 'unsafe-inline' " + mathJaxAssetOrigin + "; " +
	"style-src 'unsafe-inline'; img-src data:; connect-src 'none'; object-src 'none'; base-uri 'none';"

// mathPageHead son las etiquetas del <head> de una página de math: CSP,
// configuración y el bundle, en ese orden.
func mathPageHead() string {
	return `<meta http-equiv="Content-Security-Policy" content="` + mathRenderCSP + `">
    ` + mathJaxConfigScript + `
    ` + embeddedMathJaxScriptTag
}

const mathJaxFontDir = "third_party/mathjax-newcm-font/svg/dynamic"
