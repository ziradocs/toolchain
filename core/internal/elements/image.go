// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// ImageParser maneja elementos de imagen
type ImageParser struct{}

// CanParse determina si una línea es el inicio de una imagen
func (p *ImageParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// Strict mode: IMAGE keyword
	if mode == "strict" && strings.HasPrefix(trimmed, "IMAGE ") {
		return true
	}

	// Flex mode: Markdown image syntax
	if mode == "flex" && strings.HasPrefix(trimmed, "![") && strings.Contains(trimmed, "](") {
		return true
	}

	return false
}

// Parse parsea una imagen desde las líneas proporcionadas
func (p *ImageParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	line := strings.TrimSpace(ctx.Lines[startIndex])
	consumed := 1

	var source, alt, caption, label string

	if ctx.Mode == "strict" {
		// Parse strict mode IMAGE syntax
		source, alt, caption, label, consumed = p.parseStrictImage(ctx.Lines, startIndex)
	} else {
		// Parse flex mode Markdown syntax
		source, alt = p.parseMarkdownImage(line)
	}

	// Detectar contexto automáticamente
	context := p.detectImageContext(ctx, startIndex)

	// Crear elemento con contexto
	element := ast.NewImageElementWithContext(pos, source, alt, context)
	element.Caption = caption
	element.Label = label

	return &ParseResult{
		Element:       element,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// detectImageContext determina el contexto de una imagen basado en su posición y entorno
func (p *ImageParser) detectImageContext(ctx *ParseContext, startIndex int) ast.ImageContext {
	// Contar líneas no vacías desde el inicio del slide hasta esta imagen
	nonEmptyLines := 0
	slideStart := p.findSlideStart(ctx, startIndex)

	for i := slideStart; i < startIndex; i++ {
		if strings.TrimSpace(ctx.Lines[i]) != "" {
			nonEmptyLines++
		}
	}

	// Imagen en galería (múltiples imágenes consecutivas) - PRIORIDAD ALTA
	if p.hasMultipleImagesNearby(ctx, startIndex) {
		return ast.ImageContextGallery
	}

	// Detectar si estamos en un slide de título principal
	if p.isTitleSlide(ctx, slideStart) {
		return ast.ImageContextTitle
	}

	// Imagen de hero (primeras líneas del slide, típicamente para slides de contenido)
	if nonEmptyLines <= 2 {
		return ast.ImageContextHero
	}

	// Imagen con caption (texto descriptivo después)
	if p.hasTextAfter(ctx, startIndex) {
		return ast.ImageContextContent
	}

	return ast.ImageContextStandalone
}

// findSlideStart encuentra el inicio del slide actual
func (p *ImageParser) findSlideStart(ctx *ParseContext, currentIndex int) int {
	// Buscar hacia atrás hasta encontrar un separador de slide (---) o el inicio del archivo
	for i := currentIndex - 1; i >= 0; i-- {
		line := strings.TrimSpace(ctx.Lines[i])
		if line == "---" {
			return i + 1 // Línea después del separador
		}
	}
	return 0 // Inicio del archivo
}

// isTitleSlide detecta si el slide actual es un slide de título principal
func (p *ImageParser) isTitleSlide(ctx *ParseContext, slideStart int) bool {
	// Buscar patrones típicos de slides de título:
	// 1. Encabezado H1 seguido de H2
	// 2. Es el primer slide después del frontmatter
	// 3. Contiene principalmente título y subtítulo

	foundH1 := false
	foundH2 := false
	contentLines := 0

	// Examinar las primeras líneas del slide
	for i := slideStart; i < len(ctx.Lines) && i < slideStart+10; i++ {
		line := strings.TrimSpace(ctx.Lines[i])

		// Salir si encontramos el siguiente slide
		if line == "---" {
			break
		}

		// Ignorar líneas vacías
		if line == "" {
			continue
		}

		contentLines++

		// Detectar encabezados
		if strings.HasPrefix(line, "# ") && !foundH1 {
			foundH1 = true
		} else if strings.HasPrefix(line, "## ") && foundH1 && !foundH2 {
			foundH2 = true
		}

		// Si tenemos H1 + H2 y pocas líneas de contenido, es probablemente un slide de título
		if foundH1 && foundH2 && contentLines <= 6 {
			return true
		}
	}

	// También considerar si es el primer slide (después del frontmatter)
	isFirstSlide := slideStart == 0 || p.isAfterFrontmatter(ctx, slideStart)

	return foundH1 && isFirstSlide
}

// isAfterFrontmatter verifica si el slide está inmediatamente después del frontmatter
func (p *ImageParser) isAfterFrontmatter(ctx *ParseContext, slideStart int) bool {
	// Buscar hacia atrás para ver si hay frontmatter antes de este slide
	frontmatterEnd := -1

	for i := 0; i < slideStart; i++ {
		line := strings.TrimSpace(ctx.Lines[i])
		if line == "---" {
			frontmatterEnd = i
			break
		}
	}

	// Si encontramos frontmatter y este es el primer slide después
	if frontmatterEnd >= 0 {
		// Verificar que no haya otros slides entre el frontmatter y este slide
		for i := frontmatterEnd + 1; i < slideStart; i++ {
			line := strings.TrimSpace(ctx.Lines[i])
			if line == "---" {
				return false // Hay otro slide en el medio
			}
		}
		return true
	}

	return false
}

// hasMultipleImagesNearby detecta si hay múltiples imágenes consecutivas (galería)
func (p *ImageParser) hasMultipleImagesNearby(ctx *ParseContext, startIndex int) bool {
	imageCount := 0

	// Buscar imágenes en un rango más amplio: ±5 líneas
	start := max(0, startIndex-5)
	end := min(len(ctx.Lines), startIndex+6)

	for i := start; i < end; i++ {
		line := strings.TrimSpace(ctx.Lines[i])
		// Detectar sintaxis de imagen Markdown
		if strings.HasPrefix(line, "![") && strings.Contains(line, "](") {
			imageCount++
		}
		// Detectar sintaxis de imagen Strict
		if strings.HasPrefix(line, "IMAGE ") {
			imageCount++
		}
	}

	return imageCount >= 2 // Al menos 2 imágenes = galería
}

// hasTextAfter detecta si hay texto descriptivo después de la imagen
func (p *ImageParser) hasTextAfter(ctx *ParseContext, startIndex int) bool {
	// Buscar en las siguientes 2-3 líneas
	for i := startIndex + 1; i < min(len(ctx.Lines), startIndex+4); i++ {
		line := strings.TrimSpace(ctx.Lines[i])

		// Ignorar líneas vacías
		if line == "" {
			continue
		}

		// Si encuentra otra imagen o elemento especial, no es texto descriptivo
		if strings.HasPrefix(line, "![") || strings.HasPrefix(line, "```") ||
			strings.HasPrefix(line, ":::") || strings.HasPrefix(line, "---") {
			return false
		}

		// Si encuentra texto normal (no es encabezado ni lista), es contenido descriptivo
		if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "-") &&
			!strings.HasPrefix(line, "*") && !strings.HasPrefix(line, "1.") {
			return true
		}
	}

	return false
}

// strictElementKeywords son las palabras clave que abren un nuevo elemento en
// modo strict (mismo conjunto que despacha parser/strict.go). Se usan para
// detener el bucle de continuación de caption/label de una IMAGE.
var strictElementKeywords = []string{
	"TEXT", "POINTS", "CODE", "IMAGE", "TABLE", "QUOTE",
	"CHECKLIST", "MERMAID", "PLANTUML", "CHART", "MAP", "MATH",
}

// startsNewStrictElement reporta si trimmed (una línea ya sin espacios al
// inicio) abre un nuevo elemento en modo strict: sea por palabra clave
// (IMAGE, TEXT, …) o por marcador simbólico —@ directiva, ::: bloque especial,
// << diagrama/chart/map/math, | tabla Markdown—. Las líneas legítimas de
// continuación (caption:/label:) no empiezan con ninguno, así que devuelven
// false y siguen consumiéndose como propiedades de la imagen.
func startsNewStrictElement(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '@', '|':
		return true
	case ':':
		return strings.HasPrefix(trimmed, ":::")
	case '<':
		return strings.HasPrefix(trimmed, "<<")
	}
	for _, kw := range strictElementKeywords {
		if strings.HasPrefix(trimmed, kw) {
			return true
		}
	}
	return false
}

