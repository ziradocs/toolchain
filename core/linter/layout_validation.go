// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// SlideLayoutSchema define las propiedades válidas para cada tipo de slide
type SlideLayoutSchema struct {
	RequiredProperties []string               // Propiedades obligatorias
	OptionalProperties []string               // Propiedades opcionales
	AllowedElements    []string               // Tipos de elementos permitidos
	ForbiddenElements  []string               // Tipos de elementos prohibidos
	MinElements        int                    // Mínimo número de elementos
	MaxElements        int                    // Máximo número de elementos (0 = ilimitado)
	ValidationRules    []LayoutValidationRule // Reglas de validación específicas
}

// LayoutValidationRule define una regla de validación específica
type LayoutValidationRule struct {
	Name        string
	Description string
	Validator   func(*ast.ContentBlock) []diagnostics.Diagnostic
}

// GetSlideLayoutSchemas retorna los esquemas de validación para cada tipo de slide
func GetSlideLayoutSchemas() map[string]SlideLayoutSchema {
	return map[string]SlideLayoutSchema{
		"title": {
			RequiredProperties: []string{"heading"},
			OptionalProperties: []string{"subtitle", "logo", "author", "date"},
			AllowedElements:    []string{}, // Solo propiedades, no elementos de contenido
			ForbiddenElements:  []string{"text", "code", "points", "table", "image"},
			MinElements:        0,
			MaxElements:        0,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "title_requires_heading",
					Description: "Title slides must have a heading property",
					Validator:   validateTitleSlideHeading,
				},
				{
					Name:        "title_no_content_elements",
					Description: "Title slides should not contain content elements",
					Validator:   validateTitleSlideElements,
				},
			},
		},
		"title_slide": {
			RequiredProperties: []string{"heading"},
			OptionalProperties: []string{"subtitle", "logo"},
			AllowedElements:    []string{},
			ForbiddenElements:  []string{"text", "code", "points", "table"},
			MinElements:        0,
			MaxElements:        1, // Máximo un elemento decorativo
		},
		"content": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{},
			AllowedElements:    []string{"text", "code", "points", "table", "image", "special_block", "mermaid", "chart", "map", "directive"},
			ForbiddenElements:  []string{},
			MinElements:        1,
			MaxElements:        0, // Ilimitado
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "content_requires_title",
					Description: "Content slides must have a title",
					Validator:   validateContentSlideTitle,
				},
				{
					Name:        "content_requires_elements",
					Description: "Content slides must have at least one content element",
					Validator:   validateContentSlideElements,
				},
			},
		},
		"section": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{"subtitle"},
			AllowedElements:    []string{"text", "points"},
			ForbiddenElements:  []string{"code", "table", "chart", "map"},
			MinElements:        1,
			MaxElements:        3,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "section_simple_content",
					Description: "Section slides should have simple, introductory content",
					Validator:   validateSectionSlideSimplicity,
				},
			},
		},
		"comparison": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{},
			AllowedElements:    []string{"text", "points", "table", "special_block"},
			ForbiddenElements:  []string{"code", "mermaid", "chart"},
			MinElements:        2,
			MaxElements:        4,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "comparison_balanced_content",
					Description: "Comparison slides should have balanced content sections",
					Validator:   validateComparisonSlideBalance,
				},
			},
		},
		"stats": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{},
			AllowedElements:    []string{"text", "chart", "table", "special_block"},
			ForbiddenElements:  []string{"code", "mermaid"},
			MinElements:        1,
			MaxElements:        3,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "stats_requires_data",
					Description: "Stats slides should contain charts or tables with data",
					Validator:   validateStatsSlideData,
				},
			},
		},
		"code_example": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{},
			AllowedElements:    []string{"text", "code", "points"},
			ForbiddenElements:  []string{"table", "chart", "map"},
			MinElements:        1,
			MaxElements:        4,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "code_example_requires_code",
					Description: "Code example slides must contain at least one code block",
					Validator:   validateCodeExampleSlide,
				},
			},
		},
		"hero": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{"subtitle", "background", "cta", "overlay"},
			AllowedElements:    []string{"text", "image", "special_block"},
			ForbiddenElements:  []string{"code", "table", "chart"},
			MinElements:        0,
			MaxElements:        3,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "hero_requires_title",
					Description: "Hero slides must have a title",
					Validator:   validateHeroSlideTitle,
				},
			},
		},
		"testimonial": {
			RequiredProperties: []string{},
			OptionalProperties: []string{"quote", "author", "position", "company", "avatar", "rating"},
			AllowedElements:    []string{"text", "image", "special_block"},
			ForbiddenElements:  []string{"code", "table", "chart"},
			MinElements:        1,
			MaxElements:        3,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "testimonial_requires_quote_and_author",
					Description: "Testimonial slides should include quote and author information",
					Validator:   validateTestimonialSlide,
				},
			},
		},
		"timeline": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{"events"},
			AllowedElements:    []string{"text", "points", "special_block"},
			ForbiddenElements:  []string{"code", "table", "chart"},
			MinElements:        2,
			MaxElements:        6,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "timeline_requires_multiple_events",
					Description: "Timeline slides should have at least 2 temporal events",
					Validator:   validateTimelineSlide,
				},
			},
		},
		"before_after": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{},
			AllowedElements:    []string{"text", "image", "points", "special_block"},
			ForbiddenElements:  []string{"code", "chart"},
			MinElements:        2,
			MaxElements:        4,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "before_after_requires_both_sections",
					Description: "Before/after slides must have both \"before\" and \"after\" sections",
					Validator:   validateBeforeAfterSlide,
				},
			},
		},
		"pricing": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{"plans"},
			AllowedElements:    []string{"text", "table", "special_block"},
			ForbiddenElements:  []string{"code", "chart", "mermaid"},
			MinElements:        1,
			MaxElements:        4,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "pricing_requires_plans",
					Description: "Pricing slides should include at least one plan with price",
					Validator:   validatePricingSlide,
				},
			},
		},
		"team": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{"members"},
			AllowedElements:    []string{"text", "image", "special_block"},
			ForbiddenElements:  []string{"code", "chart", "table"},
			MinElements:        1,
			MaxElements:        8,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "team_requires_members",
					Description: "Team slides should include at least one member with role",
					Validator:   validateTeamSlide,
				},
			},
		},
		"feature_showcase": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{"features"},
			AllowedElements:    []string{"text", "points", "image", "special_block"},
			ForbiddenElements:  []string{"code", "table"},
			MinElements:        2,
			MaxElements:        6,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "feature_showcase_requires_multiple_features",
					Description: "Feature showcase slides should highlight at least 2 features",
					Validator:   validateFeatureShowcaseSlide,
				},
			},
		},
		"call_to_action": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{"subtitle", "primary_cta", "secondary_cta", "urgency"},
			AllowedElements:    []string{"text", "special_block"},
			ForbiddenElements:  []string{"code", "table", "chart", "mermaid"},
			MinElements:        1,
			MaxElements:        3,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "cta_requires_action",
					Description: "Call-to-action slides must include at least one call-to-action",
					Validator:   validateCallToActionSlide,
				},
			},
		},
		"dashboard": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{"widgets"},
			AllowedElements:    []string{"text", "chart", "table", "special_block"},
			ForbiddenElements:  []string{"code", "mermaid"},
			MinElements:        1,
			MaxElements:        6,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "dashboard_requires_metrics",
					Description: "Dashboard slides should include metrics or charts",
					Validator:   validateDashboardSlide,
				},
			},
		},
		"process": {
			RequiredProperties: []string{"title"},
			OptionalProperties: []string{"steps"},
			AllowedElements:    []string{"text", "points", "special_block"},
			ForbiddenElements:  []string{"code", "table", "chart"},
			MinElements:        2,
			MaxElements:        6,
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "process_requires_multiple_steps",
					Description: "Process slides should have at least 2 sequential steps",
					Validator:   validateProcessSlide,
				},
			},
		},
		"default": {
			RequiredProperties: []string{},
			OptionalProperties: []string{"title"},
			AllowedElements:    []string{"text", "code", "points", "table", "image", "special_block", "mermaid", "chart", "map", "directive"},
			ForbiddenElements:  []string{},
			MinElements:        0,
			MaxElements:        0, // Ilimitado
		},
		"closing": {
			RequiredProperties: []string{}, // No requiere propiedades específicas
			OptionalProperties: []string{"heading", "subtitle", "logo", "contact", "author"},
			AllowedElements:    []string{"text", "image", "points"}, // Permite texto simple e imágenes
			ForbiddenElements:  []string{"code", "table", "chart", "mermaid", "map"},
			MinElements:        0,
			MaxElements:        3, // Mantener simple
			ValidationRules: []LayoutValidationRule{
				{
					Name:        "closing_simple_content",
					Description: "Closing slides should have simple, conclusive content",
					Validator:   validateClosingSlideSimplicity,
				},
			},
		},
	}
}

