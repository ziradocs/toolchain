// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"

	"go.ziradocs.com/core/v2/renderer"
)

// directiveAttrInvalidChars es la whitelist inversa para el nombre de
// atributo data-* de un directive: cualquier carácter fuera de
// alfanuméricos/_/- se elimina (no se descarta el parámetro completo),
// mismo criterio que sanitizeBookmarkID en
// doclang/internal/generator/docx.go. "data-" ya aporta un carácter
// inicial válido, así que el resto no necesita empezar con letra.
var directiveAttrInvalidChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// HTMLTemplateFuncs retorna funciones auxiliares para los templates
func HTMLTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		// add1 convierte el índice 0-based de slide (range $index, ...)
		// a un número de slide 1-based para mostrar al usuario (issue #94).
		"add1":       func(i int) int { return i + 1 },
		"escapeHTML": template.HTMLEscapeString,
		"escapeJS":   template.JSEscapeString,
		"lower":      strings.ToLower,
		"upper":      strings.ToUpper,
		"raw": func(s string) template.HTML {
			return template.HTML(s)
		},
		"markdown": func(s string) template.HTML {
			return template.HTML(renderer.ProcessInlineMarkdownSecure(s))
		},
		// markdownInline aplica solo formatos inline (bold/italic/code/links)
		// línea por línea, uniendo con <br> — nunca interpreta "- " como
		// viñeta ni emite otro HTML de bloque (<ul>, headings). Para usar
		// dentro de un <p> ya existente, donde HTML de bloque anidado sería
		// inválido (issue #12d), pero donde un contenido multilínea (p.ej.
		// una quote de varias líneas) debe seguir mostrando saltos de línea.
		"markdownInline": func(s string) template.HTML {
			return template.HTML(renderer.ProcessInlineMarkdownSecureMultiline(s))
		},
		// dict crea un diccionario/mapa para pasar múltiples valores a sub-templates
		"dict": func(values ...interface{}) map[string]interface{} {
			if len(values)%2 != 0 {
				return nil
			}
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					continue
				}
				dict[key] = values[i+1]
			}
			return dict
		}, "toJSON": func(v interface{}) template.JS {
			if b, err := json.Marshal(v); err == nil {
				// No se des-escapan <, > y &: JSON.parse los decodifica igual,
				// cerrando el breakout de </script> (ver
				// docs/SECURITY_AUDIT_2026-07.md, CR-1/CR-5). Retorna
				// template.JS (no string): el generador ahora usa html/template,
				// y un string plano interpolado dentro de
				// <script type="application/json"> se re-envuelve y re-escapa
				// como si fuera un literal JS, corrompiendo el JSON; template.JS
				// le indica al motor "esto ya es JS/JSON válido, no lo toques".
				return template.JS(b)
			}
			return template.JS("{}")
		}, "escapeJSON": func(s string) template.HTMLAttr {
			// Para JSON en atributos HTML, solo necesitamos escapar caracteres de atributo
			// No escapamos las comillas internas del JSON ya que serán parseadas por JSON.parse()
			escaped := strings.ReplaceAll(s, `"`, `&quot;`)
			return template.HTMLAttr(escaped)
		},
		"mermaidContent": func(s string) string {
			// El contenido ya viene procesado correctamente del parser
			// Solo necesitamos retornarlo tal como está para que toJSON lo encode correctamente
			return s
		},
		// processPageFormat procesa formato de números de página con variables
		"processPageFormat": func(format string, current, total int) string {
			if format == "" {
				return fmt.Sprintf("%d / %d", current, total)
			}

			// Simple variable replacement para números de página
			result := strings.ReplaceAll(format, "{{current}}", fmt.Sprintf("%d", current))
			result = strings.ReplaceAll(result, "{{total}}", fmt.Sprintf("%d", total))
			result = strings.ReplaceAll(result, "{{page}}", fmt.Sprintf("%d", current))

			return result
		},
		// processVariables procesa variables básicas en texto
		"processVariables": func(text string, variables map[string]interface{}) string {
			if variables == nil {
				return text
			}

			result := text
			for key, value := range variables {
				placeholder := fmt.Sprintf("{{%s}}", key)
				result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
			}

			return result
		},
		// directiveDataAttrs construye los atributos data-* de un directive
		// genérico. html/template no permite templatear el NOMBRE de un
		// atributo dinámicamente — lo sustituye por el sentinel ZgotmplZ
		// (ver issue #102) — así que la cadena completa se arma aquí en Go,
		// con el nombre del atributo validado contra un allowlist y el valor
		// escapado, exponiéndola como template.HTMLAttr ya segura.
		"directiveDataAttrs": func(params map[string]interface{}) template.HTMLAttr {
			if len(params) == 0 {
				return ""
			}
			keys := make([]string, 0, len(params))
			for k := range params {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			var b strings.Builder
			for _, k := range keys {
				safeKey := directiveAttrInvalidChars.ReplaceAllString(k, "")
				if safeKey == "" {
					continue
				}
				value := fmt.Sprintf("%v", params[k])
				b.WriteString(` data-`)
				b.WriteString(safeKey)
				b.WriteString(`="`)
				b.WriteString(template.HTMLEscapeString(value))
				b.WriteString(`"`)
			}
			return template.HTMLAttr(b.String())
		},
		// chartJSType mapea tipos de chart de SlideLang a tipos compatibles con Chart.js
		"chartJSType": func(chartType string) string {
			switch chartType {
			case "combo":
				// Chart.js no soporta "combo" nativamente, usamos "bar" como base
				// Los tipos específicos por dataset se manejan via SeriesTypes
				return "bar"
			default:
				// Otros tipos son compatibles directamente: bar, line, pie, doughnut, radar, etc.
				return chartType
			}
		},
	}
}

// ProcessInlineMarkdown es un wrapper para el renderer compartido
// Mantiene compatibilidad con código existente - AHORA USA LA VERSIÓN SEGURA
func ProcessInlineMarkdown(text string) string {
	return renderer.ProcessInlineMarkdownSecure(text)
}
