// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
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

// StrictModeValidationRule valida reglas específicas para modo strict.
//
// Aplica a los DOS dialectos strict —el de presentaciones (bloques SLIDE) y
// el de documentos (bloques SECTION)— porque valida el ContentBlock, que es
// deliberadamente genérico: una diapositiva en un CLI y una sección en el
// otro. Por eso los mensajes dicen "block" y no "slide": el mismo
// diagnóstico le sale a un `.doclang`, y hablarle de diapositivas a quien
// escribe un documento manda a buscar un problema que no existe.
type StrictModeValidationRule struct{}

func (r *StrictModeValidationRule) Check(node ast.Node) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if astNode, ok := node.(*ast.AST); ok {
		// Solo aplicar si está en modo strict
		if astNode.FrontMatter != nil && astNode.FrontMatter.Mode == "strict" {
			for _, block := range astNode.ContentBlocks {
				// STRICT001 ("los bloques tipo title necesitan heading o
				// title") vivía aquí y se retiró en el issue #240: era una
				// regla MUERTA. LAYOUT001 (layout_validation.go,
				// validateTitleSlideHeading) valida exactamente el mismo
				// bloque —cualquier ContentBlock con BlockType "title", en
				// los dos dialectos strict, porque SlideLayoutValidationRule
				// también corre sobre ContentBlock genérico— y ambas están
				// en DefaultRules(). Mientras LAYOUT001 exigía `heading` a
				// secas era la más estricta de las dos, así que ganaba
				// siempre y STRICT001 nunca podía disparar sola. Ahora que
				// LAYOUT001 acepta `title` como fallback, las dos dirían lo
				// mismo, y tener el mismo chequeo con dos IDs solo produce
				// diagnósticos duplicados. STRICT002 se queda: no hay otra
				// regla que cubra "bloque de contenido sin título ni
				// elementos" en modo strict.

				// Validar que los bloques de contenido tengan al menos un elemento o título
				if block.BlockType == "content" || block.BlockType == "" {
					if block.Title == "" && len(block.Elements) == 0 {
						diags = append(diags,
							diagnostics.NewError(
								"Content blocks must have either a title or content elements",
								block.GetPosition(), "linter").WithRuleID("STRICT002"))
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
				// Validar que las tablas tengan headers y al menos una fila.
				//
				// Checked against elem.Cells too, not just the flat
				// Headers/Rows view: a `cells:`-authored table (issue #20)
				// with row/col-scoped headers scattered across rows, rather
				// than a clean all-header first row, derives Headers as
				// []string{} (ast.FlattenCellsToRows only populates Headers
				// when the entire first Cells row IsHeader) even though it
				// does have header cells — checking Headers alone made
				// TABLE001 a false positive for that legitimate shape.
				if len(elem.Headers) == 0 && !tableHasAnyCellWhere(elem.Cells, func(c ast.TableCell) bool { return c.IsHeader }) {
					diags = append(diags,
						diagnostics.NewWarning(
							"TABLE element should have headers defined",
							elem.GetPosition(), "linter").WithRuleID("TABLE001"))
				}
				if len(elem.Rows) == 0 && !tableHasAnyCellWhere(elem.Cells, func(c ast.TableCell) bool { return !c.IsHeader }) {
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

			case *ast.ChartElement:
				diags = append(diags, checkChartElement(elem)...)
			}
		}
	}

	return diags
}

// knownChartTypes es el vocabulario de tipos que el pipeline sabe dibujar:
// los ocho controladores del bundle base de Chart.js v4, más "combo" (un
// pseudo-tipo del DSL, que el renderer traduce a un bar con tipos por
// dataset) y "treemap" (que registra el plugin chartjs-chart-treemap, cargado
// junto al bundle).
//
// Deliberadamente NO se comparte con renderer.nativeChartSupportedTypes, que
// responde otra pregunta —¿se puede rasterizar sin Chromium?— y es más
// angosto a propósito. Tampoco se exporta: nadie fuera de este paquete lo
// necesita, y un símbolo público nuevo en core arrastraría el baile de
// bump-core para los dos CLIs sin ganar nada.
var knownChartTypes = map[string]bool{
	"bar":       true,
	"bubble":    true,
	"combo":     true,
	"doughnut":  true,
	"line":      true,
	"pie":       true,
	"polarArea": true,
	"radar":     true,
	"scatter":   true,
	"treemap":   true,
}

// knownChartTypesList devuelve el vocabulario ordenado, para nombrarlo en el
// mensaje del diagnóstico (un "unknown type" que no dice cuáles SÍ valen
// obliga a ir a buscar la lista a otro lado).
func knownChartTypesList() string {
	names := make([]string, 0, len(knownChartTypes))
	for name := range knownChartTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// checkChartElement produce los diagnósticos de un chart. Son tres gates
// distintos sobre el mismo elemento:
//
//   - CHART001 (preexistente): el chart no tiene datos de ninguna forma.
//   - CHART003 (issue #208): el tipo del tag no está en el vocabulario.
//   - CHART004 (issue #207): el payload de un chart en modo JSON no es una
//     config de Chart.js.
//
// Los tres comparten el mismo modo de falla observable —lienzo en blanco, sin
// error de consola ni nada en los logs— y hasta ahora solo el primero existía,
// y solo cubría el camino YAML.
func checkChartElement(elem *ast.ChartElement) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	// El tipo EFECTIVO tiene que resolverse igual que en el renderer
	// (renderer.ResolveChartJSONMode): en modo JSON manda el "type" del
	// payload, y solo si falta o viene vacío se usa el token del tag. Si acá
	// se exigiera el tipo en el tag, un <<chart: bar>> con cuerpo
	// {"data":{...}} —que renderiza perfecto— lintearía sucio.
	chartType := elem.ChartType
	var payload map[string]interface{}
	if elem.IsJSONMode && len(elem.RawJSON) > 0 {
		// El error se ignora a propósito, y no es un caso muerto: un
		// top-level que no sea objeto (un arreglo, un string, un número) es
		// JSON válido, así que pasa el json.Valid() de elements/chart.go,
		// prende IsJSONMode y nunca dispara CHART002 — pero el unmarshal a
		// map de acá sí falla y payload queda nil. Eso es justo lo que se
		// quiere: sin objeto no hay data.datasets, CHART004 dispara abajo, y
		// el renderer coincide (ResolveChartJSONMode también exige objeto y
		// degrada el chart a "{}"). Reportar aparte el fallo del unmarshal
		// solo duplicaría el mismo defecto con otro ID.
		if err := json.Unmarshal(elem.RawJSON, &payload); err == nil {
			if declared, ok := payload["type"].(string); ok && declared != "" {
				chartType = declared
			}
		}
	}

	if chartType != "" && !knownChartTypes[chartType] {
		diags = append(diags,
			diagnostics.NewWarning(
				"Unknown chart type: "+chartType+" (expected one of: "+knownChartTypesList()+")",
				elem.GetPosition(), "linter").WithRuleID("CHART003"))
	}

	if elem.IsJSONMode && len(elem.RawJSON) > 0 {
		// Chart.js exige data.datasets como arreglo; sin eso no dibuja nada,
		// por muy válido que sea el JSON. Un ID propio y no CHART001 porque el
		// arreglo es distinto: acá SÍ hay payload, lo que falla es su forma
		// (típicamente un emisor que serializó una estructura plana).
		if !hasChartJSDatasets(payload) {
			diags = append(diags,
				diagnostics.NewWarning(
					"JSON-mode chart payload is not a Chart.js config: expected data.datasets to be a non-empty array",
					elem.GetPosition(), "linter").WithRuleID("CHART004"))
		}
		// CHART001 no se evalúa en modo JSON: sí hay payload, y su calidad ya
		// es asunto de CHART004. Reportar los dos por el mismo defecto solo
		// haría ruido.
		return diags
	}

	if len(elem.Data) == 0 && len(elem.Series) == 0 {
		diags = append(diags,
			diagnostics.NewWarning(
				"Chart elements should have data defined",
				elem.GetPosition(), "linter").WithRuleID("CHART001"))
	}

	return diags
}

// hasChartJSDatasets indica si payload trae un data.datasets no vacío, que es
// lo mínimo que Chart.js necesita para dibujar algo. nil-safe: un payload que
// no era un objeto JSON llega como nil.
func hasChartJSDatasets(payload map[string]interface{}) bool {
	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		return false
	}
	datasets, ok := data["datasets"].([]interface{})
	return ok && len(datasets) > 0
}

// tableHasAnyCellWhere reports whether any cell across elem.Cells matches
// pred — used by ElementStructureRule to fall back to the real cell
// structure (issue #20) when the flat Headers/Rows view alone would
// misreport a `cells:`-authored table as missing headers/rows.
func tableHasAnyCellWhere(cells [][]ast.TableCell, pred func(ast.TableCell) bool) bool {
	for _, row := range cells {
		for _, cell := range row {
			if pred(cell) {
				return true
			}
		}
	}
	return false
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

// schemalessKnownSlideTypes son tipos de slide que los generadores SÍ
// reconocen pero para los que no hay schema en GetSlideLayoutSchemas.
//
// slidelang los mapea a una plantilla concreta —"cover"/"intro" salen con el
// layout de título y "chapter"/"with_directive" con el de contenido, ver
// config.IsSlideTitle/IsSlideContent— así que son válidos de punta a punta y
// LAYOUT_UNKNOWN no puede acusarlos de inexistentes. Simplemente no tienen
// reglas propias que validar.
//
// La lista se repite acá porque core no puede importar slidelang (la
// dependencia va al revés). Si allá se agrega un tipo, agregarlo también acá
// o LAYOUT_UNKNOWN empezará a marcarlo — es el precio de que el catálogo de
// tipos viva en el consumidor y el linter en core, y por eso conviene que
// crezca poco: un tipo nuevo casi siempre está mejor como schema, que además
// lo valida.
var schemalessKnownSlideTypes = map[string]bool{
	"cover":          true,
	"intro":          true,
	"chapter":        true,
	"with_directive": true,
}

// isSchemalessKnownSlideType reporta si slideType es uno de esos.
func isSchemalessKnownSlideType(slideType string) bool {
	return schemalessKnownSlideTypes[slideType]
}

// sortedRecognizedSlideTypes devuelve, en orden alfabético, TODOS los tipos
// de slide que se aceptan: los que tienen schema y los que no.
//
// Los cuatro sin schema (schemalessKnownSlideTypes) tienen que estar en la
// lista aunque no se validen. El mensaje de LAYOUT_UNKNOWN existe para que
// quien escribió un nombre malo encuentre el bueno, y omitirlos hacía que a
// un typo de "cover" se le sugiriera una lista sin "cover" — la sugerencia
// mandaba justo a donde no está la respuesta.
//
// El orden es alfabético porque el recorrido de un mapa en Go no es
// determinista, y un mensaje que cambia de orden entre corridas es ruido en
// cualquier diff de salida.
func sortedRecognizedSlideTypes(schemas map[string]SlideLayoutSchema) []string {
	names := make([]string, 0, len(schemas)+len(schemalessKnownSlideTypes))
	for name := range schemas {
		names = append(names, name)
	}
	for name := range schemalessKnownSlideTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *SlideLayoutValidationRule) Check(node ast.Node) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic

	if slide, ok := node.(*ast.ContentBlock); ok {
		slideType := slide.BlockType
		if slideType == "" {
			return diags // No validar slides sin tipo específico
		}

		// Obtener el esquema de validación para este tipo de slide
		if isSchemalessKnownSlideType(slideType) {
			return diags
		}

		schemas := GetSlideLayoutSchemas()
		schema, exists := schemas[slideType]
		if !exists {
			// Un tipo sin schema no se puede validar, pero tampoco se puede
			// ignorar en silencio: el slide se renderiza con el layout
			// genérico y nada avisa de que el nombre no existe. Un typo
			// ("comparision") o un layout inventado se veía exactamente
			// igual que uno correcto (issue #239).
			//
			// La lista de nombres sale de los propios schemas, no de una
			// constante aparte: agregar un layout no puede dejar este
			// mensaje desactualizado.
			diags = append(diags, diagnostics.Diagnostic{
				Severity: diagnostics.Warning,
				Code:     "LAYOUT_UNKNOWN",
				Message: "Unknown slide layout '" + slideType + "'; no schema validates it. Recognized layouts: " +
					strings.Join(sortedRecognizedSlideTypes(schemas), ", "),
				Position: slide.Position,
				Source:   "linter",
			})
			return diags
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
