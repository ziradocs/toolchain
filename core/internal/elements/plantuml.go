// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// PlantUMLParser maneja el parsing de diagramas PlantUML
type PlantUMLParser struct{}

// CanParse determina si puede parsear una línea como PlantUML
func (p *PlantUMLParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	switch mode {
	case "strict":
		return strings.HasPrefix(trimmed, "<<plantuml>>")
	case "flex":
		// En flex mode, soportar <<plantuml>>, @startuml, y ```plantuml
		if strings.HasPrefix(trimmed, "<<plantuml>>") {
			return true
		}
		if strings.HasPrefix(trimmed, "@startuml") {
			return true
		}
		// Detectar code blocks de Markdown con lenguaje "plantuml"
		if strings.HasPrefix(trimmed, "```plantuml") || strings.HasPrefix(trimmed, "````plantuml") {
			return true
		}
	}

	return false
}

// Parse parsea un elemento PlantUML
func (p *PlantUMLParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{Error: nil}
	}

	pos := diagnostics.NewPosition(startIndex+1, 1)
	openingLine := strings.TrimSpace(ctx.Lines[startIndex])

	// Detectar formato: <<plantuml>>, @startuml, o ```plantuml
	isMarkdownFormat := strings.HasPrefix(openingLine, "```plantuml") || strings.HasPrefix(openingLine, "````plantuml")
	isNativeFormat := strings.HasPrefix(openingLine, "@startuml")

	consumedLines := 1 // Cuenta la línea de apertura (<<plantuml>>, @startuml, o ```plantuml)
	var content strings.Builder
	foundStartUml := isNativeFormat // Si empezó con @startuml, ya lo encontramos

	if isMarkdownFormat {
		// Formato Markdown: ```plantuml ... ```
		// Recoger contenido hasta encontrar ``` o ````
		for i := startIndex + 1; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmed := strings.TrimSpace(line)

			// Terminar al encontrar closing backticks
			if trimmed == "```" || trimmed == "````" {
				consumedLines++
				break
			}

			if content.Len() > 0 {
				content.WriteString("\n")
			}
			content.WriteString(trimmed)
			consumedLines++
		}
	} else if isNativeFormat {
		// Formato nativo PlantUML: @startuml ... @enduml
		// Incluir @startuml en el contenido
		content.WriteString("@startuml\n")

		for i := startIndex + 1; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmed := strings.TrimSpace(line)

			// Terminar al encontrar @enduml
			if strings.HasPrefix(trimmed, "@enduml") {
				content.WriteString("@enduml")
				consumedLines++
				break
			}

			if content.Len() > len("@startuml\n") {
				content.WriteString("\n")
			}
			content.WriteString(trimmed)
			consumedLines++
		}
	} else {
		// Formato <<plantuml>> - leer hasta @enduml o nuevo elemento
		// NO usar auto-detección de indentación porque en DocLang el contenido no está indentado

		foundEnduml := false
		for i := startIndex + 1; i < len(ctx.Lines); i++ {
			line := ctx.Lines[i]
			trimmedLine := strings.TrimSpace(line)

			// Skip empty lines al inicio (antes de @startuml)
			if trimmedLine == "" && content.Len() == 0 {
				consumedLines++
				continue
			}

			// Terminar si encontramos @enduml
			if strings.HasPrefix(trimmedLine, "@enduml") {
				if content.Len() > 0 {
					content.WriteString("\n")
				}
				content.WriteString(trimmedLine)
				consumedLines++
				foundEnduml = true
				break
			}

			// En flex mode, detectar si empieza un nuevo elemento (SOLO SI NO HEMOS VISTO @STARTUML)
			if content.Len() == 0 && ctx.Mode == "flex" && IsNewElement(trimmedLine, ctx.Mode) && !strings.HasPrefix(trimmedLine, "@startuml") {
				break
			}

			// Detectar @startuml si no se ha encontrado aún
			if !foundStartUml && strings.HasPrefix(trimmedLine, "@startuml") {
				foundStartUml = true
			}

			// Agregar línea al contenido (incluso si está vacía - líneas vacías dentro del diagrama son válidas)
			if content.Len() > 0 {
				content.WriteString("\n")
			}
			content.WriteString(trimmedLine)
			consumedLines++
		}

		// Consumir el <<end>> si existe (para formato <<plantuml>> ... <<end>>)
		if foundEnduml && startIndex+consumedLines < len(ctx.Lines) {
			nextLine := strings.TrimSpace(ctx.Lines[startIndex+consumedLines])
			switch nextLine {
			case "<<end>>":
				consumedLines++
			case "":
				// Si hay línea vacía, consumirla también
				consumedLines++
				// Verificar si después de la línea vacía hay <<end>>
				if startIndex+consumedLines < len(ctx.Lines) {
					nextNextLine := strings.TrimSpace(ctx.Lines[startIndex+consumedLines])
					if nextNextLine == "<<end>>" {
						consumedLines++
					}
				}
			}
		}
	}

	// Asegurarse de que el contenido tiene @startuml y @enduml
	contentStr := content.String()
	if !strings.Contains(contentStr, "@startuml") {
		contentStr = "@startuml\n" + contentStr
	}
	if !strings.Contains(contentStr, "@enduml") {
		contentStr = contentStr + "\n@enduml"
	}

	// Detectar tipo de diagrama de forma más robusta
	diagramType := detectPlantUMLType(contentStr)

	// Crear elemento PlantUML
	element := ast.NewPlantUMLElement(pos, diagramType, contentStr)

	return &ParseResult{
		Element:       element,
		ConsumedLines: consumedLines,
	}
}

// detectPlantUMLType detecta el tipo de diagrama PlantUML
func detectPlantUMLType(content string) string {
	content = strings.ToLower(content)

	// Patrones comunes de PlantUML
	patterns := map[string]*regexp.Regexp{
		"sequence":   regexp.MustCompile(`(?m)^[a-z0-9_]+\s*-+>|^participant\s|^actor\s|^boundary\s|^control\s|^entity\s|^database\s`),
		"class":      regexp.MustCompile(`(?m)^class\s|^interface\s|^abstract\s|^enum\s|extends\s|implements\s`),
		"component":  regexp.MustCompile(`(?m)^\[.*\]|^component\s|^package\s|^node\s|^cloud\s|^database\s`),
		"usecase":    regexp.MustCompile(`(?m)^usecase\s|^actor\s|^\(.*\)|-->\s*\(|\)\s*-->`),
		"activity":   regexp.MustCompile(`(?m)^:.*;$|^if\s*\(.*\)|^while\s*\(.*\)|^repeat|^fork|^partition`),
		"state":      regexp.MustCompile(`(?m)^state\s|^\[\*\]|state.*:|\s+-->\s+\[\*\]`),
		"object":     regexp.MustCompile(`(?m)^object\s|^map\s`),
		"deployment": regexp.MustCompile(`(?m)^node\s|^artifact\s|^cloud\s|^database\s|^frame\s`),
		"timing":     regexp.MustCompile(`(?m)^robust\s|^concise\s|^clock\s|@\d+`),
		"gantt":      regexp.MustCompile(`(?m)^@startgantt|project starts|task\s|milestone\s`),
	}

	// Buscar el primer patrón que coincida
	for diagramType, pattern := range patterns {
		if pattern.MatchString(content) {
			return diagramType
		}
	}

	// Default: sequence (más común)
	return "sequence"
}
