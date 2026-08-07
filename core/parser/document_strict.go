// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// sectionKeyword abre un bloque en el dialecto strict documental, como
// SLIDE lo hace en el de presentaciones.
const sectionKeyword = "SECTION"

// maxSectionLevel es el nivel más profundo declarable, atado a que el HTML
// solo llega a <h6>.
const maxSectionLevel = 6

// DocumentStrictParser parsea documentos en el dialecto strict: el autor
// DECLARA la estructura (`SECTION "Título"` con sus propiedades y elementos
// indentados) en vez de que el parser la infiera de Markdown suelto, y nada
// se normaliza. Es el dialecto auditable: lo que se lee es lo que hay.
//
// Produce EXACTAMENTE la misma forma de AST que DocumentFlexParser —
// `SECTION` de nivel 1 abre un ContentBlock; los niveles 2-6 son elementos
// `<hN id=...>` dentro del bloque abierto, no bloques propios— y eso es
// deliberado: el renderer de documentos, el generador de TOC y las etapas
// de transform (xref/numeración) consumen esa forma, así que espejarla es
// lo que hace que los dos dialectos rindan idéntico sin una sola rama por
// modo aguas abajo.
//
// Diferencia deliberada con flex: flex DESCARTA una sección sin elementos
// (parseSection retorna nil), porque en Markdown un `#` suelto suele ser
// ruido. Acá no se descarta nada: el autor escribió el SECTION a propósito,
// y borrarlo en silencio es justo lo que el dialecto promete no hacer.
type DocumentStrictParser struct {
	strictBody
	input         string
	hasTitleBlock bool
}

// NewDocumentStrictParser crea el parser. log == nil degrada a Noop, mismo
// criterio que NewStrictParser (es API exportada; un caller externo pasando
// nil no debe panicar en el primer log).
func NewDocumentStrictParser(input string, log util.Logger) *DocumentStrictParser {
	if log == nil {
		log = util.NewNoop()
	}
	return &DocumentStrictParser{
		strictBody: strictBody{
			lines:  strings.Split(input, "\n"),
			logger: log,
		},
		input: input,
	}
}

// Parse recorre el documento: frontmatter opcional y luego una secuencia de
// SECTION a indentación 0. Cualquier otra cosa en el nivel superior es un
// error — no hay camino silencioso, que es el punto del dialecto.
func (p *DocumentStrictParser) Parse() (*ast.AST, []diagnostics.Diagnostic) {
	astNode := ast.NewAST(diagnostics.NewPosition(1, 1))

	if p.currentLine < len(p.lines) && strings.TrimSpace(p.lines[p.currentLine]) == "---" {
		lines, diags := parseDocumentFrontMatter(p.input, astNode)
		p.diagnostics = append(p.diagnostics, diags...)
		p.lines = lines
		p.currentLine = 0
	}

	for p.currentLine < len(p.lines) {
		raw := p.lines[p.currentLine]
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" {
			p.currentLine++
			continue
		}

		if !startsSectionKeyword(trimmed) {
			p.addError(fmt.Sprintf(
				"unexpected content in strict mode: %q. A DocLang strict document is a sequence of "+
					`SECTION "Title" blocks; prose and Markdown headings belong inside a section's `+
					"indented body (TEXT, POINTS, …).", trimmed))
			p.currentLine++
			continue
		}

		// Indentado: el autor quiso anidar secciones. No se anidan — la
		// jerarquía se declara con `level:`, no con espacios — y decirlo
		// explícitamente evita que el error salga como "propiedad
		// desconocida" desde el cuerpo del bloque anterior.
		if raw != trimmed {
			p.addError(fmt.Sprintf(
				"%s must start at column 1: nesting is declared with `level:`, not with indentation",
				sectionKeyword))
			p.currentLine++
			continue
		}

		p.parseSection(astNode)
	}

	return astNode, p.diagnostics
}

