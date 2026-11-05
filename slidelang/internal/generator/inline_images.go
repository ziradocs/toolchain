// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/renderer"
)

// slideInlineImageTag es exactamente el <img> que
// renderer.ProcessInlineMarkdownSecure emite para un ![alt](src) inline: src
// ya pasado por SanitizeURL, alt como texto plano escapado y el title
// opcional del destino. El texto del autor llega escapado a esa función, así
// que un "<img" que el autor escribió nunca tiene esta forma; la imagen de
// bloque la arma el template con otro formato y se resuelve aparte
// (InlinedSource).
var slideInlineImageTag = regexp.MustCompile(`<img src="([^"]*)" alt="([^"]*)"((?: title="[^"]*")?)>`)

// inlineLocalInlineImages resuelve las imágenes inline de un párrafo, un
// ítem, una celda o un encabezado (`Texto ![x](a.png)`) igual que la imagen
// de bloque: en offline-inline, una fuente local se lee del disco con
// renderer.TryInlineLocalImage (el mismo confinamiento a AssetRoot) y va al
// src como data: URI. Antes quedaba el src relativo, y como el PDF se
// imprime desde about:blank, Chromium mostraba el ícono de imagen rota aunque
// el archivo existiera. Una fuente que no se puede leer (falta, queda fuera
// de la raíz o es remota) deja el <img> como estaba, igual que la de bloque.
// En los demás modos no cambia nada. Es idempotente: un data: URI ya no es
// local.
func inlineLocalInlineImages(html string, ctx *renderer.RenderContext) string {
	if ctx == nil || ctx.ImageMode != "offline-inline" || ctx.AssetRoot == "" || !strings.Contains(html, "<img ") {
		return html
	}
	return slideInlineImageTag.ReplaceAllStringFunc(html, func(tag string) string {
		m := slideInlineImageTag.FindStringSubmatch(tag)
		uri, ok := renderer.TryInlineLocalImage(renderer.UnescapeHTML(m[1]), ctx)
		if !ok {
			return tag
		}
		return `<img src="` + uri + `" alt="` + m[2] + `"` + m[3] + `>`
	})
}
