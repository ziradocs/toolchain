// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// MediaParser maneja el parsing de elementos de audio/video embebido (issue
// #21), con el mismo marcador `<<tipo ...>>` de una sola línea que usa
// ChartParser para `<<chart: ...>>` — sintaxis: `<<video src="..." controls>>`,
// `<<audio src="..." controls autoplay loop muted>>`.
type MediaParser struct{}

// CanParse determina si puede parsear una línea como Media
func (p *MediaParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "<<video") || strings.HasPrefix(trimmed, "<<audio")
}

// Parse parsea un elemento de una sola línea `<<video ...>>` / `<<audio ...>>`.
func (p *MediaParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{Error: nil}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	mediaType := "video"
	if strings.HasPrefix(line, "<<audio") {
		mediaType = "audio"
	}

	attrStr := strings.TrimPrefix(line, "<<"+mediaType)
	attrStr = strings.TrimSuffix(strings.TrimSpace(attrStr), ">>")

	media := ast.NewMediaElement(pos, mediaType, extractAttribute(attrStr, "src"))
	media.Controls = hasBooleanAttribute(attrStr, "controls")
	media.Autoplay = hasBooleanAttribute(attrStr, "autoplay")
	media.Loop = hasBooleanAttribute(attrStr, "loop")
	media.Muted = hasBooleanAttribute(attrStr, "muted")

	return &ParseResult{
		Element:       media,
		ConsumedLines: 1,
		Error:         nil,
	}
}

// hasBooleanAttribute reporta si attrName aparece como token independiente en
// str (p. ej. "controls" en `src="x.mp4" controls autoplay`) — a diferencia
// de extractAttribute (para atributos con valor `nombre="valor"`), un
// atributo booleano HTML-style no lleva valor, solo su presencia importa.
// Se compara por token completo (strings.Fields), no por substring: evita
// que "controls" matchee falsamente dentro de un valor de otro atributo
// (p. ej. src="controls-demo.mp4").
func hasBooleanAttribute(str, attrName string) bool {
	for _, token := range strings.Fields(str) {
		if token == attrName {
			return true
		}
	}
	return false
}
