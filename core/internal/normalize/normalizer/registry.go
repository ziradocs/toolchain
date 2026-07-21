// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalizer

import (
	"go.ziradocs.com/core/internal/normalize/normalizer/base"
	"go.ziradocs.com/core/internal/normalize/normalizer/rules/content"
	"go.ziradocs.com/core/internal/normalize/normalizer/rules/enhancement"
	"go.ziradocs.com/core/internal/normalize/normalizer/rules/frontmatter"
	"go.ziradocs.com/core/internal/normalize/normalizer/rules/structure"
	"go.ziradocs.com/core/util"
)

// GetTransformRules retorna todas las reglas de transformación en orden de prioridad
func GetTransformRules(log util.Logger) []base.TransformRule {
	return []base.TransformRule{
		// Prioridad -1 - ESCAPE YAML PRIMERO (antes que cualquier transformación)
		frontmatter.NewYamlEscapingRule(), // NUEVA: Escapa valores YAML problemáticos

		// Prioridad 0 - Limpieza inicial
		&frontmatter.BackticksCleanupRule{},
		// Prioridad 1 - Frontmatter y tags
		frontmatter.NewInjectionRule(),
		enhancement.NewElementClosingTagsRule(), // NUEVA: Normaliza >> a <</element>>

		// Prioridad 2 - Estructura básica
		&structure.SeparatorsRule{},
		structure.NewMarkdownSlideStructureRule(), // NUEVA: Corrige patrón # + múltiples ##
		content.NewTitleSubtitleRule(),

		// Prioridad 3 - Contenido y slides
		content.NewHeadersRule(),
		// structure.NewTitleSlideRule(), // DISABLED: Conflicts with DocLang first section
		enhancement.NewCodeGroupFormatterRule(), // NUEVA: Normaliza sintaxis de code-groups
		enhancement.NewGraphicsRule("comment"),  // Convertir placeholders a charts válidos
		// Prioridad 4-5 - Mejoras finales
		enhancement.NewMermaidRule(),
		enhancement.NewChartJSONRule(), // NUEVA: Convierte JSON de Chart.js a YAML
		enhancement.NewMermaidFormatterRule(),
		enhancement.NewMermaidSyntaxFixerRule(), // NUEVA: Corrige sintaxis específica de Mermaid
		enhancement.NewChartFormatterRule(),
		enhancement.NewMapFormatterRule(), // NUEVA: Formatea mapas con indentación YAML correcta
		enhancement.NewImagesRule(),
		enhancement.NewTablesRule(),
	}
}

// GetRuleSet crea un RuleSet con todas las reglas
func GetRuleSet(log util.Logger) *base.RuleSet {
	rules := GetTransformRules(log)
	return base.NewRuleSet(rules)
}
