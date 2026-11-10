// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"

	"go.ziradocs.com/core/v2/internal/elements"
)

// Tipos de región literal que reconoce literalScanner: el cuerpo de un CODE
// strict, una cerca ``` (o ````), un :::code-group, un diagrama <<…>>, un
// bloque $$ y un @startuml.
const (
	literalCode  = "code"
	literalFence = "fence"
	// literalDiagramFence es una cerca ```mermaid/plantuml/chart/map/math: un
	// diagrama escrito como cerca, no código.
	literalDiagramFence = "diagram-fence"
	literalGroup        = "group"
	literalDiagram      = "diagram"
	literalMath         = "math"
	literalPlantUML     = "plantuml"
)

// literalScanner recorre una fuente línea por línea y dice en qué región
// literal está cada una. Una región literal es contenido que el parser guarda
// tal cual —el cuerpo de un CODE, una cerca, un diagrama—, así que una línea
// dentro de ella nunca es una anotación ni una directiva. La línea que abre o
// cierra una región cuenta como parte de ella.
//
// Es el recorrido que antes vivía dentro de stripNodeIDDirectives, sacado a un
// tipo para que el pre-pase de node-id y la expansión de @include
// (CodeLiteralScanner) vean las mismas regiones.
type literalScanner struct {
	// strict: la cabecera CODE se reconoce con el mismo CanParse que usa el
	// despacho strict. flexCodeHeaders mantiene, fuera de strict, la regla
	// histórica del pre-pase de node-id (una línea `CODE` o `CODE …` también
	// abre un cuerpo literal en flex).
	strict, flexCodeHeaders bool

	literal          string
	codeHeaderIndent int
	diagramIndent    int
	groupFence       bool
	fenceClose       string
}

func newLiteralScanner(strict, flexCodeHeaders bool) literalScanner {
	return literalScanner{strict: strict, flexCodeHeaders: flexCodeHeaders, codeHeaderIndent: -1, diagramIndent: -1, fenceClose: "```"}
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
		case literalFence, literalDiagramFence:
			if trimmed == s.fenceClose {
				s.literal = ""
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
			if strings.HasPrefix(trimmed, "```") {
				s.groupFence = !s.groupFence
			}
			if trimmed == ":::" && !s.groupFence {
				s.literal = ""
			}
		}
		if s.literal != "" || trimmed == s.fenceClose || trimmed == "<<end>>" || trimmed == ":::" || trimmed == "$$" || trimmed == "@enduml" {
			return current
		}
	}
	if strings.HasPrefix(trimmed, "```") {
		s.literal = literalFence
		if isDiagramFenceInfo(fenceInfo(trimmed)) {
			s.literal = literalDiagramFence
		}
		if strings.HasPrefix(trimmed, "````") {
			s.fenceClose = "````"
		} else {
			s.fenceClose = "```"
		}
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
		s.groupFence = false
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

// fenceInfo es lo que sigue a los backticks de apertura de una cerca.
func fenceInfo(open string) string {
	return strings.TrimSpace(strings.TrimLeft(open, "`"))
}

// isDiagramFenceInfo reconoce la cerca de un diagrama, un chart, un mapa o una
// ecuación, con el mismo criterio que el CanParse de cada uno.
func isDiagramFenceInfo(info string) bool {
	return strings.HasPrefix(info, "mermaid") || strings.HasPrefix(info, "plantuml") ||
		info == "chart" || info == "map" || info == "math"
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

// CodeLiteralScanner dice, línea por línea, si una línea de una fuente es
// código literal: el cuerpo de un CODE strict, una cerca ``` o un
// :::code-group, con su línea de apertura y la de cierre. El parser guarda
// esas líneas tal cual como texto del código, así que un `@include` escrito
// ahí no es una directiva: core/include no lo expande (issue #487), y el build
// emite lo mismo que el AST y el lint describen.
//
// Los diagramas (<<mermaid>>, <<chart:…>>…, y también escritos como cerca:
// ```mermaid, ```chart…), los bloques $$, ```math y @startuml no cuentan como
// código; su cuerpo sigue pudiendo venir de un @include.
//
// Se alimenta con las líneas del cuerpo (lo que sigue al frontmatter), en el
// orden en que las va a ver el parser. Reconoce las mismas regiones que el
// pre-pase de anotaciones `<!-- node-id -->` del parser, salvo la regla
// histórica de ese pre-pase que en flex también trata `CODE …` como cabecera.
type CodeLiteralScanner struct {
	s literalScanner
}

// NewCodeLiteralScanner crea el escáner. strict es el dialecto de la fuente
// (ver SourceIsStrict): solo en strict una línea CODE abre un cuerpo de código.
func NewCodeLiteralScanner(strict bool) *CodeLiteralScanner {
	return &CodeLiteralScanner{s: newLiteralScanner(strict, false)}
}

// Next consume line y dice si es código literal.
func (c *CodeLiteralScanner) Next(line string) bool {
	return isCodeLiteral(c.s.step(line))
}

// Peek dice lo mismo que Next sin consumir la línea: el escáner queda como
// estaba. core/include lo usa para decidir si expandir una línea `@include`,
// que en el texto expandido no existe (en su lugar van las líneas del archivo
// incluido, que son las que el escáner tiene que ver después).
func (c *CodeLiteralScanner) Peek(line string) bool {
	s := c.s
	return isCodeLiteral(s.step(line))
}

func isCodeLiteral(kind string) bool {
	return kind == literalCode || kind == literalFence || kind == literalGroup
}

// SourceIsStrict dice si el frontmatter de source declara `mode: strict`, el
// mismo criterio con el que Parse y ParseDocument eligen el dialecto strict.
func SourceIsStrict(source string) bool {
	fm, _, _ := (&FrontMatterParser{}).Parse(source)
	return fm != nil && fm.Mode == "strict"
}
