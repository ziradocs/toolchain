// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/elements"
	"go.ziradocs.com/core/v2/util"
)

// StrictParser es un parser simple para modo Strict (versión inicial)
type StrictParser struct {
	content     string
	lines       []string
	currentLine int
	diagnostics []diagnostics.Diagnostic
	logger      util.Logger
}

// NewStrictParser crea un parser en modo strict. Los callers de librería
// (issue #134/G1c) inyectan su propio *util.Logger en vez de depender del
// logger global de conveniencia del CLI (util.GetDefault/InitDefault), que
// solo se inicializa si el consumidor es el CLI de slidelang y llamó
// util.InitDefault primero; cualquier otro consumidor de la librería (un
// fuzz harness, doclang, un tercero) recibía silenciosamente el Noop o,
// peor, un Logger nil crudo antes de que defaultLogger se blindara con
// NewNoop() (ver el comentario sobre esa var en util/logger.go). log==nil
// degrada a un Noop en vez de propagar el nil (hallazgo de security-review
// sobre PR #177: es API exportada, así que un caller externo pasando nil no
// debe panicar en el primer log del parse).
func NewStrictParser(input string, log util.Logger) *StrictParser {
	if log == nil {
		log = util.NewNoop()
	}
	return &StrictParser{
		content: input,
		lines:   strings.Split(input, "\n"),
		logger:  log}
}

func (p *StrictParser) Parse() (*ast.AST, []diagnostics.Diagnostic) {
	astNode := ast.NewAST(diagnostics.NewPosition(1, 1))
	p.diagnostics = nil

	p.logger.Debug("PARSE", "Starting parse, total lines: %d", len(p.lines))

	// Parser simple que busca patrones básicos
	for p.currentLine < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.currentLine])
		p.logger.Debug("PARSE", "Processing line: '%s'", line)

		if line == "" {
			p.logger.Debug("PARSE", "Empty line, skipping")
			p.currentLine++
			continue
		}

		// Check for SLIDE marker (content block)
		if strings.HasPrefix(line, "SLIDE") {
			block := p.parseContentBlock()
			if block != nil {
				astNode.ContentBlocks = append(astNode.ContentBlocks, *block)
			}
		} else {
			p.addError(fmt.Sprintf("unexpected content: %s", line))
			p.currentLine++
		}
	}

	return astNode, p.diagnostics
}

