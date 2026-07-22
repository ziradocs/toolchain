// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/slidelang/internal/generator/config"
	"go.ziradocs.com/slidelang/internal/generator/css/themes"
	"go.ziradocs.com/slidelang/internal/generator/data"
	"go.ziradocs.com/slidelang/internal/generator/formatter"
	"go.ziradocs.com/slidelang/internal/generator/modules"
	templateBuilder "go.ziradocs.com/slidelang/internal/generator/template"
)

// PresentationConfig encapsula toda la configuración necesaria para generar una presentación
type PresentationConfig struct {
	AST              *ast.AST
	OutputDir        string
	Options          GeneratorOptions
	Theme            *themes.Theme
	RequiredModules  []string
	RequiredElements []string
	Builder          *templateBuilder.TemplateBuilder
	// RenderContext controla el modo de rendering (browser/offline-assets/
	// offline-inline) de mermaid/chart/map — pasado explícitamente por el
	// caller (issue #134/G1a) en vez de leído de un global de core.
	RenderContext *renderer.RenderContext
}

// AssetGenerationStrategy define cómo generar los assets
type AssetGenerationStrategy struct {
	EmbedAssets     bool
	SeparateModules bool
	CoreOnlyJS      bool
	ModularModules  []string
}

// ModuleAssetGenerator define la interfaz para generar assets de módulos
type ModuleAssetGenerator interface {
	GenerateAssets(outputDir string, logger interface {
		Info(string, string, ...interface{})
	}) error
}

// NavigationModuleGenerator genera assets para el módulo de navegación
type NavigationModuleGenerator struct{}

