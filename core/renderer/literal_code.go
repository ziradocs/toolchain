// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"

	"go.ziradocs.com/core/v2/internal/fence"
)

// literalCodeMarker es el valor que marca un mapa de variables con la opción de
// código literal (RenderContext.LiteralCode, InlineHTMLOptions.LiteralCode).
// Es un tipo no exportado: fuera de este paquete nadie puede construirlo, así
// que la marca no se puede poner ni falsificar desde un frontmatter ni desde
// otro paquete.
type literalCodeMarker struct{}

// literalCodeKey es la clave de la marca. No es UTF-8 válido, así que ningún
// frontmatter (que el parser de YAML exige en UTF-8) produce esa clave, y el
// patrón de {{nombre}} tampoco la puede nombrar.
const literalCodeKey = "\xffliteral-code"

// withLiteralCode devuelve una copia de variables marcada con la opción de
// código literal. La opción viaja así con el mapa por todas las funciones de
// render y de población del AST, sin cambiar sus firmas: las que sustituyen
// prosa (proseVariables) y cuerpos de código (codeBody) la consultan. Un mapa
// nil queda nil: sin variables no se sustituye nada y todo ya es literal.
func withLiteralCode(variables map[string]interface{}) map[string]interface{} {
	if variables == nil || isLiteralCode(variables) {
		return variables
	}
	marked := make(map[string]interface{}, len(variables)+1)
	for k, v := range variables {
		marked[k] = v
	}
	marked[literalCodeKey] = literalCodeMarker{}
	return marked
}

// isLiteralCode dice si variables lleva la marca de withLiteralCode.
func isLiteralCode(variables map[string]interface{}) bool {
	_, ok := variables[literalCodeKey].(literalCodeMarker)
	return ok
}

// codeBody es el cuerpo de un bloque de código listo para escapar: literal con
// la opción de código literal (issue #487), con `{{nombre}}` sustituido si no.
func codeBody(content string, variables map[string]interface{}) string {
	if isLiteralCode(variables) {
		return content
	}
	return ProcessVariables(content, variables)
}

// proseVariables sustituye `{{nombre}}` en un texto de prosa que después pasa
// por Markdown. Con la opción de código literal no toca lo que ese Markdown
// lee como código: los code spans (`…`, emparejados línea por línea, como los
// empareja el renderer) y, en un texto de varias líneas (un TEXT, una cita, el
// contenido de un ::: sin tipar), las cercas de código escritas adentro, con la
// regla de cercas del parser (internal/fence). En un texto de una sola línea
// (una celda, un encabezado) no hay cerca: una línea que empieza con ``` es
// prosa. Lo literal se decide por la sintaxis de la fuente. Sin la opción es
// ProcessVariables.
func proseVariables(text string, variables map[string]interface{}) string {
	if !isLiteralCode(variables) || !strings.Contains(text, "{{") {
		return ProcessVariables(text, variables)
	}
	if !strings.Contains(text, "\n") {
		return substituteOutsideSpans(text, variables)
	}
	return substituteOutsideCode(text, variables)
}

// substituteOutsideCode es la sustitución de proseVariables con la opción
// prendida en un texto de varias líneas: deja intactas las cercas de código y,
// en el resto, sustituye fuera de los code spans de cada línea.
func substituteOutsideCode(text string, variables map[string]interface{}) string {
	var out strings.Builder
	inFence := false
	for _, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case inFence:
			out.WriteString(line)
			if fence.Closes(trimmed) {
				inFence = false
			}
		case fence.Opens(trimmed):
			inFence = true
			out.WriteString(line)
		default:
			out.WriteString(substituteOutsideSpans(line, variables))
		}
	}
	return out.String()
}

// substituteOutsideSpans sustituye en una línea salvo dentro de sus code
// spans, que empareja findCodeSpans con la misma regla con la que el Markdown
// los renderiza.
func substituteOutsideSpans(line string, variables map[string]interface{}) string {
	spans := findCodeSpans(line)
	if len(spans) == 0 {
		return ProcessVariables(line, variables)
	}
	var b strings.Builder
	last := 0
	for _, sp := range spans {
		b.WriteString(ProcessVariables(line[last:sp.fullStart], variables))
		b.WriteString(line[sp.fullStart:sp.fullEnd])
		last = sp.fullEnd
	}
	b.WriteString(ProcessVariables(line[last:], variables))
	return b.String()
}