// parseBlockPropertyLine decide si trimmedLine es una propiedad de bloque
// "key: value" (heading/title/subtitle/logo, más el diagnóstico de propiedad
// desconocida) o el opener de un elemento cuyo contenido incluye ":".
//
// La heurística original era "contiene ':' y no empieza con TEXT/POINTS/:::/
// <</@": cualquier OTRA línea con dos puntos se tragaba como propiedad. Eso
// rompía IMAGE con una fuente que contuviera ":" — `IMAGE "https://…"` o
// `IMAGE "data:image/png;base64,…"` se malinterpretaban, emitiendo
// `Unknown content block property: IMAGE "https`. La causa: la lista de
// exclusión estaba INCOMPLETA (le faltaba IMAGE y el resto de openers de
// elemento), no que la detección de propiedad en sí fuera errónea.
//
// El fix completa esa exclusión: una línea es propiedad si contiene ':' y NO
// abre un elemento strict (startsStrictElement). `IMAGE "https://…"` abre un
// elemento (empieza con "IMAGE") → NO es propiedad → cae al despacho de imagen,
// donde su cadena entrecomillada (que YA soporta ':') sobrevive. Cualquier otra
// línea con ':' que no abra un elemento SÍ es una propiedad: si la clave no es
// conocida (p. ej. `título:` no-ASCII, `title.es:` con punto, `titel:` mal
// escrito) se DIAGNOSTICA como "Unknown content block property" en vez de
// descartarse en silencio — a diferencia de un intento anterior que exigía una
// clave "identificador simple" y hacía caer esas claves malformadas al
// catch-all silencioso.
func parseBlockPropertyLine(trimmedLine string) (key, value string, ok bool) {
	if startsStrictElement(trimmedLine) {
		return "", "", false
	}
	idx := strings.Index(trimmedLine, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(trimmedLine[:idx])
	value = strings.Trim(strings.TrimSpace(trimmedLine[idx+1:]), "\"")
	return key, value, true
}

// strictElementKeywords son las palabras clave (mayúsculas) que abren un
// elemento en modo strict — el mismo conjunto que despacha parseContentBlock
// (incluidas MERMAID/PLANTUML/CHART/MAP/MATH, cuyas ramas emiten el error
// "usa <<...>>"). Espejo local del despacho, no importado, igual que el
// formatter mantiene su propio espejo.
var strictElementKeywords = []string{
	"TEXT", "POINTS", "CODE", "IMAGE", "TABLE", "QUOTE",
	"CHECKLIST", "MERMAID", "PLANTUML", "CHART", "MAP", "MATH",
}

// startsStrictElement reporta si trimmedLine (ya sin espacios al inicio) abre
// un elemento strict — por palabra clave (IMAGE/TEXT/…) o por marcador
// simbólico (@ directiva, ::: bloque, << diagrama/chart/map/math/grid, | tabla
// Markdown). Se usa para excluir esos openers de la detección de propiedad de
// bloque: son elementos (su ':' es contenido, no un delimitador key/value), no
// propiedades. Espeja las condiciones HasPrefix del despacho de
// parseContentBlock para no discrepar de él.
func startsStrictElement(trimmedLine string) bool {
	if trimmedLine == "" {
		return false
	}
	switch trimmedLine[0] {
	case '@', '|':
		return true
	case ':':
		return strings.HasPrefix(trimmedLine, ":::")
	case '<':
		return strings.HasPrefix(trimmedLine, "<<")
	}
	for _, kw := range strictElementKeywords {
		if strings.HasPrefix(trimmedLine, kw) {
			return true
		}
	}
	return false
}

func (p *StrictParser) parseContentBlock() *ast.ContentBlock {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	line := strings.TrimSpace(p.lines[p.currentLine])
	parts := strings.Fields(line)

	if len(parts) < 1 || parts[0] != "SLIDE" {
		// Issue #45 (fuzzing): el caller (Parse(), arriba) solo confirma
		// strings.HasPrefix(line, "SLIDE") antes de llamar acá — una línea
		// como "SLIDEtitle" (sin espacio) pasa ese prefix check pero falla
		// este match exacto de parts[0]=="SLIDE". Sin avanzar p.currentLine
		// acá, Parse() vuelve a ver la MISMA línea sin cambios en la
		// siguiente iteración, la reconoce de nuevo como "empieza con
		// SLIDE", y llama a esta función otra vez — loop infinito
		// consumiendo CPU sin nunca progresar, encontrado fuzzeando el
		// parser directamente (reproducido: input crafted causaba un hang,
		// no un panic, invisible a un fuzz run que solo mira panics).
		p.addError("expected SLIDE")
		p.currentLine++
		return nil
	}

	pos := diagnostics.NewPosition(p.currentLine+1, 1)
	blockType := ""
	if len(parts) > 1 {
		blockType = parts[1]
	}

	block := ast.NewContentBlock(pos, blockType)
	p.currentLine++

	// Parsear contenido del bloque (elementos indentados)
	for p.currentLine < len(p.lines) {
		// Issue #45 (fuzzing): guarda genérica de forward-progress, igual a
		// la de flex.go/document_flex.go. Los 2 hangs que encontró el fuzzer
		// se parcharon en el handler puntual que los causaba
		// (parseMarkdownTableElement), pero este loop despacha a más de una
		// docena de parseXElement — cualquiera de ellos que en el futuro
		// gane un camino de retorno sin avanzar p.currentLine reproduciría
		// la misma clase de bug. Esta guarda lo cierra de forma genérica en
		// vez de depender de auditar cada handler a mano.
		startLine := p.currentLine
		line := p.lines[p.currentLine]

		// Si no está indentado, es el fin del slide
		if !strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" {
			break
		}

		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			p.currentLine++
			continue
		} // Parsear propiedades del bloque
		if key, value, ok := parseBlockPropertyLine(trimmedLine); ok {
			switch key {
			case "title":
				block.Title = value
			case "heading":
				block.Heading = value
			case "subtitle":
				block.Subtitle = value
			case "logo":
				block.Logo = value
			default:
				p.addError(fmt.Sprintf("Unknown content block property: %s. Check DSL Strict syntax documentation.", key))
			}
			p.currentLine++
			continue
		} // Parsear elementos
		if strings.HasPrefix(trimmedLine, "TEXT") {
			element := p.parseTextElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "POINTS") {
			element := p.parsePointsElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "CODE") {
			element := p.parseCodeElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "IMAGE") {
			element := p.parseImageElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "TABLE") {
			element := p.parseTableElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "QUOTE") {
			element := p.parseQuoteElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "CHECKLIST") {
			element := p.parseChecklistElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, ":::code-group") {
			element := p.parseCodeGroupElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, ":::") {
			p.logger.Debug("PARSE", "Found special block element, calling parseSpecialBlockElement")
			element := p.parseSpecialBlockElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
			p.logger.Debug("PARSE", "Finished parseSpecialBlockElement")
		} else if strings.HasPrefix(trimmedLine, "<<mermaid>>") {
			element := p.parseMermaidElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "<<plantuml>>") {
			element := p.parsePlantUMLElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "<<chart:") {
			element := p.parseChartElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "<<map>>") {
			element := p.parseMapElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "<<grid>>") {
			element := p.parseGridElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "<<math>>") {
			element := p.parseMathElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "MERMAID") {
			p.addError("Invalid syntax for Mermaid diagram. In strict mode, use: <<mermaid>>")
			p.currentLine++
		} else if strings.HasPrefix(trimmedLine, "PLANTUML") {
			p.addError("Invalid syntax for PlantUML diagram. In strict mode, use: <<plantuml>>")
			p.currentLine++
		} else if strings.HasPrefix(trimmedLine, "CHART") {
			p.addError("Invalid syntax for Chart element. In strict mode, use: <<chart:type>>")
			p.currentLine++
		} else if strings.HasPrefix(trimmedLine, "MAP") {
			p.addError("Invalid syntax for Map element. In strict mode, use: <<map>>")
			p.currentLine++
		} else if strings.HasPrefix(trimmedLine, "MATH") {
			p.addError("Invalid syntax for Math element. In strict mode, use: <<math>>")
			p.currentLine++
		} else if strings.HasPrefix(trimmedLine, "@") {
			// Directivas empiezan con @
			element := p.parseDirectiveElement()
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
		} else if strings.HasPrefix(trimmedLine, "|") && strings.Contains(trimmedLine, "|") {
			// Markdown table detection
			p.logger.Debug("PARSE", "Found markdown table at line %d: '%s'", p.currentLine, trimmedLine)
			element := p.parseMarkdownTableElement()
			if element != nil {
				p.logger.Debug("PARSE", "Successfully parsed markdown table with %d headers", len(element.(*ast.TableElement).Headers))
				block.Elements = append(block.Elements, element)
			} else {
				p.logger.Debug("PARSE", "Failed to parse markdown table")
			}
		} else {
			p.logger.Debug("PARSE", "No matching element pattern for line %d: '%s'", p.currentLine, trimmedLine)
			p.currentLine++
		}

		if p.currentLine == startLine {
			p.currentLine++
		}
	}

	return block
}

