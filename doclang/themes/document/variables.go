// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package document

// ThemeVariables contiene las variables CSS para temas de documentos
type ThemeVariables map[string]string

// Theme representa un tema completo de documento
type Theme struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Variables   ThemeVariables `json:"variables"`
	Author      string         `json:"author"`
	Version     string         `json:"version"`
	IsExternal  bool           `json:"-"` // No se serializa en JSON
}

// StandardVariables lista todas las variables CSS estándar para documentos
// Estas son las variables que DEBEN estar presentes en un tema válido
var StandardVariables = []string{
	// Page & Layout
	"--doclang-page-bg",
	"--doclang-page-max-width",
	"--doclang-page-padding",
	"--doclang-page-margin",

	// Typography
	"--doclang-font-main",
	"--doclang-font-code",
	"--doclang-font-heading",
	"--doclang-font-size-base",
	"--doclang-line-height",

	// Headings
	"--doclang-h1-color",
	"--doclang-h1-size",
	"--doclang-h1-weight",
	"--doclang-h1-border",
	"--doclang-h1-border-color",
	"--doclang-h2-color",
	"--doclang-h2-size",
	"--doclang-h3-color",
	"--doclang-h3-size",
	"--doclang-h4-color",
	"--doclang-h4-size",

	// Text Colors
	"--doclang-text-color",
	"--doclang-text-light",
	"--doclang-text-muted",
	"--doclang-link-color",
	"--doclang-link-hover-color",

	// Code
	"--doclang-code-bg",
	"--doclang-code-color",
	"--doclang-code-inline-bg",
	"--doclang-code-inline-color",
	"--doclang-code-border",
	"--doclang-code-font",

	// Tables
	"--doclang-table-header-bg",
	"--doclang-table-header-color",
	"--doclang-table-border",
	"--doclang-table-hover-bg",
	"--doclang-table-stripe-bg",

	// Blockquotes
	"--doclang-quote-bg",
	"--doclang-quote-border",
	"--doclang-quote-color",

	// Alerts/Special Blocks
	"--doclang-alert-info-bg",
	"--doclang-alert-info-border",
	"--doclang-alert-info-color",
	"--doclang-alert-success-bg",
	"--doclang-alert-success-border",
	"--doclang-alert-warning-bg",
	"--doclang-alert-warning-border",
	"--doclang-alert-danger-bg",
	"--doclang-alert-danger-border",

	// TOC
	"--doclang-toc-bg",
	"--doclang-toc-border",
	"--doclang-toc-color",
	"--doclang-toc-title-color",
	"--doclang-toc-link-color",
	"--doclang-toc-link-hover",
	"--doclang-toc-accent",
	"--doclang-toc-subsection-color",
	"--doclang-toc-border-nested",

	// Shadows & Effects
	"--doclang-shadow-sm",
	"--doclang-shadow-md",
	"--doclang-shadow-lg",
	"--doclang-border-radius",

	// Page View específicas (opcionales, solo para tema page-view)
	"--doclang-page-shadow",
	"--doclang-page-break-margin",
	"--doclang-page-number-bg",
	"--doclang-header-footer-bg",
	"--doclang-header-height",
	"--doclang-footer-height",
}

// RequiredVariables lista las variables CSS que son OBLIGATORIAS en todo tema
// Si faltan, se usarán los valores del tema Professional como fallback
var RequiredVariables = []string{
	"--doclang-page-bg",
	"--doclang-font-main",
	"--doclang-h1-color",
	"--doclang-text-color",
	"--doclang-code-bg",
	"--doclang-table-border",
}
