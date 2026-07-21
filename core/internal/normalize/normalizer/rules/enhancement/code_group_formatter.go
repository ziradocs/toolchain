// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"regexp"
	"strings"
)

// Patrones compilados una sola vez a nivel de paquete (en vez de en cada
// llamada a Apply/dentro del loop por línea) — evitan recompilar el mismo
// regex por cada documento/línea normalizada (ver
// docs/SECURITY_AUDIT_2026-07.md, BA-9).
var (
	// codeGroupStartPattern detecta inicio de code-group (4 dos puntos).
	// IMPORTANTE: usar [ \t]* (solo espacios/tabs) en vez de [\s]* — \s incluye
	// \n, y con (?m) eso permite que ^...$ "engulla" la línea en blanco anterior
	// o posterior al marcador, corrompiendo el conteo de líneas (y por tanto las
	// posiciones reportadas) de contenido que ni siquiera usa la sintaxis
	// :::code-item{} (detectado en el corpus de examples/ al activar esta regla
	// también para contenido no detectado como IA — issue #174).
	// El `\r?` opcional antes de `$` preserva la compatibilidad con archivos
	// CRLF: sin él, `$` (en modo multilínea) exige que no quede nada entre el
	// último carácter y el `\n`, pero en un archivo CRLF ese "nada" en
	// realidad es un `\r` — con solo [ \t]* el match fallaría siempre en
	// CRLF y la regla dejaría de normalizar code-groups en esos archivos.
	codeGroupStartPattern = regexp.MustCompile(`(?m)^[ \t]*::::code-group[ \t]*\r?$`)

	// codeItemPattern detecta code-item con metadata: :::code-item{title="filename"}
	codeItemPattern = regexp.MustCompile(`(?m)^[ \t]*:::code-item\{([^}]+)\}[ \t]*\r?$`)

	// codeItemEndPattern detecta cierre de code-item: :::
	codeItemEndPattern = regexp.MustCompile(`(?m)^[ \t]*:::[ \t]*\r?$`)

	// codeItemTitlePattern extrae el title= de la metadata de un code-item.
	codeItemTitlePattern = regexp.MustCompile(`title\s*=\s*"([^"]+)"`)
)

// CodeGroupFormatterRule normaliza la sintaxis de code-groups desde Markdown extended
// Convierte ::::code-group + :::code-item{title="..."} a formato reconocido por el parser
type CodeGroupFormatterRule struct{}

// NewCodeGroupFormatterRule crea una nueva regla de formateo de code-groups
func NewCodeGroupFormatterRule() *CodeGroupFormatterRule {
	return &CodeGroupFormatterRule{}
}

// Description retorna la descripción de la regla
func (r *CodeGroupFormatterRule) Description() string {
	return "CodeGroupFormatter: Normaliza sintaxis de code-groups (::::code-group → :::code-group)"
}

// Priority retorna la prioridad de la regla
func (r *CodeGroupFormatterRule) Priority() int {
	return 3 // Aplicar después de estructura básica pero antes de otros enhancements
}

