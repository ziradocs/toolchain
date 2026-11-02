// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"embed"
	"path"
	"regexp"
	"slices"
	"strings"
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
// Los dos se embeben acá (mathJaxAssets) y se inyectan inline solo cuando la
// fórmula los pide: el primer render va con el bundle solo; si la página
// reporta que le faltó un archivo embebido, se vuelve a renderizar con ese
// archivo precargado (ver RenderMathToSVGWithTheme). Una fórmula común no
// paga nada extra, y una que necesita varios archivos los va sumando.
//
//go:embed third_party/mathjax/tex-svg.js
var embeddedMathJaxJS string

var embeddedMathJaxScriptTag = inlineScriptTag(embeddedMathJaxJS)

// mathJaxAssets son las extensiones de TeX de mathjax@4.1.3 que tex-svg.js no
// precarga (menos html y texhtml, que meten HTML/enlaces del autor en la
// salida) y los archivos dinámicos de la fuente @mathjax/mathjax-newcm-font
// 4.1.2 (la versión contra la que tex-svg.js 4.1.3 chequea) que una fórmula
// matemática usa: no los de alfabetos como cirílico, hebreo o braille, ni las
// variantes negrita/cursiva de los acentuados. Una fórmula que necesite uno
// que no está acá falla con un aviso, no en silencio.
//
//go:embed third_party/mathjax/input/tex/extensions/*.js
//go:embed third_party/mathjax-newcm-font/svg/dynamic/*.js
var mathJaxAssets embed.FS

const (
	mathJaxExtensionDir = "third_party/mathjax/input/tex/extensions"
	mathJaxFontDir      = "third_party/mathjax-newcm-font/svg/dynamic"
)

// mathPreload es lo que se precarga inline en una página de math, además de
// tex-svg.js: nombres de extensión ("color") y de archivo de fuente
// ("double-struck").
type mathPreload struct {
	extensions []string
	fonts      []string
}

// scripts arma los <script> inline de la precarga: cada archivo seguido de
// MathJax.loader.preLoaded con el mismo nombre que el loader pediría, así
// el pedido se resuelve sin red. Un nombre que no está embebido se omite (no
// debería pasar: missingMathAsset solo devuelve nombres embebidos).
func (p mathPreload) scripts() string {
	var b strings.Builder
	add := func(dir, name, loaderName string) {
		data, err := mathJaxAssets.ReadFile(path.Join(dir, name+".js"))
		if err != nil {
			return
		}
		b.WriteString(inlineScriptTag(string(data)))
		b.WriteString(`<script>MathJax.loader.preLoaded("` + loaderName + `")</script>`)
	}
	for _, e := range p.extensions {
		add(mathJaxExtensionDir, e, "[tex]/"+e)
	}
	for _, f := range p.fonts {
		add(mathJaxFontDir, f, "[mathjax-newcm]/svg/dynamic/"+f)
	}
	return b.String()
}

var (
	mathMissingFontRe      = regexp.MustCompile(`dynamic file '([A-Za-z0-9-]+)' failed to load`)
	mathMissingExtensionRe = regexp.MustCompile(`Extension "([A-Za-z0-9-]+)" failed to load`)
)

// withMissing devuelve la precarga más el archivo embebido que pide msg (el
// error que reportó la página), y false si msg no nombra un archivo
// embebido que todavía no esté precargado: ahí reintentar no cambiaría nada.
func (p mathPreload) withMissing(msg string) (mathPreload, bool) {
	if m := mathMissingFontRe.FindStringSubmatch(msg); m != nil {
		if slices.Contains(p.fonts, m[1]) || !mathAssetEmbedded(mathJaxFontDir, m[1]) {
			return p, false
		}
		return mathPreload{extensions: p.extensions, fonts: append(slices.Clone(p.fonts), m[1])}, true
	}
	if m := mathMissingExtensionRe.FindStringSubmatch(msg); m != nil {
		if slices.Contains(p.extensions, m[1]) || !mathAssetEmbedded(mathJaxExtensionDir, m[1]) {
			return p, false
		}
		return mathPreload{extensions: append(slices.Clone(p.extensions), m[1]), fonts: p.fonts}, true
	}
	return p, false
}

func mathAssetEmbedded(dir, name string) bool {
	_, err := mathJaxAssets.Open(path.Join(dir, name+".js"))
	return err == nil
}

// mathMaxAttempts acota los reintentos de una fórmula: cada uno agrega un
// archivo, y una fórmula real no necesita más que unos pocos.
const mathMaxAttempts = 8

// mathJaxConfigScript configura MathJax antes de que cargue:
//
//   - loader.paths.mathjax es la URL del CDN de la que el bundle se habría
//     cargado. Inline, MathJax no tiene de dónde sacar su raíz y la página
//     es about:blank, así que `new URL(raíz, location)` lanzaba "Invalid URL"
//     y MathJax no arrancaba. Nada se pide a esa URL: lo que hace falta va
//     precargado y la CSP no deja salir a la red.
//   - si el typeset se rechaza (un archivo de fuente que no cargó), o termina
//     sin ninguna ecuación (un "{" sin cerrar se come el \] de cierre y
//     MathJax no ve nada que tipografiar), el motivo queda en
//     window.__renderError y la espera termina de inmediato. El
//     éxito no se señala desde acá: la promesa de MathJax puede quedar
//     pendiente del worker de accesibilidad (que la CSP bloquea), así que
//     mathStateJS mira el DOM.
const mathJaxConfigScript = `<script>window.MathJax = { loader: { paths: { mathjax: 'https://cdn.jsdelivr.net/npm/mathjax@4.1.3' } }, startup: { pageReady: function () {
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

// mathPageHead son las etiquetas del <head> de una página de math: CSP,
// configuración, el bundle y la precarga, en ese orden (la precarga usa
// MathJax.loader, que existe recién después del bundle).
func mathPageHead(p mathPreload) string {
	return `<meta http-equiv="Content-Security-Policy" content="` + mathRenderCSP + `">
    ` + mathJaxConfigScript + `
    ` + embeddedMathJaxScriptTag + p.scripts()
}
