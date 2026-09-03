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

// FlexParser parsea archivos SlideLang en modo flexible
type FlexParser struct {
	input         string
	lines         []string
	currentLine   int
	diagnostics   []diagnostics.Diagnostic
	logger        util.Logger
	registry      *elements.Registry
	hasTitleBlock bool // Rastrea si ya hemos encontrado un bloque de título
	// pendingLayout guarda el `layout:` de un bloque de metadata por slide
	// hasta que aparezca el bloque al que le toca (issue #239). Vive en el
	// parser y no en una variable local porque el bloque de metadata y el
	// slide son DOS llamadas distintas a parseContentBlock: la primera
	// consume la metadata y devuelve nil, la segunda arma el slide.
	pendingLayout string
	// reportedInertKeys recuerda por qué llaves inertes ya se avisó, para
	// avisar UNA vez por documento y no una por slide. Sin esto, un deck que
	// repite `header:`/`footer:` en cada bloque sacaba una decena de avisos
	// idénticos — el corpus escribe esas dos en 17 ejemplos cada una.
	reportedInertKeys map[string]bool
	// frontMatterConsumed marca que quien construyó este parser YA sacó el
	// front matter global y le está pasando solo el cuerpo. Ver
	// newFlexBodyParser.
	frontMatterConsumed bool
}

// NewFlexParser crea un nuevo parser flexible. log==nil degrada a un Noop
// — ver el comentario en NewStrictParser (issue #134/G1c).
func NewFlexParser(input string, log util.Logger) *FlexParser {
	if log == nil {
		log = util.NewNoop()
	}
	lines := strings.Split(input, "\n")
	return &FlexParser{
		input:         input,
		lines:         lines,
		currentLine:   0,
		diagnostics:   make([]diagnostics.Diagnostic, 0),
		logger:        log,
		registry:      elements.GetDefaultRegistry(),
		hasTitleBlock: false,
	}
}

// newFlexBodyParser construye un FlexParser para un cuerpo al que YA se le
// sacó el front matter global — el caso de Parser.Parse, que lo separa con
// FrontMatterParser antes de elegir dialecto.
//
// La distinción importa desde el issue #239. Un bloque de metadata por slide
// (`---\nlayout: stats\n---`) es idéntico, carácter por carácter, a un front
// matter global: los dos son "---", líneas `clave: valor`, "---". Lo único
// que los separa es la posición, y eso solo lo sabe quien llama. Sin esta
// marca, un `layout:` puesto pegado al front matter global caía en el
// parseo de front matter de Parse, se consumía como tal, y el layout se
// perdía — con una línea en blanco de por medio funcionaba y sin ella no,
// una diferencia que la gramática no declara en ningún lado.
//
// Es un constructor aparte y no un parámetro de NewFlexParser porque ese es
// API pública de core (ver core/doc.go): quien lo use con el documento
// completo sigue teniendo el mismo comportamiento de siempre.
func newFlexBodyParser(body string, log util.Logger) *FlexParser {
	p := NewFlexParser(body, log)
	p.frontMatterConsumed = true
	return p
}

// Parse parsea el input y retorna el AST y diagnósticos
func (p *FlexParser) Parse() (*ast.AST, []diagnostics.Diagnostic) {
	pos := diagnostics.NewPosition(1, 1)
	astNode := ast.NewAST(pos)

	// Parse front matter if present. Si quien llamó ya lo sacó, el "---"
	// que abre el cuerpo NO es front matter: es el bloque de metadata del
	// primer slide, y lo resuelve parseContentBlock.
	if !p.frontMatterConsumed &&
		p.currentLine < len(p.lines) && strings.TrimSpace(p.lines[p.currentLine]) == "---" {
		p.parseFrontMatter(astNode)
	}

	// Parse content blocks (bloques de contenido para presentaciones y documentos)
	for p.currentLine < len(p.lines) {
		block := p.parseContentBlock()
		if block != nil {
			astNode.ContentBlocks = append(astNode.ContentBlocks, *block)
		}
	}

	return astNode, p.diagnostics
}

