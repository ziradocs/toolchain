// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// PointsParser maneja elementos de listas/puntos
type PointsParser struct{}

// CanParse determina si una línea es el inicio de una lista de puntos
func (p *PointsParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// Strict mode: POINTS keyword
	if mode == "strict" && strings.HasPrefix(trimmed, "POINTS") {
		return true
	}

	// "* * *" es un thematic break, no una viñeta — ver el mismo guard en
	// isListItem (issue #242). Va acá TAMBIÉN, y no solo allá, porque es
	// CanParse lo que consulta el registry para elegir parser: dejarlo solo
	// en isListItem no cambiaba nada, PointsParser seguía reclamando la
	// línea y el bullet "* *" seguía saliendo en la diapositiva.
	if isThematicBreak(trimmed) {
		return false
	}

	// Both modes: Markdown list syntax
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return true
	}

	// Numbered lists, "1." o "1)" como en CommonMark. Hasta dos dígitos, como
	// siempre: "100. x" sin list-start-v1 sigue siendo un párrafo.
	if digits, _, ok := orderedNumber(trimmed); ok && len(digits) <= 2 {
		return true
	}
	if len(trimmed) > 2 && (trimmed[1] == '.' || trimmed[2] == '.') {
		for i, char := range trimmed {
			if char == '.' {
				if i > 0 && i+1 < len(trimmed) && trimmed[i+1] == ' ' {
					return true
				}
				break
			}
			if char < '0' || char > '9' {
				if i == 0 {
					return false
				}
				break
			}
		}
	}

	return false
}

// Parse parsea una lista de puntos desde las líneas proporcionadas
func (p *PointsParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}
	pos := ctx.Position(startIndex)
	element := ast.NewPointsElement(pos)
	var diags []diagnostics.Diagnostic
	if ctx.Mode == "strict" && strings.HasPrefix(strings.TrimSpace(ctx.Lines[startIndex]), "POINTS") {
		// El texto de la propia línea POINTS no se lee (STRICT004).
		if d := keywordLineRestWarning(ctx, startIndex, "POINTS", "points-parser"); d != nil {
			diags = append(diags, *d)
		}
	}
	if ctx.NestedListTypes || ctx.ListStart {
		result := p.parseTypedList(ctx, startIndex, element)
		result.Diagnostics = append(diags, result.Diagnostics...)
		return result
	}
	consumed := 0
	line := strings.TrimSpace(ctx.Lines[startIndex])

	if ctx.Mode == "strict" && strings.HasPrefix(line, "POINTS") {
		// Skip POINTS line
		consumed++
		startIndex++

		// Detectar tipo de lista basado en el primer elemento
		firstItemProcessed := false
		nest := legacyNest{strict: true}
		expectedIndent := -1 // Auto-detect indentation level

		// Collect indented lines
		for i := startIndex; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmed := strings.TrimSpace(line)

			if ctx.Logger != nil {
				ctx.Logger.Debug("POINTS-PARSER", " Line %d: processing line: '%s'", i+1, line)
			}
			// Calculate current line indentation
			currentIndent := CalculateIndentLevel(line)
			// Skip empty lines
			if trimmed == "" {
				consumed++
				continue
			}

			// Auto-detect expected indentation from first non-empty line
			if expectedIndent == -1 && currentIndent > 0 {
				expectedIndent = currentIndent
				if ctx.Logger != nil {
					ctx.Logger.Debug("POINTS-PARSER", " Line %d: auto-detected indentation level: %d", i+1, expectedIndent)
				}
			}

			// Check if this line should be part of the points block
			if expectedIndent > 0 && currentIndent < expectedIndent && trimmed != "" {
				if ctx.Logger != nil {
					ctx.Logger.Debug("POINTS-PARSER", " Line %d: end of indented block (expected: %d, got: %d), breaking", i+1, expectedIndent, currentIndent)
				}
				break
			}

			// If we haven't detected indentation yet and line has no indentation, break
			if expectedIndent == -1 && currentIndent == 0 && trimmed != "" {
				if ctx.Logger != nil {
					ctx.Logger.Debug("POINTS-PARSER", " Line %d: non-indented content found, breaking", i+1)
				}
				break
			}

			// Check if this is any type of list item (-, numbers, letters)
			if p.isListItem(trimmed) {
				indent := CalculateIndentLevel(line) // Detectar tipo de lista en el primer elemento principal
				if !firstItemProcessed && indent == expectedIndent {
					element.ListType = p.detectListType(trimmed)
					firstItemProcessed = true
				}

				content := p.extractListContent(trimmed)

				if content != "" && indent >= expectedIndent {
					item := ast.NewPointItem(ctx.Position(i), content)
					nest.add(element, item, indent, trimmed, p.detectListType(trimmed))
				}
			} else {
				// Una línea que no es item no tiene dónde ir: el POINTS strict
				// no tiene continuaciones. Se consume como siempre, pero con
				// aviso (POINTS001). La línea puede estar a la sangría de los
				// items o más adentro, y también a la del propio POINTS
				// cuando los items van a esa misma sangría.
				diags = append(diags, ignoredLineWarning(ctx, i, PointsLineRuleID, "points-parser",
					"This line of the POINTS is not a list item and was ignored: %q. Start it with \"- \" to make it an item; if it is meant to come after the list, write it at a shallower indentation than the items.",
					trimmed))
			}

			consumed++
		}
	} else {
		// Parse Markdown-style list (flex mode or compatibility)
		consumed = p.parseMarkdownList(ctx, startIndex, element)
	}

	return &ParseResult{
		Element:       element,
		ConsumedLines: consumed,
		Error:         nil,
		Diagnostics:   diags,
	}
}

