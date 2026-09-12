// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package modules

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
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
	hasQuizPoll := false
	hasMaps := false
	hasDirectives := false

	// Recorrer los slides para detectar contenido especial
	for _, slide := range astNode.ContentBlocks {
		for _, element := range slide.Elements {
			// No hay case para CodeElement, SpecialBlockElement ni
			// CodeGroupElement, y es deliberado:
			//
			//   - CodeElement salió porque decidía por el LENGUAJE del fence:
			//     un `Language: "mermaid"` pedía el módulo de Mermaid, pero
			//     renderiza `.slidelang-code`, no el `.slidelang-mermaid`
			//     anidado que el módulo busca. Un ```mermaid escrito en el
			//     fuente llega como MermaidElement y entra por su propio case,
			//     abajo; el que caía acá era el construido por API, con un
			//     módulo empaquetado que no tenía a qué engancharse.
			//
			//   - SpecialBlockElement salió porque decidía por el NOMBRE del
			//     bloque: `::: chart` y sus hermanos emiten un contenedor de
			//     prosa —sin `.slidelang-chart-canvas`, sin
			//     `.slidelang-map-container`, sin `.slidelang-tab`— y pedían
			//     charts/maps/mermaid/utilities igual.
			//
			//   - CodeGroupElement (y el `details` de SpecialBlockElement)
			//     solo alimentaban las banderas `hasCodeGroups`/
			//     `hasCollapsibles`, que no decidían nada: ver el gate de
			//     `utilities` más abajo. Si la carga de utilities se vuelve
			//     condicional algún día, estos dos casos vuelven junto con el
			//     código suelto, que es el otro tercio del predicado real.
			switch element.(type) {
			case *ast.QuizElement, *ast.PollElement:
				hasQuizPoll = true
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

	// Las dos ramas que había acá —una con `hasCodeGroups || hasCollapsibles`,
	// la otra sin— agregaban EXACTAMENTE el mismo módulo, así que ninguna de
	// las dos banderas decidía nada: `utilities` entra siempre que la opción
	// esté prendida. Escribirlo de una sola forma es lo que el código hace, y
	// borra la sugerencia de una carga condicional que nunca existió.
	//
	// Hacer la carga condicional DE VERDAD es otra decisión y otra PR: el
	// predicado no serían code-group y details, porque `initCopyButtons`
	// (template/utilities.go) le cuelga un botón a TODO `.slidelang-code`, así
	// que habría que contar también el código suelto — y el bundle cambiaría
	// en todos los decks que no tienen ninguno de los tres. Esta PR es sobre
	// el metadato.
	//
	// El `!contains(ExcludeModules, …)` es por simetría con los demás módulos.
	// Hoy es redundante —`--no-utilities` apaga EnableUtilities y además mete
	// "utilities" en la lista de exclusión (html_modular.go buildExcludeList)—
	// pero era el único módulo que no consultaba la lista.
	if config.EnableUtilities && !contains(config.ExcludeModules, "utilities") {
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
	if hasQuizPoll && !contains(config.ExcludeModules, "quizpoll") {
		modules = append(modules, "quizpoll")
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
