// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package base

import "strings"

// PathNormalizer maneja normalización de rutas y archivos
type PathNormalizer struct{}

// NewPathNormalizer crea una nueva instancia del normalizador de rutas
func NewPathNormalizer() *PathNormalizer {
	return &PathNormalizer{}
}

// NormalizeImagePath normaliza rutas de imágenes a la estructura assets/images/
func (pn *PathNormalizer) NormalizeImagePath(path string) string {
	// Extraer el nombre del archivo
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]

	// Si ya está en assets/, no modificar
	if strings.HasPrefix(path, "assets/") {
		return path
	}

	// Normalizar a estructura assets/images/
	return "assets/images/" + filename
}

// IsImagePath verifica si un path es de una imagen
func (pn *PathNormalizer) IsImagePath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".svg")
}

// ChartTypeInference infiere el tipo de gráfico basado en la descripción
type ChartTypeInference struct{}

// NewChartTypeInference crea una nueva instancia del inferidor de gráficos
func NewChartTypeInference() *ChartTypeInference {
	return &ChartTypeInference{}
}

// InferChartType infiere el tipo de gráfico basado en la descripción
func (cti *ChartTypeInference) InferChartType(description string) string {
	description = strings.ToLower(description)

	if strings.Contains(description, "barra") || strings.Contains(description, "bar") {
		return "bar"
	}
	if strings.Contains(description, "línea") || strings.Contains(description, "line") || strings.Contains(description, "tendencia") {
		return "line"
	}
	if strings.Contains(description, "circular") || strings.Contains(description, "pie") || strings.Contains(description, "pastel") {
		return "pie"
	}
	if strings.Contains(description, "área") || strings.Contains(description, "area") {
		return "area"
	}
	if strings.Contains(description, "flujo") || strings.Contains(description, "flow") || strings.Contains(description, "proceso") || strings.Contains(description, "fluss") {
		return "flow"
	}

	return "bar" // Por defecto
}

// ContentClassifier ayuda a clasificar contenido para tablas
type ContentClassifier struct{}

// NewContentClassifier crea una nueva instancia del clasificador
func NewContentClassifier() *ContentClassifier {
	return &ContentClassifier{}
}

// LooksLikeDescriptiveText detecta si el texto es descriptivo (no tabla)
func (cc *ContentClassifier) LooksLikeDescriptiveText(line string) bool {
	line = strings.ToLower(line)

	// Patrones que indican texto descriptivo, no tabla
	descriptiveWords := []string{
		"técnica", "technique", "gestión", "management", "organización", "organization",
		"seguimiento", "tracking", "análisis", "analysis", "comunicación", "communication",
		"eficiente", "efficient", "digital", "app", "aplicación", "application",
		"herramienta", "tool", "software", "sistema", "system", "plataforma", "platform",
	}

	for _, word := range descriptiveWords {
		if strings.Contains(line, word) {
			return true
		}
	}

	// Si tiene más de 5 palabras después del ":", probablemente es descriptivo
	parts := strings.SplitN(line, ":", 2)
	if len(parts) == 2 {
		secondPart := strings.TrimSpace(parts[1])
		wordCount := len(strings.Fields(secondPart))
		if wordCount > 4 {
			return true
		}
	}

	return false
}

// LooksLikeKeyValuePair detecta pares clave-valor reales
func (cc *ContentClassifier) LooksLikeKeyValuePair(line string) bool {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return false
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	keyWords := strings.Fields(key)
	valueWords := strings.Fields(value)

	// Claves muy largas probablemente no son tablas
	if len(keyWords) > 3 {
		return false
	}

	// Valores muy largos sugieren texto descriptivo
	if len(valueWords) > 6 {
		return false
	}

	return true
}
