// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/elements"
	"go.ziradocs.com/core/v2/internal/normalize"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
	"go.ziradocs.com/core/v2/util"
)

// DocumentFlexParser parsea documentos Markdown puros con jerarquía correcta
// Diferencia clave con FlexParser: `##` y `###` NO crean nuevos slides,
// sino que se convierten en elementos <h2> y <h3> dentro del slide actual
type DocumentFlexParser struct {
	input         string
	originalInput string // Input original antes de normalización
	lines         []string
	currentLine   int
	diagnostics   []diagnostics.Diagnostic
	logger        util.Logger
	registry      *elements.Registry
	hasTitleBlock bool
	normalized    bool // Indica si el contenido fue normalizado
	inCodeBlock   bool // Track si estamos dentro de un code block
}

// NewDocumentFlexParser crea un nuevo parser para documentos
func NewDocumentFlexParser(input string, log util.Logger) *DocumentFlexParser {
	lines := strings.Split(input, "\n")
	return &DocumentFlexParser{
		input:         input,
		originalInput: input,
		lines:         lines,
		currentLine:   0,
		diagnostics:   make([]diagnostics.Diagnostic, 0),
		logger:        log,
		registry:      elements.GetDefaultRegistry(),
		hasTitleBlock: false,
		normalized:    false,
	}
}

// NewDocumentFlexParserWithNormalization crea un parser y aplica normalización AI
func NewDocumentFlexParserWithNormalization(input string, log util.Logger) *DocumentFlexParser {
	// Detectar si el contenido parece ser generado por IA
	detector := normalizer.NewDetector()
	detectionResult := detector.Detect(input)

	// Logging de detección
	if detectionResult.Detected {
		log.Info("NORMALIZE", "🔍 Detectado contenido AI (score: %.2f, %d patrones)",
			detectionResult.Score, len(detectionResult.Patterns))

		// Detalles de los patrones solo en modo debug
		for i, pattern := range detectionResult.Patterns {
			log.Debug("NORMALIZE", "  [%d] %s (confianza: %.2f, línea: %d): %s",
				i+1, pattern.Type, pattern.Confidence, pattern.Line, pattern.Description)
		}
	}

	// Normalizar el contenido usando la API del factory
	processedContent, report := normalize.ProcessWithDetection(input, detectionResult, base.DialectDocuments, log)

	wasModified := false
	if report.WasModified {
		wasModified = true
		input = processedContent // Usar el contenido normalizado

		// Información de normalización
		rulesApplied := len(report.GetTransformationsApplied())
		changeBytes := len(processedContent) - len(input)
		log.Info("NORMALIZE", "Normalización aplicada → %d reglas, %+d bytes", rulesApplied, changeBytes)

		// Detalles de las reglas aplicadas solo en modo debug
		for i, rule := range report.GetTransformationsApplied() {
			log.Debug("NORMALIZE", "  [%d] %s", i+1, rule)
		}
	}

	// Crear el parser con el contenido normalizado
	parser := NewDocumentFlexParser(input, log)
	parser.normalized = wasModified

	return parser
}

// Parse parsea el input y retorna el AST y diagnósticos
func (p *DocumentFlexParser) Parse() (*ast.AST, []diagnostics.Diagnostic) {
	pos := diagnostics.NewPosition(1, 1)
	astNode := ast.NewAST(pos)

	// Parse front matter if present
	if p.currentLine < len(p.lines) && strings.TrimSpace(p.lines[p.currentLine]) == "---" {
		p.parseFrontMatter(astNode)
	}

	// Parse document sections (content blocks in AST terms)
	for p.currentLine < len(p.lines) {
		block := p.parseSection()
		if block != nil {
			astNode.ContentBlocks = append(astNode.ContentBlocks, *block)
		}
	}

	return astNode, p.diagnostics
}

// parseFrontMatter parsea el front matter YAML usando FrontMatterParser
func (p *DocumentFlexParser) parseFrontMatter(astNode *ast.AST) {
	if p.currentLine >= len(p.lines) {
		return
	}

	lines, fmDiagnostics := parseDocumentFrontMatter(p.input, astNode)
	p.diagnostics = append(p.diagnostics, fmDiagnostics...)
	p.lines = lines
	p.currentLine = 0
}

