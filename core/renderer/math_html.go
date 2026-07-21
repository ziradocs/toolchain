// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

// math_html.go concentra la construcción de la salida de ecuaciones del lado
// Go en un único lugar — mismo motivo que mermaid_html.go: un solo punto de
// reuso para que un futuro sink no reproduzca el patrón "literal <div> +
// EscapeHTML independiente" que causó el XSS del issue #73 en Mermaid.
// Cualquier builder Go nuevo de math DEBE pasar por acá.

// BuildMathDiv es el único dueño del literal <div class="math-content"> Y de
// la llamada a EscapeHTML sobre el LaTeX. El contenido es dato del usuario;
// MathJax lo lee desde el textContent del nodo (los delimitadores \[...\]
// son ASCII plano, no necesitan escaparse — el LaTeX entre ellos sí, por si
// contiene "<"/">"/"&" que MathJax de otro modo podría malinterpretar al
// tipografiar sobre un DOM ya parseado). Escapar cierra el mismo vector de
// XSS que AL-6 (issue #73) cerró para Mermaid, sin romper el typesetting.
func BuildMathDiv(latex string) string {
	return `<div class="math-content">\[` + EscapeHTML(latex) + `\]</div>`
}
