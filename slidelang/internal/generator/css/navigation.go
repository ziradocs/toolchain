// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package css

// GetNavigationCSS retorna los estilos CSS para la navegación completa con menú flotante integrado
func GetNavigationCSS() string {
	loader := NewCSSFileLoader()
	css, err := loader.LoadModuleCSS([]string{"navigation"})
	if err != nil {
		// Fallback if module loading fails
		return "/* Navigation CSS not available */"
	}
	return css
}
