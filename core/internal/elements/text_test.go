// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"
)

// TestTextParser_IsOtherElementType_Pipes cubre issue #174: una línea de
// prosa con 2+ "|" que no ABRE con "|" desaparecía en silencio. El
// predicado exigía strings.Contains en vez de strings.HasPrefix, así que
// no espejaba a TableParser.CanParse (table.go) — el único parser que en
// verdad reclama tablas markdown, y que sí exige un "|" inicial.
func TestTextParser_IsOtherElementType_Pipes(t *testing.T) {
	p := &TextParser{}

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"prosa con 1 pipe no es tabla", "**A:** 1 | **B:** 2", false},
		{"prosa con 2 pipes sin | inicial no es tabla (issue #174)", "**A:** 1 | **B:** 2 | **C:** 3", false},
		{"prosa con 3 pipes sin | inicial no es tabla", "$1,200 | $3,400 | $5,600", false},
		{"fila de tabla markdown con | inicial sí es tabla", "| A | B |", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.isOtherElementType(tt.line, "flex"); got != tt.expected {
				t.Errorf("isOtherElementType(%q) = %v, want %v", tt.line, got, tt.expected)
			}
		})
	}
}

// TestTextParser_IsOtherElementType_Headings cubre el otro habitante
// confirmado de la misma zona muerta: "### "+ (3 o más hashes) no lo
// reclama ningún nivel del pipeline flex — el loop externo de flex.go
// (parseSection/parseSlide) solo trata "# "/"## " (con espacio) como
// límite de bloque/subtítulo, nunca "### " o más. El chequeo viejo de
// isOtherElementType (HasPrefix(line, "##"), sin exigir el espacio)
// capturaba de más: "### Foo" se marcaba "otro elemento" pese a que nadie
// lo reclamaba, y por lo tanto desaparecía en silencio en vez de
// sobrevivir como texto.
func TestTextParser_IsOtherElementType_Headings(t *testing.T) {
	p := &TextParser{}

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"## con espacio sí es heading reclamado", "## Subtítulo", true},
		{"### con espacio NO lo reclama nadie (issue #174)", "### Subsección", false},
		{"#### con espacio tampoco", "#### Más profundo", false},
		{"## sin espacio no es el heading exacto que flex.go reconoce", "##SinEspacio", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.isOtherElementType(tt.line, "flex"); got != tt.expected {
				t.Errorf("isOtherElementType(%q) = %v, want %v", tt.line, got, tt.expected)
			}
		})
	}
}

// TestTextParser_Parse_SurvivesDeadZoneShapes verifica el efecto extremo a
// extremo (no solo el predicado): estas líneas deben sobrevivir como
// contenido de un TextElement en vez de desaparecer con ConsumedLines:0.
func TestTextParser_Parse_SurvivesDeadZoneShapes(t *testing.T) {
	p := &TextParser{}

	tests := []struct {
		name string
		line string
	}{
		{"2+ pipes sin | inicial", "**A:** 1 | **B:** 2 | **C:** 3"},
		{"### heading que nadie reclama", "### Subsección"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &ParseContext{Mode: "flex", Lines: []string{tt.line}}
			result := p.Parse(ctx, 0)
			if result.Element == nil {
				t.Fatalf("Parse(%q) produjo Element=nil, ConsumedLines=%d — la línea desapareció en silencio", tt.line, result.ConsumedLines)
			}
			if result.ConsumedLines != 1 {
				t.Errorf("ConsumedLines = %d, want 1", result.ConsumedLines)
			}
		})
	}
}

// TestTextParser_Parse_TableStillParsesAsTable confirma que angostar el
// predicado de pipes no rompe la detección de tablas markdown legítimas:
// el "|" inicial sigue siendo el discriminador, y una tabla inmediatamente
// después de prosa (sin línea en blanco entre medio) sigue cortando el
// párrafo antes de la tabla — TextParser nunca ve la fila de la tabla
// porque TableParser (de mayor prioridad en el registry) la reclama
// primero; acá solo se verifica el lado de TextParser: debe seguir
// tratando una fila "| A | B |" como límite de párrafo.
func TestTextParser_Parse_TableStillParsesAsTable(t *testing.T) {
	p := &TextParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"Prosa antes de la tabla.",
			"| A | B |",
			"| 1 | 2 |",
		},
	}

	result := p.Parse(ctx, 0)
	if result.Element == nil {
		t.Fatal("Parse produjo Element=nil para la línea de prosa")
	}
	if result.ConsumedLines != 1 {
		t.Errorf("ConsumedLines = %d, want 1 (debe detenerse ANTES de la fila de tabla)", result.ConsumedLines)
	}
}

// TestTextParser_Parse_MidParagraphLookahead confirma que angostar "##" a
// "## " (con espacio) no perdió el caso de lookahead que sí es necesario:
// un "## " genuino que aparece como SEGUNDA línea (dentro del mismo
// Parse(), antes de que el loop externo de flex.go llegue a esa línea)
// todavía debe cortar el párrafo, no absorberse como continuación.
func TestTextParser_Parse_MidParagraphLookahead(t *testing.T) {
	p := &TextParser{}
	ctx := &ParseContext{
		Mode: "flex",
		Lines: []string{
			"Prosa antes del heading.",
			"## Un heading real",
		},
	}

	result := p.Parse(ctx, 0)
	if result.Element == nil {
		t.Fatal("Parse produjo Element=nil para la línea de prosa")
	}
	if result.ConsumedLines != 1 {
		t.Errorf("ConsumedLines = %d, want 1 (debe detenerse ANTES del \"## \")", result.ConsumedLines)
	}
}

// TestTextParser_Parse_OrphanEndTagStillDropped es una regresión
// explícita: la primera versión de este fix (issue #174) intentó ensanchar
// CUÁNDO se evalúa isOtherElementType (guard i > startIndex) en vez de
// angostar SOLO los predicados específicos que #174 pedía. Ese guard
// ancho rompía TestFormatDocument_RoundTrip_Corpus (formatter,
// examples/dimensions_test.doclang): un "<<end>>" huérfano — el cierre
// genérico que formatChart/formatMap emiten — dejaba de descartarse en
// silencio y pasaba a renderizarse como su propio TextElement con
// contenido literal "<<end>>". Este test fija ese comportamiento (el
// descarte silencioso de un "<<end>>" solitario que nadie más reclama)
// como parte del contrato de TextParser, para que una futura ampliación
// del guard lo vuelva a romper de forma visible.
func TestTextParser_Parse_OrphanEndTagStillDropped(t *testing.T) {
	p := &TextParser{}
	ctx := &ParseContext{Mode: "flex", Lines: []string{"<<end>>"}}

	result := p.Parse(ctx, 0)
	if result.Element != nil {
		t.Errorf("Parse(%q) = %+v, want Element=nil (un <<end>> huérfano no debe convertirse en TextElement)", ctx.Lines[0], result.Element)
	}
}
