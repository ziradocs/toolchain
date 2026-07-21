// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"go.ziradocs.com/slidelang/internal/generator/css/themes"
)

// Theme adapta el nuevo sistema de themes para compatibilidad
type Theme = themes.Theme

// Themes expone los themes del nuevo sistema
var Themes = themes.EmbeddedThemes

// GetTheme obtiene un theme del nuevo sistema
func GetTheme(name string) Theme {
	return themes.GetTheme(name)
}

// GetThemeNames obtiene los nombres de themes disponibles
func GetThemeNames() []string {
	return themes.GetThemeNames()
}
