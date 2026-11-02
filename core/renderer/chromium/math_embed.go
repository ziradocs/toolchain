// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"embed"
	"path"
	"regexp"
	"strings"
	"sync"
)

// embeddedMathJaxJS es el mismo bundle que carga el HTML en modo browser
// (renderer.MathCDNScriptTag: mathjax@4.1.3/tex-svg.js), copiado tal cual de
// jsDelivr; TestEmbeddedMathJax_MatchesCDNIntegrity fija que su sha384 sea el
// hash SRI de ese tag. Licencia Apache-2.0: third_party/mathjax/LICENSE y
// NOTICE.
//
// tex-svg.js solo no basta sin red. MathJax 4 carga a demanda, desde
// jsDelivr, dos cosas que el bundle no trae:
//
//   - los glifos menos comunes de la fuente (\mathbb, \mathfrak, \mathscr,
//     \mathsf, \mathtt, letras acentuadas dentro de \text{}, algunas flechas
//     y símbolos): sin ellos el typeset entero se rechaza con "dynamic file
//     'double-struck' failed to load" y no sale nada;
//   - las extensiones de TeX que no vienen precargadas (\boldsymbol, \color,
//     \ce, \bra/\ket, ...): sin ellas la fórmula sale con el error en rojo
//     "Extension "color" failed to load".
//
// Los dos se embeben acá (mathJaxAssets). El primer render va con el bundle
// solo, que alcanza para la mayoría de las fórmulas; si la página reporta que
// le faltó un archivo embebido, se repite una sola vez con TODOS los archivos
// precargados (ver renderMathWithPreload). Antes se agregaba un archivo por
// intento y una fórmula que pedía muchos agotaba los intentos.
//
// Además de lo que pide la fórmula, siempre va ui/safe (mathjax@4.1.3
// ui/safe.js, 6 KB): filtra los atributos que el autor puede poner con
// \bbox, \color y compañía (URLs externas en estilos, clases, ids), para que
// el SVG que termina en el HTML offline o en el PDF no pida nada a la red.
//
//go:embed third_party/mathjax/tex-svg.js
var embeddedMathJaxJS string

var embeddedMathJaxScriptTag = inlineScriptTag(embeddedMathJaxJS)

//go:embed third_party/mathjax/ui/safe.js
var embeddedMathJaxSafeJS string

var embeddedMathJaxSafeScriptTag = inlineScriptTag(embeddedMathJaxSafeJS) +
	`<script>MathJax.loader.preLoaded("ui/safe")</script>`

// mathJaxAssets son las extensiones de TeX de mathjax@4.1.3 que tex-svg.js no
// precarga (menos html y texhtml, que meten HTML/enlaces del autor en la
// salida) y los 40 archivos dinámicos de la fuente
// @mathjax/mathjax-newcm-font 4.1.3, la versión de la que depende
// mathjax@4.1.3 (su package.json pide ^4.1.3) y la que el modo browser carga
// del CDN. Una fórmula que necesite algo que no está acá (una extensión
// excluida) falla con un aviso, no en silencio.
//
//go:embed third_party/mathjax/input/tex/extensions/*.js
//go:embed third_party/mathjax-newcm-font/svg/dynamic/*.js
var mathJaxAssets embed.FS

const (
	mathJaxExtensionDir = "third_party/mathjax/input/tex/extensions"
	mathJaxFontDir      = "third_party/mathjax-newcm-font/svg/dynamic"
)

// mathPreload dice qué se precarga en una página de math además de
// tex-svg.js y ui/safe: nada (el primer intento) o todos los archivos
// embebidos.
type mathPreload struct {
	all bool
}

// scripts devuelve los <script> inline de la precarga.
func (p mathPreload) scripts() string {
	if !p.all {
		return ""
	}
	allMathPreloadOnce.Do(func() { allMathPreloadScripts = buildAllMathPreloadScripts() })
	return allMathPreloadScripts
}

var (
	allMathPreloadOnce    sync.Once
	allMathPreloadScripts string
)

// buildAllMathPreloadScripts arma cada archivo embebido seguido de
// MathJax.loader.preLoaded con el mismo nombre que el loader pediría, así el
// pedido se resuelve sin red.
func buildAllMathPreloadScripts() string {
	var b strings.Builder
	add := func(dir, loaderPrefix string) {
		entries, err := mathJaxAssets.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			data, err := mathJaxAssets.ReadFile(path.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			b.WriteString(inlineScriptTag(string(data)))
			b.WriteString(`<script>MathJax.loader.preLoaded("` + loaderPrefix + strings.TrimSuffix(e.Name(), ".js") + `")</script>`)
		}
	}
	add(mathJaxExtensionDir, "[tex]/")
	add(mathJaxFontDir, "[mathjax-newcm]/svg/dynamic/")
	return b.String()
}