// legacyNest arma el árbol de una lista sin nested-list-types-v1 con la regla
// de CommonMark: un item es hijo del item abierto de arriba solo si su
// marcador empieza en la columna de contenido de ese item o más adentro (la
// columna que sigue al marcador y su espacio: 2 para "- a", 3 para "1. a").
// Si no llega, es hermano de ese item o vuelve a un nivel de afuera; una
// escalera descuidada ("- a" / " - b" / "  - c") son tres hermanos. Antes
// todo lo que pasaba del nivel base se colgaba del último item base, y un
// tercer nivel salía como hermano del segundo.
//
// El JSON sigue con la forma de siempre (subPoints recursivo, que el esquema
// ya admitía, y sin subListType): el tipo de cada sublista, el marcador, el
// número y el delimitador de cada item quedan solo en memoria
// (SubListMarker, Marker, Ordinal, Delimiter), así que el documento no cambia
// de versión ni declara capacidades.
//
// Los punteros de la pila siguen válidos porque se saca un nivel antes de
// agregar un hermano a la lista de ese nivel, igual que en parseTypedList.
type legacyNest struct {
	stack []legacyLevel
	// strict usa la regla de su propia gramática, que es por sangría y no
	// Markdown: cualquier item más sangrado que el de arriba es su hijo. Es lo
	// que leyó siempre y lo que escribió `fmt` hasta ahora (hijos a dos
	// espacios también bajo "1."), así que esos archivos siguen leyéndose
	// igual.
	strict bool
}

type legacyLevel struct {
	content int // columna de contenido del item
	item    *ast.PointItem
}

// topLevel dice si un item con esta sangría queda en la lista de afuera. La
// columna de contenido crece con cada nivel, así que basta mirar la del item
// base abierto.
func (n *legacyNest) topLevel(indent int) bool {
	return len(n.stack) == 0 || indent < n.stack[0].content
}

// last es el último item abierto, el más profundo; nil si no hay ninguno.
func (n *legacyNest) last() *ast.PointItem {
	if len(n.stack) == 0 {
		return nil
	}
	return n.stack[len(n.stack)-1].item
}

// lastContent es la columna de contenido del último item abierto.
func (n *legacyNest) lastContent() int {
	return n.stack[len(n.stack)-1].content
}

