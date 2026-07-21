// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package css

import (
	"strings"
	"testing"
)

func TestCSSBuilderDefault(t *testing.T) {
	builder := NewCSSBuilder()
	css := builder.Build()

	// Verificar que contiene las secciones esperadas
	expectedSections := []string{
		"/* === THEME VARIABLES === */",
		"/* === BASE STYLES === */",
		"/* === ELEMENT MODULES === */",
		"/* === RESPONSIVE STYLES === */",
	}

	for _, section := range expectedSections {
		if !strings.Contains(css, section) {
			t.Errorf("CSS no contiene la sección esperada: %s", section)
		}
	}

	// Verificar que contiene variables de theme
	if !strings.Contains(css, "--slidelang-primary-color") {
		t.Error("CSS no contiene variables de theme namespaced")
	}

	// Verificar que es válido (no vacío)
	if len(css) < 1000 {
		t.Errorf("CSS generado es muy corto: %d caracteres", len(css))
	}
}

func TestCSSBuilderWithThemes(t *testing.T) {
	themes := []string{"default", "dark", "minimal"}

	for _, theme := range themes {
		builder := NewCSSBuilder().WithTheme(theme)
		css := builder.Build()

		if !strings.Contains(css, ":root") {
			t.Errorf("Theme %s no genera variables CSS", theme)
		}

		if len(css) < 500 {
			t.Errorf("Theme %s genera CSS muy corto", theme)
		}
	}
}

func TestCSSBuilderModules(t *testing.T) {
	modules := []string{"text", "code", "images", "tables", "blocks", "quotes"}

	for _, module := range modules {
		builder := NewCSSBuilder().WithRequiredElements([]string{module})
		css := builder.Build()

		// Verificar que el módulo está incluido de alguna forma
		if !strings.Contains(css, "element") && module != "base" {
			t.Errorf("Módulo %s no parece estar incluido en el CSS", module)
		}
	}
}

// TestAvailableModulesHaveCSSFiles evita que GetAvailableModules() y los
// archivos reales embebidos bajo assets/css/elements/ vuelvan a divergir en
// silencio — LoadElementCSS no falla el build si un módulo listado no tiene
// archivo (ver #90), así que esta cobertura debe hacerlo explícito en tests.
func TestAvailableModulesHaveCSSFiles(t *testing.T) {
	loader := NewCSSFileLoader()

	for _, module := range GetAvailableModules() {
		// LoadElementCSS nunca retorna error (swallow por diseño, ver #90);
		// un módulo sin archivo simplemente produce salida vacía.
		css, _ := loader.LoadElementCSS([]string{module})
		if strings.TrimSpace(css) == "" {
			t.Errorf("módulo %q está en GetAvailableModules() pero assets/css/elements/%s.css no existe o está vacío", module, module)
		}
	}
}

func TestCSSBuilderLayouts(t *testing.T) {
	layouts := []string{"specialized", "infographics"}

	for _, layout := range layouts {
		builder := NewCSSBuilder().WithRequiredLayouts([]string{layout})
		css := builder.Build()

		// Verificar que los layouts están incluidos
		if !strings.Contains(css, "/* === LAYOUT MODULES === */") {
			t.Errorf("Layout %s no incluye sección de layouts", layout)
		}
	}
}

func TestGenerateCSS(t *testing.T) {
	config := DefaultCSSConfig()
	css, err := GenerateCSS(config)

	if err != nil {
		t.Fatalf("Error generando CSS: %v", err)
	}

	if len(css) < 1000 {
		t.Errorf("CSS generado es muy corto: %d caracteres", len(css))
	}

	// Verificar estructura básica
	if !strings.Contains(css, ":root") {
		t.Error("CSS no contiene variables de root")
	}

	if !strings.Contains(css, ".slidelang-slide") {
		t.Error("CSS no contiene estilos namespaced de slide")
	}
}

func TestValidateConfig(t *testing.T) {
	// Config válida
	validConfig := DefaultCSSConfig()
	if err := ValidateConfig(validConfig); err != nil {
		t.Errorf("Config válida falló validación: %v", err)
	}

	// Theme inválido
	invalidTheme := DefaultCSSConfig()
	invalidTheme.Theme = "nonexistent"
	if err := ValidateConfig(invalidTheme); err == nil {
		t.Error("Config con theme inválido debería fallar validación")
	}

	// Elements inválidos se ignoran silenciosamente en la nueva API
	// No hay validación estricta de elements porque son opcionales
}

func TestCSSStats(t *testing.T) {
	config := DefaultCSSConfig()
	css, _ := GenerateCSS(config)
	stats := GetCSSStats(css, config)

	if stats.TotalLines == 0 {
		t.Error("Stats debería tener líneas contadas")
	}

	if stats.TotalSize == 0 {
		t.Error("Stats debería tener tamaño calculado")
	}

	if stats.ThemeVars == 0 {
		t.Error("Stats debería contar variables de theme")
	}

	// ModulesUsed ya no está disponible en la nueva API
}

func BenchmarkCSSGeneration(b *testing.B) {
	config := DefaultCSSConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateCSS(config)
		if err != nil {
			b.Fatalf("Error en benchmark: %v", err)
		}
	}
}

func BenchmarkCSSBuilder(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := NewCSSBuilder()
		_ = builder.Build()
	}
}
