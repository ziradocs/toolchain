// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strconv"
	"strings"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// PresentationHasSlidesRule verifica que la presentación tenga al menos un slide
type PresentationHasSlidesRule struct{}

func (r *PresentationHasSlidesRule) Check(node ast.Node) []diagnostics.Diagnostic {
	if astNode, ok := node.(*ast.AST); ok {
		if len(astNode.ContentBlocks) == 0 {
			return []diagnostics.Diagnostic{
				diagnostics.NewError("Presentation must contain at least one slide",
					astNode.GetPosition(), "linter").WithRuleID("CORE001"),
			}
		}
	}
	return nil
}

// FrontMatterValidRule verifica que el FrontMatter sea válido
type FrontMatterValidRule struct{}

func (r *FrontMatterValidRule) Check(node ast.Node) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if astNode, ok := node.(*ast.AST); ok {
		if astNode.FrontMatter == nil {
			diags = append(diags,
				diagnostics.NewError("Missing FrontMatter",
					astNode.GetPosition(), "linter").WithRuleID("FRONT003"))
		} else {
			if astNode.FrontMatter.Mode == "" {
				diags = append(diags,
					diagnostics.NewError("FrontMatter must specify 'mode'",
						astNode.FrontMatter.GetPosition(), "linter").WithRuleID("FRONT001"))
			}
		}
	}

	return diags
}

// SlideNotEmptyRule verifica que los slides no estén vacíos
type SlideNotEmptyRule struct{}

func (r *SlideNotEmptyRule) Check(node ast.Node) []diagnostics.Diagnostic {
	if slide, ok := node.(*ast.ContentBlock); ok {
		if len(slide.Elements) == 0 && slide.Title == "" {
			return []diagnostics.Diagnostic{
				diagnostics.NewWarning("Slide appears to be empty (no title or elements)",
					slide.GetPosition(), "linter").WithRuleID("SLIDE002"),
			}
		}
	}
	return nil
}

// ImageHasSourceRule verifica que las imágenes tengan una fuente
type ImageHasSourceRule struct{}

func (r *ImageHasSourceRule) Check(node ast.Node) []diagnostics.Diagnostic {
	if slide, ok := node.(*ast.ContentBlock); ok {
		var diags []diagnostics.Diagnostic

		for _, element := range slide.Elements {
			if imageElement, ok := element.(*ast.ImageElement); ok {
				if imageElement.Source == "" {
					diags = append(diags,
						diagnostics.NewError("IMAGE element must have a source",
							imageElement.GetPosition(), "linter").WithRuleID("IMG001"))
				}
			}
		}

		return diags
	}
	return nil
}

// CodeHasContentRule verifica que los bloques de código no estén vacíos
type CodeHasContentRule struct{}

func (r *CodeHasContentRule) Check(node ast.Node) []diagnostics.Diagnostic {
	if slide, ok := node.(*ast.ContentBlock); ok {
		var diags []diagnostics.Diagnostic

		for _, element := range slide.Elements {
			if codeElement, ok := element.(*ast.CodeElement); ok {
				if strings.TrimSpace(codeElement.Content) == "" {
					diags = append(diags,
						diagnostics.NewWarning("CODE element appears to be empty",
							codeElement.GetPosition(), "linter").WithRuleID("CODE001"))
				}
			}
		}

		return diags
	}
	return nil
}

// ParseErrorDetectionRule detecta problemas comunes de sintaxis que pueden causar bucles infinitos
type ParseErrorDetectionRule struct{}

