// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package frontmatter

import (
	"fmt"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// Patrones compilados una sola vez a nivel de paquete en vez de dentro de
// escapeYamlValue/needsEscaping, que corren por línea del frontmatter (ver
// docs/SECURITY_AUDIT_2026-07.md, BA-9).
var (
	// yamlKeyValuePattern detecta líneas YAML key: value.
	yamlKeyValuePattern = regexp.MustCompile(`^(\s*)([^:]+):\s*(.+)$`)

	// numericPrefixWithTextPattern detecta un valor que empieza con número
	// pero contiene texto (caso edge que también necesita escaping).
	numericPrefixWithTextPattern = regexp.MustCompile(`^\d+[^\d\s]`)
)

// YamlEscapingRule escapa valores YAML que contienen caracteres especiales que pueden causar errores de parsing
type YamlEscapingRule struct {
	parser *base.FrontmatterParser
}

// NewYamlEscapingRule crea una nueva instancia de la regla
func NewYamlEscapingRule() *YamlEscapingRule {
	return &YamlEscapingRule{
		parser: base.NewFrontmatterParser(),
	}
}

func (r *YamlEscapingRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")

	// El frontmatter YAML real solo puede empezar en la primera línea del
	// documento (misma regla que base.DocumentAnalyzer.SkipFrontmatter /
	// base.FrontmatterParser.HasFrontmatter). Antes, esta función buscaba
	// las primeras DOS líneas "---" en CUALQUIER parte del documento como
	// delimitadores de frontmatter. Eso la hacía confundir separadores
	// "---" usados como divisores de sección en el body (comunes entre
	// secciones "##" de un documento DocLang) con un bloque de
	// frontmatter, y terminaba re-escribiendo líneas arbitrarias del body
	// como si fueran YAML de frontmatter — p.ej. "marker: lat, lng, ..."
	// o "lng: ..." dentro de un <<map>>, corrompiendo el contenido.
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, nil
	}

	frontmatterEnd := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			frontmatterEnd = i
			break
		}
	}
	// Si el frontmatter no se cierra, no hacer nada
	if frontmatterEnd == -1 {
		return content, nil
	}

	// Procesar las líneas del frontmatter
	modified := false
	for i := 1; i < frontmatterEnd; i++ {
		line := lines[i]
		if newLine, wasModified := r.escapeYamlValue(line); wasModified {
			lines[i] = newLine
			modified = true
		}
	}

	if modified {
		return strings.Join(lines, "\n"), nil
	}
	return content, nil
}

// escapeYamlValue escapa una línea YAML si contiene caracteres problemáticos
func (r *YamlEscapingRule) escapeYamlValue(line string) (string, bool) {
	// Regex para detectar líneas YAML key: value
	matches := yamlKeyValuePattern.FindStringSubmatch(line)

	if len(matches) != 4 {
		return line, false // No es una línea key: value
	}

	indent := matches[1]
	key := strings.TrimSpace(matches[2])
	value := strings.TrimSpace(matches[3])

	// Si el valor ya está entre comillas, no modificar
	if r.isAlreadyQuoted(value) {
		return line, false
	}

	// Si el valor contiene caracteres problemáticos, escapar
	if r.needsEscaping(value) {
		escapedValue := r.escapeValue(value)
		newLine := fmt.Sprintf("%s%s: %s", indent, key, escapedValue)
		return newLine, true
	}

	return line, false
}

// isAlreadyQuoted verifica si un valor ya está correctamente entre comillas
func (r *YamlEscapingRule) isAlreadyQuoted(value string) bool {
	return (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
		(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`))
}

// needsEscaping determina si un valor necesita ser escapado
func (r *YamlEscapingRule) needsEscaping(value string) bool {
	// Si ya está entre comillas, no necesita escaping
	if r.isAlreadyQuoted(value) {
		return false
	}

	// Caracteres que causan problemas en YAML
	problematicChars := []string{
		":", // Dos puntos - interpretado como nueva clave
		"[", // Corchetes - arrays YAML
		"]",
		"{", // Llaves - objetos YAML
		"}",
		"#", // Comentarios YAML
		"&", // Referencias YAML
		"*", // Alias YAML
		"!", // Tags YAML
		"|", // Literales YAML
		">", // Folded scalars YAML
		"@", // Puede causar problemas en algunos parsers
		"`", // Backticks
	}

	for _, char := range problematicChars {
		if strings.Contains(value, char) {
			return true
		}
	}

	// También escapar si empieza con número pero contiene texto (caso edge)
	if numericPrefixWithTextPattern.MatchString(value) {
		return true
	}

	return false
}

// escapeValue escapa un valor problemático con comillas dobles
func (r *YamlEscapingRule) escapeValue(value string) string {
	// Si el valor parece ser un array JSON, no escapar las comillas internas
	// (ej: ["tag1", "tag2"] debe quedar como "["tag1", "tag2"]")
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return fmt.Sprintf(`"%s"`, value)
	}

	// En otros casos, escapar comillas dobles existentes dentro del valor
	escaped := strings.ReplaceAll(value, `"`, `\"`)
	return fmt.Sprintf(`"%s"`, escaped)
}

func (r *YamlEscapingRule) Description() string {
	return "Escapa valores YAML que contienen caracteres especiales (dos puntos, corchetes, etc.) para evitar errores de parsing"
}

func (r *YamlEscapingRule) Priority() int {
	return 2 // Aplicar pronto en el pipeline
}

func (r *YamlEscapingRule) Category() base.RuleCategory {
	return base.CategoryFrontmatter
}
