// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TextParser maneja elementos de texto (fallback parser)
type TextParser struct{}

// CanParse siempre retorna true ya que es el parser de fallback
func (p *TextParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// In strict mode, handle TEXT keyword
	if mode == "strict" && strings.HasPrefix(trimmed, "TEXT") {
		return true
	}

	// In flex mode, any non-empty line that's not handled by other parsers
	// becomes text. This is the fallback parser.
	if mode == "flex" && trimmed != "" {
		return true
	}

	return false
}

// Parse parsea texto desde las líneas proporcionadas
func (p *TextParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])
	consumed := 0

	var content strings.Builder

	if ctx.Mode == "strict" && strings.HasPrefix(line, "TEXT") { // Skip TEXT line
		consumed++
		startIndex++

		// Collect indented lines as content
		expectedIndent := -1 // Auto-detect indentation level
		for i := startIndex; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			currentIndent := CalculateIndentLevel(line)
			trimmedLine := strings.TrimSpace(line)

			// Skip empty lines
			if trimmedLine == "" {
				consumed++
				continue
			}

			// Auto-detect expected indentation from first non-empty line
			if expectedIndent == -1 && currentIndent > 0 {
				expectedIndent = currentIndent
			}

			// If we haven't detected indentation yet and line has no indentation, break
			if expectedIndent == -1 && currentIndent == 0 {
				break
			}

			// Check if this line should be part of the text block
			if expectedIndent > 0 && currentIndent < expectedIndent {
				break
			}

			// Add the content with space separator
			if trimmedLine != "" {
				content.WriteString(trimmedLine)
				content.WriteString(" ")
			}
			consumed++
		}
	} else {
		// Flex mode: collect consecutive text lines but be smart about lists
		for i := startIndex; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmed := strings.TrimSpace(line)

			// Stop at empty lines
			if trimmed == "" {
				break
			}

			// Un thematic break de CommonMark ("***", "___", "* * *") se
			// descarta en vez de salir como texto literal (issue #242).
			// Dentro de un bloque no tiene dónde ir: el único separador
			// estructural del dialecto es "---", y no hay nodo de regla
			// horizontal en el AST — así que la alternativa a descartarlo
			// era seguir imprimiendo los asteriscos en la diapositiva, que
			// es lo único que nadie quiere.
			//
			// Si abre el párrafo se consume la línea y se devuelve un
			// elemento vacío: el loop del dialecto flex avanza sin emitir
			// nada y sin FLEX001 (la rama ConsumedLines > 0 con Element nil
			// existe justo para esto). Si aparece a media prosa se corta SIN
			// consumir, y la siguiente vuelta cae en el caso de arriba —
			// misma disciplina que el resto de los cortes de este loop.
			//
			// Va ANTES de isOtherElementType y no después: el predicado de
			// listas de ahí abajo mira el prefijo "* ", así que "* * *"
			// entraba por lista, cortaba con ConsumedLines 0 y terminaba en
			// el failsafe con un FLEX001 — la línea no desaparecía, pero se
			// reportaba como sintaxis no reconocida en vez de descartarse
			// como lo que es.
			if isThematicBreak(trimmed) {
				if i == startIndex {
					consumed++
				}
				break
			}

			// Stop if another element type is detected.
			//
			// issue #174 — historial de este bloque: la primera versión de
			// este fix intentó agregar un guard `i > startIndex` acá, para
			// que isOtherElementType dejara de evaluarse contra la PRIMERA
			// línea (la que TextParser.CanParse ya aceptó porque ningún
			// parser anterior del registry la reclamó). La intención era
			// correcta para el caso de #174 (pipes/### que nadie reclama),
			// pero el guard era demasiado ancho: TestFormatDocument_RoundTrip_Corpus
			// (formatter, examples/dimensions_test.doclang) reveló una
			// regresión real — un "<<end>>" huérfano que sobrevive al
			// round-trip de ChartElement/MapElement (formatChart/formatMap
			// cierran con "<<end>>" genérico) deja de ser descartado en
			// silencio por el chequeo `<<...>>` de isOtherElementType y
			// pasa a renderizarse como su propio TextElement con contenido
			// literal "<<end>>". Ese descarte silencioso, aunque accidental,
			// es comportamiento del que este mismo corpus depende hoy.
			//
			// La corrección real, más angosta, vive en isOtherElementType:
			// cada predicado se sincronizó con lo que el dispatcher de
			// verdad reclama (ver los comentarios de "|" y "##" ahí abajo),
			// en vez de ensanchar CUÁNDO se llama al predicado completo.
			if p.isOtherElementType(trimmed, ctx.Mode) {
				break
			}

			// In flex mode, stop if we detect a numbered list that should be separate
			if i > startIndex && p.isStartOfNumberedList(trimmed) {
				break
			}

			// Process line as normal text content
			if content.Len() > 0 {
				content.WriteString(" ")
			}
			content.WriteString(trimmed)
			consumed++
		}
	}

	// Don't create empty text elements
	text := strings.TrimSpace(content.String())
	if text == "" {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: consumed,
			Error:         nil,
		}
	}

	return &ParseResult{
		Element:       ast.NewTextElement(pos, text),
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// isOtherElementType checks if a line indicates another element type
func (p *TextParser) isOtherElementType(line string, mode string) bool {
	// Check for various element indicators

	// Special element tags (<<chart>>, <<map>>, <<mermaid>>, etc.)
	if strings.HasPrefix(line, "<<") && strings.Contains(line, ">>") {
		return true
	}

	// Subsection headers — SOLO "## " (issue #174). Antes esto era
	// HasPrefix(line, "##") sin exigir el espacio, así que además de "## "
	// también capturaba "### "/"#### " y hasta "##SinEspacio". El único
	// consumidor real de un heading "##" es el loop externo de flex.go
	// (parseSection/parseSlide), que exige EXACTAMENTE "# "/"## " —con
	// espacio— para tratar una línea como límite de bloque/subtítulo;
	// nunca reconoce "### " ni más hashes como nada especial. Ese
	// desacople es lo que causaba la "zona muerta": un "### Foo" que
	// llegaba como primera línea de un párrafo (nadie más lo reclama)
	// hacía que este chequeo lo tratara como "otro elemento", Parse
	// retornaba ConsumedLines:0 sin error, y el failsafe de flex.go
	// avanzaba una línea sin emitir nada — la línea desaparecía en
	// silencio. Angostar el predicado a "## " exacto dos cosas: sigue
	// deteniendo el párrafo antes de un "## " genuino que aparezca más
	// adelante en el mismo bloque (necesario — el loop externo de flex.go
	// solo revisa `p.currentLine`, no las líneas que TextParser ya
	// escaneó hacia adelante dentro de un mismo Parse()), y dejar que
	// "###"+ (que nadie reclama en ningún nivel) se trate como texto
	// normal en vez de desaparecer.
	if strings.HasPrefix(line, "## ") {
		return true
	}

	// Special blocks
	if strings.HasPrefix(line, ":::") {
		return true
	}

	// Directives
	if strings.HasPrefix(line, "@") {
		return true
	}

	// Lists
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}

	// Code blocks (markdown style)
	if strings.HasPrefix(line, "```") {
		return true
	}

	// Images (markdown style)
	if strings.HasPrefix(line, "![") && strings.Contains(line, "](") {
		return true
	}

	// Tables (markdown style). Issue #174: este predicado exigía CONTAINS en
	// vez de HasPrefix, así que prosa con 2+ "|" que no abre con "|" (p. ej.
	// "**A:** 1 | **B:** 2 | **C:** 3") se clasificaba como "otro elemento"
	// aunque TableParser.CanParse (table.go) — el único parser que en
	// verdad reclama tablas markdown — exige un "|" INICIAL, igual que
	// IsNewElement (common.go). Ahora espeja esa regla exactamente.
	if strings.HasPrefix(line, "|") && strings.Count(line, "|") >= 2 {
		return true
	}

	// Strict mode keywords
	if mode == "strict" {
		keywords := []string{"TEXT", "POINTS", "CODE", "IMAGE", "TABLE", "MERMAID", "CHART", "MAP"}
		for _, keyword := range keywords {
			if strings.HasPrefix(line, keyword+" ") || line == keyword {
				return true
			}
		}
	}

	return false
}

