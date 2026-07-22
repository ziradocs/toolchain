// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"embed"
	"io/fs"

	"go.ziradocs.com/slidelang/v2/internal/generator/css"
)

//go:embed assets/js/modules/*.js
var jsModulesFS embed.FS

// GetNavigationJS retorna el JavaScript completo de navegación cargado desde assets
func GetNavigationJS() string {
	content, err := fs.ReadFile(jsModulesFS, "assets/js/modules/navigation.js")
	if err != nil {
		// Fallback en caso de error
		return "console.error('Navigation JS module not found');"
	}
	return string(content)
}

// GetNavigationCSS retorna los estilos CSS para la navegación básica
func GetNavigationCSS() string {
	return css.GetNavigationCSS()
}
