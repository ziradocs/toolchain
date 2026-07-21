// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"fmt"
	"strings"
)

// mermaid_html.go concentra la construcción de la salida Mermaid del lado Go en
// un único lugar. La intención es la misma que documenta navigateAndSetContent
// en chromium_renderer.go: un solo punto de reuso para que un futuro sink no
// pueda re-copiar el patrón crudo "literal <div> + EscapeHTML independiente" que
// dejó pasar el XSS del issue #73. Cualquier builder Go nuevo de mermaid DEBE
// pasar por aquí en vez de volver a escribir el markup a mano.

// BuildMermaidDiv es el único dueño del literal <div class="mermaid"> Y de la
// llamada a EscapeHTML sobre el contenido del diagrama. El contenido es dato del
// usuario; se escapa porque Mermaid lo lee desde el textContent del nodo (así
// que escapar no rompe el parser) sin exponer un sink de XSS en el DOM.
// Ver docs/SECURITY_AUDIT_2026-07.md, AL-6 (issue #73).
func BuildMermaidDiv(content string) string {
	return `<div class="mermaid">` + EscapeHTML(content) + `</div>`
}

// MermaidExtra es una clave adicional para MermaidInitConfigJS, más allá de la
// base fija (startOnLoad/theme/securityLevel/htmlLabels). Key es un
// identificador JS de desarrollador (siempre un literal Go en tiempo de
// compilación en los call sites actuales, nunca input de usuario/AST/CLI).
// Value se serializa con encoding/json — nunca se concatena como string crudo
// — así que ningún valor puede romper el object literal ni inyectar código,
// sin importar qué contenga (p. ej. `"} ); alert(1); //"` se serializa como el
// string JSON literal `"} ); alert(1); //"`, sintácticamente inerte).
//
// Nace de una revisión de código sobre la versión anterior (extraKeys
// ...string, que unía strings crudos): aunque en la práctica extraKeys nunca
// tuvo un call site con input controlable por un atacante, el diseño de
// concatenación de strings-como-JS era un patrón inseguro que un futuro caller
// descuidado podía reproducir con datos reales. Este tipo lo hace
// estructuralmente imposible en vez de solo "no explotado hoy".
type MermaidExtra struct {
	Key   string
	Value any
}

// MermaidInitConfigJS retorna el object-literal (con llaves) que se pasa a
// mermaid.initialize(...). securityLevel:'strict' y htmlLabels:false — la
// invariante de seguridad del issue #70 — van horneados y no son
// sobrescribibles por el llamador, de modo que la garantía sea estructural y no
// incidental. startOnLoad es la única diferencia legítima entre consumidores:
// true para las páginas raster server-side y el HTML por defecto de doclang;
// false para los decks de slides, que renderizan en eventos de cambio de slide.
//
// extra permite añadir claves por-consumidor que NO son parte de la base (p.
// ej. doclang añade flowchart:{htmlLabels:false} para forzar el guard también
// a nivel del diagrama flowchart: el htmlLabels top-level está deprecado en
// Mermaid y no gobierna flowchart.htmlLabels, cuyo default es true).
//
// El par de seguridad se emite AL FINAL, después de extra: bajo la semántica
// de objeto JS (last-wins), un extra que intente redefinir
// securityLevel/htmlLabels queda estructuralmente neutralizado por el valor
// canónico posterior — la garantía es estructural, no incidental (issue #85).
// Con extra vacío el orden y formato son idénticos al literal previo
// (startOnLoad, theme, securityLevel, htmlLabels).
//
// Ver docs/SECURITY_AUDIT_2026-07.md (issue #85). El espaciado del literal
// base es significativo: los tests aseguran las subcadenas exactas
// "securityLevel: 'strict'" y "htmlLabels: false".
func MermaidInitConfigJS(startOnLoad bool, extra ...MermaidExtra) string {
	var b strings.Builder
	fmt.Fprintf(&b, "{ startOnLoad: %v, theme: 'default'", startOnLoad)
	for _, opt := range extra {
		valueJSON, err := json.Marshal(opt.Value)
		if err != nil {
			// No debería ocurrir con los tipos usados por los call sites actuales
			// (bool/map/string); si alguna vez ocurriera, "null" es un valor JS
			// válido y no rompe el object literal.
			valueJSON = []byte("null")
		}
		fmt.Fprintf(&b, ", %s: %s", opt.Key, valueJSON)
	}
	// Par de seguridad al final: last-wins garantiza que ningún extra lo anule.
	b.WriteString(", securityLevel: 'strict', htmlLabels: false }")
	return b.String()
}
