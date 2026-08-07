// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

// buildHeadingElement construye el TextElement RawHTML `<hN id="...">` con
// que un documento representa un encabezado de nivel 2-6, para los dos
// dialectos: el `## Título` de flex y el `SECTION "Título" / level: N` de
// strict.
//
// Es una función compartida y no dos implementaciones porque el HTML y el
// anchor que produce son un CONTRATO, no un detalle: el generador de TOC
// (renderer.extractSubsections) los re-extrae con una expresión regular
// sobre este mismo `<hN id=...>`. Dos productores que derivaran aunque sea
// en el orden de los atributos romperían el TOC de uno de los dialectos en
// silencio.
//
// lineIndex es el índice 0-based de la línea que origina el encabezado; la
// posición del diagnóstico se deriva sumándole 1. Se pasa explícito en vez
// de leerse del estado del parser porque el parser strict construye el
// encabezado DESPUÉS de consumir el cuerpo de la sección, cuando su línea
// actual ya no es la del `SECTION`.
//
// explicitID vacío deriva el anchor del texto (lo que hace flex, único
// camino posible ahí). No vacío, lo usa como base — pero pasa por
// EXACTAMENTE el mismo saneado: el anchor se interpola crudo dentro de un
// atributo HTML, así que un id de autor sin sanear sería una inyección
// directa (`id: x"><script>`). Ver sanitizeAnchor.
func buildHeadingElement(text string, level, lineIndex int, explicitID string) *ast.TextElement {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}

	// Procesar Markdown inline básico (**, *, `, etc.) de forma segura:
	// escapa el HTML del texto y aplica formatos con un procesador de un
	// solo paso (RE2, lineal) — evita el DoS de bucle infinito del antiguo
	// scanner hand-written y cierra el XSS de subsección en el mismo golpe
	// (ver docs/SECURITY_AUDIT_2026-07.md, AL-3 y CR-2).
	// Se usa la variante "Line" (no la ProcessInlineMarkdownSecure genérica):
	// un header es una sola línea y nunca debe interpretarse como lista
	// ("- Foo" no debe volverse <ul><li>Foo</li></ul> dentro de un <h3>).
	processedText := renderer.ProcessInlineMarkdownSecureLine(text)

	anchorSource := text
	if explicitID != "" {
		anchorSource = explicitID
	}
	anchor := deriveAnchor(anchorSource)

	pos := diagnostics.NewPosition(lineIndex+1, 1)
	htmlContent := fmt.Sprintf("<h%d id=\"%s\">%s</h%d>", level, anchor, processedText, level)

	// Expose the level as a semantic field alongside the rendered `<hN>`, so
	// a linter rule doesn't have to re-parse the HTML (issue #22).
	el := ast.NewRawHTMLTextElement(pos, htmlContent)
	el.Level = level
	return el
}

// deriveAnchor convierte un texto libre en el anchor de un encabezado:
// minúsculas, espacios a guiones y saneado. Mismo algoritmo que usa el
// renderer para generar los enlaces del TOC.
func deriveAnchor(text string) string {
	return sanitizeAnchor(strings.ToLower(strings.ReplaceAll(text, " ", "-")))
}

// sanitizeAnchor limpia un anchor para usarlo en href/id.
//
// La lista de ReplaceAll de arriba es cosmética (quita puntuación antes de
// que el filtro final la borre igual); lo que hace segura a esta función es
// el bucle del final, que construye la salida con una LISTA BLANCA de
// [a-z0-9_-]. Cualquier carácter capaz de escapar del atributo HTML donde
// se interpola el anchor —comillas, `<`, `>`— no está en esa lista y se
// descarta. Por eso un id provisto por el autor puede pasar por acá sin
// abrir una inyección; ver buildHeadingElement.
func sanitizeAnchor(anchor string) string {
	anchor = strings.ReplaceAll(anchor, ".", "")
	anchor = strings.ReplaceAll(anchor, ",", "")
	anchor = strings.ReplaceAll(anchor, ":", "")
	anchor = strings.ReplaceAll(anchor, ";", "")
	anchor = strings.ReplaceAll(anchor, "!", "")
	anchor = strings.ReplaceAll(anchor, "?", "")
	anchor = strings.ReplaceAll(anchor, "(", "")
	anchor = strings.ReplaceAll(anchor, ")", "")
	anchor = strings.ReplaceAll(anchor, "[", "")
	anchor = strings.ReplaceAll(anchor, "]", "")
	anchor = strings.ReplaceAll(anchor, "{", "")
	anchor = strings.ReplaceAll(anchor, "}", "")
	anchor = strings.ReplaceAll(anchor, "/", "")
	anchor = strings.ReplaceAll(anchor, "\\", "")
	anchor = strings.ReplaceAll(anchor, "'", "")
	anchor = strings.ReplaceAll(anchor, "\"", "")
	anchor = strings.ReplaceAll(anchor, "`", "")
	// Eliminar emojis y caracteres especiales (mantener solo letras, números, guiones)
	var cleaned strings.Builder
	for _, r := range anchor {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			cleaned.WriteRune(r)
		}
	}
	return cleaned.String()
}
