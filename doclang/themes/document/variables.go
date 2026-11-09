// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package document

import "go.ziradocs.com/doclang/v2/internal/inks"

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

// StandardVariables lista el vocabulario de variables CSS que un tema de
// documento puede definir. Ninguna es obligatoria por estar aquí: las
// obligatorias son las de RequiredVariables. Incluye las de superficie que
// doclang deriva para temas externos, con las tintas por superficie
// (--doclang-<superficie>-ink-<tinta>).
var StandardVariables = append([]string{
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
	"--doclang-alert-success-color",
	"--doclang-alert-warning-color",
	"--doclang-alert-danger-color",
	"--doclang-alert-tip-bg",
	"--doclang-alert-tip-border",
	"--doclang-alert-tip-color",

	// Tintas de estado ([x]{.danger}, .info, .success, .warning, .accent).
	"--doclang-danger-color",
	"--doclang-info-color",
	"--doclang-success-color",
	"--doclang-warning-color",
	"--doclang-accent-color",

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

	// Superficies del HTML/PDF (opcionales). Ningún tema embebido las define;
	// para un tema externo, doclang deriva las que falten de las de arriba
	// (internal/generator/html_theme_bindings.go). El README de doclang
	// documenta el fallback y el origen de cada una.
	"--doclang-body-link-color",
	"--doclang-body-link-active-color",
	"--doclang-table-bg",
	"--doclang-table-color",
	"--doclang-panel-bg",
	"--doclang-panel-border",
	"--doclang-panel-color",
	"--doclang-kbd-bg",
	"--doclang-kbd-border",
	"--doclang-kbd-color",
	"--doclang-chart-bg",
	"--doclang-blockquote-bg",
	"--doclang-blockquote-border",
	"--doclang-blockquote-color",
	"--doclang-blockquote-footer-color",
	"--doclang-callout-info-bg",
	"--doclang-callout-info-border",
	"--doclang-callout-info-color",
	"--doclang-callout-warning-bg",
	"--doclang-callout-warning-border",
	"--doclang-callout-warning-color",
	"--doclang-callout-danger-bg",
	"--doclang-callout-danger-border",
	"--doclang-callout-danger-color",
	"--doclang-callout-success-bg",
	"--doclang-callout-success-border",
	"--doclang-callout-success-color",
	"--doclang-callout-tip-bg",
	"--doclang-callout-tip-border",
	"--doclang-callout-tip-color",
	"--doclang-element-title-color",
	"--doclang-caption-color",
	"--doclang-code-tab-active-bg",
	"--doclang-code-tab-active-color",
	"--doclang-code-tab-accent",
	"--doclang-print-margin-bg",
	"--doclang-blockquote-link-color",
	"--doclang-callout-info-link-color",
	"--doclang-callout-warning-link-color",
	"--doclang-callout-danger-link-color",
	"--doclang-callout-success-link-color",
	"--doclang-callout-tip-link-color",
	"--doclang-table-header-link-color",
	"--doclang-table-link-color",
	"--doclang-table-stripe-link-color",
}, inks.Variables()...)

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
