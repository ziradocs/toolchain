// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package modules

import (
	"strings"

	"go.ziradocs.com/core/ast"
)

// ModuleConfig define la configuración de módulos
type ModuleConfig struct {
	EnableNavigation bool     // Si incluir navegación básica
	EnableUtilities  bool     // Si incluir utilidades
	ForceModules     []string // Módulos a incluir forzosamente
	ExcludeModules   []string // Módulos a excluir
}

// DefaultModuleConfig retorna la configuración por defecto
func DefaultModuleConfig() ModuleConfig {
	return ModuleConfig{
		EnableNavigation: true,
		EnableUtilities:  true,
		ForceModules:     []string{},
		ExcludeModules:   []string{},
	}
}

// DetectRequiredModules analiza el AST para determinar qué módulos JavaScript se necesitan
func DetectRequiredModules(astNode *ast.AST) []string {
	return DetectRequiredModulesWithConfig(astNode, DefaultModuleConfig())
}

// DetectRequiredModulesWithConfig analiza el AST con configuración específica
func DetectRequiredModulesWithConfig(astNode *ast.AST, config ModuleConfig) []string {
	modules := []string{"core"} // core siempre se incluye

	if astNode == nil {
		return modules
	}

	// Verificar si hay contenido que requiere módulos específicos
	hasMermaid := false
	hasCharts := false
	hasMaps := false
	hasDirectives := false
	hasCodeGroups := false
	hasCollapsibles := false

	// Recorrer los slides para detectar contenido especial
	for _, slide := range astNode.ContentBlocks {
		for _, element := range slide.Elements {
			switch elem := element.(type) {
			case *ast.CodeElement:
				// Detectar diagramas Mermaid
				if strings.HasPrefix(strings.ToLower(elem.Language), "mermaid") {
					hasMermaid = true
				}
			case *ast.SpecialBlockElement:
				switch strings.ToLower(elem.BlockType) {
				case "mermaid", "diagram":
					hasMermaid = true
				case "chart", "charts":
					hasCharts = true
				case "map", "maps":
					hasMaps = true
				case "code-group", "codegroup":
					hasCodeGroups = true
				case "details", "collapsible":
					hasCollapsibles = true
				}
			case *ast.MermaidElement:
				hasMermaid = true
			case *ast.ChartElement:
				hasCharts = true
			case *ast.MapElement:
				hasMaps = true
			case *ast.DirectiveNode:
				hasDirectives = true
			}
		}
	}

	// Detectar si hay directivas en el frontmatter
	if astNode.FrontMatter != nil {
		// Buscar directivas en las variables del frontmatter
		if astNode.FrontMatter.Variables != nil {
			for key := range astNode.FrontMatter.Variables {
				if strings.HasPrefix(key, "@") {
					hasDirectives = true
					break
				}
			}
		}
	}

	// Agregar módulos base según configuración
	if config.EnableNavigation {
		modules = append(modules, "navigation")
		// Incluir menú flotante por defecto con navegación
		modules = append(modules, "floatingMenu")
	}

	// Incluir responsive CSS siempre (a menos que se excluya explícitamente)
	if !contains(config.ExcludeModules, "responsive") {
		modules = append(modules, "responsive")
	}

	if config.EnableUtilities && (hasCodeGroups || hasCollapsibles) {
		modules = append(modules, "utilities")
	} else if config.EnableUtilities {
		// Incluir utilities por defecto si está habilitado
		modules = append(modules, "utilities")
	}

	// Agregar módulos según el contenido detectado
	if hasMermaid && !contains(config.ExcludeModules, "mermaid") {
		modules = append(modules, "mermaid")
	}
	if hasCharts && !contains(config.ExcludeModules, "charts") {
		modules = append(modules, "charts")
	}
	if hasMaps && !contains(config.ExcludeModules, "maps") {
		modules = append(modules, "maps")
	}
	if hasDirectives && !contains(config.ExcludeModules, "directives") {
		modules = append(modules, "directives")
	}

	// Agregar módulos forzados
	for _, module := range config.ForceModules {
		if !contains(modules, module) && !contains(config.ExcludeModules, module) {
			modules = append(modules, module)
		}
	}

	// Remover módulos excluidos
	filteredModules := []string{}
	for _, module := range modules {
		if !contains(config.ExcludeModules, module) {
			filteredModules = append(filteredModules, module)
		}
	}

	return filteredModules
}

// GetModuleCSS retorna el CSS necesario para los módulos
func GetModuleCSS(modules []string) string {
	css := ""

	for _, module := range modules {
		switch module {
		case "navigation":
			// Incluir CSS de navegación
			css += "/* Navigation CSS */\n"
			// Aquí se incluiría el CSS de navegación
		case "utilities":
			// CSS para utilities (si es necesario)
			css += "/* Utilities CSS */\n"
		}
	}

	return css
}

// contains verifica si un slice contiene un string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
