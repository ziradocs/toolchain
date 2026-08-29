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

// Dialect identifica cuál de los dos DSLs se está normalizando. El
// normalizador lo recibe de quien lo llama (parser.Parser para slidelang,
// parser.DocumentFlexParser para doclang), que lo sabe con certeza, en vez de
// adivinarlo por la forma del contenido.
//
// Existe porque adivinarlo salió mal: HeadersRule (rules/content/headers.go)
// es una heurística de slidelang — "un solo '##' titula el slide, cualquier
// otro '##' de la misma región delimitada por '---' es contenido" — y en
// doclang los '##' son subsecciones legítimas de la sección que abre un '#'.
// Sin este dato, HeadersRule degradaba a "**negrita**" el segundo y siguientes
// '##' de cada sección de un documento, en `doclang build` y no solo en `fmt`:
// el heading desaparecía de la jerarquía y del TOC, sin diagnóstico.
type Dialect int

const (
	// DialectAny es el cero: quien llama no declaró dialecto, así que se
	// aplican todas las reglas (el comportamiento previo a que esto
	// existiera). No es un descuido tolerado: hay entry points del
	// normalizador fuera de los dos parsers, y romperlos por omisión sería
	// peor que dejarlos como estaban.
	DialectAny Dialect = iota
	// DialectSlides es slidelang: cada bloque es un slide.
	DialectSlides
	// DialectDocuments es doclang: cada bloque es una sección, con jerarquía
	// de subsecciones adentro.
	DialectDocuments
)

// String devuelve el nombre del dialecto
func (d Dialect) String() string {
	switch d {
	case DialectSlides:
		return "slides"
	case DialectDocuments:
		return "documents"
	default:
		return "any"
	}
}

// DialectScopedRule es la interfaz OPCIONAL (mismo patrón que CategorizedRule
// más abajo) que implementa una regla cuya premisa solo vale para un dialecto.
// Una regla que no la implementa se aplica siempre, que es el caso de casi
// todas.
//
// Hoy la implementa solo HeadersRule. TitleSubtitleRule
// (rules/content/title_subtitle.go) y MarkdownSlideStructureRule
// (rules/structure/markdown_slides.go) son las siguientes candidatas — ambas
// ya traen su propio isDocLangDocument, dos implementaciones distintas que en
// realidad chequean `lines[0] == "---"` bajo un nombre de dialecto. Migrarlas
// NO es mecánico y por eso no se hizo acá: como todo `.slidelang` arranca con
// frontmatter y las dos rutas de slidelang le pasan el documento completo al
// normalizador, ese guard hoy las apaga para prácticamente todo slidelang bien
// formado. Cambiarlo por `dialect == slides → aplicar` las volvería a ENCENDER
// para slidelang, un cambio de comportamiento en dirección contraria a este
// arreglo y que necesita su propia validación contra el corpus de slidelang.
type DialectScopedRule interface {
	TransformRule

	// AppliesTo reporta si la regla debe correr para d.
	AppliesTo(d Dialect) bool
}

// FilterByDialect descarta las reglas que declaran no aplicar a d. Con
// DialectAny no descarta nada.
func FilterByDialect(rules []TransformRule, d Dialect) []TransformRule {
	if d == DialectAny {
		return rules
	}
	filtered := make([]TransformRule, 0, len(rules))
	for _, rule := range rules {
		if scoped, ok := rule.(DialectScopedRule); ok && !scoped.AppliesTo(d) {
			continue
		}
		filtered = append(filtered, rule)
	}
	return filtered
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
