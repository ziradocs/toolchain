// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package css

// GetResetCSS retorna solo los estilos CSS de reset
func GetResetCSS() string {
	loader := NewCSSFileLoader()
	css, err := loader.LoadResetCSS()
	if err != nil {
		// Fallback if module loading fails
		return "/* Reset CSS not available */"
	}
	return css
}