// parseFrontMatter parsea el front matter YAML
func (p *FlexParser) parseFrontMatter(astNode *ast.AST) {
	if p.currentLine >= len(p.lines) {
		return
	}

	pos := diagnostics.NewPosition(p.currentLine+1, 1)
	frontMatter := ast.NewFrontMatterNode(pos)
	p.currentLine++ // skip opening ---

	var content strings.Builder
	for p.currentLine < len(p.lines) {
		line := p.lines[p.currentLine]
		if strings.TrimSpace(line) == "---" {
			p.currentLine++ // skip closing ---
			break
		}
		content.WriteString(line)
		content.WriteString("\n")
		p.currentLine++
	}

	frontMatter.Raw = strings.TrimSuffix(content.String(), "\n")
	astNode.FrontMatter = frontMatter
}

// parseContentBlock parsea un bloque de contenido (slide en presentaciones, sección en documentos)
func (p *FlexParser) parseContentBlock() *ast.ContentBlock {
	// Skip empty lines
	for p.currentLine < len(p.lines) && strings.TrimSpace(p.lines[p.currentLine]) == "" {
		p.currentLine++
	}

	if p.currentLine >= len(p.lines) {
		return nil
	}
	// Check for explicit slide markers
	line := strings.TrimSpace(p.lines[p.currentLine])

	// A bare "---" closed by ANOTHER bare "---" before any heading, with
	// ONLY simple "key: value" metadata lines in between (e.g.
	// "---\nlayout: title\n---"), is a per-slide metadata/layout-override
	// block — not a real slide separator. A real separator is a lone
	// "---" immediately followed by a new "# "/"## " heading (or EOF).
	//
	// Hasta el issue #239 el bloque se consumía entero y se tiraba, así que
	// `layout:` no hacía nada y el modo flex solo podía producir slides
	// `title`/`content` — aunque el linter trae 19 schemas por layout y las
	// plantillas los estilan. Ahora se LEE: `layout` queda pendiente para el
	// bloque siguiente y el resto de las llaves se reportan como inertes.
	if line == "---" {
		if closeIdx := metadataBlockCloseIndex(p.lines, p.currentLine); closeIdx != -1 {
			p.readMetadataBlock(p.currentLine, closeIdx)
			p.currentLine = closeIdx + 1
			return nil
		}
	}

	pos := diagnostics.NewPosition(p.currentLine+1, 1)

	blockType := "content" // Default block type for flex mode
	blockTitle := ""
	blockSubtitle := ""

	// Un `layout:` explícito le gana a la heurística posicional de abajo
	// ("solo el primer # es title"). Es lo que lo hace útil: sin esto no
	// habría forma de que el segundo slide de un deck fuera `stats`, ni de
	// que el primero NO fuera `title`.
	layout := p.pendingLayout
	p.pendingLayout = ""
	if layout != "" {
		blockType = layout
		if isTitleLayout(layout) {
			p.hasTitleBlock = true
		}
	}

	// Check for block type indicators and extract titles
	if strings.HasPrefix(line, "# ") {
		// Solo el primer bloque con # se marca como title, los demás como
		// content — salvo que un `layout:` ya haya decidido el tipo.
		if layout == "" {
			if !p.hasTitleBlock {
				blockType = "title"
				p.hasTitleBlock = true
			} else {
				blockType = "content"
			}
		}
		blockTitle = strings.TrimSpace(line[2:]) // Extract title text
		p.currentLine++                          // consume the title line

		// Check if the next line is a subtitle (## immediately after #,
		// zero blank lines — same strict adjacency as before). A more
		// lenient, blank-line-tolerant rescue for "content" blocks that
		// would otherwise end up with zero elements is applied below,
		// after the element loop, instead of here: doing it here as a
		// blanket rule also misfires on a deck's opening "title" block
		// (which legitimately has zero elements by design), silently
		// swallowing what was meant to be that deck's separate first
		// content slide.
		if p.currentLine < len(p.lines) {
			nextLine := strings.TrimSpace(p.lines[p.currentLine])
			if strings.HasPrefix(nextLine, "## ") {
				blockSubtitle = strings.TrimSpace(nextLine[3:]) // Extract subtitle text
				p.currentLine++                                 // consume the subtitle line
			}
		}
	} else if strings.HasPrefix(line, "## ") {
		if layout == "" {
			blockType = "content" // Map ## to content type for template compatibility
		}
		blockTitle = strings.TrimSpace(line[3:]) // Extract section title text
		p.currentLine++                          // consume the section line
	}

	block := ast.NewContentBlock(pos, blockType)

	// Set the title if we extracted one.
	//
	// A qué campo va depende del TIPO ya resuelto, no de la forma del
	// heading: un slide de título se titula con Heading (es lo que la
	// plantilla y el schema `title` esperan) y cualquier otro con Title. Con
	// `layout: title` sobre un "## Foo", eso manda el texto a Heading —
	// antes habría ido a Title y el slide habría quedado sin heading.
	if blockTitle != "" {
		if isTitleLayout(blockType) {
			block.Heading = blockTitle
		} else {
			block.Title = blockTitle
		}
	}

	// Set the subtitle if we extracted one
	if blockSubtitle != "" {
		block.Subtitle = blockSubtitle
	}

	// Parse block elements using the registry
	ctx := &elements.ParseContext{
		Mode:        "flex",
		CurrentLine: p.currentLine,
		Logger:      p.logger,
		Lines:       p.lines,
	}
	for p.currentLine < len(p.lines) {
		// Check for next slide boundary
		nextLine := strings.TrimSpace(p.lines[p.currentLine])

		// Always break on a new "# " heading — that always starts a new
		// deck-level slide, unambiguously.
		if strings.HasPrefix(nextLine, "# ") {
			break
		}

		// A "## " normally starts a new content block too. But if THIS
		// block would otherwise be invalid — a "content" block with a
		// title and zero elements so far (the linter's "Content slides
		// must have at least one element" rule) — absorb it as this
		// block's subtitle instead, so "# Title\n\n## Subtitle\n\ncontent"
		// (blank lines in between are already skipped above, tolerating
		// that common formatting choice) still produces one valid slide.
		// Once this block already has elements or a subtitle, "## " goes
		// back to unambiguously starting a new block — this only rescues
		// the specific otherwise-broken shape, not "# "/"## " pairs in
		// general (which would also misfire on the deck's own opening
		// "title" block, since that legitimately has zero elements by
		// design and its "## " is meant to start a separate first slide).
		if strings.HasPrefix(nextLine, "## ") {
			if blockType == "content" && block.Title != "" && len(block.Elements) == 0 && block.Subtitle == "" {
				block.Subtitle = strings.TrimSpace(nextLine[3:])
				p.currentLine++
				continue
			}
			break
		}

		// Stop at slide separators (always skip ---, don't include as
		// content) — UNLESS this "---" actually opens a per-slide
		// metadata block (see metadataBlockCloseIndex): in that case,
		// leave it unconsumed so the next parseContentBlock call sees it
		// starting fresh and its own metadata-block check (above) can
		// recognize and skip the whole block, instead of this loop
		// eating the opening "---" and leaving "key: value" behind to be
		// misparsed as a stray body text element on the next call.
		if nextLine == "---" {
			if metadataBlockCloseIndex(p.lines, p.currentLine) != -1 {
				break
			}
			p.currentLine++
			break
		}

		// Skip empty lines
		if nextLine == "" {
			p.currentLine++
			continue
		}

		// `###`..`######` es un encabezado de SUBSECCIÓN dentro del slide
		// (issue #194). `# ` y `## ` no llegan acá: los intercepta el
		// bloque de arriba, donde son frontera de slide y subtítulo
		// respectivamente — por eso el predicado empieza en 3 y no copia
		// tal cual el isSubsectionHeader de DocumentFlexParser, que sí
		// cuenta `##` (en un documento no hay slides que delimitar).
		//
		// Va DESPUÉS del salto de líneas en blanco y ANTES de
		// registry.Parse: si el registry corre primero, TextParser reclama
		// la línea como párrafo y el heading se pierde. Un `###` dentro de
		// un fence o de un `:::` no pasa por acá — CodeParser y
		// SpecialBlockParser consumen el bloque entero de una sola vez.
		if level := flexSubsectionLevel(nextLine); level > 0 {
			block.Elements = append(block.Elements,
				buildHeadingElement(strings.TrimSpace(nextLine[level:]), level, p.currentLine, ""))
			p.currentLine++
			continue
		}

		// Update context
		ctx.CurrentLine = p.currentLine

		// Try to parse element using registry
		result := p.registry.Parse(ctx, p.currentLine)
		if result.Element != nil {
			block.Elements = append(block.Elements, result.Element)
			p.currentLine += result.ConsumedLines
		} else if result.ConsumedLines > 0 {
			// Even if no element was created, advance if lines were consumed
			p.currentLine += result.ConsumedLines
		} else {
			// Failsafe: advance at least one line to prevent infinite loops.
			// issue #192: antes esto avanzaba sin emitir nada — la línea
			// desaparecía en silencio, --lint-only reportaba éxito. Capturar
			// el índice ANTES de p.currentLine++ (el warning debe apuntar a
			// la línea descartada, no a la siguiente).
			droppedLine := p.currentLine
			p.currentLine++
			if !isFlexFailsafeExempt(nextLine) {
				p.addWarningAtWithRuleID(droppedLine,
					fmt.Sprintf("Unrecognized line, content was discarded: %q. Check DSL Flex syntax documentation.", nextLine),
					"FLEX001")
			}
		}

		// Handle errors
		if result.Error != nil {
			p.addError(result.Error.Error())
		}

		// Propagar diagnósticos no-fatales del ElementParser (p. ej. CHART002)
		// tal cual, sin pasar por addError (que siempre fuerza severidad Error).
		p.diagnostics = append(p.diagnostics, result.Diagnostics...)
	}

	// Un bloque se emite si tiene ALGO: elementos, o un título con el que
	// presentarse.
	//
	// Antes la condición era un allowlist de tipos —"title", "section", o
	// "content" con título— y eso alcanzaba mientras flex solo sabía
	// producir "title" y "content". Con `layout:` (issue #239) cualquier
	// nombre de layout llega hasta acá, así que un slide `layout: stats` con
	// su heading y sin elementos todavía —el estado normal de un deck a
	// medio escribir— caía al `return nil` y DESAPARECÍA sin diagnóstico. Es
	// la misma clase de pérdida silenciosa que el resto de este lote cierra,
	// destapada por la función nueva.
	//
	// Las dos condiciones de tipo se conservan por compatibilidad: un bloque
	// "title"/"section" sin título ni elementos seguía emitiéndose antes, y
	// el linter ya lo reporta con SLIDE002/PARSE001. Quitarlas sería un
	// cambio aparte, sin relación con este.
	if len(block.Elements) > 0 || block.Title != "" || block.Heading != "" ||
		blockType == "title" || blockType == "section" {
		return block
	}

	return nil
}