func (n *NavigationModuleGenerator) GenerateAssets(outputDir string, logger interface {
	Info(string, string, ...interface{})
}) error {
	navCSS := templateBuilder.GetNavigationCSS()
	navJS := templateBuilder.GetNavigationJS()

	// Agregar lógica de auto-registro al módulo de navegación
	navJSWithAutoRegister := navJS + `


// Auto-register navigation module
(function() {
	
	function registerNavigation() {
		
		if (typeof window !== 'undefined' && window.SlideLang) {
			SlideLang.registerModule('navigation', SlideNavigation);
			SlideNavigation.init();
		} else {
			setTimeout(registerNavigation, 50);
		}
	}

	// Iniciar el proceso de registro
	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', registerNavigation);
	} else {
		registerNavigation();
	}
})();`

	cssPath := filepath.Join(outputDir, "navigation.css")
	jsPath := filepath.Join(outputDir, "navigation.js")

	if err := os.WriteFile(cssPath, []byte(navCSS), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(jsPath, []byte(navJSWithAutoRegister), 0644); err != nil {
		return err
	}

	logger.Info("MODULES", "Generated navigation assets: %s, %s", cssPath, jsPath)
	return nil
}

// UtilitiesModuleGenerator genera assets para el módulo de utilities
type UtilitiesModuleGenerator struct{}

func (u *UtilitiesModuleGenerator) GenerateAssets(outputDir string, logger interface {
	Info(string, string, ...interface{})
}) error {
	utilJS := templateBuilder.GetUtilitiesJS()

	// Agregar lógica de auto-registro al módulo de utilities
	utilJSWithAutoRegister := utilJS + `


// Auto-register utilities module
(function() {
	function registerUtilities() {
		if (typeof window !== 'undefined' && window.SlideLang) {
			SlideLang.registerModule('utilities', SlideUtilities);
			SlideUtilities.init();
		} else {
			// Intentar de nuevo en 50ms si SlideLang no está disponible
			setTimeout(registerUtilities, 50);
		}
	}

	// Iniciar el proceso de registro
	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', registerUtilities);
	} else {
		registerUtilities();
	}
})();`

	jsPath := filepath.Join(outputDir, "utilities.js")

	if err := os.WriteFile(jsPath, []byte(utilJSWithAutoRegister), 0644); err != nil {
		return err
	}

	logger.Info("MODULES", "Generated utilities assets: %s", jsPath)
	return nil
}

// ResponsiveModuleGenerator genera assets para el módulo responsive
type ResponsiveModuleGenerator struct{}

func (r *ResponsiveModuleGenerator) GenerateAssets(outputDir string, logger interface {
	Info(string, string, ...interface{})
}) error {
	responsiveCSS := templateBuilder.GetResponsiveCSS()
	cssPath := filepath.Join(outputDir, "responsive.css")

	if err := os.WriteFile(cssPath, []byte(responsiveCSS), 0644); err != nil {
		return err
	}

	logger.Info("MODULES", "Generated responsive assets: %s", cssPath)
	return nil
}

// ChartsModuleGenerator genera assets para el módulo de charts
type ChartsModuleGenerator struct{}

func (c *ChartsModuleGenerator) GenerateAssets(outputDir string, logger interface {
	Info(string, string, ...interface{})
}) error {
	chartsJS := templateBuilder.GetChartsJS()

	// The charts.js file already includes its own auto-registration, so no need to add more
	chartsJSWithAutoRegister := chartsJS

	jsPath := filepath.Join(outputDir, "charts.js")

	if err := os.WriteFile(jsPath, []byte(chartsJSWithAutoRegister), 0644); err != nil {
		return err
	}

	logger.Info("MODULES", "Generated charts assets: %s", jsPath)
	return nil
}

// MermaidModuleGenerator genera assets para el módulo de mermaid
type MermaidModuleGenerator struct{}

func (m *MermaidModuleGenerator) GenerateAssets(outputDir string, logger interface {
	Info(string, string, ...interface{})
}) error {
	mermaidJS := templateBuilder.GetMermaidJS()

	// El módulo externo ya incluye auto-registro, no necesitamos agregarlo
	// Solo usar el módulo tal como viene del archivo externo
	jsPath := filepath.Join(outputDir, "mermaid.js")

	if err := os.WriteFile(jsPath, []byte(mermaidJS), 0644); err != nil {
		return err
	}

	logger.Info("MODULES", "Generated mermaid assets from external file: %s", jsPath)
	return nil
}

// MapsModuleGenerator genera assets para el módulo de maps
type MapsModuleGenerator struct{}

func (m *MapsModuleGenerator) GenerateAssets(outputDir string, logger interface {
	Info(string, string, ...interface{})
}) error {
	mapsJS := templateBuilder.GetMapsJS()

	// El módulo externo ya incluye auto-registro, no necesitamos agregarlo aquí
	jsPath := filepath.Join(outputDir, "maps.js")

	if err := os.WriteFile(jsPath, []byte(mapsJS), 0644); err != nil {
		return err
	}

	logger.Info("MODULES", "Generated maps assets: %s", jsPath)
	return nil
}

// generateHTMLWithOptions crea una presentación HTML con opciones modulares - versión refactorizada.
//
// ctx controla el modo de rendering offline (issue #92) de mermaid/chart/map.
// Se arma UNA vez en runBuild, envolviendo todo el loop de formatos, para que
// un fallo de Chromium falle antes de escribir cualquier formato — y se pasa
// explícitamente hasta acá (issue #134/G1a) en vez de leerse de un
// RenderContext global.
func (g *Generator) generateHTMLWithOptions(astNode *ast.AST, outputDir string, opts GeneratorOptions, ctx *renderer.RenderContext) error {
	// 1. Preparar configuración
	presentationConfig, err := g.preparePresentationConfig(astNode, outputDir, opts, ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare presentation config: %w", err)
	}

	// 2. Determinar estrategia de generación de assets
	strategy := g.determineAssetStrategy(presentationConfig)

	// 3. Generar HTML template
	htmlContent := presentationConfig.Builder.Build()

	// 4. Generar assets si no están embebidos
	if !strategy.EmbedAssets {
		if err := g.generateAssets(presentationConfig, strategy); err != nil {
			return fmt.Errorf("failed to generate assets: %w", err)
		}
	}

	// 5. Generar archivo HTML final
	if err := g.generateHTMLFile(presentationConfig, htmlContent); err != nil {
		return fmt.Errorf("failed to generate HTML file: %w", err)
	}

	return nil
}

// getModuleGenerator devuelve el generador apropiado para cada módulo
func getModuleGenerator(module string) ModuleAssetGenerator {
	switch module {
	case "navigation":
		return &NavigationModuleGenerator{}
	case "utilities":
		return &UtilitiesModuleGenerator{}
	case "responsive":
		return &ResponsiveModuleGenerator{}
	case "charts":
		return &ChartsModuleGenerator{}
	case "mermaid":
		return &MermaidModuleGenerator{}
	case "maps":
		return &MapsModuleGenerator{}
	default:
		return nil
	}
}

// generateModularAssets genera archivos CSS y JS separados por módulo
// detectRequiredElementsFromAST analyzes the AST and returns required CSS element modules
func (g *Generator) detectRequiredElementsFromAST(astNode *ast.AST) []string {
	elementTypes := make(map[string]bool)

	// Always include core elements
	elementTypes["text"] = true

	if astNode == nil {
		return []string{"text", "images", "code"}
	}

	// Header/footer bars come from frontmatter config (astNode.FrontMatter.
	// HeaderFooter), not from a per-slide ast.Element, so the element-type
	// switch below never sees them. If the feature is configured at all,
	// load its CSS module — cheaper than replicating the full
	// global/layout/per-slide enabled-cascade just to decide this (see #90).
	if astNode.FrontMatter != nil && astNode.FrontMatter.HeaderFooter != nil {
		elementTypes["headers_footers"] = true
	}

	// Analyze all slides and their elements
	for _, slide := range astNode.ContentBlocks {
		for _, element := range slide.Elements {
			switch elem := element.(type) {
			case *ast.ImageElement:
				elementTypes["images"] = true
			case *ast.CodeElement:
				elementTypes["code"] = true
			case *ast.TableElement:
				elementTypes["tables"] = true
			case *ast.SpecialBlockElement:
				// Map special block types to the blocks module
				switch strings.ToLower(elem.BlockType) {
				case "info", "warning", "danger", "success", "tip", "note", "error":
					elementTypes["blocks"] = true
				}
			case *ast.QuoteElement:
				elementTypes["quotes"] = true
			case *ast.ChecklistElement:
				elementTypes["checklists"] = true
			case *ast.MapElement:
				elementTypes["maps"] = true
			case *ast.GridElement:
				elementTypes["grids"] = true
			case *ast.ColumnElement:
				elementTypes["grids"] = true
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(elementTypes))
	for elementType := range elementTypes {
		result = append(result, elementType)
	}

	// Ensure we always have core elements
	if !elementTypes["images"] {
		result = append(result, "images")
	}
	if !elementTypes["code"] {
		result = append(result, "code")
	}

	sort.Strings(result)
	return result
}

// preparePresentationConfig prepara toda la configuración necesaria
func (g *Generator) preparePresentationConfig(astNode *ast.AST, outputDir string, opts GeneratorOptions, ctx *renderer.RenderContext) (*PresentationConfig, error) {
	presentationConfig := &PresentationConfig{
		AST:           astNode,
		OutputDir:     outputDir,
		Options:       opts,
		RenderContext: ctx,
	}

	// Resolver tema
	theme, err := g.resolveTheme(astNode, opts)
	if err != nil {
		return nil, err
	}
	presentationConfig.Theme = theme

	// Detectar módulos y elementos requeridos
	presentationConfig.RequiredModules = g.detectRequiredModules(astNode, opts)
	presentationConfig.RequiredElements = g.detectRequiredElementsFromAST(astNode)

	// Crear builder
	presentationConfig.Builder = g.createTemplateBuilder(presentationConfig)

	g.logConfiguration(presentationConfig)

	return presentationConfig, nil
}

// resolveTheme resuelve el tema a usar con la prioridad correcta.
// El nombre de tema es confiable si viene de opts.Theme (flag --theme del
// operador) o de la config global; NO es confiable si viene del frontmatter
// del documento, contenido controlado por el atacante bajo el threat model
// de este repo (ver docs/SECURITY_AUDIT_2026-07.md, ME-2).
func (g *Generator) resolveTheme(astNode *ast.AST, opts GeneratorOptions) (*themes.Theme, error) {
	selectedTheme := opts.Theme
	trusted := true

	// Lógica de resolución simplificada
	if selectedTheme == "" {
		if frontmatterTheme := config.ExtractThemeFromFrontmatter(astNode.FrontMatter); frontmatterTheme != "default" {
			selectedTheme = frontmatterTheme
			trusted = false
		} else if opts.Config != nil && opts.Config.Theme.Default != "" {
			selectedTheme = opts.Config.Theme.Default
		} else {
			selectedTheme = "default"
		}
	}

	// Cargar tema
	themeLoader := themes.NewThemeLoader()
	theme, err := themeLoader.LoadTheme(selectedTheme, trusted)
	if err != nil {
		g.logger.Warn("THEME", "Failed to load theme '%s': %v, using default", selectedTheme, err)
		theme, _ = themeLoader.LoadTheme("default", true)
	}

	return theme, nil
}

// detectRequiredModules detecta los módulos requeridos de manera centralizada
func (g *Generator) detectRequiredModules(astNode *ast.AST, opts GeneratorOptions) []string {
	moduleConfig := modules.ModuleConfig{
		EnableNavigation: !opts.NoNavigation,
		EnableUtilities:  !opts.NoUtilities,
		ForceModules:     []string{},
		ExcludeModules:   g.buildExcludeList(opts),
	}

	return modules.DetectRequiredModulesWithConfig(astNode, moduleConfig)
}

// buildExcludeList construye la lista de módulos a excluir basado en las opciones
func (g *Generator) buildExcludeList(opts GeneratorOptions) []string {
	var excludeList []string

	if opts.NoNavigation {
		excludeList = append(excludeList, "navigation")
	}
	if opts.NoUtilities {
		excludeList = append(excludeList, "utilities")
	}

	// En modos offline, mermaid/chart/map se pre-renderizan en build time, así que
	// sus módulos JS client-side (y los archivos mermaid.js/charts.js/maps.js) no
	// se necesitan: excluirlos evita cargar código muerto contra CDNs (issue #92).
	if opts.IsOffline() {
		excludeList = append(excludeList, "mermaid", "charts", "maps")
	}

	return excludeList
}

// createTemplateBuilder crea el builder de template con toda la configuración
func (g *Generator) createTemplateBuilder(presentationConfig *PresentationConfig) *templateBuilder.TemplateBuilder {
	return templateBuilder.NewTemplateBuilder().
		WithTheme(presentationConfig.Theme.Name).
		WithEmbedAssets(presentationConfig.Options.EmbedAssets).
		WithModules(presentationConfig.RequiredModules).
		WithRequiredElements(presentationConfig.RequiredElements).
		WithNavigation(!presentationConfig.Options.NoNavigation).
		WithUtilities(!presentationConfig.Options.NoUtilities).
		WithRenderMode(presentationConfig.Options.RenderMode)
}

// determineAssetStrategy determina cómo generar los assets
func (g *Generator) determineAssetStrategy(presentationConfig *PresentationConfig) *AssetGenerationStrategy {
	strategy := &AssetGenerationStrategy{
		EmbedAssets:     presentationConfig.Options.EmbedAssets,
		SeparateModules: len(presentationConfig.RequiredModules) > 1,
		CoreOnlyJS:      len(presentationConfig.RequiredModules) > 1,
	}

	// Filtrar módulos que necesitan archivos separados
	if strategy.SeparateModules {
		strategy.ModularModules = g.filterModularModules(presentationConfig.RequiredModules)
	}

	return strategy
}

// filterModularModules filtra los módulos que necesitan archivos separados
func (g *Generator) filterModularModules(modules []string) []string {
	var modularModules []string
	for _, module := range modules {
		if module != "core" {
			modularModules = append(modularModules, module)
		}
	}
	return modularModules
}

// generateAssets genera todos los assets necesarios
func (g *Generator) generateAssets(presentationConfig *PresentationConfig, strategy *AssetGenerationStrategy) error {
	// Generar reset.css
	if err := g.generateResetCSS(presentationConfig); err != nil {
		return err
	}

	// Generar presentation.css
	if err := g.generateMainCSS(presentationConfig); err != nil {
		return err
	}

	// Generar JavaScript
	if err := g.generateJavaScript(presentationConfig, strategy); err != nil {
		return err
	}

	// Generar assets modulares si es necesario
	if strategy.SeparateModules {
		if err := g.generateModularAssetsRefactored(presentationConfig, strategy.ModularModules); err != nil {
			return err
		}
	}

	return nil
}

// generateResetCSS genera el archivo reset.css
func (g *Generator) generateResetCSS(presentationConfig *PresentationConfig) error {
	resetContent := templateBuilder.GetResetCSS()
	resetPath := filepath.Join(presentationConfig.OutputDir, "reset.css")

	if err := os.WriteFile(resetPath, []byte(resetContent), 0644); err != nil {
		return fmt.Errorf("failed to write reset CSS file: %w", err)
	}

	g.logger.Info("FILE", "Generated reset CSS file: %s", resetPath)
	return nil
}

// generateMainCSS genera el archivo presentation.css principal
func (g *Generator) generateMainCSS(presentationConfig *PresentationConfig) error {
	cssContent, err := presentationConfig.Builder.BuildCSS()
	if err != nil {
		g.logger.Warn("CSS", "Error generating CSS: %v", err)
	}

	cssPath := filepath.Join(presentationConfig.OutputDir, "presentation.css")
	if err := os.WriteFile(cssPath, []byte(cssContent), 0644); err != nil {
		return fmt.Errorf("failed to write CSS file: %w", err)
	}

	g.logger.Info("FILE", "Generated CSS file: %s", cssPath)
	return nil
}

// generateJavaScript genera el archivo JavaScript principal
func (g *Generator) generateJavaScript(presentationConfig *PresentationConfig, strategy *AssetGenerationStrategy) error {
	var jsContent string

	if strategy.CoreOnlyJS {
		// Generar presentation.js solo con el core
		jsContent = presentationConfig.Builder.BuildJSWithModules([]string{"core"})
	} else {
		// Incluir todos los módulos en presentation.js
		jsContent = presentationConfig.Builder.BuildJSWithModules(presentationConfig.RequiredModules)
	}

	jsPath := filepath.Join(presentationConfig.OutputDir, "presentation.js")
	if err := os.WriteFile(jsPath, []byte(jsContent), 0644); err != nil {
		return fmt.Errorf("failed to write JS file: %w", err)
	}

	g.logger.Info("FILE", "Generated JS file: %s", jsPath)
	return nil
}

// renderHTML ejecuta el template HTML modular contra los datos del AST y
// devuelve el HTML final formateado como string, sin tocar el disco (issue
// #128, refactor F1). Es la función pura que antes vivía inline dentro de
// generateHTMLFile — separada para que generatePDF (pdf.go) pueda reusarla
// para producir el HTML auto-contenido que alimenta a Chromium, en vez de
// reimplementar el mismo parseo+ejecución+formatting de template.
func (g *Generator) renderHTML(presentationConfig *PresentationConfig, htmlContent string) (string, error) {
	// Parse template
	tmpl, err := template.New("presentation").Funcs(config.HTMLTemplateFuncs()).Parse(htmlContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML template: %w", err)
	}

	// Prepare template data (con modo de rendering para el pre-render offline, #92)
	templateData := data.PrepareTemplateDataWithRenderMode(presentationConfig.AST, presentationConfig.Theme.Name, presentationConfig.Options.RenderMode, g.logger, presentationConfig.RenderContext)

	// Execute template to buffer first (for formatting)
	var htmlBuffer strings.Builder
	if err := tmpl.Execute(&htmlBuffer, templateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Apply HTML formatting
	htmlFormatter := formatter.NewHTMLFormatter()
	return htmlFormatter.FormatHTML(htmlBuffer.String()), nil
}

// generateHTMLFile renderiza (renderHTML) y escribe el HTML final al disco.
func (g *Generator) generateHTMLFile(presentationConfig *PresentationConfig, htmlContent string) error {
	filename := g.resolveHTMLFilename(presentationConfig.AST)
	outputPath := filepath.Join(presentationConfig.OutputDir, filename)

	formattedHTML, err := g.renderHTML(presentationConfig, htmlContent)
	if err != nil {
		return err
	}

	// Create and write formatted HTML to file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create HTML file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.WriteString(formattedHTML); err != nil {
		return fmt.Errorf("failed to write formatted HTML: %w", err)
	}

	g.logger.Info("FILE", "Generated and formatted HTML file: %s", outputPath)
	return nil
}

// resolveHTMLFilename resuelve el nombre del archivo HTML basado en el AST
func (g *Generator) resolveHTMLFilename(astNode *ast.AST) string {
	return resolveOutputFilename(astNode, "html")
}

// resolveOutputFilename deriva el nombre de archivo de salida a partir del
// FilePath del AST, con extensión `ext` — única fuente de verdad para
// resolveHTMLFilename y resolvePDFFilename (pdf.go), que antes duplicaban la
// misma lógica base/ext-stripping (issue #128, hallazgo de code-review sobre
// PR #160).
func resolveOutputFilename(astNode *ast.AST, ext string) string {
	if astNode.FilePath == "" {
		return "presentation." + ext
	}

	base := filepath.Base(astNode.FilePath)
	origExt := filepath.Ext(base)
	name := base[:len(base)-len(origExt)]
	return name + "." + ext
}

// logConfiguration registra la configuración para debugging
func (g *Generator) logConfiguration(presentationConfig *PresentationConfig) {
	g.logger.Info("THEME", "Using theme: %s v%s (type: %s)",
		presentationConfig.Theme.Name,
		presentationConfig.Theme.Version,
		func() string {
			if presentationConfig.Theme.IsExternal {
				return "external"
			}
			return "embedded"
		}())

	g.logger.Info("MODULES", "Required modules: %v", presentationConfig.RequiredModules)
	g.logger.Info("ELEMENTS", "Required CSS elements: %v", presentationConfig.RequiredElements)
}

// generateModularAssetsRefactored genera archivos separados por módulo usando el patrón Strategy
func (g *Generator) generateModularAssetsRefactored(presentationConfig *PresentationConfig, modules []string) error {
	for _, module := range modules {
		generator := getModuleGenerator(module)
		if generator != nil {
			if err := generator.GenerateAssets(presentationConfig.OutputDir, g.logger); err != nil {
				return fmt.Errorf("failed to generate %s module assets: %w", module, err)
			}
		} else {
			g.logger.Info("MODULES", "Skipping unknown module: %s", module)
		}
	}
	return nil
}