var (
	mathMissingFontRe      = regexp.MustCompile(`dynamic file '([A-Za-z0-9-]+)' failed to load`)
	mathMissingExtensionRe = regexp.MustCompile(`Extension "([A-Za-z0-9-]+)" failed to load`)
)

// needsFullPreload dice si msg (el error que reportó la página) nombra un
// archivo embebido: ahí vale la pena repetir con todo precargado. Si nombra
// uno que no está embebido, o es otro error (LaTeX inválido), repetir no
// cambiaría nada.
func needsFullPreload(msg string) bool {
	if m := mathMissingFontRe.FindStringSubmatch(msg); m != nil {
		return mathAssetEmbedded(mathJaxFontDir, m[1])
	}
	if m := mathMissingExtensionRe.FindStringSubmatch(msg); m != nil {
		return mathAssetEmbedded(mathJaxExtensionDir, m[1])
	}
	return false
}

func mathAssetEmbedded(dir, name string) bool {
	_, err := mathJaxAssets.Open(path.Join(dir, name+".js"))
	return err == nil
}

// mathJaxConfigScript configura MathJax antes de que cargue:
//
//   - loader.paths.mathjax es la URL del CDN de la que el bundle se habría
//     cargado. Inline, MathJax no tiene de dónde sacar su raíz y la página
//     es about:blank, así que `new URL(raíz, location)` lanzaba "Invalid URL"
//     y MathJax no arrancaba. Nada se pide a esa URL: lo que hace falta va
//     precargado y la CSP no deja salir a la red.
//   - startup.ready espera a mathAssetsLoadedScript, el último <script> del
//     <head>: el bundle puede llegar a su ready entre un <script> inline y el
//     siguiente, y ui/safe tiene que registrarse antes de que MathJax arme el
//     documento (si no, no filtra nada).
//   - si el typeset se rechaza (un archivo de fuente que no cargó), o termina
//     sin ninguna ecuación (un "{" sin cerrar se come el \] de cierre y
//     MathJax no ve nada que tipografiar), el motivo queda en
//     window.__renderError y la espera termina de inmediato. El
//     éxito no se señala desde acá: la promesa de MathJax puede quedar
//     pendiente del worker de accesibilidad (que la CSP bloquea), así que
//     mathStateJS mira el DOM.
const mathJaxConfigScript = `<script>window.MathJax = { loader: { paths: { mathjax: 'https://cdn.jsdelivr.net/npm/mathjax@4.1.3' } }, startup: { ready: function () {
  if (window.__mathAssetsLoaded) { MathJax.startup.defaultReady(); } else { window.__mathReady = function () { MathJax.startup.defaultReady(); }; }
}, pageReady: function () {
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
// dibujó un error dentro de la fórmula (data-mjx-error: LaTeX inválido o una
// extensión que no cargó). Antes ese error en rojo terminaba en el
// documento sin ningún aviso.
const mathStateJS = `(() => {
  if (window.__renderError) { return {error: String(window.__renderError)}; }
  const c = document.querySelector('mjx-container');
  if (!c) { return null; }
  const e = c.querySelector('[data-mjx-error]');
  if (e) { return {error: 'MathJax: ' + e.getAttribute('data-mjx-error')}; }
  if (c.querySelector('svg')) { return {ready: true}; }
  return null;
})()`

// mathRenderCSP es la CSP de las páginas de math: sin ningún origen de red,
// porque MathJax y lo que precarga van inline.
const mathRenderCSP = mermaidRenderCSP

// mathAssetsLoadedScript libera el startup.ready de mathJaxConfigScript
// cuando ya corrieron ui/safe y la precarga.
const mathAssetsLoadedScript = `<script>window.__mathAssetsLoaded = true; if (window.__mathReady) { window.__mathReady(); }</script>`

// mathPageHead son las etiquetas del <head> de una página de math: CSP,
// configuración, el bundle, ui/safe, la precarga y la señal de que ya está
// todo, en ese orden (ui/safe y la precarga usan MathJax.loader, que existe
// recién después del bundle).
func mathPageHead(p mathPreload) string {
	return `<meta http-equiv="Content-Security-Policy" content="` + mathRenderCSP + `">
    ` + mathJaxConfigScript + `
    ` + embeddedMathJaxScriptTag + embeddedMathJaxSafeScriptTag + p.scripts() + mathAssetsLoadedScript
}