func (n *legacyNest) add(element *ast.PointsElement, item *ast.PointItem, indent int, trimmed, kind string) {
	item.Ordinal = listOrdinal(trimmed)
	item.Delimiter = orderedDelimiter(trimmed)
	level := legacyLevel{content: indent + listMarkerWidth(trimmed) + 1}
	if n.strict {
		level.content = indent + 1
	}
	for len(n.stack) > 0 && indent < n.stack[len(n.stack)-1].content {
		n.stack = n.stack[:len(n.stack)-1]
	}
	if len(n.stack) == 0 {
		element.Items = append(element.Items, *item)
		level.item = &element.Items[len(element.Items)-1]
		n.stack = append(n.stack, level)
		return
	}
	parent := n.stack[len(n.stack)-1].item
	if parent.SubListMarker == "" {
		parent.SubListMarker = kind
	}
	item.Marker = kind
	parent.SubPoints = append(parent.SubPoints, *item)
	level.item = &parent.SubPoints[len(parent.SubPoints)-1]
	n.stack = append(n.stack, level)
}

// orderedNumber separa el número y el delimitador ("." o ")") de un item
// ordenado ya recortado: "12) x" da ("12", ")", true).
func orderedNumber(line string) (string, string, bool) {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i+1 >= len(line) || (line[i] != '.' && line[i] != ')') || line[i+1] != ' ' {
		return "", "", false
	}
	return line[:i], line[i : i+1], true
}

// listOrdinal devuelve el número decimal que abre un item ordenado ("12. x"
// o "12) x" es 12, "0. x" es 0), o nil si el marcador es una viñeta, una
// letra o no cabe en int64.
func listOrdinal(line string) *int64 {
	digits, _, ok := orderedNumber(strings.TrimSpace(line))
	if !ok {
		return nil
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || n < 0 {
		return nil
	}
	return &n
}

// orderedDelimiter es "." o ")" para un item numerado ("." también para una
// letra, "a."), "" para una viñeta.
func orderedDelimiter(line string) string {
	trimmed := strings.TrimSpace(line)
	if _, delim, ok := orderedNumber(trimmed); ok {
		return delim
	}
	if len(trimmed) >= 3 && trimmed[1] == '.' && trimmed[2] == ' ' {
		return "."
	}
	return ""
}

// listMarkerWidth es el ancho del marcador de un item ya recortado: 1 para
// una viñeta, los dígitos más el delimitador para un número, 2 para "a.".
func listMarkerWidth(line string) int {
	if digits, _, ok := orderedNumber(line); ok {
		return len(digits) + 1
	}
	if len(line) >= 2 && line[1] == '.' {
		return 2
	}
	return 1
}

// parseTypedList is the opt-in path. Each level has one list type, owned by
// the item immediately above it. Pointers to ancestors remain valid because
// we pop a level before appending a sibling to that level's slice.
func (p *PointsParser) parseTypedList(ctx *ParseContext, startIndex int, element *ast.PointsElement) *ParseResult {
	strict := ctx.Mode == "strict" && strings.HasPrefix(strings.TrimSpace(ctx.Lines[startIndex]), "POINTS")
	start := startIndex
	if strict {
		start++
	}
	type level struct {
		indent int
		item   *ast.PointItem
	}
	var stack []level
	baseIndent := -1
	consumed := start - startIndex
	var diags []diagnostics.Diagnostic
	for i := start; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if strict || (i+1 < len(ctx.Lines) && p.isListItem(strings.TrimSpace(ctx.Lines[i+1]))) {
				consumed++
				continue
			}
			break
		}
		indent := CalculateIndentLevel(line)
		// "1)" es un marcador numerado también con nested-list-types-v1 y
		// list-start-v1, como en CommonMark y como sin la capacidad. Desde que
		// CanParse lo reclama (#458), rechazarlo aquí dejaba la primera línea
		// sin consumir y el bucle flex no avanzaba: la CLI se colgaba.
		if !p.isListItem(trimmed) {
			if malformedPointMarker(trimmed) && (strict || ctx.ListStart || len(stack) > 0) {
				if baseIndent < 0 {
					baseIndent = indent
				}
				if indent >= baseIndent {
					diags = append(diags, diagnostics.NewError("empty or invalid list marker", ctx.Position(i), "points-parser"))
					consumed++
					continue
				}
			}
			break
		}
		if baseIndent < 0 {
			baseIndent = indent
		}
		if indent < baseIndent {
			break
		}
		kind := p.detectListType(trimmed)
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			if indent != baseIndent {
				diags = append(diags, diagnostics.NewError("orphan nested list item", ctx.Position(i), "points-parser"))
				consumed++
				continue
			}
			// Entre items numerados del nivel base, cambiar de delimitador
			// ("1." a "1)") abre otra lista, como en CommonMark y en
			// parseMarkdownList. strict no parte el bloque POINTS.
			if !strict && len(element.Items) > 0 && kind == "ordered" && kind == element.ListType && orderedDelimiter(trimmed) != element.Items[0].Delimiter {
				break
			}
			if len(element.Items) > 0 && kind != element.ListType {
				if strict {
					diags = append(diags, diagnostics.NewError("mixed markers in one list level", ctx.Position(i), "points-parser"))
				} else {
					break
				}
			}
			if ctx.ListStart && kind == "ordered" {
				if err := captureListOrdinal(trimmed, &element.Start, len(element.Items)); err != nil {
					diags = append(diags, diagnostics.NewError(err.Error(), ctx.Position(i), "points-parser"))
				}
			}
			if len(element.Items) == 0 {
				element.ListType = kind
			}
			item := ast.NewPointItem(ctx.Position(i), p.extractListContent(trimmed))
			item.Ordinal = listOrdinal(trimmed)
			item.Delimiter = orderedDelimiter(trimmed)
			if item.Content == "" {
				diags = append(diags, diagnostics.NewError("empty list item", ctx.Position(i), "points-parser"))
			} else {
				element.Items = append(element.Items, *item)
				stack = append(stack, level{indent: indent, item: &element.Items[len(element.Items)-1]})
			}
		} else {
			parent := stack[len(stack)-1].item
			if ctx.ListStart && !ctx.NestedListTypes {
				diags = append(diags, diagnostics.NewError("nested list starts require nested-list-types-v1", ctx.Position(i), "points-parser"))
				consumed++
				continue
			}
			if ctx.ListStart && kind == "ordered" {
				if err := captureListOrdinal(trimmed, &parent.SubListStart, len(parent.SubPoints)); err != nil {
					diags = append(diags, diagnostics.NewError(err.Error(), ctx.Position(i), "points-parser"))
				}
			}
			if parent.SubListType != "" && parent.SubListType != kind {
				diags = append(diags, diagnostics.NewError("mixed markers in one nested list level", ctx.Position(i), "points-parser"))
			} else if parent.SubListType == "" {
				parent.SubListType = kind
			}
			item := ast.NewPointItem(ctx.Position(i), p.extractListContent(trimmed))
			item.Ordinal = listOrdinal(trimmed)
			item.Delimiter = orderedDelimiter(trimmed)
			if item.Content == "" {
				diags = append(diags, diagnostics.NewError("empty nested list item", ctx.Position(i), "points-parser"))
			} else {
				parent.SubPoints = append(parent.SubPoints, *item)
				stack = append(stack, level{indent: indent, item: &parent.SubPoints[len(parent.SubPoints)-1]})
			}
		}
		consumed++
	}
	if len(element.Items) == 0 {
		diags = append(diags, diagnostics.NewError("empty POINTS list", ctx.Position(startIndex), "points-parser"))
	}
	return &ParseResult{Element: element, ConsumedLines: consumed, Diagnostics: diags}
}

