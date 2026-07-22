// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"go.ziradocs.com/core/ast"
	globalConfig "go.ziradocs.com/core/config"
	"go.ziradocs.com/slidelang/internal/generator/config"
	"go.ziradocs.com/slidelang/internal/generator/css/themes"
	"go.ziradocs.com/slidelang/internal/generator/data"
	templateBuilder "go.ziradocs.com/slidelang/internal/generator/template"
)

// generateHTML crea una presentación HTML autocontenida o con archivos separados
func (g *Generator) generateHTML(astNode *ast.AST, outputDir string, themeName string, embedAssets bool, cfg *globalConfig.SlideLangConfig) error {
	// Generar nombre de archivo basado en el archivo de entrada
	filename := "presentation.html"
	if astNode.FilePath != "" {
		base := filepath.Base(astNode.FilePath)
		ext := filepath.Ext(base)
		name := base[:len(base)-len(ext)]
		filename = name + ".html"
	}

	outputPath := filepath.Join(outputDir, filename)

	// Crear el archivo HTML
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create HTML file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Extraer tema con la prioridad correcta:
	// 1. CLI flag (themeName) — confiable, viene del operador
	// 2. Frontmatter — NO confiable, contenido del documento (ver
	//    docs/SECURITY_AUDIT_2026-07.md, ME-2)
	// 3. Config global — confiable
	// 4. Default
	selectedTheme := themeName
	themeTrusted := true
	if selectedTheme == "" {
		// Si no hay CLI flag, extraer del frontmatter
		frontmatterTheme := config.ExtractThemeFromFrontmatter(astNode.FrontMatter)
		if frontmatterTheme != "default" {
			selectedTheme = frontmatterTheme
			themeTrusted = false
		} else {
			// Si frontmatter es "default" o vacío, usar config global como fallback
			if cfg != nil && cfg.Theme.Default != "" {
				selectedTheme = cfg.Theme.Default
			} else {
				selectedTheme = "default"
			}
		}
	}

	// Crear loader de themes para soporte de themes externos
	themeLoader := themes.NewThemeLoader()

	// Cargar el theme (puede ser embebido o externo)
	theme, err := themeLoader.LoadTheme(selectedTheme, themeTrusted)
	if err != nil {
		g.logger.Warn("THEME", "Failed to load theme '%s': %v, using default", selectedTheme, err)
		// Fallback al theme default
		theme, _ = themeLoader.LoadTheme("default", true)
	}

	g.logger.Info("THEME", "Using theme: %s v%s (type: %s)", theme.Name, theme.Version, func() string {
		if theme.IsExternal {
			return "external"
		}
		return "embedded"
	}())

	// Crear el builder de template modular - TODOS LOS MÓDULOS INCLUIDOS POR DEFECTO
	builder := templateBuilder.NewTemplateBuilder().
		WithTheme(theme.Name).
		WithEmbedAssets(embedAssets)
	htmlTemplateContent := builder.Build()

	// Si no se embeben los assets, generar archivos CSS y JS separados
	if !embedAssets {
		// Generar archivo CSS
		cssContent, err := builder.BuildCSS()
		if err != nil {
			g.logger.Warn("CSS", "Error generating CSS: %v", err)
		}
		cssPath := filepath.Join(outputDir, "presentation.css")
		if err := os.WriteFile(cssPath, []byte(cssContent), 0644); err != nil {
			return fmt.Errorf("failed to write CSS file: %w", err)
		}
		g.logger.Info("FILE", "Generated CSS file: %s", cssPath)

		// Generar archivo JS
		jsContent := builder.BuildJS()
		jsPath := filepath.Join(outputDir, "presentation.js")
		if err := os.WriteFile(jsPath, []byte(jsContent), 0644); err != nil {
			return fmt.Errorf("failed to write JS file: %w", err)
		}
		g.logger.Info("FILE", "Generated JS file: %s", jsPath)
	}
	// Crear el template de Go
	tmpl, err := template.New("presentation").Funcs(config.HTMLTemplateFuncs()).Parse(htmlTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}
	// Preparar datos para el template
	templateData := data.PrepareTemplateData(astNode, g.logger)

	// Ejecutar el template
	if err := tmpl.Execute(file, templateData); err != nil {
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}
	g.logger.Info("FILE", "Generated HTML presentation: %s", outputPath)
	return nil
}
