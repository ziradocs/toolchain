// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalizer

import (
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/rules/content"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/rules/enhancement"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/rules/frontmatter"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/rules/structure"
	"go.ziradocs.com/core/v2/util"
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
		// Acá vivía ElementClosingTagsRule, que decía normalizar un `>>`
		// suelto a `<</element>>`. Se borró por muerta y por mal encarada:
		//
		//   - Nunca podía dispararse. Su guard de apertura era
		//     `HasPrefix(trimmed, "<<chart") && !HasSuffix(trimmed, ">>")`
		//     (misma forma para map y plantuml), y toda apertura bien formada
		//     termina en `>>` (`<<chart: bar>>`, `<<map>>`, `<<plantuml>>`).
		//     Sus tests solo ejercitaban aperturas SIN cerrar (`<<chart` a
		//     secas), que la gramática no admite — por eso el hueco pasó
		//     desapercibido. Verificado a mano: un `>>` cerrando un `<<map>>`
		//     o un `<<plantuml>>` seguía saliendo como un `> >` suelto.
		//   - Emitía `<</plantuml>>`, que internal/elements/plantuml.go no
		//     acepta (solo toma `<<end>>`).
		//   - Y aunque se le arreglara el guard, no serviría: el normalizador
		//     entero corre solo cuando el Detector puntúa el documento por
		//     encima de 0.3, así que `>>` andaría o no según cosas ajenas
		//     (p. ej. si el documento trae un mermaid sin indentar). Una
		//     cuestión de sintaxis no puede depender de eso.
		//
		// Si algún día se quiere tolerar `>>` como terminador, va en los
		// parsers de internal/elements/ (determinista) y en
		// spec/language-specification.md, que hoy dice
		// `element_terminator ::= "<<end>>" | block_boundary | EOF`.

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
		enhancement.NewChartJSONRule(), // Limpia comentarios inline y comas finales en el JSON de un chart
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
