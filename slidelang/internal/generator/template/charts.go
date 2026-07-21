// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"io/fs"
)

// GetChartsJS retorna el JavaScript completo de charts cargado desde assets
func GetChartsJS() string {
	content, err := fs.ReadFile(jsModulesFS, "assets/js/modules/charts.js")
	if err != nil {
		// Fallback en caso de error
		return "console.error('Charts JS module not found');"
	}
	return string(content)
}
