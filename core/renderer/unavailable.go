// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"strings"
)

// unavailableKind es el tipo de contenido que no se pudo generar.
type unavailableKind int

const (
	unavailableImage unavailableKind = iota
	unavailableMap
	unavailableChart
	unavailableDiagram
	unavailableEquation
)

// unavailableNote es el texto que reemplaza en el documento a un contenido
// que no se pudo generar (una imagen que falta, un mapa sin red, un diagrama
// cuya captura falló). Antes el cuerpo del PDF mostraba el error interno en
// inglés ("Failed to render map: failed to render inline map: map rendering
// failed: context deadline exceeded") y una imagen faltante dejaba el ícono
// de imagen rota de Chromium. El detalle técnico se queda en el aviso del
// log; el lector ve solo qué falta, en el idioma del documento (mismo
// criterio de idioma que xref.Labels: "en" o, por defecto, español).
func unavailableNote(kind unavailableKind, lang string) string {
	primary, _, _ := strings.Cut(lang, "-")
	en := strings.EqualFold(primary, "en")
	switch kind {
	case unavailableMap:
		if en {
			return "Map not available"
		}
		return "Mapa no disponible"
	case unavailableChart:
		if en {
			return "Chart not available"
		}
		return "Gráfica no disponible"
	case unavailableDiagram:
		if en {
			return "Diagram not available"
		}
		return "Diagrama no disponible"
	case unavailableEquation:
		if en {
			return "Equation not available"
		}
		return "Ecuación no disponible"
	default:
		if en {
			return "Image not available"
		}
		return "Imagen no disponible"
	}
}

// contextLang es ctx.Lang tolerando un ctx nil.
func contextLang(ctx *RenderContext) string {
	if ctx == nil {
		return ""
	}
	return ctx.Lang
}

// unavailableDiv es el marcador de un chart, mapa, diagrama o ecuación que no
// se pudo generar. class es la clase de error de siempre ("map-error",
// "chart-error", ...), que se conserva exacta: slidelang la reescribe por
// OfflineElementClasses y los temas la pueden estilar.
func unavailableDiv(class string, kind unavailableKind, ctx *RenderContext) string {
	note := unavailableNote(kind, contextLang(ctx))
	return fmt.Sprintf(`<div class="%s" role="img" aria-label="%s"><span class="unavailable-note">%s</span></div>`,
		class, EscapeHTMLAttribute(note), EscapeHTML(note))
}

// imageUnavailableHTML es el marcador de una imagen que no se pudo cargar
// (falta, quedó fuera de la raíz de assets o su URL es peligrosa): la nota y
// el texto alternativo, con la leyenda y el ancla de la figura si los tiene.
// alt, caption y captionPrefix llegan ya escapados; idAttr ya armado.
func imageUnavailableHTML(alt, caption, idAttr, captionPrefix string, ctx *RenderContext) string {
	note := unavailableNote(unavailableImage, contextLang(ctx))
	label := EscapeHTMLAttribute(note)
	var b strings.Builder
	if alt != "" {
		label = alt
	}
	fmt.Fprintf(&b, `<div class="image-unavailable" role="img" aria-label="%s"><span class="unavailable-note">%s</span>`, label, EscapeHTML(note))
	if alt != "" {
		fmt.Fprintf(&b, `<span class="unavailable-title">%s</span>`, alt)
	}
	b.WriteString(`</div>`)
	if caption != "" {
		return fmt.Sprintf(`<figure%s>%s<figcaption>%s%s</figcaption></figure>`, idAttr, b.String(), captionPrefix, caption)
	}
	if idAttr != "" {
		return fmt.Sprintf(`<figure%s>%s</figure>`, idAttr, b.String())
	}
	return b.String()
}
