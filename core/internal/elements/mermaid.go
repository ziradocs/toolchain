// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// MermaidParser maneja el parsing de diagramas Mermaid
type MermaidParser struct{}

// CanParse determina si puede parsear una línea como Mermaid
func (p *MermaidParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// <<mermaid>> y <<mermaid title="…">> en los dos modos; un tag con
	// atributos inválidos también se reclama para reportarlo en vez de
	// dejarlo caer a prosa (ver parseDiagramTag).
	if matched, _, _ := parseDiagramTag(trimmed, "mermaid"); matched {
		return mode == "strict" || mode == "flex"
	}
	switch mode {
	case "flex":
		// Detectar code blocks de Markdown con lenguaje "mermaid"
		if strings.HasPrefix(trimmed, "```mermaid") || strings.HasPrefix(trimmed, "````mermaid") {
			return true
		}
	}

	return false
}

// Parse parsea un elemento Mermaid
func (p *MermaidParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{Error: nil}
	}

	pos := ctx.Position(startIndex)
	openingLine := strings.TrimSpace(ctx.Lines[startIndex])

	// Detectar formato: <<mermaid>> o ```mermaid
	isMarkdownFormat := strings.HasPrefix(openingLine, "```mermaid") || strings.HasPrefix(openingLine, "````mermaid")

	consumedLines := 1 // skip opening line (<<mermaid>> or ```mermaid)
	var contentLines []string

	if isMarkdownFormat {
		// Formato Markdown: ```mermaid ... ```
		// Recoger contenido hasta encontrar ``` o ````
		for i := startIndex + 1; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmed := strings.TrimSpace(line)

			// Terminar al encontrar closing backticks
			if trimmed == "```" || trimmed == "````" {
				consumedLines++
				break
			}

			contentLines = append(contentLines, line)
			consumedLines++
		}
	} else if end, ok := unindentedMermaidBody(ctx.Lines, startIndex+1); ok {
		// Formato <<mermaid>> con el cuerpo sin sangrar y cerrado con <<end>>:
		// la detección de sangría de abajo cortaba en la primera línea, así
		// que el diagrama quedaba vacío y su cuerpo caía como prosa.
		contentLines = append(contentLines, ctx.Lines[startIndex+1:end]...)
		consumedLines += end - startIndex
	} else {
		// Formato <<mermaid>> - usar indentación o tag de cierre <<end>>
		indentDetector := NewAutoDetectIndentation()

		// Recoger contenido del diagrama
		for i := startIndex + 1; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmedLine := strings.TrimSpace(line)

			// Check for closing tag FIRST
			if trimmedLine == "<<end>>" {
				consumedLines++
				break
			}

			// Check if this line should be processed based on indentation
			// ALWAYS check ShouldProcessLine to detect when indentation changes or stops
			if !indentDetector.ShouldProcessLine(line, false, i+1, "MERMAID-PARSER") {
				break
			}

			// Mermaid es un bloque opaco. El separador `---` es válido en su
			// frontmatter y no puede terminarlo antes de <<end>>. Conservamos la
			// línea cruda: la dedent común se aplica una sola vez al final.
			contentLines = append(contentLines, line)
			consumedLines++
		}
	}

	contentStr := dedentMermaidContent(contentLines)
	diagramType := detectMermaidType(contentStr)

	// Crear elemento Mermaid usando el constructor existente
	mermaid := ast.NewMermaidElement(pos, diagramType, contentStr)
	var tagErr error
	if !isMarkdownFormat {
		_, mermaid.Title, tagErr = parseDiagramTag(openingLine, "mermaid")
	}

	return &ParseResult{
		Element:       mermaid,
		ConsumedLines: consumedLines,
		Error:         tagErr,
	}
}

// unindentedMermaidBody dice si el cuerpo de un <<mermaid>> que empieza en
// lines[from] va sin sangría (la primera línea con texto en la columna 0) y
// está cerrado con <<end>>, y devuelve el índice de ese <<end>>. La búsqueda
// del cierre se detiene en una frontera del documento (mermaidBodyBoundary):
// sin <<end>> antes de una frontera, el cuerpo sin sangría no se reclama y el
// bloque se lee como siempre, por sangría. Así un <<mermaid>> sin cerrar no se
// traga el <<end>> de un <<chart>> o un <<quiz>> que venga después.
func unindentedMermaidBody(lines []string, from int) (int, bool) {
	first := from
	for first < len(lines) && strings.TrimSpace(lines[first]) == "" {
		first++
	}
	if first >= len(lines) || CalculateIndentLevel(lines[first]) > 0 {
		return 0, false
	}
	for i := first; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "<<end>>" {
			return i, true
		}
		if mermaidBodyBoundary(lines[i], trimmed) {
			return 0, false
		}
	}
	return 0, false
}