func (p *StrictParser) parseTextElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular TextParser
	textParser := &elements.TextParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := textParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines

	// Return the element directly (it's already of type ast.Element)
	return result.Element
}

func (p *StrictParser) parsePointsElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular PointsParser completely
	pointsParser := &elements.PointsParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := pointsParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseCodeElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular CodeParser
	codeParser := &elements.CodeParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := codeParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseImageElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular ImageParser
	imageParser := &elements.ImageParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := imageParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) addError(msg string) {
	pos := diagnostics.NewPosition(p.currentLine+1, 1)
	p.diagnostics = append(p.diagnostics,
		diagnostics.NewError(msg, pos, "parser"))
}

func (p *StrictParser) addWarning(msg string) {
	pos := diagnostics.NewPosition(p.currentLine+1, 1)
	p.diagnostics = append(p.diagnostics,
		diagnostics.NewWarning(msg, pos, "parser"))
}

func (p *StrictParser) parseTableElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular TableParser
	tableParser := &elements.TableParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := tableParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseQuoteElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular QuoteParser
	quoteParser := &elements.QuoteParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := quoteParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseChecklistElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular ChecklistParser
	checklistParser := &elements.ChecklistParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := checklistParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseSpecialBlockElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular SpecialBlockParser
	specialBlockParser := &elements.SpecialBlockParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := specialBlockParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseCodeGroupElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular CodeGroupParser
	codeGroupParser := &elements.CodeGroupParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := codeGroupParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseMermaidElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular MermaidParser
	mermaidParser := &elements.MermaidParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := mermaidParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseMathElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular MathParser (issue #239-B)
	mathParser := &elements.MathParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := mathParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parsePlantUMLElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular PlantUMLParser
	plantUMLParser := &elements.PlantUMLParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := plantUMLParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseChartElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular ChartParser
	chartParser := &elements.ChartParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := chartParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseGridElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular GridParser (strict form: <<grid>> … <<end>>)
	gridParser := &elements.GridParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := gridParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

func (p *StrictParser) parseMapElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular MapParser
	mapParser := &elements.MapParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}

	result := mapParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}