func (r *ParseErrorDetectionRule) Check(node ast.Node) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if astNode, ok := node.(*ast.AST); ok {
		// Verificar si hay slides con elementos mal formateados
		for i, slide := range astNode.ContentBlocks {
			// Verificar si el slide tiene título pero no elementos (posible error de sintaxis)
			if slide.Title == "" && len(slide.Elements) == 0 {
				diags = append(diags,
					diagnostics.NewWarning(
						"Slide appears empty - check syntax. Content should be indented under SLIDE declaration",
						slide.GetPosition(), "linter").WithRuleID("SYNTAX001"))
			}

			// Verificar slides consecutivos vacíos (posible bucle de parsing)
			if i > 0 && len(slide.Elements) == 0 && len(astNode.ContentBlocks[i-1].Elements) == 0 {
				diags = append(diags,
					diagnostics.NewError(
						"Multiple consecutive empty slides detected - possible parsing error",
						slide.GetPosition(), "linter").WithRuleID("PARSE001"))
			}
		}

		// Verificar si el AST parece mal formado (muy pocos slides vs contenido esperado)
		if len(astNode.ContentBlocks) == 1 && astNode.ContentBlocks[0].Title == "" && len(astNode.ContentBlocks[0].Elements) == 0 {
			diags = append(diags,
				diagnostics.NewWarning(
					"Document appears to have parsing issues - ensure content is properly indented",
					astNode.GetPosition(), "linter").WithRuleID("PARSE002"))
		}
	}

	return diags
}

// StrictModeValidationRule valida reglas específicas para modo strict
type StrictModeValidationRule struct{}

func (r *StrictModeValidationRule) Check(node ast.Node) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if astNode, ok := node.(*ast.AST); ok {
		// Solo aplicar si está en modo strict
		if astNode.FrontMatter != nil && astNode.FrontMatter.Mode == "strict" {
			// Validar que slides tipo "title" tengan heading o title
			for _, slide := range astNode.ContentBlocks {
				if slide.BlockType == "title" {
					if slide.Heading == "" && slide.Title == "" {
						diags = append(diags,
							diagnostics.NewError(
								"Title slides must have either 'heading' or 'title' property",
								slide.GetPosition(), "linter").WithRuleID("STRICT001"))
					}
				}

				// Validar que slides de contenido tengan al menos un elemento o título
				if slide.BlockType == "content" || slide.BlockType == "" {
					if slide.Title == "" && len(slide.Elements) == 0 {
						diags = append(diags,
							diagnostics.NewError(
								"Content slides must have either a title or content elements",
								slide.GetPosition(), "linter").WithRuleID("STRICT002"))
					}
				}
			}
		}
	}

	return diags
}

// ElementStructureRule valida la estructura de elementos
type ElementStructureRule struct{}

func (r *ElementStructureRule) Check(node ast.Node) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if slide, ok := node.(*ast.ContentBlock); ok {
		for _, element := range slide.Elements {
			switch elem := element.(type) {
			case *ast.TableElement:
				// Validar que las tablas tengan headers y al menos una fila
				if len(elem.Headers) == 0 {
					diags = append(diags,
						diagnostics.NewWarning(
							"TABLE element should have headers defined",
							elem.GetPosition(), "linter").WithRuleID("TABLE001"))
				}
				if len(elem.Rows) == 0 {
					diags = append(diags,
						diagnostics.NewWarning(
							"TABLE element should have at least one row",
							elem.GetPosition(), "linter").WithRuleID("TABLE002"))
				} // Validar que todas las filas tengan el mismo número de columnas
				if len(elem.Headers) > 0 {
					expectedCols := len(elem.Headers)
					for _, row := range elem.Rows {
						if len(row) != expectedCols {
							diags = append(diags,
								diagnostics.NewError(
									"Table row has incorrect number of columns",
									elem.GetPosition(), "linter").WithRuleID("TABLE003"))
							break
						}
					}
				}

			case *ast.CodeGroupElement:
				// Validar que los grupos de código tengan al menos un bloque
				if len(elem.CodeBlocks) == 0 {
					diags = append(diags,
						diagnostics.NewError(
							"Code groups must contain at least one code block",
							elem.GetPosition(), "linter").WithRuleID("CODEGROUP001"))
				}

			case *ast.SpecialBlockElement:
				// Detectar un ":::code-item{...}" huérfano, es decir, uno que
				// quedó fuera de un "::::code-group"/":::code-group" reconocido.
				// Este wrapper NO es sintaxis del parser: es azúcar que el
				// normalizador de IA (CodeGroupFormatterRule) debe reescribir
				// a la forma canónica ```lang [label] antes de llegar aquí. Si
				// aparece como SpecialBlockElement, algo falló en el parseo o
				// la normalización del code-group.
				if strings.HasPrefix(elem.BlockType, "code-item") {
					diags = append(diags,
						diagnostics.NewError(
							"Orphaned code-group tab: '"+elem.BlockType+"' looks like a "+
								":::code-item{...} tab that ended up outside a recognized "+
								"::::code-group/:::code-group block. This is likely caused by "+
								"a code-group parsing/normalization issue (the AI normalizer "+
								"should rewrite :::code-item{title=\"...\"} wrappers into "+
								"```lang [label] fenced blocks before parsing)",
							elem.GetPosition(), "linter").WithRuleID("CODEGROUP002"))
					break
				}

				// Validar tipos de bloques especiales
				validTypes := []string{"info", "warning", "danger", "success", "tip", "details"}
				isValid := false
				for _, validType := range validTypes {
					if elem.BlockType == validType {
						isValid = true
						break
					}
				}
				if !isValid {
					diags = append(diags,
						diagnostics.NewWarning(
							"Unknown special block type: "+elem.BlockType,
							elem.GetPosition(), "linter").WithRuleID("SPECIAL001"))
				}

			case *ast.ChartElement: // Validar que los gráficos tengan datos
				hasData := false

				// Verificar si tiene datos en formato tradicional (YAML)
				if len(elem.Data) > 0 || len(elem.Series) > 0 {
					hasData = true
				}

				// Verificar si tiene datos en formato JSON
				if elem.IsJSONMode && len(elem.RawJSON) > 0 {
					hasData = true
				}

				if !hasData {
					diags = append(diags,
						diagnostics.NewWarning(
							"Chart elements should have data defined",
							elem.GetPosition(), "linter").WithRuleID("CHART001"))
				}
			}
		}
	}

	return diags
}

