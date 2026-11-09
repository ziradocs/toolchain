// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "strings"

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
// lee como código: los code spans (`…`) y las cercas de código escritas dentro
// de la prosa (``` … ```, en un TEXT o en un ::: sin tipar). Sin la opción es
// ProcessVariables.
func proseVariables(text string, variables map[string]interface{}) string {
	if !isLiteralCode(variables) || !strings.Contains(text, "{{") {
		return ProcessVariables(text, variables)
	}
	return substituteOutsideCode(text, variables)
}

// substituteOutsideCode es la sustitución de proseVariables con la opción
// prendida: recorre text por líneas, deja intactas las cercas de código (una
// línea de tres o más backticks sin backticks en el info string abre; una de
// solo backticks, al menos tantos, cierra, como en el parser) y, en el resto,
// sustituye fuera de los code spans.
func substituteOutsideCode(text string, variables map[string]interface{}) string {
	lines := strings.SplitAfter(text, "\n")
	var out, prose strings.Builder
	flush := func() {
		out.WriteString(substituteOutsideSpans(prose.String(), variables))
		prose.Reset()
	}
	fence := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if fence > 0 {
			out.WriteString(line)
			if len(trimmed) >= fence && strings.Trim(trimmed, "`") == "" {
				fence = 0
			}
			continue
		}
		if n := proseFenceOpen(trimmed); n > 0 {
			flush()
			fence = n
			out.WriteString(line)
			continue
		}
		prose.WriteString(line)
	}
	flush()
	return out.String()
}

// proseFenceOpen es elements.CodeFenceOpen, repetida acá para no importar el
// paquete de parsers desde el renderer.
func proseFenceOpen(trimmed string) int {
	n := 0
	for n < len(trimmed) && trimmed[n] == '`' {
		n++
	}
	if n < 3 || strings.Contains(trimmed[n:], "`") {
		return 0
	}
	return n
}

// substituteOutsideSpans sustituye en text salvo dentro de sus code spans, que
// empareja findCodeSpans con la misma regla con la que el Markdown los
// renderiza.
func substituteOutsideSpans(text string, variables map[string]interface{}) string {
	spans := findCodeSpans(text)
	if len(spans) == 0 {
		return ProcessVariables(text, variables)
	}
	var b strings.Builder
	last := 0
	for _, sp := range spans {
		b.WriteString(ProcessVariables(text[last:sp.fullStart], variables))
		b.WriteString(text[sp.fullStart:sp.fullEnd])
		last = sp.fullEnd
	}
	b.WriteString(ProcessVariables(text[last:], variables))
	return b.String()
}