// parseDocumentFrontMatter parsea el frontmatter de input, lo cuelga de
// astNode y devuelve las líneas del CUERPO (ya sin frontmatter) junto con
// sus diagnósticos. Compartida por los dos dialectos documentales para que
// ambos traten el frontmatter idéntico: mismas validaciones (FRONT001/002)
// y, sobre todo, mismo re-encuadre de las líneas, que es lo que hace que
// las posiciones de los diagnósticos del cuerpo sean relativas al cuerpo.
func parseDocumentFrontMatter(input string, astNode *ast.AST) ([]string, []diagnostics.Diagnostic) {
	// Use the proper FrontMatterParser to parse all YAML fields including Theme
	fmParser := &FrontMatterParser{}
	frontMatter, remainingContent, fmDiagnostics := fmParser.Parse(input)

	// Set the parsed frontmatter in the AST
	astNode.FrontMatter = frontMatter

	return strings.Split(remainingContent, "\n"), fmDiagnostics
}

// parseSection parsea una sección del documento (equivalente a un "slide" en el AST)
// SOLO `#` crea nuevas secciones. `##` y `###` son subsecciones dentro de la sección.
func (p *DocumentFlexParser) parseSection() *ast.ContentBlock {
	// Skip empty lines
	for p.currentLine < len(p.lines) && strings.TrimSpace(p.lines[p.currentLine]) == "" {
		p.currentLine++
	}

	if p.currentLine >= len(p.lines) {
		return nil
	}

	line := strings.TrimSpace(p.lines[p.currentLine])
	pos := diagnostics.NewPosition(p.currentLine+1, 1)

	// SOLO `#` crea una nueva sección (content block en el AST)
	if !strings.HasPrefix(line, "# ") {
		// Si no hay `#`, esta línea no inicia una sección válida
		// Avanzar para evitar loops infinitos
		p.currentLine++
		return nil
	}

	// Primer H1 se marca como title, los demás como content
	blockType := "content"
	if !p.hasTitleBlock {
		blockType = "title"
		p.hasTitleBlock = true
	}

	blockTitle := strings.TrimSpace(line[2:])
	p.currentLine++

	block := ast.NewContentBlock(pos, blockType)

	// Set the title
	if blockTitle != "" {
		if blockType == "title" {
			block.Heading = blockTitle
		} else {
			block.Title = blockTitle
		}
	}

	// Parse section content
	// TODO: `##` y `###` se convierten en elementos dentro del bloque
	p.parseSectionContent(block)

	// Only return section if it has content or is a title
	if len(block.Elements) > 0 || blockType == "title" {
		return block
	}

	return nil
}

