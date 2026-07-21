// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/renderer"
)

// convertPointItems convierte items de puntos del AST a template data
func ConvertPointItems(items []ast.PointItem) []PointItemData {
	return ConvertPointItemsWithVariables(items, nil)
}

// ConvertPointItemsWithVariables convierte items de puntos del AST a template data procesando variables
func ConvertPointItemsWithVariables(items []ast.PointItem, variables map[string]interface{}) []PointItemData {
	result := make([]PointItemData, 0, len(items))
	for _, item := range items {
		pointData := PointItemData{
			Content:   ProcessVariables(item.Content, variables), // Markdown se procesa en template
			SubPoints: ConvertPointItemsWithVariables(item.SubPoints, variables),
		}
		result = append(result, pointData)
	}
	return result
}

// convertCodeBlocks convierte bloques de código del AST a template data
func ConvertCodeBlocks(blocks []ast.CodeBlock) []CodeBlockData {
	return ConvertCodeBlocksWithVariables(blocks, nil)
}

// ConvertCodeBlocksWithVariables convierte bloques de código del AST a template data procesando variables
func ConvertCodeBlocksWithVariables(blocks []ast.CodeBlock, variables map[string]interface{}) []CodeBlockData {
	var result []CodeBlockData
	for _, block := range blocks {
		codeBlockData := CodeBlockData{
			Language: block.Language,
			Label:    ProcessVariables(block.Label, variables),
			Content:  ProcessVariables(block.Content, variables),
		}
		result = append(result, codeBlockData)
	}
	return result
}

// convertMapMarkers convierte marcadores del AST a template data
func ConvertMapMarkers(markers []ast.MapMarker) []MapMarkerData {
	return ConvertMapMarkersWithVariables(markers, nil)
}

// ConvertMapMarkersWithVariables convierte marcadores del AST a template data procesando variables
//
// Color se valida contra una allowlist porque maps.js lo interpola en un
// atributo style (divIcon) sin volver a escapar. Label/Details NO se
// escapan aquí: viajan a través del JSON de metadata (JSON.parse no
// decodifica entidades HTML, a diferencia de un atributo HTML), y maps.js
// los inserta en el popup vía textContent/DOM, no vía concatenación de HTML
// — escaparlos en Go solo produciría entidades visibles al usuario sin
// aportar seguridad adicional (ver docs/SECURITY_AUDIT_2026-07.md, AL-7).
func ConvertMapMarkersWithVariables(markers []ast.MapMarker, variables map[string]interface{}) []MapMarkerData {
	var result []MapMarkerData
	for _, marker := range markers {
		markerData := MapMarkerData{
			Lat:     marker.Lat,
			Lng:     marker.Lng,
			Label:   ProcessVariables(marker.Label, variables),
			Value:   marker.Value,
			Color:   renderer.SanitizeColor(ProcessVariables(marker.Color, variables)),
			Size:    ProcessVariables(marker.Size, variables),
			Details: ProcessVariables(marker.Details, variables),
		}
		result = append(result, markerData)
	}
	return result
}

// ConvertChecklistItems convierte items de checklist del AST a template data
func ConvertChecklistItems(items []ast.ChecklistItem) []ChecklistItemData {
	return ConvertChecklistItemsWithVariables(items, nil)
}

// ConvertChecklistItemsWithVariables convierte items de checklist del AST a template data procesando variables
func ConvertChecklistItemsWithVariables(items []ast.ChecklistItem, variables map[string]interface{}) []ChecklistItemData {
	result := make([]ChecklistItemData, 0, len(items))
	for _, item := range items {
		checklistData := ChecklistItemData{
			Content:  ProcessVariables(item.Content, variables), // Markdown se procesa en template
			Checked:  item.Checked,
			SubItems: ConvertChecklistItemsWithVariables(item.SubItems, variables),
		}
		result = append(result, checklistData)
	}
	return result
}

// ProcessMapOptions procesa variables en las opciones del mapa
func ProcessMapOptions(options map[string]interface{}, variables map[string]interface{}) map[string]interface{} {
	if options == nil {
		return nil
	}

	processedOptions := make(map[string]interface{})
	for key, value := range options {
		switch v := value.(type) {
		case string:
			processedOptions[key] = ProcessVariables(v, variables)
		default:
			processedOptions[key] = value
		}
	}
	return processedOptions
}

// ConvertColumnsWithVariables convierte columnas del AST a template data procesando variables
func ConvertColumnsWithVariables(columns []ast.ColumnElement, variables map[string]interface{}) []ColumnData {
	var result []ColumnData
	for _, column := range columns {
		columnData := ColumnData{
			Content: ProcessVariables(column.Content, variables),
		}
		result = append(result, columnData)
	}
	return result
}
