// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package config

import "go.ziradocs.com/core/v2/ast"

// ExtractThemeFromFrontmatter extrae el tema del frontmatter o retorna "default"
func ExtractThemeFromFrontmatter(frontmatter *ast.FrontMatterNode) string {
	if frontmatter == nil {
		return "default"
	}

	// El campo Theme está directamente en la estructura
	if frontmatter.Theme != "" {
		// Por ahora retornamos el tema especificado sin validación hardcodeada
		// La validación de existencia se hará en el tema loader
		return frontmatter.Theme
	}

	return "default"
}