// parseMarkdownTableElement parsea tablas en formato markdown
func (p *StrictParser) parseMarkdownTableElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Issue #45 (fuzzing): el caller solo confirma que la línea empieza con
	// "|" y contiene otro "|" en algún lado (ver parseContentBlock) antes de
	// llamar acá — una línea como "|xyz" que empieza con "|" pero NO termina
	// en "|" no cumple ninguna de las condiciones de abajo (ni header row,
	// ni separator, ni data row), así que esta función podía retornar sin
	// avanzar p.currentLine en absoluto. El caller no re-chequea nada
	// especial en ese caso: vuelve a ver la MISMA línea, la reconoce de
	// nuevo como "posible tabla", y llama a esta función otra vez — loop
	// infinito consumiendo CPU. defer garantiza progreso sin importar por
	// qué código de salida se llegue al return.
	startLine := p.currentLine
	defer func() {
		if p.currentLine == startLine {
			p.currentLine++
		}
	}()

	pos := diagnostics.NewPosition(p.currentLine+1, 1)
	table := ast.NewTableElement(pos)

	// Inicializados como slices vacíos (no nil): Headers/Rows no llevan
	// omitempty en el AST, así que un valor nil se serializaría como JSON
	// null en vez de [] (issue #8 - viola el JSON Schema del contrato).
	headers := []string{}
	rows := [][]string{}

	// Parse header row
	headerParsed := false
	line := strings.TrimSpace(p.lines[p.currentLine])
	if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
		cells := strings.Split(line, "|")
		for i := 1; i < len(cells)-1; i++ { // Skip first and last empty cells
			headers = append(headers, strings.TrimSpace(cells[i]))
		}
		p.currentLine++
		headerParsed = true
	}

	// Skip separator row (|---------|--------|)
	if p.currentLine < len(p.lines) {
		separatorLine := strings.TrimSpace(p.lines[p.currentLine])
		if strings.Contains(separatorLine, "---") {
			p.currentLine++
		}
	}

	// Parse data rows
	for p.currentLine < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.currentLine])

		// Ninguna continuación de tabla en absoluto: termina acá.
		if !strings.HasPrefix(line, "|") {
			break
		}

		if !strings.HasSuffix(line, "|") {
			// Fila malformada — empieza con "|" pero no cierra (p. ej.
			// "|xyz"). Issue #155: antes esto SIEMPRE cortaba el loop, y
			// el caller (parseContentBlock) volvía a ver esta MISMA línea,
			// la reconocía de nuevo como "posible tabla" (empieza con
			// "|") y llamaba a parseMarkdownTableElement desde cero —
			// fragmentando una tabla legítima en 2-3 TableElement
			// separados, con pérdida silenciosa de las filas restantes.
			if !headerParsed {
				// Sin header válido: no hay tabla que continuar. Cortar
				// acá preserva la garantía de issue #8
				// (TestStrictParser_ParseMarkdownTableElement_MalformedHeader_HeadersNotNull):
				// "| A | B" sola sigue devolviendo Headers/Rows como
				// slice vacío [] (nunca nil), sin tratarla como error
				// fatal ni intentar "continuar" una tabla que nunca
				// existió.
				break
			}
			// Con header válido ya parseado: absorber la fila malformada
			// (saltarla, con un warning visible) y seguir acumulando las
			// filas siguientes en la MISMA tabla, en vez de fragmentar.
			p.addWarning(fmt.Sprintf("malformed table row skipped: %q", line))
			p.currentLine++
			continue
		}

		cells := strings.Split(line, "|")
		var row []string
		for i := 1; i < len(cells)-1; i++ { // Skip first and last empty cells
			row = append(row, strings.TrimSpace(cells[i]))
		}

		if len(row) > 0 {
			rows = append(rows, row)
		}

		p.currentLine++
	}

	table.Headers = headers
	table.Rows = rows
	return table
}

func (p *StrictParser) parseDirectiveElement() ast.Element {
	if p.currentLine >= len(p.lines) {
		return nil
	}

	// Use modular DirectiveParser
	directiveParser := &elements.DirectiveParser{}

	ctx := &elements.ParseContext{
		Mode:        "strict",
		Lines:       p.lines,
		CurrentLine: p.currentLine,
		Logger:      p.logger,
	}
	result := directiveParser.Parse(ctx, p.currentLine)
	if result.Error != nil {
		p.addError(result.Error.Error())
	}
	p.diagnostics = append(p.diagnostics, result.Diagnostics...)

	p.currentLine += result.ConsumedLines
	return result.Element
}
