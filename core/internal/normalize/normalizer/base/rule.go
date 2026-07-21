// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package base

// RuleCategory define las categorías de reglas de transformación
type RuleCategory int

const (
	CategoryFrontmatter RuleCategory = iota // Reglas que procesan frontmatter
	CategoryStructure                       // Reglas que afectan estructura del documento/slides
	CategoryContent                         // Reglas que procesan contenido dentro de slides
	CategoryEnhancement                     // Reglas que mejoran/enriquecen el contenido
)

// String devuelve el nombre de la categoría
func (c RuleCategory) String() string {
	switch c {
	case CategoryFrontmatter:
		return "Frontmatter"
	case CategoryStructure:
		return "Structure"
	case CategoryContent:
		return "Content"
	case CategoryEnhancement:
		return "Enhancement"
	default:
		return "Unknown"
	}
}

// TransformRule representa una regla de transformación específica
type TransformRule interface {
	// Apply aplica la regla al contenido y retorna el contenido transformado
	Apply(content string) (string, error)

	// Description retorna una descripción de lo que hace la regla
	Description() string

	// Priority retorna la prioridad de ejecución (menor número = mayor prioridad)
	Priority() int
}

// CategorizedRule extiende TransformRule con información de categoría
type CategorizedRule interface {
	TransformRule
	Category() RuleCategory
}

// RuleSet organiza las reglas por categorías para mejor control de ejecución
type RuleSet struct {
	frontmatterRules []TransformRule
	structureRules   []TransformRule
	contentRules     []TransformRule
	enhancementRules []TransformRule
}

// NewRuleSet crea un nuevo conjunto de reglas organizadas por categorías
func NewRuleSet(rules []TransformRule) *RuleSet {
	ruleSet := &RuleSet{
		frontmatterRules: make([]TransformRule, 0),
		structureRules:   make([]TransformRule, 0),
		contentRules:     make([]TransformRule, 0),
		enhancementRules: make([]TransformRule, 0),
	}

	// Clasificar reglas por categoría
	for _, rule := range rules {
		if categorizedRule, ok := rule.(CategorizedRule); ok {
			switch categorizedRule.Category() {
			case CategoryFrontmatter:
				ruleSet.frontmatterRules = append(ruleSet.frontmatterRules, rule)
			case CategoryStructure:
				ruleSet.structureRules = append(ruleSet.structureRules, rule)
			case CategoryContent:
				ruleSet.contentRules = append(ruleSet.contentRules, rule)
			case CategoryEnhancement:
				ruleSet.enhancementRules = append(ruleSet.enhancementRules, rule)
			}
		} else {
			// Reglas legacy sin categoría van a Enhancement por defecto
			ruleSet.enhancementRules = append(ruleSet.enhancementRules, rule)
		}
	}

	return ruleSet
}

// GetAllRules devuelve todas las reglas en orden de categoría
func (rs *RuleSet) GetAllRules() []TransformRule {
	var allRules []TransformRule
	allRules = append(allRules, rs.frontmatterRules...)
	allRules = append(allRules, rs.structureRules...)
	allRules = append(allRules, rs.contentRules...)
	allRules = append(allRules, rs.enhancementRules...)
	return allRules
}

// GetRulesByCategory devuelve reglas de una categoría específica
func (rs *RuleSet) GetRulesByCategory(category RuleCategory) []TransformRule {
	switch category {
	case CategoryFrontmatter:
		return rs.frontmatterRules
	case CategoryStructure:
		return rs.structureRules
	case CategoryContent:
		return rs.contentRules
	case CategoryEnhancement:
		return rs.enhancementRules
	default:
		return []TransformRule{}
	}
}
