// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"

	"go.ziradocs.com/core/v2/renderer"
)

// inlineImageTagPrefix abre exactamente el <img> que
// renderer.ProcessInlineMarkdownSecure emite para un ![alt](src) inline:
// `<img src="…" alt="…">`, con ` title="…"` opcional antes del ">". El src ya
// pasó por SanitizeURL y el alt es texto plano escapado, así que ninguno lleva
// comillas. El texto del autor llega escapado a esa función, así que un
// "<img" que el autor escribió nunca tiene esta forma; la imagen de bloque la
// arma el template con otro formato y se resuelve aparte (InlinedSource).
const inlineImageTagPrefix = `<img src="`

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
//
// Recorre el HTML una vez buscando el prefijo y salta los data: URI sin
// leerlos (un regex con capturas sobre el HTML entero recorría cada imagen
// de bloque ya incrustada). Cada fuente se lee del disco una sola vez aunque
// aparezca muchas veces.
func inlineLocalInlineImages(html string, ctx *renderer.RenderContext) string {
	if ctx == nil || ctx.ImageMode != "offline-inline" || ctx.AssetRoot == "" || !strings.Contains(html, inlineImageTagPrefix) {
		return html
	}
	type resolved struct {
		uri string
		ok  bool
	}
	cache := map[string]resolved{}
	var b strings.Builder
	last := 0
	for i := 0; ; {
		k := strings.Index(html[i:], inlineImageTagPrefix)
		if k < 0 {
			break
		}
		start := i + k
		srcStart := start + len(inlineImageTagPrefix)
		i = srcStart
		if strings.HasPrefix(html[srcStart:], "data:") {
			continue
		}
		src, alt, title, end, ok := parseInlineImageTag(html, srcStart)
		if !ok {
			continue
		}
		r, seen := cache[src]
		if !seen {
			r.uri, r.ok = tryInlineLocalImage(renderer.UnescapeHTML(src), ctx)
			cache[src] = r
		}
		i = end
		if !r.ok {
			continue
		}
		if last == 0 {
			b.Grow(len(html))
		}
		b.WriteString(html[last:start])
		b.WriteString(`<img src="` + r.uri + `" alt="` + alt + `"` + title + `>`)
		last = end
	}
	if last == 0 {
		return html
	}
	b.WriteString(html[last:])
	return b.String()
}

// tryInlineLocalImage es renderer.TryInlineLocalImage; las pruebas la
// cambian para contar lecturas.
var tryInlineLocalImage = renderer.TryInlineLocalImage

// parseInlineImageTag lee, desde srcStart (justo después de `<img src="`),
// el resto de la forma `<img src="S" alt="A">` o `<img src="S" alt="A"
// title="T">`: devuelve S, A, el ` title="T"` completo (o "") y dónde termina
// el tag.
func parseInlineImageTag(html string, srcStart int) (src, alt, title string, end int, ok bool) {
	q := strings.IndexByte(html[srcStart:], '"')
	if q < 0 {
		return "", "", "", 0, false
	}
	src = html[srcStart : srcStart+q]
	p := srcStart + q + 1
	const altOpen = ` alt="`
	if !strings.HasPrefix(html[p:], altOpen) {
		return "", "", "", 0, false
	}
	p += len(altOpen)
	q = strings.IndexByte(html[p:], '"')
	if q < 0 {
		return "", "", "", 0, false
	}
	alt = html[p : p+q]
	p += q + 1
	const titleOpen = ` title="`
	if strings.HasPrefix(html[p:], titleOpen) {
		t := p + len(titleOpen)
		q = strings.IndexByte(html[t:], '"')
		if q < 0 {
			return "", "", "", 0, false
		}
		title = html[p : t+q+1]
		p = t + q + 1
	}
	if p >= len(html) || html[p] != '>' {
		return "", "", "", 0, false
	}
	return src, alt, title, p + 1, true
}