// Apply aplica la transformación
func (r *CodeGroupFormatterRule) Apply(content string) (string, error) {
	// Convertir ::::code-group → :::code-group
	result := codeGroupStartPattern.ReplaceAllString(content, ":::code-group")

	// Procesar cada code-item
	lines := strings.Split(result, "\n")
	var processedLines []string
	inCodeGroup := false
	inCodeItem := false
	inCodeBlock := false
	inBareFence := false
	currentLanguage := ""
	currentTitle := ""
	var codeBlockContent []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Detectar inicio de code-group
		if trimmed == ":::code-group" {
			inCodeGroup = true
			processedLines = append(processedLines, line)
			continue
		}

		// Detectar fin de code-group (wrapper legacy, cierre de 4 dos puntos)
		if inCodeGroup && trimmed == "::::" {
			inCodeGroup = false
			processedLines = append(processedLines, ":::")
			continue
		}

		// Trackear apertura/cierre de un fence ```lang [label] que vive
		// DIRECTAMENTE dentro de un code-group ya canónico (sin wrapper
		// :::code-item{}) — necesario para que un ":::" que aparezca como
		// CONTENIDO LITERAL dentro de ese fence (p.ej. un ejemplo de
		// documentación que muestra esta misma sintaxis de code-group) no
		// dispare por error la detección de cierre de abajo. Sin este
		// tracking, inCodeBlock solo se activaba dentro del wrapper legacy
		// :::code-item{} (ver más abajo), dejando el camino canónico sin
		// protección — el mismo bug de "code-group pierde tabs" que esta
		// regla existe para arreglar, en un disparador distinto.
		if inCodeGroup && !inCodeItem && !inBareFence && strings.HasPrefix(trimmed, "```") {
			inBareFence = true
			processedLines = append(processedLines, line)
			continue
		}
		if inBareFence && trimmed == "```" {
			inBareFence = false
			processedLines = append(processedLines, line)
			continue
		}

		// Detectar fin de code-group ya en formato canónico: un code-group que
		// usa fences ```lang [label] directamente (sin wrapper :::code-item{})
		// SIEMPRE cierra con un solo ":::" — igual que exige el parser real en
		// elements/code_group.go, sin importar si el opener fue
		// "::::code-group" o ":::code-group". Sin este caso, inCodeGroup se
		// quedaba en true para el resto del documento tras un code-group ya
		// canónico, y una línea "::::" posterior y no relacionada (p. ej. un
		// divisor de 4 dos puntos en prosa) terminaba reescribiéndose a ":::"
		// por error (ver TestCodeGroupFormatterRule_CanonicalCloseDoesNotLeakState).
		if inCodeGroup && !inCodeItem && !inCodeBlock && !inBareFence && codeItemEndPattern.MatchString(trimmed) {
			inCodeGroup = false
			processedLines = append(processedLines, line)
			continue
		}

		// Detectar code-item con metadata
		if inCodeGroup && codeItemPattern.MatchString(trimmed) {
			matches := codeItemPattern.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				// Extraer title de la metadata
				metadata := matches[1]
				titleMatch := codeItemTitlePattern.FindStringSubmatch(metadata)
				if len(titleMatch) > 1 {
					currentTitle = titleMatch[1]
				}
			}
			inCodeItem = true
			continue // No agregar esta línea, esperamos el bloque de código
		}

		// Detectar inicio de bloque de código dentro de code-item
		if inCodeItem && !inCodeBlock && strings.HasPrefix(trimmed, "```") {
			// Extraer lenguaje del bloque
			currentLanguage = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			if currentLanguage == "" {
				currentLanguage = "plaintext"
			}
			inCodeBlock = true
			codeBlockContent = []string{}
			continue
		}

		// Detectar fin de bloque de código
		if inCodeBlock && trimmed == "```" {
			// Generar el bloque formateado con título y lenguaje
			label := ""
			if currentTitle != "" {
				label = " [" + currentTitle + "]"
			}
			processedLines = append(processedLines, "```"+currentLanguage+label)
			processedLines = append(processedLines, codeBlockContent...)
			processedLines = append(processedLines, "```")

			// Reset state
			inCodeBlock = false
			currentTitle = ""
			currentLanguage = ""
			codeBlockContent = []string{}
			continue
		}

		// Detectar cierre de code-item (:::)
		if inCodeItem && !inCodeBlock && codeItemEndPattern.MatchString(trimmed) {
			inCodeItem = false
			continue // No agregar esta línea
		}

		// Acumular contenido del bloque de código
		if inCodeBlock {
			codeBlockContent = append(codeBlockContent, line)
			continue
		}

		// Líneas normales
		processedLines = append(processedLines, line)
	}

	return strings.Join(processedLines, "\n"), nil
}