// parseStrictImage parsea la sintaxis IMAGE de strict mode
func (p *ImageParser) parseStrictImage(lines []string, startIndex int) (string, string, string, string, int) {
	line := strings.TrimSpace(lines[startIndex])
	consumed := 1

	// Parse arguments from IMAGE line: IMAGE "source" "alt"
	parts := p.parseImageArguments(line)

	source := ""
	alt := ""
	caption := ""
	label := ""

	if len(parts) >= 2 {
		source = parts[1]
	}
	if len(parts) >= 3 {
		alt = parts[2]
	}
	// Look for additional properties (like caption) in indented lines
	expectedIndent := -1 // Auto-detect indentation level
	for i := startIndex + 1; i < len(lines); i++ {
		line := lines[i]
		currentIndent := CalculateIndentLevel(line)
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			consumed++
			continue
		}

		// Detener el bucle si esta línea abre un nuevo elemento strict (otra
		// IMAGE, TEXT, POINTS, <<chart>>, ::: bloque, etc.). Sin esto, el
		// bucle de continuación de caption/label se tragaba silenciosamente
		// los elementos hermanos al mismo nivel de indentación —p. ej. tres
		// IMAGE consecutivas en un slide colapsaban a una sola.
		if startsNewStrictElement(trimmed) {
			break
		}

		// Auto-detect expected indentation from first non-empty line
		if expectedIndent == -1 && currentIndent > 0 {
			expectedIndent = currentIndent
		}

		// If we haven't detected indentation yet and line has no indentation, break
		if expectedIndent == -1 && currentIndent == 0 {
			break
		}

		// Check if this line should be part of the image block
		if expectedIndent > 0 && currentIndent < expectedIndent {
			break
		}
		if strings.Contains(trimmed, ":") {
			propParts := strings.SplitN(trimmed, ":", 2)
			if len(propParts) == 2 {
				key := strings.TrimSpace(propParts[0])
				value := strings.Trim(strings.TrimSpace(propParts[1]), "\"")

				switch key {
				case "caption":
					caption = value
				case "label":
					// issue #239: identificador de referencia cruzada (p. ej. "fig:arquitectura").
					label = value
				}
			}
		}
		consumed++
	}

	return source, alt, caption, label, consumed
}