// Funciones de validación específicas

func validateTitleSlideHeading(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if slide.Heading == "" {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Error,
			Code:     "LAYOUT001",
			Message:  "Title slides must have a 'heading' property",
			Position: slide.Position,
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateTitleSlideElements(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	for _, element := range slide.Elements {
		elementType := string(element.GetType())
		if elementType == "text" || elementType == "code" || elementType == "points" {
			diag := diagnostics.Diagnostic{
				Severity: diagnostics.Warning,
				Code:     "LAYOUT002",
				Message:  "Title slides typically should not contain content elements like " + elementType,
				Position: element.GetPosition(),
			}
			diags = append(diags, diag)
		}
	}

	return diags
}

// validateContentSlideTitle degrada LAYOUT003 a Warning siempre: un slide
// "content" sin título es un diseño deliberado (p. ej. una cita/testimonio
// de pantalla completa), no bloquea el build (issue #103, Causa A).
// Layouts headless de primera clase (quote/testimonial/etc. con su propio
// schema) quedan como follow-up (#150); esto desbloquea el corpus
// existente sin ampliar el schema de layouts antes del lanzamiento OSS.
//
// No hay caso especial para "sin título Y sin elementos": ese escenario ya
// lo cubre validateContentSlideElements (LAYOUT004) como Error por separado
// — ambos validators corren juntos en el schema "content" (ver
// ValidationRules más abajo), así que un slide vacío sigue fallando el
// build vía LAYOUT004 sin que LAYOUT003 necesite escalar de vuelta a Error
// para el mismo caso (hallazgo de code-review de PR #152: la rama especial
// original era puramente redundante, nunca cambiaba el resultado pass/fail).
func validateContentSlideTitle(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if slide.Title == "" {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT003",
			Message:  "Content slides must have a 'title' property",
			Position: slide.Position,
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateContentSlideElements(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if len(slide.Elements) == 0 {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Error,
			Code:     "LAYOUT004",
			Message:  "Content slides must have at least one content element",
			Position: slide.Position,
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateSectionSlideSimplicity(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	complexElements := 0
	for _, element := range slide.Elements {
		elementType := string(element.GetType())
		if elementType == "code" || elementType == "table" || elementType == "chart" || elementType == "mermaid" {
			complexElements++
		}
	}

	if complexElements > 0 {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT005",
			Message:  "Section slides should contain simple content (text, points) rather than complex elements",
			Position: slide.Position,
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateComparisonSlideBalance(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if len(slide.Elements) < 2 {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT006",
			Message:  "Comparison slides should have at least 2 elements to compare",
			Position: slide.Position,
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateStatsSlideData(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	hasDataElement := false
	for _, element := range slide.Elements {
		elementType := string(element.GetType())
		if elementType == "chart" || elementType == "table" {
			hasDataElement = true
			break
		}
	}

	if !hasDataElement {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT007",
			Message:  "Stats slides should contain at least one chart or table element",
			Position: slide.Position,
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateCodeExampleSlide(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	hasCodeElement := false
	for _, element := range slide.Elements {
		if element.GetType() == ast.NodeTypeCode {
			hasCodeElement = true
			break
		}
	}

	if !hasCodeElement {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Error,
			Code:     "LAYOUT008",
			Message:  "Code example slides must contain at least one code block",
			Position: slide.Position,
		}
		diags = append(diags, diag)
	}

	return diags
}

// Validaciones para nuevos layouts especializados

func validateHeroSlideTitle(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if slide.Title == "" {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Error,
			Code:     "LAYOUT009",
			Message:  "Hero slides must have a title",
			Position: slide.Position,
			Source:   "linter",
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateTestimonialSlide(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	// Buscar contenido que parezca una cita o testimonio
	hasQuote := false

	for _, element := range slide.Elements {
		if element.GetType() == ast.NodeTypeText {
			if textElement, ok := element.(*ast.TextElement); ok {
				// Buscar patrones de cita (líneas que empiecen con > o contengan comillas)
				if len(textElement.Content) > 0 && (textElement.Content[0] == '"' || textElement.Content[0] == '\'' || strings.HasPrefix(textElement.Content, ">")) {
					hasQuote = true
				}
			}
		}
		if element.GetType() == ast.NodeTypeSpecialBlock {
			if blockElement, ok := element.(*ast.SpecialBlockElement); ok {
				if blockElement.BlockType == "quote" || blockElement.BlockType == "testimonial" {
					hasQuote = true
				}
			}
		}
	}

	// En modo flex, se puede detectar autor en texto que siga ciertos patrones
	// En modo strict, se esperaría propiedades específicas

	if !hasQuote {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT010",
			Message:  "Testimonial slides should include quote and author information",
			Position: slide.Position,
			Source:   "linter",
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateTimelineSlide(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if len(slide.Elements) < 2 {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT011",
			Message:  "Timeline slides should have at least 2 temporal events",
			Position: slide.Position,
			Source:   "linter",
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateBeforeAfterSlide(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	hasBeforeSection := false
	hasAfterSection := false

	for _, element := range slide.Elements {
		if element.GetType() == ast.NodeTypeText {
			if textElement, ok := element.(*ast.TextElement); ok {
				content := textElement.Content
				if len(content) > 0 {
					lowerContent := strings.ToLower(content)
					if strings.Contains(lowerContent, "antes") || strings.Contains(lowerContent, "before") {
						hasBeforeSection = true
					}
					if strings.Contains(lowerContent, "después") || strings.Contains(lowerContent, "after") {
						hasAfterSection = true
					}
				}
			}
		}
	}

	if !hasBeforeSection || !hasAfterSection {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT012",
			Message:  "Before/after slides must have both \"before\" and \"after\" sections",
			Position: slide.Position,
			Source:   "linter",
		}
		diags = append(diags, diag)
	}

	return diags
}

func validatePricingSlide(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	hasPricing := false

	for _, element := range slide.Elements {
		if element.GetType() == ast.NodeTypeTable {
			hasPricing = true
			break
		}
		if element.GetType() == ast.NodeTypeText {
			if textElement, ok := element.(*ast.TextElement); ok {
				content := textElement.Content
				// Buscar símbolos de moneda o patrones de precio
				if strings.Contains(content, "$") || strings.Contains(content, "€") ||
					strings.Contains(content, "£") || strings.Contains(content, "precio") ||
					strings.Contains(content, "price") || strings.Contains(content, "/mes") ||
					strings.Contains(content, "/month") {
					hasPricing = true
					break
				}
			}
		}
	}

	if !hasPricing {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT013",
			Message:  "Pricing slides should include at least one plan with price",
			Position: slide.Position,
			Source:   "linter",
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateTeamSlide(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	hasTeamMember := false

	for _, element := range slide.Elements {
		if element.GetType() == ast.NodeTypeImage {
			hasTeamMember = true
			break
		}
		if element.GetType() == ast.NodeTypeText {
			if textElement, ok := element.(*ast.TextElement); ok {
				content := textElement.Content
				// Buscar patrones típicos de información de equipo
				if strings.Contains(content, "CEO") || strings.Contains(content, "CTO") ||
					strings.Contains(content, "Director") || strings.Contains(content, "Manager") ||
					strings.Contains(content, "@") { // Email pattern
					hasTeamMember = true
					break
				}
			}
		}
	}

	if !hasTeamMember {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT014",
			Message:  "Team slides should include at least one member with role",
			Position: slide.Position,
			Source:   "linter",
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateFeatureShowcaseSlide(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if len(slide.Elements) < 2 {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT015",
			Message:  "Feature showcase slides should highlight at least 2 features",
			Position: slide.Position,
			Source:   "linter",
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateCallToActionSlide(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	hasCTA := false

	for _, element := range slide.Elements {
		if element.GetType() == ast.NodeTypeText {
			if textElement, ok := element.(*ast.TextElement); ok {
				content := textElement.Content
				// Buscar patrones típicos de CTA
				if strings.Contains(content, "Comenzar") || strings.Contains(content, "Empezar") ||
					strings.Contains(content, "Registr") || strings.Contains(content, "Prueba") ||
					strings.Contains(content, "Descargar") || strings.Contains(content, "Contactar") ||
					strings.Contains(content, "Start") || strings.Contains(content, "Try") ||
					strings.Contains(content, "Download") || strings.Contains(content, "Contact") ||
					strings.Contains(content, "Sign up") || strings.Contains(content, "Get started") {
					hasCTA = true
					break
				}
			}
		}
		if element.GetType() == ast.NodeTypeSpecialBlock {
			if blockElement, ok := element.(*ast.SpecialBlockElement); ok {
				if blockElement.BlockType == "info" || blockElement.BlockType == "success" {
					hasCTA = true
					break
				}
			}
		}
	}

	if !hasCTA {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT016",
			Message:  "Call-to-action slides must include at least one call-to-action",
			Position: slide.Position,
			Source:   "linter",
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateDashboardSlide(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	hasDataElement := false

	for _, element := range slide.Elements {
		elementType := string(element.GetType())
		if elementType == "chart" || elementType == "table" {
			hasDataElement = true
			break
		}
		if element.GetType() == ast.NodeTypeText {
			if textElement, ok := element.(*ast.TextElement); ok {
				content := textElement.Content
				// Buscar patrones de métricas
				if strings.Contains(content, "%") || strings.Contains(content, "$") ||
					strings.Contains(content, "€") || strings.Contains(content, "usuarios") ||
					strings.Contains(content, "users") || strings.Contains(content, "métricas") ||
					strings.Contains(content, "metrics") {
					hasDataElement = true
					break
				}
			}
		}
	}

	if !hasDataElement {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT017",
			Message:  "Dashboard slides should include metrics or charts",
			Position: slide.Position,
			Source:   "linter",
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateProcessSlide(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if len(slide.Elements) < 2 {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT018",
			Message:  "Process slides should have at least 2 sequential steps",
			Position: slide.Position,
			Source:   "linter",
		}
		diags = append(diags, diag)
	}

	return diags
}

func validateClosingSlideSimplicity(slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	// Verificar que el contenido sea apropiado para un slide de cierre
	complexElements := 0
	for _, element := range slide.Elements {
		elementType := string(element.GetType())
		if elementType == "code" || elementType == "table" || elementType == "chart" || elementType == "mermaid" || elementType == "map" {
			complexElements++
		}
	}

	if complexElements > 0 {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT006",
			Message:  "Closing slides should contain simple content (text, images, contact info) rather than complex elements",
			Position: slide.Position,
		}
		diags = append(diags, diag)
	}

	// Verificar que no tenga demasiados elementos
	if len(slide.Elements) > 3 {
		diag := diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT007",
			Message:  "Closing slides should be kept simple with 3 or fewer elements",
			Position: slide.Position,
		}
		diags = append(diags, diag)
	}

	return diags
}
