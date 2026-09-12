// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// SpecialBlockParser maneja bloques especiales como :::info, :::warning, etc.
type SpecialBlockParser struct{}

// CanParse determina si una línea es el inicio de un bloque especial
func (p *SpecialBlockParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, ":::") && len(trimmed) > 3
}

// nestedContentParsers son los parsers que el cuerpo de un ":::bloque"
// delega para poblar Elements (issue #9, audit 2026-09-11, F9) — un
// subconjunto DELIBERADO del registry completo (GetDefaultRegistry), no el
// registry entero: Points/Checklist/Quote/Text quedan fuera a propósito,
// porque esa prosa ya se renderiza bien desde Content vía markdown (no hay
// nada que ganar tipándola), y promoverla habría sido un cambio de
// comportamiento no pedido ni probado por la auditoría. El orden importa —
// mismo criterio de especificidad que GetDefaultRegistry — porque
// SpecialBlockParser (nested) y CanParse de sobra aceptan cualquier ":::algo"
// y tienen que perder contra CodeGroupParser/GridParser en sus prefijos
// más específicos (":::code-group", "::: grid").
var nestedContentParsers = []ElementParser{
	&MermaidParser{},
	&PlantUMLParser{},
	&ChartParser{},
	&MapParser{},
	&CodeGroupParser{},
	&CodeParser{},
	&GridParser{},
	&SpecialBlockParser{},
	&ImageParser{},
	&TableParser{},
}

// tryParseNestedContent intenta delegar la línea ctx.Lines[i] a alguno de
// nestedContentParsers. Devuelve (elemento, líneas consumidas, true) si algún
// parser la reclamó y produjo un elemento real — un CanParse que dice sí pero
// cuyo Parse falla o devuelve consumo 0 (bloque vacío, YAML inválido, etc.) NO
// cuenta como delegación: cae al llamador, que la trata como prosa suelta,
// igual que ANTES de que esta función existiera.
func tryParseNestedContent(ctx *ParseContext, i int) (ast.Element, int, bool) {
	line := ctx.Lines[i]
	for _, parser := range nestedContentParsers {
		if !parser.CanParse(line, ctx.Mode) {
			continue
		}
		result := parser.Parse(ctx, i)
		if result.Element != nil && result.ConsumedLines > 0 {
			return result.Element, result.ConsumedLines, true
		}
		return nil, 0, false
	}
	return nil, 0, false
}