// PropertyValidationRule valida propiedades específicas
type PropertyValidationRule struct{}

func (r *PropertyValidationRule) Check(node ast.Node) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if slide, ok := node.(*ast.ContentBlock); ok {
		// Validar que las propiedades de logo apunten a archivos válidos
		if slide.Logo != "" {
			// Verificar extensión de archivo
			validExts := []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp"}
			hasValidExt := false
			lowerLogo := strings.ToLower(slide.Logo)
			for _, ext := range validExts {
				if strings.HasSuffix(lowerLogo, ext) {
					hasValidExt = true
					break
				}
			}
			if !hasValidExt {
				diags = append(diags,
					diagnostics.NewWarning(
						"Logo should have a valid image file extension",
						slide.GetPosition(), "linter").WithRuleID("PROP001"))
			}
		}
	}

	return diags
}

// SlideLayoutValidationRule valida que los slides cumplan con los esquemas específicos de layout
type SlideLayoutValidationRule struct {
	// policy, si no-nil (inyectado vía setLayoutPolicy desde
	// Linter.WithPolicy), resuelve overrides de Min/MaxElements/
	// ForbiddenElements por tipo de layout antes de validar (issue #207).
	policy *PolicyConfig
}

// setLayoutPolicy implementa layoutPolicyAware (ver linter.go).
func (r *SlideLayoutValidationRule) setLayoutPolicy(p *PolicyConfig) {
	r.policy = p
}

func (r *SlideLayoutValidationRule) Check(node ast.Node) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if slide, ok := node.(*ast.ContentBlock); ok {
		slideType := slide.BlockType
		if slideType == "" {
			return diags // No validar slides sin tipo específico
		}

		// Obtener el esquema de validación para este tipo de slide
		schemas := GetSlideLayoutSchemas()
		schema, exists := schemas[slideType]
		if !exists {
			return diags // No validar tipos desconocidos
		}
		schema = r.policy.ResolveLayoutSchema(slideType, schema)

		// Ejecutar validaciones específicas de layout
		for _, rule := range schema.ValidationRules {
			if rule.Validator != nil {
				ruleDiags := rule.Validator(slide)
				diags = append(diags, ruleDiags...)
			}
		}

		// Validar límites básicos de elementos (solo warnings)
		diags = append(diags, validateElementCountLimits(slideType, schema, slide)...)

		// Validar elementos permitidos/prohibidos
		for _, element := range slide.Elements {
			elementType := string(element.GetType())

			// Verificar elementos prohibidos
			for _, forbidden := range schema.ForbiddenElements {
				if elementType == forbidden {
					diag := diagnostics.Diagnostic{
						Severity: diagnostics.Warning,
						Code:     "LAYOUT_FORBIDDEN_ELEMENT",
						Message:  "Element type '" + elementType + "' is not recommended for '" + slideType + "' slides",
						Position: element.GetPosition(),
						Source:   "linter",
					}
					diags = append(diags, diag)
				}
			}
		}
	}
	return diags
}

