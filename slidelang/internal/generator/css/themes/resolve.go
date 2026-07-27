// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import "fmt"

// ResolveTheme resuelve y carga el tema activo con la prioridad
// flag --theme > frontmatter > config default > "default", sin depender de
// un logger ni de *ast.AST/GeneratorOptions — así build.go puede llamarla
// directamente, ANTES de que exista ningún *Generator (issue #30: el lint
// necesita el mapa de colores del tema ya resuelto antes de correr, y
// resolveTheme vivía enterrado dentro del generador, alcanzable solo desde
// GenerateWithOptions).
//
// frontmatterTheme debe venir de config.ExtractThemeFromFrontmatter (paquete
// slidelang/internal/generator/config) — esa función YA devuelve "default"
// tanto si el frontmatter no declara tema como si lo declara explícitamente
// igual a "default"; ese matiz (un documento con `theme: default` se trata
// como "sin tema") se preserva tal cual, sin resolverlo acá, para no
// cambiar comportamiento existente en este refactor.
//
// Si falla la carga del tema seleccionado, cae a "default" (embebido,
// siempre disponible) — el *Theme devuelto NUNCA es nil — y el error
// devuelto describe qué tema falló y por qué, para que el caller decida
// cómo reportarlo (hoy, un log.Warn en ambos callers).
func ResolveTheme(flagTheme, frontmatterTheme, configDefault string) (*Theme, error) {
	selectedTheme := flagTheme
	trusted := true

	if selectedTheme == "" {
		if frontmatterTheme != "default" {
			selectedTheme = frontmatterTheme
			trusted = false
		} else if configDefault != "" {
			selectedTheme = configDefault
		} else {
			selectedTheme = "default"
		}
	}

	loader := NewThemeLoader()
	theme, err := loader.LoadTheme(selectedTheme, trusted)
	if err != nil {
		fallback, _ := loader.LoadTheme("default", true)
		return fallback, fmt.Errorf("theme '%s': %w", selectedTheme, err)
	}
	return theme, nil
}