func captureListOrdinal(line string, start **int64, count int) error {
	marker, _, ok := orderedNumber(line)
	if !ok {
		marker, _, _ = strings.Cut(line, ". ")
	}
	n, err := strconv.ParseInt(marker, 10, 64)
	if err != nil || n < 1 || n > ast.MaxListStart {
		return fmt.Errorf("invalid ordered list ordinal %q: expected 1..%d", marker, ast.MaxListStart)
	}
	if count == 0 {
		*start = &n
		return nil
	}
	if *start == nil || int64(count) > ast.MaxListStart-**start || n != **start+int64(count) {
		return fmt.Errorf("discontinuous ordered list ordinal %q", marker)
	}
	return nil
}

func malformedPointMarker(line string) bool {
	marker, _, _ := strings.Cut(line, " ")
	if len(marker) > 1 && (marker[0] >= '0' && marker[0] <= '9') && (strings.HasSuffix(marker, ".") || strings.HasSuffix(marker, ")")) {
		return true
	}
	if len(line) > 1 && (line[0] == '-' || line[0] == '+') && line[1] >= '0' && line[1] <= '9' {
		return malformedPointMarker(line[1:])
	}
	if line == "-" || line == "*" || line == "+" {
		return true
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(line) || (line[i] != '.' && line[i] != ')') {
		return false
	}
	return i+1 == len(line) || line[i+1] == ' ' || line[i+1] == '\t'
}

// parseMarkdownList parsea una lista en formato Markdown
func (p *PointsParser) parseMarkdownList(ctx *ParseContext, startIndex int, element *ast.PointsElement) int {
	lines := ctx.Lines
	consumed := 0
	baseIndent := -1
	firstItemProcessed := false
	var nest legacyNest

	// paragraph: la línea anterior fue texto de un item (su marcador o una
	// continuación), no una línea en blanco. Es lo que en CommonMark decide la
	// continuación perezosa y si una lista numerada puede interrumpir.
	paragraph := false
	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Stop at empty lines
		if trimmed == "" {
			// Allow one empty line within list
			if i+1 < len(lines) && p.isListItem(strings.TrimSpace(lines[i+1])) {
				paragraph = false
				consumed++
				continue
			}
			break
		}

		// Sangría en columnas, con tabuladores a paradas de 4 como CommonMark.
		indent := IndentColumns(line)

		// Una línea que no es item continúa el último item, como en
		// CommonMark: sangrada hasta su columna de contenido ("1. Una frase
		// larga\n   que sigue") o, justo después de su texto, con cualquier
		// sangría (continuación perezosa: "1. a\nque sigue\n2. b"). Antes
		// cortaba la lista: la continuación salía como párrafo y el item
		// siguiente como otra lista. Una línea que abre otro bloque (un
		// chart, una ecuación, un encabezado, una cerca, una tabla, "---")
		// nunca es continuación: corta la lista, como siempre.
		if !p.isListItem(trimmed) {
			last := nest.last()
			if last != nil && !StartsOtherBlock(trimmed) && (paragraph || indent >= nest.lastContent()) {
				last.Content += " " + trimmed
				paragraph = true
				consumed++
				continue
			}
			break
		}
		if baseIndent == -1 {
			baseIndent = indent
		}

		if indent < baseIndent {
			// Indentación menor que la base, terminar parsing
			break
		}

		kind := p.detectListType(trimmed)
		delim := orderedDelimiter(trimmed)

		// En CommonMark una lista numerada solo interrumpe un párrafo si
		// empieza en 1: un item que abriría una sublista dentro del último
		// item, justo después de su texto ("- a\n  1998. el año"), es texto
		// de ese item. Más afuera de su columna de contenido el item cierra
		// el de arriba y abre su lista sin esa condición ("- a\n2. b").
		if last := nest.last(); last != nil && paragraph && kind == "ordered" && indent >= nest.lastContent() {
			// Una lista de letras ("a. x") no es de CommonMark: es del dialecto
			// y sigue siendo lista.
			if n := listOrdinal(trimmed); n != nil && *n != 1 {
				last.Content += " " + trimmed
				consumed++
				continue
			}
		}

		// Detectar tipo de lista en el primer elemento principal
		if !firstItemProcessed {
			element.ListType = kind
			firstItemProcessed = true
		}

		// Un item de nivel base que cambia de marcador cierra la lista SIN
		// consumirse: es el primer item de la lista SIGUIENTE (issue #241).
		//
		// El tipo se detectaba una sola vez, en el primer item, y de ahí en
		// más cualquier item entraba al mismo elemento. Así, el patrón más
		// común de un deck —"los puntos clave, y luego los pasos"— terminaba
		// en un único PointsElement `unordered` con los cuatro items
		// pegados: la numeración se perdía sin ningún diagnóstico. Ni la
		// línea en blanco ni el cambio de marcador lo separaban, porque la
		// blanca ya se tolera a propósito dentro de una lista (más arriba en
		// este mismo loop) y el marcador no se volvía a mirar nunca.
		//
		// Solo aplica al NIVEL BASE (un item que no cae dentro de la columna de
		// contenido del item base anterior, ver legacyNest). Un item más
		// indentado es un sub-punto y lo maneja el bloque de abajo: "- alpha"
		// con "  1. anidado" debajo sigue siendo una sola lista con un
		// sub-punto, no dos listas.
		//
		// Entre items numerados, cambiar el delimitador ("1." a "1)") también
		// abre otra lista, como en CommonMark.
		//
		// Un cambio de carácter DENTRO de las viñetas ("-" a "*") no separa:
		// detectListType devuelve "unordered" para los dos. CommonMark sí
		// abriría lista nueva ahí; replicarlo es un cambio aparte, y este
		// issue es sobre perder el listType, no sobre partir viñetas.
		if len(element.Items) > 0 && nest.topLevel(indent) {
			if kind != element.ListType || kind == "ordered" && delim != element.Items[0].Delimiter {
				break
			}
		}

		// Extract content
		content := p.extractListContent(trimmed)
		if content != "" {
			item := ast.NewPointItem(ctx.Position(i), content)
			nest.add(element, item, indent, trimmed, kind)
		}
		paragraph = true

		consumed++
	}

	return consumed
}