// addError añade un error diagnóstico
func (p *FlexParser) addError(msg string) {
	pos := diagnostics.NewPosition(p.currentLine+1, 1)
	diag := diagnostics.NewError(msg, pos, "flex-parser")
	p.diagnostics = append(p.diagnostics, diag)
}

// addWarningAtWithRuleID añade un warning diagnóstico anclado a lineIndex
// (0-based) con un RuleID adjunto — mismo patrón que strictBody.addWarningWithRuleID
// (strict.go), adaptado para recibir el índice explícito: a diferencia de
// strict, el failsafe de este parser llama a esto DESPUÉS de p.currentLine++
// (issue #192), así que no puede apoyarse en p.currentLine como strict sí
// hace.
func (p *FlexParser) addWarningAtWithRuleID(lineIndex int, msg, ruleID string) {
	pos := diagnostics.NewPosition(lineIndex+1, 1)
	p.diagnostics = append(p.diagnostics,
		diagnostics.NewWarning(msg, pos, "flex-parser").WithRuleID(ruleID))
}

// readMetadataBlock lee el bloque de metadata por slide que abre en openIdx
// y cierra en closeIdx (los dos "---"), y guarda su `layout:` para el bloque
// que sigue (issue #239).
//
// El nombre del layout NO se valida contra la lista de schemas: eso vive en
// el linter, y el parser no puede importarlo sin invertir la dirección de
// dependencias. Acá solo se exige la forma de un identificador; de un nombre
// desconocido se encarga LAYOUT_UNKNOWN, que es donde está la lista.
//
// Las demás llaves se reportan como inertes con severidad Info, no Warning:
// el corpus las escribe a montones (`header:`/`footer:` en 17 ejemplos cada
// una, `rating`/`position`/`avatar` en 8) y subirlas a Warning llenaría de
// ruido decks que hoy compilan limpios. Que sean visibles alcanza para que
// alguien note que no hacen nada.
func (p *FlexParser) readMetadataBlock(openIdx, closeIdx int) {
	for i := openIdx + 1; i < closeIdx; i++ {
		trimmed := strings.TrimSpace(p.lines[i])
		if trimmed == "" {
			continue
		}

		sep := strings.Index(trimmed, ":")
		if sep <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:sep])
		value := strings.Trim(strings.TrimSpace(trimmed[sep+1:]), `"'`)

		if key != "layout" {
			if p.reportedInertKeys == nil {
				p.reportedInertKeys = make(map[string]bool)
			}
			if !p.reportedInertKeys[key] {
				p.reportedInertKeys[key] = true
				p.diagnostics = append(p.diagnostics,
					diagnostics.NewInfo(
						fmt.Sprintf("Per-slide metadata key %q has no effect; only 'layout' is read here.", key),
						diagnostics.NewPosition(i+1, 1), "flex-parser").WithRuleID("FLEX002"))
			}
			continue
		}

		value = strings.ToLower(value)
		if !isLayoutName(value) {
			p.addWarningAtWithRuleID(i,
				fmt.Sprintf("Invalid layout name %q; expected an identifier like 'stats' or 'comparison' — ignored.", value),
				"FLEX003")
			continue
		}
		p.pendingLayout = value
	}
}

