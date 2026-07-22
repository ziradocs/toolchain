// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
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

// Parse parsea el input y retorna el AST y diagnósticos
func (p *FlexParser) Parse() (*ast.AST, []diagnostics.Diagnostic) {
	pos := diagnostics.NewPosition(1, 1)
	astNode := ast.NewAST(pos)

	// Parse front matter if present
	if p.currentLine < len(p.lines) && strings.TrimSpace(p.lines[p.currentLine]) == "---" {
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
	// This DSL shape has no parser support today (tracked separately);
	// consume it as inert metadata rather than let its lines leak into
	// the next slide as orphaned/misattributed content.
	if line == "---" {
		if closeIdx := metadataBlockCloseIndex(p.lines, p.currentLine); closeIdx != -1 {
			p.currentLine = closeIdx + 1
			return nil
		}
	}

	pos := diagnostics.NewPosition(p.currentLine+1, 1)

	blockType := "content" // Default block type for flex mode
	blockTitle := ""
	blockSubtitle := ""
	// Check for block type indicators and extract titles
	if strings.HasPrefix(line, "# ") {
		// Solo el primer bloque con # se marca como title, los demás como content
		if !p.hasTitleBlock {
			blockType = "title"
			p.hasTitleBlock = true
		} else {
			blockType = "content"
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
		blockType = "content"                    // Map ## to content type for template compatibility
		blockTitle = strings.TrimSpace(line[3:]) // Extract section title text
		p.currentLine++                          // consume the section line
	}

	block := ast.NewContentBlock(pos, blockType)

	// Set the title if we extracted one
	if blockTitle != "" {
		if blockType == "title" {
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
			// Failsafe: advance at least one line to prevent infinite loops
			p.currentLine++
		}

		// Handle errors
		if result.Error != nil {
			p.addError(result.Error.Error())
		}

		// Propagar diagnósticos no-fatales del ElementParser (p. ej. CHART002)
		// tal cual, sin pasar por addError (que siempre fuerza severidad Error).
		p.diagnostics = append(p.diagnostics, result.Diagnostics...)
	}

	// Only return block if it has elements or is a title/section/content block with a title
	if len(block.Elements) > 0 || blockType == "title" || blockType == "section" || (blockType == "content" && block.Title != "") {
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