// IsListItem checks if a line is a list item (public method)
func (p *PointsParser) IsListItem(line string) bool {
	return p.isListItem(line)
}

// ExtractListContent extracts the content from a list item line (public method)
func (p *PointsParser) ExtractListContent(line string) string {
	return p.extractListContent(line)
}

// isListItem checks if a line is a list item
func (p *PointsParser) isListItem(line string) bool {
	// "* * *" es un thematic break de CommonMark, no una viñeta (issue
	// #242). Sin este guard, PointsParser lo reclamaba antes que nadie —
	// empieza con "* "— y la diapositiva mostraba un bullet con el texto
	// "* *". El guard solo alcanza a las formas con "*" y "_": la de guiones
	// ("- - -") queda fuera a propósito, igual que en isThematicBreak, para
	// no cambiar de paso el comportamiento de un marcador que además es el
	// separador de bloque del dialecto.
	if isThematicBreak(line) {
		return false
	}

	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}

	// Check for numbered lists (1. 2. etc., o 1) 2) como en CommonMark)
	if _, _, ok := orderedNumber(line); ok {
		return true
	}
	if len(line) > 2 {
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
	}

	// Check for alphabetic lists (a. b. c. etc.)
	if len(line) >= 3 && line[1] == '.' && line[2] == ' ' {
		char := line[0]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			return true
		}
	}

	return false
}

