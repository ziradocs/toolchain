// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"regexp"
)

// ProcessVariables reemplaza variables de plantilla en texto
// Formato: {{variable_name}} -> valor de la variable
func ProcessVariables(text string, variables map[string]interface{}) string {
	if variables == nil {
		return text
	}

	// Regex para encontrar variables {{variable_name}}
	varRegex := regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

	result := varRegex.ReplaceAllStringFunc(text, func(match string) string {
		// Extraer el nombre de la variable (sin las llaves)
		varName := match[2 : len(match)-2]

		if value, exists := variables[varName]; exists {
			return fmt.Sprintf("%v", value)
		}

		// Si la variable no existe, devolver el texto original
		return match
	})

	return result
}

// ProcessStringArray procesa un array de strings aplicando reemplazo de variables
func ProcessStringArray(arr []string, variables map[string]interface{}) []string {
	if arr == nil {
		return nil
	}

	result := make([]string, len(arr))
	for i, s := range arr {
		result[i] = ProcessVariables(s, variables)
	}
	return result
}
