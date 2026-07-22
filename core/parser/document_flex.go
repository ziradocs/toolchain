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
	"go.ziradocs.com/core/v2/renderer"
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
	processedContent, report := normalize.ProcessWithDetection(input, detectionResult, log)

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

	// Use the proper FrontMatterParser to parse all YAML fields including Theme
	fmParser := &FrontMatterParser{}
	frontMatter, remainingContent, fmDiagnostics := fmParser.Parse(p.input)

	// Append frontmatter diagnostics to our diagnostics
	p.diagnostics = append(p.diagnostics, fmDiagnostics...)

	// Set the parsed frontmatter in the AST
	astNode.FrontMatter = frontMatter

	// Update our state to use the remaining content (without frontmatter)
	// This ensures p.lines and p.currentLine are aligned
	p.lines = strings.Split(remainingContent, "\n")
	p.currentLine = 0
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

		// Failsafe: advance at least one line if nothing was parsed
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

// parseSubsectionHeader convierte ##, ###, etc. en TextElement con HTML
func (p *DocumentFlexParser) parseSubsectionHeader(line string) ast.Element {
	// Contar cuántos # tiene
	level := 0
	for i := 0; i < len(line) && line[i] == '#'; i++ {
		level++
	}

	// Limitar a H6 como máximo
	if level > 6 {
		level = 6
	}

	// Extraer el texto del header
	text := strings.TrimSpace(line[level:])

	// Procesar Markdown inline básico (**, *, `, etc.) de forma segura:
	// escapa el HTML del texto y aplica formatos con un procesador de un
	// solo paso (RE2, lineal) — evita el DoS de bucle infinito del antiguo
	// scanner hand-written y cierra el XSS de subsección en el mismo golpe
	// (ver docs/SECURITY_AUDIT_2026-07.md, AL-3 y CR-2).
	// Se usa la variante "Line" (no la ProcessInlineMarkdownSecure genérica):
	// un header es una sola línea y nunca debe interpretarse como lista
	// ("- Foo" no debe volverse <ul><li>Foo</li></ul> dentro de un <h3>).
	processedText := renderer.ProcessInlineMarkdownSecureLine(text)

	// Generar ID para el anchor (mismo algoritmo que en el renderer)
	// Usar el texto original (sin HTML) para el anchor
	anchor := strings.ToLower(strings.ReplaceAll(text, " ", "-"))
	anchor = p.sanitizeAnchor(anchor)

	// Crear elemento de texto con HTML incluyendo el id
	pos := diagnostics.NewPosition(p.currentLine+1, 1)
	htmlContent := fmt.Sprintf("<h%d id=\"%s\">%s</h%d>", level, anchor, processedText, level)

	return ast.NewRawHTMLTextElement(pos, htmlContent)
}

// sanitizeAnchor limpia un anchor para usarlo en href/id
func (p *DocumentFlexParser) sanitizeAnchor(anchor string) string {
	anchor = strings.ReplaceAll(anchor, ".", "")
	anchor = strings.ReplaceAll(anchor, ",", "")
	anchor = strings.ReplaceAll(anchor, ":", "")
	anchor = strings.ReplaceAll(anchor, ";", "")
	anchor = strings.ReplaceAll(anchor, "!", "")
	anchor = strings.ReplaceAll(anchor, "?", "")
	anchor = strings.ReplaceAll(anchor, "(", "")
	anchor = strings.ReplaceAll(anchor, ")", "")
	anchor = strings.ReplaceAll(anchor, "[", "")
	anchor = strings.ReplaceAll(anchor, "]", "")
	anchor = strings.ReplaceAll(anchor, "{", "")
	anchor = strings.ReplaceAll(anchor, "}", "")
	anchor = strings.ReplaceAll(anchor, "/", "")
	anchor = strings.ReplaceAll(anchor, "\\", "")
	anchor = strings.ReplaceAll(anchor, "'", "")
	anchor = strings.ReplaceAll(anchor, "\"", "")
	anchor = strings.ReplaceAll(anchor, "`", "")
	// Eliminar emojis y caracteres especiales (mantener solo letras, números, guiones)
	var cleaned strings.Builder
	for _, r := range anchor {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			cleaned.WriteRune(r)
		}
	}
	return cleaned.String()
}

// addError añade un error diagnóstico
func (p *DocumentFlexParser) addError(msg string) {
	pos := diagnostics.NewPosition(p.currentLine+1, 1)
	diag := diagnostics.NewError(msg, pos, "document-flex-parser")
	p.diagnostics = append(p.diagnostics, diag)
}
