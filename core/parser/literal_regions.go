// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"

	"go.ziradocs.com/core/v2/internal/elements"
	"go.ziradocs.com/core/v2/internal/fence"
)

// Tipos de región literal que reconoce literalScanner: el cuerpo de un CODE
// strict, una cerca ``` (o ````), un :::code-group, un diagrama <<…>>, un
// bloque $$ y un @startuml.
const (
	literalCode     = "code"
	literalFence    = "fence"
	literalGroup    = "group"
	literalDiagram  = "diagram"
	literalMath     = "math"
	literalPlantUML = "plantuml"
)

// literalScanner recorre una fuente línea por línea y dice en qué región
// literal está cada una. Una región literal es contenido que el parser guarda
// tal cual —el cuerpo de un CODE, una cerca, un diagrama—, así que una línea
// dentro de ella nunca es una anotación ni una directiva. La línea que abre o
// cierra una región cuenta como parte de ella.
//
// Es el recorrido del pre-pase de anotaciones `<!-- node-id -->`
// (stripNodeIDDirectives), que corre antes de parsear y por eso no le puede
// preguntar al parser. Qué @include son código lo decide core/include sobre el
// AST del parser.
type literalScanner struct {
	// strict: la cabecera CODE se reconoce con el mismo CanParse que usa el
	// despacho strict. flexCodeHeaders mantiene, fuera de strict, la regla
	// histórica del pre-pase de node-id (una línea `CODE` o `CODE …` también
	// abre un cuerpo literal en flex).
	strict, flexCodeHeaders bool

	literal          string
	codeHeaderIndent int
	diagramIndent    int
	// fenceEnd es la línea que cierra la cerca abierta y groupEnd la de la
	// cerca de una pestaña de :::code-group, como las lee fence.At (len(lines)
	// si no cierran); groupEnd es -1 fuera de una pestaña.
	fenceEnd int
	groupEnd int
	// groupClose es el ":::" que cierra el code-group abierto
	// (fence.ContainerEnd), y blocks los finales de los bloques ::: con tipo
	// que se están recorriendo, del más externo al más interno: una cerca
	// busca su cierre solo antes del final de su contenedor, como en el
	// parser.
	groupClose int
	blocks     []int
}

func newLiteralScanner(strict, flexCodeHeaders bool) literalScanner {
	return literalScanner{strict: strict, flexCodeHeaders: flexCodeHeaders, codeHeaderIndent: -1, diagramIndent: -1}
}

// step consume line y devuelve la región literal en la que está ("" si en
// ninguna).
func (s *literalScanner) step(lines []string, i int) string {
	line := lines[i]
	trimmed := strings.TrimSpace(line)
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	for len(s.blocks) > 0 && i >= s.blocks[len(s.blocks)-1] {
		s.blocks = s.blocks[:len(s.blocks)-1]
	}
	limit := len(lines)
	if len(s.blocks) > 0 {
		limit = s.blocks[len(s.blocks)-1]
	}
	// El cuerpo de un CODE son las líneas con más sangría que su cabecera
	// (elements.CodeParser.parseStrictCode): termina en la primera línea con
	// texto que no pasa de ella.
	if s.literal == literalCode && trimmed != "" && elements.CalculateIndentLevel(line) <= s.codeHeaderIndent {
		s.literal = ""
	}
	// Un diagrama (<<mermaid>>, <<plantuml>>, <<chart:…>>, <<map>>,
	// <<math>>) no exige <<end>>: el parser también lo cierra cuando una
	// línea vuelve a la indentación de la apertura o menos (siguiente
	// elemento, SLIDE/SECTION, encabezado de flex). El recorrido tiene que
	// cerrarlo en el mismo punto; si esperaba solo a <<end>>, el pre-pase de
	// node-id se tragaba el `<!-- node-id -->` del elemento siguiente y el
	// build fallaba con un node-id huérfano.
	if s.literal == literalDiagram && trimmed != "" && trimmed != "<<end>>" && indent <= s.diagramIndent {
		s.literal = ""
	}
	if s.literal != "" {
		current := s.literal
		switch s.literal {
		case literalFence:
			if i == s.fenceEnd {
				s.literal = ""
				return current
			}
		case literalDiagram:
			if trimmed == "<<end>>" {
				s.literal = ""
			}
		case literalMath:
			if trimmed == "$$" {
				s.literal = ""
			}
		case literalPlantUML:
			if trimmed == "@enduml" {
				s.literal = ""
			}
		case literalGroup:
			// Las cercas de cada pestaña siguen la misma regla que el
			// code-group (internal/fence): un ":::" dentro de una cerca es
			// código, no el cierre del grupo.
			if s.groupEnd >= 0 {
				if i == s.groupEnd {
					s.groupEnd = -1
				}
			} else if b, ok := fence.AtWithin(lines, i, s.groupClose); ok {
				s.groupEnd = blockEnd(b)
			} else if trimmed == ":::" {
				s.literal = ""
			}
		}
		if s.literal != "" || trimmed == "<<end>>" || trimmed == ":::" || trimmed == "$$" || trimmed == "@enduml" {
			return current
		}
	}
	if b, ok := fence.AtWithin(lines, i, limit); ok {
		s.literal = literalFence
		s.fenceEnd = blockEnd(b)
		return s.literal
	}
	if trimmed == "$$" {
		s.literal = literalMath
		return s.literal
	}
	if strings.HasPrefix(trimmed, "@startuml") {
		s.literal = literalPlantUML
		return s.literal
	}
	if isDiagramOpener(trimmed) {
		s.literal = literalDiagram
		s.diagramIndent = indent
		return s.literal
	}
	if fence.IsCodeGroupOpener(trimmed) {
		s.literal = literalGroup
		s.groupEnd = -1
		s.groupClose, _ = fence.ContainerEnd(lines, i, limit, nil)
		return s.literal
	}
	if fence.IsBlockOpener(trimmed) {
		end, _ := fence.ContainerEnd(lines, i, limit, func(line string) bool {
			return strings.TrimSpace(line) == "---" || elements.IsStrictBlockBoundary(line)
		})
		s.blocks = append(s.blocks, end)
	}
	codeHeader := false
	if s.strict {
		// Match dispatch, including strings.Fields whitespace, without
		// broadening the verbatim attribute's grammar.
		codeHeader = (&elements.CodeParser{}).CanParse(trimmed, "strict")
	} else if s.flexCodeHeaders {
		codeHeader = trimmed == "CODE" || strings.HasPrefix(trimmed, "CODE ") ||
			trimmed == "CODE"+elements.CodeVerbatimMarker || strings.HasPrefix(trimmed, "CODE"+elements.CodeVerbatimMarker+" ")
	}
	if codeHeader {
		s.literal = literalCode
		s.codeHeaderIndent = elements.CalculateIndentLevel(line)
		return s.literal
	}
	return ""
}

// blockEnd es la línea que cierra b, o el final de su contenedor si no
// cierra.
func blockEnd(b fence.Block) int {
	if b.End < 0 {
		return b.Limit
	}
	return b.End
}

// literalLineKinds aplica literalScanner a lines desde from: el elemento i es
// la región de la línea i ("" antes de from).
func literalLineKinds(lines []string, from int, strict, flexCodeHeaders bool) []string {
	kinds := make([]string, len(lines))
	s := newLiteralScanner(strict, flexCodeHeaders)
	for i := from; i < len(lines); i++ {
		kinds[i] = s.step(lines, i)
	}
	return kinds
}
