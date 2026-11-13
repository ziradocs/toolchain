// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"

	"go.ziradocs.com/core/v2/renderer"
)

// fieldVariables sustituye `{{nombre}}` en un campo de texto de una línea
// (una leyenda, un título, un texto alternativo) fuera de sus code spans: lo
// que va entre backticks es código y queda literal (issue #487), como en el
// HTML de core con RenderContext.LiteralCode, que empareja los code spans
// sobre el campo entero. El campo se escribe sin Markdown, así que los
// backticks se quedan en el texto, igual que en el HTML.
//
// Un `{{nombre}}` no tiene backticks, así que nunca cruza el borde de un code
// span: sustituir tramo por tramo da lo mismo que sustituir todo fuera de los
// spans.
func fieldVariables(text string, variables map[string]interface{}) string {
	if !strings.Contains(text, "{{") || !strings.Contains(text, "`") {
		return renderer.ProcessVariables(text, variables)
	}
	var b strings.Builder
	last := 0
	for _, sp := range markdownCodeSpans(text) {
		b.WriteString(renderer.ProcessVariables(text[last:sp[0]], variables))
		b.WriteString(text[sp[0]:sp[1]])
		last = sp[1]
	}
	b.WriteString(renderer.ProcessVariables(text[last:], variables))
	return b.String()
}