// validateElementCountLimits valida que la cantidad de elementos del slide
// esté dentro de los límites Min/MaxElements del schema, generando warnings
// LAYOUT_MIN_ELEMENTS / LAYOUT_MAX_ELEMENTS. Extraída como función standalone
// (en vez de vivir inline en Check) para poder testearse con un
// SlideLayoutSchema construido a mano — incluyendo límites >= 10, algo que
// ningún schema hardcodeado en GetSlideLayoutSchemas() alcanza hoy — sin
// depender del dispatch completo de Check() ni del mapa de schemas real.
func validateElementCountLimits(slideType string, schema SlideLayoutSchema, slide *ast.ContentBlock) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if schema.MinElements > 0 && len(slide.Elements) < schema.MinElements {
		diags = append(diags, diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT_MIN_ELEMENTS",
			Message:  "Slide type '" + slideType + "' should have at least " + strconv.Itoa(schema.MinElements) + " elements",
			Position: slide.Position,
			Source:   "linter",
		})
	}

	if schema.MaxElements > 0 && len(slide.Elements) > schema.MaxElements {
		diags = append(diags, diagnostics.Diagnostic{
			Severity: diagnostics.Warning,
			Code:     "LAYOUT_MAX_ELEMENTS",
			Message:  "Slide type '" + slideType + "' should have at most " + strconv.Itoa(schema.MaxElements) + " elements",
			Position: slide.Position,
			Source:   "linter",
		})
	}

	return diags
}

// Helper function to check if slide has required property
func hasRequiredProperty(slide *ast.ContentBlock, property string) bool {
	switch property {
	case "title":
		return slide.Title != ""
	case "heading":
		return slide.Heading != ""
	case "subtitle":
		return slide.Subtitle != ""
	case "logo":
		return slide.Logo != ""
	case "author":
		// This would typically come from frontmatter, but we can check slide-level if needed
		return true // For now, assume author is always available from frontmatter
	case "date":
		// This would typically come from frontmatter
		return true // For now, assume date is always available from frontmatter
	default:
		return false
	}
}

// LastSlideClosingRule detecta automáticamente el último slide como closing si no tiene título
type LastSlideClosingRule struct{}

func (r *LastSlideClosingRule) Check(node ast.Node) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	// Solo procesar cuando recibimos el AST completo
	if astNode, ok := node.(*ast.AST); ok {
		if len(astNode.ContentBlocks) == 0 {
			return diags
		}

		// Obtener el último slide
		lastSlideIndex := len(astNode.ContentBlocks) - 1
		lastSlide := &astNode.ContentBlocks[lastSlideIndex]

		// Verificar si el último slide no tiene título y no tiene layout específico
		if lastSlide.Title == "" && lastSlide.Heading == "" &&
			(lastSlide.BlockType == "" || lastSlide.BlockType == "content" || lastSlide.BlockType == "default") {

			// Auto-asignar layout closing al último slide sin título
			if lastSlide.BlockType == "" || lastSlide.BlockType == "content" || lastSlide.BlockType == "default" {
				lastSlide.BlockType = "closing"

				// Generar información de que se aplicó auto-detección
				diag := diagnostics.Diagnostic{
					Severity: diagnostics.Info,
					Code:     "LAYOUT_AUTO_CLOSING",
					Message:  "Last slide automatically detected as 'closing' layout (no title required)",
					Position: lastSlide.Position,
					Source:   "linter",
				}
				diags = append(diags, diag)
			}
		}
	}

	return diags
}