// startsSectionKeyword reporta si trimmed abre una sección. Exige frontera
// de palabra tras el keyword por el mismo motivo que el parser de slides
// tuvo que dejar de confiar en un HasPrefix pelado (issue #45): "SECTIONfoo"
// pasaría un prefix check pero no es un SECTION, y tratarlo como tal lleva
// a que el caller lo re-despache indefinidamente.
func startsSectionKeyword(trimmed string) bool {
	if !strings.HasPrefix(trimmed, sectionKeyword) {
		return false
	}
	rest := trimmed[len(sectionKeyword):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

// parseSection consume una sección completa y la incorpora al AST.
func (p *DocumentStrictParser) parseSection(astNode *ast.AST) {
	headerLine := p.currentLine
	title, err := parseSectionHeader(strings.TrimSpace(p.lines[headerLine]))
	if err != "" {
		p.addError(err)
		p.currentLine++
		return
	}
	p.currentLine++

	// El cuerpo se acumula en un bloque provisional: hasta terminar de leer
	// las propiedades no se sabe si esta sección ES un bloque (nivel 1) o un
	// encabezado DENTRO del bloque anterior (niveles 2-6). Por lo mismo el
	// título se sostiene en una variable y no se escribe en el bloque
	// todavía.
	scratch := ast.NewContentBlock(diagnostics.NewPosition(headerLine+1, 1), "content")

	level := 0 // 0 = no declarado; el default (1) se aplica al final
	explicitID := ""
	idLine := 0

	p.parseIndentedElements(scratch, func(key, value string) {
		switch key {
		case "level":
			n, convErr := strconv.Atoi(strings.TrimSpace(value))
			if convErr != nil {
				p.addError(fmt.Sprintf("level must be an integer between 1 and %d, got %q", maxSectionLevel, value))
				return
			}
			if n < 1 || n > maxSectionLevel {
				p.addError(fmt.Sprintf("level must be between 1 and %d, got %d", maxSectionLevel, n))
				return
			}
			level = n
		case "id":
			explicitID = value
			idLine = p.currentLine
			if deriveAnchor(value) == "" {
				p.addError(fmt.Sprintf(
					"id %q has no usable characters for an anchor (only letters, digits, '-' and '_' survive)", value))
			}
		case "title", "heading", "subtitle", "logo":
			// Propiedades de SLIDE. Aceptarlas acá dejaría dos fuentes de
			// verdad para el título de una sección (la cadena del SECTION y
			// esta propiedad) y silenciaría un archivo escrito contra el
			// dialecto equivocado.
			p.addError(fmt.Sprintf(
				"%q is a SLIDE property and has no meaning in a %s; a section's title is the quoted string on the %s line",
				key, sectionKeyword, sectionKeyword))
		default:
			p.addError(fmt.Sprintf(
				"Unknown %s property: %s. A section accepts `level:` and `id:`.", sectionKeyword, key))
		}
	}, startsSectionKeyword, func(trimmed string) {
		// Sin esto, una línea que el despacho no reconoce —un `TEXXT` mal
		// tecleado, un marcador inventado— se descartaba en silencio junto
		// con su cuerpo indentado, y el build terminaba en verde habiendo
		// borrado un párrafo. Es exactamente lo que el dialecto declarativo
		// promete no hacer, así que acá es un error.
		//
		// Se reporta línea por línea (el cuerpo del keyword desconocido cae
		// acá también, ya que sin un opener válido el parser no tiene forma
		// de saber que era un cuerpo). Es más ruidoso que un solo error por
		// bloque, y es la elección correcta: cada línea que no llegó al AST
		// queda nombrada, en vez de que el autor tenga que adivinar cuánto
		// se perdió.
		p.addError(fmt.Sprintf(
			"unexpected content inside %s: %q is not an element, a property, or a new %s",
			sectionKeyword, trimmed, sectionKeyword))
	})

	if level == 0 {
		level = 1
	}

	if level == 1 {
		if explicitID != "" {
			// Un ContentBlock no tiene campo de id en el AST, así que
			// aceptarlo sería aceptar-e-ignorar: el ancla seguiría saliendo
			// derivada del título y el `\ref` del autor apuntaría a otra
			// parte.
			p.addErrorAt(idLine, fmt.Sprintf(
				"`id:` is only supported on level 2-%d sections; a level-1 section's anchor is derived from its title",
				maxSectionLevel))
		}
		p.appendTopLevelSection(astNode, title, headerLine, scratch.Elements)
		return
	}

	if len(astNode.ContentBlocks) == 0 {
		// Sin bloque padre no hay dónde colgar el encabezado. Se reporta y
		// se promueve a nivel 1 en vez de descartarlo: el build falla igual
		// (hay un error), pero el contenido sigue en el AST y el autor ve
		// su documento en vez de un vacío inexplicable.
		p.addErrorAt(headerLine, fmt.Sprintf(
			"a level-%d %s cannot be the first section: there is no level-1 section to nest it under",
			level, sectionKeyword))
		p.appendTopLevelSection(astNode, title, headerLine, scratch.Elements)
		return
	}

	parent := &astNode.ContentBlocks[len(astNode.ContentBlocks)-1]
	parent.Elements = append(parent.Elements, buildHeadingElement(title, level, headerLine, explicitID))
	parent.Elements = append(parent.Elements, scratch.Elements...)
}

// appendTopLevelSection agrega una sección de nivel 1 como ContentBlock
// nuevo, con la misma regla posicional que el dialecto flex: el primero es
// el bloque "title" del documento (su texto va a Heading), los demás son
// "content" (su texto va a Title).
func (p *DocumentStrictParser) appendTopLevelSection(astNode *ast.AST, title string, headerLine int, els []ast.Element) {
	blockType := "content"
	if !p.hasTitleBlock {
		blockType = "title"
		p.hasTitleBlock = true
	}

	block := ast.NewContentBlock(diagnostics.NewPosition(headerLine+1, 1), blockType)
	if blockType == "title" {
		block.Heading = title
	} else {
		block.Title = title
	}
	block.Elements = els

	astNode.ContentBlocks = append(astNode.ContentBlocks, *block)
}

// parseSectionHeader extrae el título entrecomillado de una línea SECTION.
// Devuelve el mensaje de error como string vacío cuando la línea es válida.
//
// Las comillas son obligatorias y las propiedades NO van en línea: se
// declaran indentadas debajo, igual que en un SLIDE. Un boceto previo del
// dialecto mostraba `SECTION "Foo" level: 1` en su tabla de migración pero
// propiedades indentadas en todos sus ejemplos; se adopta la forma indentada,
// que es la que ya usa el resto del dialecto strict.
func parseSectionHeader(trimmed string) (title string, errMsg string) {
	rest := strings.TrimSpace(trimmed[len(sectionKeyword):])

	if rest == "" {
		return "", fmt.Sprintf(`%s requires a quoted title, e.g. %s "Introduction"`, sectionKeyword, sectionKeyword)
	}

	if !strings.HasPrefix(rest, `"`) {
		return "", fmt.Sprintf(`%s title must be quoted, e.g. %s "Introduction"`, sectionKeyword, sectionKeyword)
	}

	// Se busca la ÚLTIMA comilla, no la segunda: así un título que contiene
	// comillas ("El informe \"final\"") sobrevive sin necesitar un mecanismo
	// de escape propio.
	end := strings.LastIndex(rest, `"`)
	if end == 0 {
		return "", fmt.Sprintf("%s title is missing its closing quote", sectionKeyword)
	}

	if trailing := strings.TrimSpace(rest[end+1:]); trailing != "" {
		return "", fmt.Sprintf(
			"unexpected %q after the %s title: properties go on indented lines below, not on the %s line",
			trailing, sectionKeyword, sectionKeyword)
	}

	return rest[1:end], ""
}
