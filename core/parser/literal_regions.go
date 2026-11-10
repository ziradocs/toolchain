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
	// fenceTicks son los backticks de la cerca abierta, groupTicks los de la
	// cerca de una pestaña de :::code-group (internal/fence).
	fenceTicks int
	groupTicks int
}

func newLiteralScanner(strict, flexCodeHeaders bool) literalScanner {
	return literalScanner{strict: strict, flexCodeHeaders: flexCodeHeaders, codeHeaderIndent: -1, diagramIndent: -1}
}

// step consume line y devuelve la región literal en la que está ("" si en
// ninguna).
func (s *literalScanner) step(line string) string {
	trimmed := strings.TrimSpace(line)
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
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
			if fence.Closes(trimmed, s.fenceTicks) {
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
			if s.groupTicks > 0 {
				if fence.Closes(trimmed, s.groupTicks) {
					s.groupTicks = 0
				}
			} else if n := fence.Open(trimmed); n > 0 {
				s.groupTicks = n
			} else if trimmed == ":::" {
				s.literal = ""
			}
		}
		if s.literal != "" || trimmed == "<<end>>" || trimmed == ":::" || trimmed == "$$" || trimmed == "@enduml" {
			return current
		}
	}
	if n := fence.Open(trimmed); n > 0 {
		s.literal = literalFence
		s.fenceTicks = n
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
	if strings.HasPrefix(trimmed, ":::code-group") {
		s.literal = literalGroup
		s.groupTicks = 0
		return s.literal
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

// literalLineKinds aplica literalScanner a lines desde from: el elemento i es
// la región de la línea i ("" antes de from).
func literalLineKinds(lines []string, from int, strict, flexCodeHeaders bool) []string {
	kinds := make([]string, len(lines))
	s := newLiteralScanner(strict, flexCodeHeaders)
	for i := from; i < len(lines); i++ {
		kinds[i] = s.step(lines[i])
	}
	return kinds
}
