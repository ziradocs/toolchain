// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
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
//
// HeadingParser (PR-9 paso 3) SÍ se agrega acá aunque no viva en
// GetDefaultRegistry: a nivel top el encabezado lo intercepta cada parser
// de nivel superior ANTES del registry (flexSubsectionLevel/
// isSubsectionHeader), así que agregarlo al registry compartido no lo
// haría disparar ahí. Acá adentro, en cambio, es el ÚNICO lugar que puede
// reconocer un "### Título" anidado como encabezado real — antes de esto,
// esa línea nunca se delegaba y quedaba como prosa cruda en Content (ver
// el comentario de TestSpecialBlockParser_NestedSpecialBlock, que este PR
// actualiza). Va primero porque "#" no es ambiguo con ningún otro prefijo
// de esta lista.
var nestedContentParsers = []ElementParser{
	&HeadingParser{},
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
//
// El Error de un elemento que sí se delega (un CODE con un argumento de más,
// una cerca con info que no se lee, un atributo de <<mermaid>> que no es
// title, un marcador de <<map>> sin coordenadas) se descartaba acá: en el
// cuerpo strict es un error, y adentro del bloque el elemento salía sin esa
// parte y sin ningún diagnóstico. Ahora sale como aviso (SPECIAL003) en la
// línea del elemento; no como error, para que ningún documento que compila
// deje de compilar. opener es la línea que abre el bloque, para el mensaje.
func tryParseNestedContent(ctx *ParseContext, i int, opener string) (ast.Element, int, []diagnostics.Diagnostic, bool) {
	line := ctx.Lines[i]
	for _, parser := range nestedContentParsers {
		if !parser.CanParse(line, ctx.Mode) {
			continue
		}
		result := parser.Parse(ctx, i)
		if result.Element != nil && result.ConsumedLines > 0 {
			diags := result.Diagnostics
			if result.Error != nil {
				// En la línea interna a la que apunta el error (un marcador de
				// <<map>>), si trae una; si no, en la del elemento.
				line, end := ErrorLines(result.Error, i)
				d := ignoredLineWarning(ctx, line, NestedElementErrorRuleID, "special-block-parser",
					"Part of this element inside the %q block was not read: %s. The element is kept without it; fix the line.",
					shortLine(opener), result.Error.Error())
				d = withLineRange(ctx, d, end)
				diags = append([]diagnostics.Diagnostic{d}, diags...)
			}
			return result.Element, result.ConsumedLines, diags, true
		}
		return nil, 0, nil, false
	}
	return nil, 0, nil, false
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

	// ":::" o "::: :::" (sin tipo, con o sin autocierre): mismo caso vacío
	// que el guard de arriba, solo que llega hasta acá porque el ":::" del
	// cierre lo dejó vacío recién en el CutSuffix, no en el TrimSpace
	// original. Sin este chequeo, `strings.Fields("")` devuelve un slice
	// vacío y `parts[0]` abajo entra en pánico (encontrado por advisor
	// contra "::: :::" — antes de este fix ":::" producía blockContent=":::"
	// y NO paniqueaba; el pánico es nuevo del autocierre de este PR).
	if blockContent == "" {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 1,
			Error:         nil,
		}
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

	// Un atributo `typed` en la línea de apertura de un bloque STRICT
	// (`:::card{typed}`, `:::details{typed} Título`, `:::card{type="success"
	// typed}`) pide leer el cuerpo con los mismos reconocedores de contenido
	// anidado que usa flex, de modo que Content y Elements salgan idénticos a
	// los del bloque flex con ese cuerpo (docs/portable-typed-blocks.md). Es un
	// atributo y no una palabra del título para que nunca se cruce con él:
	// ninguna fuente existente cambia de significado. Flex no lo necesita —ya lo
	// lee así— y ahí `{typed}` sigue siendo parte del tipo, como cualquier otro
	// atributo.
	typed := false
	if ctx.Mode == "strict" {
		if bt, rest, ok := splitTypedAttribute(blockContent); ok {
			typed = true
			blockType = bt
			title = rest
		}
	}
	// nestedCtx es el contexto con el que se delega el cuerpo: el del propio
	// bloque, o el de flex cuando es typed.
	nestedCtx := ctx
	if typed {
		flexCtx := *ctx
		flexCtx.Mode = "flex"
		// Un bloque strict vive sangrado dentro de su SLIDE; un bloque flex, al
		// ras. Los reconocedores de flex que toman las líneas crudas (el
		// contenido de una cerca de código) tienen que ver las mismas líneas
		// que verían en flex, así que se les quita la sangría del bloque.
		indent := CalculateIndentLevel(ctx.Lines[startIndex])
		lines := make([]string, len(ctx.Lines))
		copy(lines, ctx.Lines)
		for k := startIndex + 1; k < len(lines); k++ {
			lines[k] = trimIndent(lines[k], indent)
		}
		flexCtx.Lines = lines
		nestedCtx = &flexCtx
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
	//
	// Cada línea sale recortada salvo los items de lista, que guardan la
	// sangría que tienen por encima de la primera línea del cuerpo
	// (keepListIndent): sin ella una lista anidada dentro del bloque llegaba
	// al AST, y al render, como un solo nivel.
	bodyIndent := firstBodyIndent(ctx.Lines, startIndex+1, func(l string) bool { return strings.TrimSpace(l) == ":::" })
	var content strings.Builder
	appendRawLine := func(line string) {
		if content.Len() > 0 {
			content.WriteString("\n")
		}
		content.WriteString(keepListIndent(line, bodyIndent))
	}

	var nested []ast.Element
	var nestedDiagnostics []diagnostics.Diagnostic
	// delegatedCount cuenta SOLO los elementos realmente delegados (no la
	// prosa sintética de flushProseRun): si termina en 0, `nested` se
	// descarta entero más abajo — un bloque sin ningún elemento anidado no
	// debe pasar a tener Elements no vacío solo porque tiene prosa, o el
	// renderer lo desviaría de la rama de Content que usa hoy sin ganar
	// nada (Elements vacío es, y debe seguir siendo, la señal de "no hay
	// nada tipado adentro").
	delegatedCount := 0

	// proseRun acumula las líneas de prosa suelta ENTRE dos elementos
	// delegados (o antes del primero / después del último), para volcarlas
	// como un ast.TextElement sintético en `nested` cuando corresponda —
	// necesario para que Elements sea una reconstrucción COMPLETA del
	// cuerpo, no solo "las partes delegadas". Sin esto, el renderer (que
	// una vez que Elements no está vacío deja de mirar Content del todo,
	// ver renderSpecialBlockElement en core/renderer/html.go) perdía en
	// silencio toda la prosa que rodeaba a un elemento anidado: agregado en
	// PR-9 paso 3 (encabezados) porque un heading dentro de un ":::bloque"
	// casi siempre viene acompañado de prosa/listas alrededor (ver
	// examples/01_title_and_content/01.6_ui_elements_flex.slidelang), lo
	// que hacía este bug mucho más fácil de disparar que con el chart/
	// tabla/imagen aislados de paso 2 — hallazgo del advisor. IsRawHTML
	// queda false (a diferencia del heading): es texto de markdown normal,
	// el mismo que renderTextElement ya sabía procesar.
	var proseRun strings.Builder
	proseStartLine := -1
	flushProseRun := func() {
		if proseRun.Len() == 0 {
			return
		}
		nested = append(nested, ast.NewTextElement(ctx.Position(proseStartLine), proseRun.String()))
		proseRun.Reset()
		proseStartLine = -1
	}
	appendProseLine := func(lineIndex int) {
		if proseStartLine == -1 {
			proseStartLine = lineIndex
		}
		if proseRun.Len() > 0 {
			proseRun.WriteString("\n")
		}
		proseRun.WriteString(keepListIndent(ctx.Lines[lineIndex], bodyIndent))
	}

	i := startIndex + 1
	closed := false
	for i < len(ctx.Lines) {
		trimmedLine := strings.TrimSpace(ctx.Lines[i])

		if trimmedLine == ":::" {
			// Found closing :::
			closed = true
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

		if elem, n, diags, ok := tryParseNestedContent(nestedCtx, i, line); ok {
			flushProseRun()
			nested = append(nested, elem)
			delegatedCount++
			nestedDiagnostics = append(nestedDiagnostics, diags...)
			for k := 0; k < n; k++ {
				appendRawLine(ctx.Lines[i+k])
			}
			i += n
			continue
		}

		appendRawLine(ctx.Lines[i])
		appendProseLine(i)
		i++
	}
	flushProseRun()
	consumed := i - startIndex

	if delegatedCount == 0 {
		nested = nil
	}

	// Sin su ":::" el bloque se queda con todo lo que sigue hasta el fin de la
	// sección, del slide o del documento (issue #490): otros callouts, CODE,
	// POINTS… pasan a ser su contenido. Eso no se puede distinguir de un
	// bloque que de verdad los contiene, así que el cuerpo se lee igual que
	// antes y se reporta en la línea que lo abrió.
	if !closed {
		// Va antes de los diagnósticos del contenido anidado: la apertura es
		// la línea más temprana del bloque, y así los diagnósticos salen en
		// orden de línea (el de un `::: raro` interno sin cerrar, después).
		nestedDiagnostics = append([]diagnostics.Diagnostic{unclosedSpecialBlockDiagnostic(ctx, startIndex, i, line)}, nestedDiagnostics...)
	}

	block := ast.NewSpecialBlockElement(pos, blockType, content.String())
	block.Title = title
	block.Elements = nested
	block.Icon = KnownSpecialBlockTypes()[blockType]

	return &ParseResult{
		Element:       block,
		ConsumedLines: consumed,
		Error:         nil,
		Diagnostics:   nestedDiagnostics,
	}
}

// UnclosedSpecialBlockRuleID es el diagnóstico de un bloque ":::" sin su
// ":::" de cierre. El parser de elementos lo emite como warning en todos los
// dialectos; el dialecto strict de documentos lo sube a error (ver
// DocumentStrictParser), igual que hace con una línea no reconocida.
const UnclosedSpecialBlockRuleID = "SPECIAL002"

// NestedElementErrorRuleID es el aviso de un elemento dentro de un bloque ":::"
// cuyo parser devolvió un error: fuera del bloque ese error se reporta, y
// adentro se perdía (ver tryParseNestedContent).
const NestedElementErrorRuleID = "SPECIAL003"

// unclosedSpecialBlockDiagnostic arma el SPECIAL002 de un bloque abierto en
// ctx.Lines[start] cuyo cuerpo terminó en end sin ":::". En strict, si el
// cuerpo tragado tiene una palabra clave de elemento a la misma sangría que la
// apertura (un CODE, un POINTS, otro elemento de la sección), la nombra: es lo
// más probable que el autor quisiera escribir después del cierre.
func unclosedSpecialBlockDiagnostic(ctx *ParseContext, start, end int, opener string) diagnostics.Diagnostic {
	where := "the end of what contains it (the section, slide, column or document)"
	if end < len(ctx.Lines) {
		if strings.TrimSpace(ctx.Lines[end]) == "---" {
			where = "the next ---"
		} else {
			where = "the next block (" + strings.TrimSpace(ctx.Lines[end]) + ")"
		}
	}
	msg := fmt.Sprintf("%q is never closed with \":::\": it takes everything up to %s as its content", opener, where)
	if ctx.Mode == "strict" {
		indent := CalculateIndentLevel(ctx.Lines[start])
		for k := start + 1; k < end; k++ {
			trimmed := strings.TrimSpace(ctx.Lines[k])
			if trimmed == "" || strings.HasPrefix(trimmed, ":::") || CalculateIndentLevel(ctx.Lines[k]) != indent {
				continue
			}
			if IsNewElement(trimmed, "strict") {
				// Sin número de línea (un @include o un node-id lo mueven): la
				// línea se nombra por su contenido.
				msg += fmt.Sprintf("; the line %q is at the block's own indentation, so the block probably ends before it", shortLine(trimmed))
				break
			}
		}
	}
	return diagnostics.NewWarning(msg+". Add a line with \":::\" where the block ends.", ctx.Position(start), "special-block-parser").WithRuleID(UnclosedSpecialBlockRuleID)
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

// trimIndent quita hasta n espacios iniciales de line.
func trimIndent(line string, n int) string {
	i := 0
	for i < len(line) && i < n && line[i] == ' ' {
		i++
	}
	return line[i:]
}

// splitTypedAttribute reconoce `tipo{... typed ...}` al inicio de la línea de
// apertura de un bloque. Devuelve el tipo SIN la bandera (con sus demás
// atributos separados por un espacio), el resto de la línea como título, y true.
// Solo cuenta `typed` como palabra completa fuera de comillas, y las palabras se
// separan por espacios o por comas: `{typed}`, `{ typed }` y `{type="x",typed}`
// son la bandera; `{title="a typed b"}` no.
func splitTypedAttribute(s string) (blockType, title string, ok bool) {
	open := strings.IndexByte(s, '{')
	if open <= 0 || strings.ContainsAny(s[:open], " \t") {
		return "", "", false
	}
	var tokens []string
	var cur strings.Builder
	inQuote := false
	closeAt := -1
	for i := open + 1; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			inQuote = !inQuote
		}
		if !inQuote && c == '}' {
			closeAt = i
			break
		}
		if !inQuote && (c == ' ' || c == '\t' || c == ',') {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteByte(c)
	}
	if closeAt < 0 {
		return "", "", false
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	var rest []string
	found := false
	for _, t := range tokens {
		if t == "typed" {
			found = true
			continue
		}
		rest = append(rest, t)
	}
	if !found {
		return "", "", false
	}
	blockType = s[:open]
	if len(rest) > 0 {
		blockType += "{" + strings.Join(rest, " ") + "}"
	}
	return blockType, strings.TrimSpace(s[closeAt+1:]), true
}