// extractListContent extracts the content from a list item line
func (p *PointsParser) extractListContent(line string) string {
	// Handle unordered lists
	if strings.HasPrefix(line, "- ") {
		return strings.TrimSpace(line[2:])
	}
	if strings.HasPrefix(line, "* ") {
		return strings.TrimSpace(line[2:])
	}
	if strings.HasPrefix(line, "+ ") {
		return strings.TrimSpace(line[2:])
	}

	// Handle numbered lists
	if digits, _, ok := orderedNumber(line); ok {
		return strings.TrimSpace(line[len(digits)+2:])
	}
	dotIndex := strings.Index(line, ". ")
	if dotIndex > 0 {
		return strings.TrimSpace(line[dotIndex+2:])
	}

	return ""
}

// detectListType detecta si una línea indica una lista ordenada o no ordenada
func (p *PointsParser) detectListType(line string) string {
	trimmed := strings.TrimSpace(line)

	// Lista no ordenada (bullets)
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return "unordered"
	}

	// Lista numerada (1. 2. 3. etc., o 1) 2) 3))
	if _, _, ok := orderedNumber(trimmed); ok {
		return "ordered"
	}
	if len(trimmed) > 2 {
		for i, char := range trimmed {
			if char == '.' && i > 0 {
				// Check if followed by space
				if i+1 < len(trimmed) && trimmed[i+1] == ' ' {
					return "ordered"
				}
				break
			}
			if char < '0' || char > '9' {
				break
			}
		}
	}

	// Lista alfabética (a. b. c. etc.) - también se considera ordenada
	if len(trimmed) >= 3 && trimmed[1] == '.' && trimmed[2] == ' ' {
		char := trimmed[0]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			return "ordered"
		}
	}

	return "unordered" // default
}

// isSubItem detecta si una línea es un sub-elemento basado en la indentación
func (p *PointsParser) isSubItem(line string, baseIndent int) bool {
	trimmed := strings.TrimSpace(line)
	if !p.isListItem(trimmed) {
		return false
	}

	indent := CalculateIndentLevel(line)
	return indent > baseIndent
}