// Parse parsea un bloque especial desde las líneas proporcionadas
func (p *SpecialBlockParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := ctx.Position(startIndex)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	// Verify it starts with :::
	if !strings.HasPrefix(line, ":::") {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	// Extract block type: can be ":::info" or "::: info"
	blockContent := strings.TrimSpace(line[3:]) // Remove :::
	if blockContent == "" {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 1,
			Error:         nil,
		}
	}

	// Autocierre en una sola línea (":::tipo attrs ... :::", C25/F13 del
	// audit — la sintaxis canónica de atributos que PR-13 termina de
	// implementar): acá solo se detecta lo justo para NO abrir un cuerpo a
	// escanear cuando ya cerró en esta misma línea. Sin este chequeo, un
	// bloque que en realidad ya cerró (":::embed ... :::" en un renglón,
	// como examples/01_title_and_content/01.7_advanced_inline_syntax_flex.slidelang)
	// seguía escaneando hacia adelante buscando un cierre que ya pasó, y con
	// la delegación de esta misma función terminaba ADOPTANDO como hijos
	// anidados a los siguientes bloques ":::" del documento — un bloque roto
	// se volvía MÁS dañino que antes (se tragaba hermanos enteros, headings
	// incluidos) en vez de simplemente quedar mal parseado como ya pasaba.
	// El parseo real de atributos ("type=\"youtube\"" como clave/valor, no
	// como texto de Title) queda para PR-13; acá alcanza con no colgarse.
	selfClosing := false
	if rest, ok := strings.CutSuffix(blockContent, ":::"); ok && (rest == "" || strings.HasSuffix(rest, " ")) {
		blockContent = strings.TrimSpace(rest)
		selfClosing = true
	}

	parts := strings.Fields(blockContent)
	blockType := parts[0]
	title := ""
	if len(parts) > 1 {
		title = strings.Join(parts[1:], " ")
	}

	if selfClosing {
		block := ast.NewSpecialBlockElement(pos, blockType, "")
		block.Title = title
		block.Icon = KnownSpecialBlockTypes()[blockType]
		return &ParseResult{Element: block, ConsumedLines: 1, Error: nil}
	}

	// Collect content until ::: (cierre) — Content acumula CADA línea del
	// cuerpo tal cual (mismo TrimSpace de siempre, sin cambios: sigue siendo
	// lo que `fmt` reemite, formatSpecialBlock nunca mira Elements), incluso
	// las que TAMBIÉN se delegan a un parser del registry: Content y
	// Elements son dos vistas paralelas de la MISMA fuente, no una partición
	// — el renderer elige una u otra (Elements si no está vacío), nunca las
	// concatena.
	//
	// Un elemento anidado (tabla, imagen, chart, otro ":::" anidado...) se
	// reconoce por DELEGACIÓN a tryParseNestedContent en vez de contar
	// profundidad de ":::" a mano (C32 del audit: el corpus no cierra estos
	// bloques de forma consistente, así que cualquier conteo textual se
	// desincroniza tarde o temprano). Una línea ":::" PELADA sigue siendo,
	// a este nivel, el cierre — nunca se delega — porque es exactamente lo
	// que separa "otro ::: anidado" (con tipo) de "mi propio cierre" (sin
	// tipo): un ":::sub ... :::" anidado consume su PROPIO cierre en la
	// llamada recursiva antes de que este loop lo vuelva a ver.
	var content strings.Builder
	appendRawLine := func(line string) {
		if content.Len() > 0 {
			content.WriteString("\n")
		}
		content.WriteString(strings.TrimSpace(line))
	}

	var nested []ast.Element
	i := startIndex + 1
	for i < len(ctx.Lines) {
		trimmedLine := strings.TrimSpace(ctx.Lines[i])

		if trimmedLine == ":::" {
			// Found closing :::
			i++
			break
		}

		// Un límite de bloque que no es MI cierre — "SLIDE "/"SECTION "
		// (strict), un "---" de separador de slide/frontmatter (flex) —
		// corta sin consumir, para que el parser de nivel superior lo vea.
		// Antes de este fix (audit 2026-09-11, F9) esta línea solo chequeaba
		// "SLIDE ": un ":::bloque" sin su propio cierre, seguido de un "---"
		// de límite de slide, se tragaba ese "---" como si fuera prosa —
		// fusionando dos slides en una sola en silencio.
		//
		// A propósito NO se usa IsEmbeddedBlockBoundary acá (el helper
		// compartido de chart/quiz/poll): ese helper también corta en
		// "##"/"###" y en cualquier "<<...>>" — correcto para un chart, que
		// es una sola unidad sin prosa propia, pero errado acá, donde un
		// ":::bloque" SÍ puede llevar un "### heading-looking" como prosa
		// suelta (TestFlexParser_SubsectionHeadingNotStolenFromCodeOrSpecialBlock)
		// y SÍ tiene que poder delegar un "<<chart:...>>" anidado en vez de
		// cortar antes de llegar a tryParseNestedContent.
		if strings.TrimSpace(ctx.Lines[i]) == "---" || IsStrictBlockBoundary(ctx.Lines[i]) {
			break
		}

		if elem, n, ok := tryParseNestedContent(ctx, i); ok {
			nested = append(nested, elem)
			for k := 0; k < n; k++ {
				appendRawLine(ctx.Lines[i+k])
			}
			i += n
			continue
		}

		appendRawLine(ctx.Lines[i])
		i++
	}
	consumed := i - startIndex

	block := ast.NewSpecialBlockElement(pos, blockType, content.String())
	block.Title = title
	block.Elements = nested
	block.Icon = KnownSpecialBlockTypes()[blockType]

	return &ParseResult{
		Element:       block,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// KnownSpecialBlockTypes es la lista única de tipos de special block con
// soporte de primera clase, mapeados a su ícono por defecto ("" si no lleva
// uno). Antes de esto había TRES listas a mano que ya se contradecían
// (issue del audit 2026-09-11, C12): el switch de arriba (7 tipos, sin
// "details"/"left"/"right"/"highlight"), la whitelist del linter
// (rules.go — 6 tipos, sin "note"/"example" que este parser SÍ reconoce), y
// spec/language-specification.md:328-336, que documenta "left, right,
// highlight" como válidos sin que ningún código los reconociera como tales.
// Esta función es la fuente única que las tres deberían consultar — el
// linter (SPECIAL001) ya lo hace; slidelang (template/base.go) queda para
// el PR que consume el bump de core.
func KnownSpecialBlockTypes() map[string]string {
	return map[string]string{
		"info":      "💡",
		"warning":   "⚠️",
		"danger":    "🚨",
		"success":   "✅",
		"tip":       "💡",
		"note":      "📝",
		"example":   "💻",
		"details":   "",
		"left":      "",
		"right":     "",
		"highlight": "",
	}
}