// mermaidBodyBoundary dice si una línea de un cuerpo de mermaid sin sangría es
// una frontera del documento (IsEmbeddedBlockBoundary) y no parte del
// diagrama. No lo son una línea sangrada (una frontera de verdad va en la
// columna 0), `---` (el frontmatter de mermaid) ni una anotación de mermaid
// como `<<interface>> Shape`, que sigue después del `>>`: un tag de DocLang
// ocupa la línea entera.
func mermaidBodyBoundary(line, trimmed string) bool {
	if CalculateIndentLevel(line) > 0 || trimmed == "---" {
		return false
	}
	if strings.HasPrefix(trimmed, "<<") && !strings.HasSuffix(trimmed, ">>") {
		return false
	}
	return IsEmbeddedBlockBoundary(line)
}

// dedentMermaidContent aplica solo el dedent común del bloque. Mermaid usa
// sangría como sintaxis (mindmap y frontmatter YAML), así que TrimSpace por
// línea o descartar blancos internos corrompe el diagrama y hace que fmt
// persista esa corrupción en el fuente.
func dedentMermaidContent(lines []string) string {
	return dedentCommonPrefix(lines)
}

// dedentCommonPrefix quita a lines las líneas en blanco del principio y del
// final y la sangría que comparten todas las líneas con texto (dedentLines), y
// las une con "\n".
func dedentCommonPrefix(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(dedentLines(lines), "\n")
}

// dedentLines quita a cada línea la sangría que comparten todas las líneas
// con texto y conserva la relativa; una línea de solo espacios queda vacía.
// La sangría común se quita en dos pasos. Primero el prefijo de espacios y
// tabs que comparten (commonIndentPrefix, el mismo criterio que el cuerpo de
// un CODE), comparado como cadena: un tab vale un tab. Antes se medía por
// columnas (un tab contaba cuatro) pero se cortaba carácter a carácter, así
// que un cuerpo sangrado con tabs perdía toda la sangría relativa (un mindmap
// quedaba plano). Después, si todavía ninguna línea con texto queda en la
// columna 0 (unas sangradas con espacios y otras con tabs, sin prefijo
// común), se cortan las columnas que comparten, con un tab de cuatro, como
// antes: así un `---` de frontmatter escrito con espacios junto a un cuerpo
// con tabs sigue en la columna 0, donde mermaid lo lee.
func dedentLines(lines []string) []string {
	out := make([]string, len(lines))
	base, hasText := commonIndentPrefix(lines)
	if !hasText {
		return out
	}
	common := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out[i] = line[len(base):]
		if indent := CalculateIndentLevel(out[i]); common == -1 || indent < common {
			common = indent
		}
	}
	if common > 0 {
		for i, line := range out {
			if line != "" {
				out[i] = cutIndentColumns(line, common)
			}
		}
	}
	return out
}

// cutIndentColumns quita los primeros columns de sangría de line, una línea
// con texto sangrada al menos esas columnas, con un tab de cuatro columnas
// (CalculateIndentLevel). Un tab que pasa del corte deja las columnas que
// sobran como espacios.
func cutIndentColumns(line string, columns int) string {
	width, cut := 0, 0
	for cut < len(line) && width < columns {
		if line[cut] == '\t' {
			width += 4
		} else {
			width++
		}
		cut++
	}
	return strings.Repeat(" ", width-columns) + line[cut:]
}

// detectMermaidType mira solo la primera instrucción real. Un frontmatter
// YAML opcional precedido por --- no es una instrucción Mermaid.
func detectMermaidType(content string) string {
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter {
			continue
		}
		keyword := strings.Fields(strings.ToLower(trimmed))
		if len(keyword) == 0 {
			return "unknown"
		}
		switch keyword[0] {
		case "flowchart", "graph":
			return "flowchart"
		case "sequencediagram":
			return "sequence"
		case "classdiagram":
			return "class"
		case "statediagram", "statediagram-v2":
			return "state"
		case "gitgraph":
			return "git"
		case "mindmap", "timeline", "erdiagram", "quadrantchart", "xychart-beta", "sankey-beta", "c4context", "c4container", "c4component", "c4dynamic", "gantt", "pie", "journey":
			return keyword[0]
		default:
			return "unknown"
		}
	}
	return "unknown"
}