// isLayoutName exige la forma de un identificador de layout: minúscula
// inicial, después letras, dígitos, "_" o "-". No dice nada sobre si el
// nombre existe — de eso se ocupa el linter, que es quien tiene la lista.
func isLayoutName(value string) bool {
	if value == "" {
		return false
	}
	if value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

// isTitleLayout reporta si un layout se titula con `heading` en vez de con
// `title`. Son los dos únicos que el linter declara con `heading` en
// RequiredProperties (linter.GetSlideLayoutSchemas: "title" y "title_slide"),
// y core no puede consultar esa lista sin importar el linter desde el
// parser, así que se repite acá — corta, y con el puntero para que se
// mantengan sincronizadas.
//
// flexSubsectionLevel devuelve el nivel (3 a 6) si line es un encabezado de
// subsección Markdown, y 0 si no lo es (issue #194).
//
// Empieza en 3 a propósito: en un slide, `# ` abre un slide nuevo y `## ` es
// frontera o subtítulo, y las dos las resuelve el loop de parseContentBlock
// antes de llegar acá. Termina en 6 porque Markdown tampoco reconoce más:
// `####### x` (siete) es un párrafo, y como tal cae al registry igual que
// hoy.
//
// Exige espacio o tab después de los `#` y algo de texto después: `###` a
// secas, o `###foo`, no son encabezados — el primero es una línea suelta y
// el segundo bien podría ser un hashtag o un ancla escrita a mano. En los
// dos casos la línea sigue su curso hacia el registry.
func flexSubsectionLevel(line string) int {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level < 3 || level > 6 || level >= len(line) {
		return 0
	}
	if line[level] != ' ' && line[level] != '\t' {
		return 0
	}
	if strings.TrimSpace(line[level:]) == "" {
		return 0
	}
	return level
}

// slidelang tiene su propia versión más ancha (config.IsSlideTitle, que suma
// "cover" e "intro" para elegir plantilla). No se comparte porque core no
// puede importar slidelang; la diferencia es deliberada: acá la pregunta es
// "¿qué campo del AST se llena?" y allá "¿qué plantilla se usa?".
func isTitleLayout(layout string) bool {
	return layout == "title" || layout == "title_slide"
}

// metadataBlockCloseIndex checks whether lines[openIdx] == "---" opens a
// per-slide metadata/layout-override block (e.g. "---\nlayout: title\n---")
// — every line up to the next bare "---" must look like simple "key:
// value" metadata (isMetadataLine), and no "# "/"## " heading may appear
// first. Returns the closing "---" line's index, or -1 if this isn't a
// metadata block (e.g. a real separator, or ordinary prose in between —
// requiring every line to look like metadata, not just "no heading
// before the next ---", matters: without it, two ordinary "---"
// separators with a real paragraph in between would be misread as a
// metadata block and that paragraph would be silently discarded).
func metadataBlockCloseIndex(lines []string, openIdx int) int {
	for i := openIdx + 1; i < len(lines); i++ {
		probe := strings.TrimSpace(lines[i])
		if strings.HasPrefix(probe, "# ") || strings.HasPrefix(probe, "## ") {
			return -1
		}
		if probe == "---" {
			return i
		}
		if !isMetadataLine(probe) {
			return -1
		}
	}
	return -1
}

// isMetadataLine indica si una línea (ya trimmed) tiene forma de "key:
// value" simple — un identificador (letras/números/_/-) inmediatamente
// seguido de ":", sin espacios antes del ":". Prosa normal casi nunca
// calza este patrón exacto (tiene espacios/puntuación antes del primer
// ":", o no tiene ":" en absoluto), lo cual es la señal que se usa para
// distinguir un bloque de metadata real de contenido de slide legítimo.
func isMetadataLine(line string) bool {
	if line == "" {
		return true // blank lines are fine within a metadata block
	}
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return false
	}
	key := line[:idx]
	for _, r := range key {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !isAlnum && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

// isFlexFailsafeExempt reporta si trimmed es un residuo de cierre
// arquitectónico conocido que el failsafe de FLEX001 no debe advertir —
// compartido entre FlexParser (flex.go) y DocumentFlexParser
// (document_flex.go), issue #192, paso (c): la lista se derivó corriendo
// el registro completo sobre examples/ y tabulando cada FLEX001 por forma
// de línea, no adivinando.
//
//   - "<<end>>": el cierre genérico de bloques GRID/COLUMN/chart/map
//     (formatChart/formatMap en core/formatter/strict.go lo emiten así).
//     TestTextParser_Parse_OrphanEndTagStillDropped y
//     TestFormatDocument_RoundTrip_Corpus fijan este descarte como
//     contrato — NO se puede tocar.
//   - ":::" (bare, sin tipo): el cierre de un sub-bloque anidado (columna
//     dentro de grid, tab dentro de tabs) comparte la misma sintaxis ":::"
//     que el cierre del bloque padre (ver el comentario de
//     IsJustASeparator en internal/elements/common.go, issue #57). Cuando
//     un bloque padre se corta temprano porque un hijo abre su propio
//     ":::tipo" (SpecialBlockParser: "Don't consume this line, let the
//     next parser handle it"), el ":::" que en verdad cerraba al padre
//     queda huérfano una vez que el hijo consume el suyo — confirmado
//     recorriendo examples/: SIEMPRE es el segundo de dos ":::" bare
//     consecutivos, nunca uno aislado sin un bloque anidado por delante.
func isFlexFailsafeExempt(trimmed string) bool {
	return trimmed == "<<end>>" || trimmed == ":::"
}