// parseSectionContent parsea el contenido de una sección
// Aquí `##` y `###` se convierten en TextElements con HTML
func (p *DocumentFlexParser) parseSectionContent(block *ast.ContentBlock) {
	ctx := &elements.ParseContext{
		Mode:        "flex",
		CurrentLine: p.currentLine,
		Logger:      p.logger,
		Lines:       p.lines,
	}

	for p.currentLine < len(p.lines) {
		if p.currentLine >= len(p.lines) {
			break
		}

		line := p.lines[p.currentLine]
		trimmed := strings.TrimSpace(line)

		// Stop at next section (only `#`, not `##` or `###`)
		// Check: starts with "# " but NOT with "## " (to exclude ##, ###, etc.)
		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") && len(block.Elements) > 0 {
			break
		}

		// Handle horizontal rules (---)
		// In documents, --- should be IGNORED (they're section separators)
		// They don't need to be rendered as <hr> elements
		if trimmed == "---" {
			p.currentLine++
			continue
		}

		// Skip empty lines
		if trimmed == "" {
			p.currentLine++
			continue
		}

		// Update context
		ctx.CurrentLine = p.currentLine

		// Check for subsection headers FIRST (before registry)
		// But skip if we're inside a code block
		if p.isSubsectionHeader(trimmed) && !p.inCodeBlock {
			element := p.parseSubsectionHeader(trimmed)
			if element != nil {
				block.Elements = append(block.Elements, element)
			}
			p.currentLine++
			continue
		}

		// Try to parse element using registry
		// This handles code blocks, text, tables, etc.
		result := p.registry.Parse(ctx, p.currentLine)
		if result.Element != nil {
			block.Elements = append(block.Elements, result.Element)
			p.currentLine += result.ConsumedLines
			// Track code blocks AFTER consuming lines from registry
			// Check each consumed line for ```
			for i := 0; i < result.ConsumedLines; i++ {
				if p.currentLine-result.ConsumedLines+i < len(p.lines) {
					line := strings.TrimSpace(p.lines[p.currentLine-result.ConsumedLines+i])
					if strings.HasPrefix(line, "```") {
						p.inCodeBlock = !p.inCodeBlock
					}
				}
			}
			// Handle errors
			if result.Error != nil {
				p.addError(result.Error.Error())
			}
			p.diagnostics = append(p.diagnostics, result.Diagnostics...)
			continue
		} else if result.ConsumedLines > 0 {
			p.currentLine += result.ConsumedLines
			// Track code blocks for consumed lines
			for i := 0; i < result.ConsumedLines; i++ {
				if p.currentLine-result.ConsumedLines+i < len(p.lines) {
					line := strings.TrimSpace(p.lines[p.currentLine-result.ConsumedLines+i])
					if strings.HasPrefix(line, "```") {
						p.inCodeBlock = !p.inCodeBlock
					}
				}
			}
			// Handle errors
			if result.Error != nil {
				p.addError(result.Error.Error())
			}
			p.diagnostics = append(p.diagnostics, result.Diagnostics...)
			continue
		}

		// Track code blocks for current line
		if strings.HasPrefix(trimmed, "```") {
			p.inCodeBlock = !p.inCodeBlock
		}

		// Failsafe: advance at least one line if nothing was parsed.
		// issue #192: antes esto avanzaba sin emitir nada — a diferencia de
		// flex.go, acá las dos ramas exitosas hacen `continue`, así que este
		// camino ni siquiera miraba result.Error/result.Diagnostics: era el
		// más silencioso de los dos failsafes. p.currentLine todavía apunta
		// a la línea culpable en este punto (el warning se ancla ANTES del
		// ++, a diferencia de flex.go donde había que capturar el índice).
		if !isFlexFailsafeExempt(trimmed) {
			p.addWarningWithRuleID(
				fmt.Sprintf("Unrecognized line, content was discarded: %q. Check DSL Flex syntax documentation.", trimmed),
				"FLEX001")
		}
		p.currentLine++
	}
}

// isSubsectionHeader detecta si una línea es un header de subsección (##, ###, etc.)
func (p *DocumentFlexParser) isSubsectionHeader(line string) bool {
	// Detectar ##, ###, ####, etc. pero NO #
	if len(line) < 3 {
		return false
	}
	if line[0] != '#' || line[1] != '#' {
		return false
	}
	// Verificar que después de los # hay un espacio
	for i := 2; i < len(line); i++ {
		if line[i] == ' ' {
			return true
		}
		if line[i] != '#' {
			return false
		}
	}
	return false
}

// parseSubsectionHeader convierte ##, ###, etc. en TextElement con HTML.
// El armado del elemento vive en buildHeadingElement, compartido con el
// dialecto strict; acá solo se cuenta el nivel a partir de los `#`.
func (p *DocumentFlexParser) parseSubsectionHeader(line string) ast.Element {
	// Contar cuántos # tiene
	level := 0
	for i := 0; i < len(line) && line[i] == '#'; i++ {
		level++
	}

	// Extraer el texto del header
	text := strings.TrimSpace(line[level:])

	// En flex el anchor siempre se deriva del texto: no hay sintaxis para
	// declarar un id (a diferencia de `SECTION … / id:` en strict).
	return buildHeadingElement(text, level, p.currentLine, "")
}

// addError añade un error diagnóstico
func (p *DocumentFlexParser) addError(msg string) {
	pos := diagnostics.NewPosition(p.currentLine+1, 1)
	diag := diagnostics.NewError(msg, pos, "document-flex-parser")
	p.diagnostics = append(p.diagnostics, diag)
}

// addWarningWithRuleID añade un warning diagnóstico anclado a p.currentLine
// con un RuleID adjunto — mismo patrón que strictBody.addWarningWithRuleID
// (strict.go:602-606).
func (p *DocumentFlexParser) addWarningWithRuleID(msg, ruleID string) {
	pos := diagnostics.NewPosition(p.currentLine+1, 1)
	p.diagnostics = append(p.diagnostics,
		diagnostics.NewWarning(msg, pos, "document-flex-parser").WithRuleID(ruleID))
}