// isStartOfNumberedList checks if a line starts a numbered list (1. 2. etc.)
func (p *TextParser) isStartOfNumberedList(line string) bool {
	if len(line) < 3 {
		return false
	}

	// Check for numbered lists (1. 2. 3. etc.)
	for i, char := range line {
		if char == '.' && i > 0 {
			// Check if followed by space
			if i+1 < len(line) && line[i+1] == ' ' {
				return true
			}
			break
		}
		if char < '0' || char > '9' {
			break
		}
	}

	return false
}

// isThematicBreak reporta si line (ya trimmed) es un thematic break de
// CommonMark escrito con asteriscos o guiones bajos: tres o más del MISMO
// carácter, con espacios o tabs opcionales entre ellos y nada más en la
// línea. Acepta "***", "___", "* * *" y "_____"; rechaza "**", "*-*" y
// cualquier línea con texto.
//
// La forma con guiones ("---") queda deliberadamente afuera: en los dos
// dialectos flex esa línea ya es el separador de bloque, y el loop de nivel
// superior la resuelve antes de que ningún elemento la vea. La forma con
// viñetas ("- - -") también: PointsParser la reclama antes como lista, y
// arrebatársela sería un cambio de otro alcance.
func isThematicBreak(line string) bool {
	if len(line) < 3 {
		return false
	}

	var marker rune
	count := 0
	for _, r := range line {
		switch r {
		case ' ', '\t':
			continue
		case '*', '_':
			if marker == 0 {
				marker = r
			} else if r != marker {
				return false
			}
			count++
		default:
			return false
		}
	}

	return count >= 3
}