// parseMarkdownImage parsea la sintaxis ![alt](src) de Markdown
func (p *ImageParser) parseMarkdownImage(line string) (string, string) {
	// ![alt](src) format
	if !strings.HasPrefix(line, "![") {
		return "", ""
	}

	// Find the closing ]
	altEnd := strings.Index(line, "](")
	if altEnd == -1 {
		return "", ""
	}

	alt := line[2:altEnd] // Extract alt text (skip ![)

	// Find the closing )
	srcStart := altEnd + 2
	srcEnd := strings.Index(line[srcStart:], ")")
	if srcEnd == -1 {
		return "", alt
	}

	source := line[srcStart : srcStart+srcEnd]

	return source, alt
}

// parseImageArguments parses arguments from an IMAGE line (handles quoted strings)
func (p *ImageParser) parseImageArguments(line string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, char := range line {
		if escaped {
			current.WriteRune(char)
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			if inQuotes {
				// End of quoted string
				parts = append(parts, current.String())
				current.Reset()
				inQuotes = false
			} else {
				// Start of quoted string
				inQuotes = true
			}
			continue
		}

		if inQuotes {
			current.WriteRune(char)
		} else if char == ' ' {
			// Separator outside quotes
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(char)
		}
	}

	// Add last token if exists
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// Función helper para min/max (Go < 1.21 compatibility)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
