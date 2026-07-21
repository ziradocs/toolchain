// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package document

// GetProfessionalTheme retorna el tema corporativo moderno (default)
func GetProfessionalTheme() Theme {
	return Theme{
		Name:        "professional",
		Description: "Modern corporate documents with clean design and vibrant colors",
		Author:      "DocLang",
		Version:     "1.0.0",
		Variables: ThemeVariables{
			// Page & Layout
			"--doclang-page-bg":        "#ffffff",
			"--doclang-page-max-width": "210mm", // A4
			"--doclang-page-padding":   "20mm",
			"--doclang-page-margin":    "0 auto",

			// Typography
			"--doclang-font-main":      "'Segoe UI', -apple-system, BlinkMacSystemFont, 'Roboto', 'Helvetica Neue', Arial, sans-serif",
			"--doclang-font-code":      "'Consolas', 'Monaco', 'Courier New', monospace",
			"--doclang-font-heading":   "'Segoe UI', sans-serif",
			"--doclang-font-size-base": "12pt",
			"--doclang-line-height":    "1.7",

			// Headings - Modern corporate blue
			"--doclang-h1-color":        "#1a202c",
			"--doclang-h1-size":         "2.5em",
			"--doclang-h1-weight":       "700",
			"--doclang-h1-border":       "3px solid",
			"--doclang-h1-border-color": "#3498db",
			"--doclang-h2-color":        "#2d3748",
			"--doclang-h2-size":         "2em",
			"--doclang-h3-color":        "#4a5568",
			"--doclang-h3-size":         "1.5em",
			"--doclang-h4-color":        "#718096",
			"--doclang-h4-size":         "1.2em",

			// Text
			"--doclang-text-color":       "#2c3e50",
			"--doclang-text-light":       "#718096",
			"--doclang-text-muted":       "#a0aec0",
			"--doclang-link-color":       "#3498db",
			"--doclang-link-hover-color": "#2980b9",

			// Code
			"--doclang-code-bg":           "#1e1e1e",
			"--doclang-code-color":        "#d4d4d4",
			"--doclang-code-inline-bg":    "#f6f8fa",
			"--doclang-code-inline-color": "#e83e8c",
			"--doclang-code-border":       "#e1e4e8",
			"--doclang-code-font":         "'Consolas', 'Monaco', monospace",

			// Tables
			"--doclang-table-header-bg":    "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
			"--doclang-table-header-color": "#ffffff",
			"--doclang-table-border":       "#e2e8f0",
			"--doclang-table-hover-bg":     "#edf2f7",
			"--doclang-table-stripe-bg":    "#f8fafc",

			// Blockquotes
			"--doclang-quote-bg":     "#f8fafc",
			"--doclang-quote-border": "#3498db",
			"--doclang-quote-color":  "#4a5568",

			// Alerts
			"--doclang-alert-info-bg":        "#ebf8ff",
			"--doclang-alert-info-border":    "#3182ce",
			"--doclang-alert-info-color":     "#2c5282",
			"--doclang-alert-success-bg":     "#f0fff4",
			"--doclang-alert-success-border": "#38a169",
			"--doclang-alert-warning-bg":     "#fffaf0",
			"--doclang-alert-warning-border": "#ed8936",
			"--doclang-alert-danger-bg":      "#fff5f5",
			"--doclang-alert-danger-border":  "#e53e3e",

			// TOC - Modern blue accents
			"--doclang-toc-bg":               "#f8f9fa",
			"--doclang-toc-border":           "#3498db",
			"--doclang-toc-color":            "#2c3e50",
			"--doclang-toc-title-color":      "#1a202c",
			"--doclang-toc-link-color":       "#2c3e50",
			"--doclang-toc-link-hover":       "#3498db",
			"--doclang-toc-accent":           "#3498db",
			"--doclang-toc-subsection-color": "#4a5568",
			"--doclang-toc-border-nested":    "#cbd5e0",

			// Shadows & Effects
			"--doclang-shadow-sm":     "0 1px 3px rgba(0,0,0,0.1)",
			"--doclang-shadow-md":     "0 2px 4px rgba(0,0,0,0.1)",
			"--doclang-shadow-lg":     "0 4px 6px rgba(0,0,0,0.1)",
			"--doclang-border-radius": "6px",

			// Page View (no aplicables para este tema por defecto)
			"--doclang-page-shadow":       "none",
			"--doclang-page-break-margin": "0",
			"--doclang-page-number-bg":    "#ffffff",
			"--doclang-header-footer-bg":  "#fafafa",
			"--doclang-header-height":     "0",
			"--doclang-footer-height":     "0",
		},
	}
}

// GetAcademicTheme retorna el tema académico formal
func GetAcademicTheme() Theme {
	return Theme{
		Name:        "academic",
		Description: "Formal academic documents with serif typography and high contrast",
		Author:      "DocLang",
		Version:     "1.0.0",
		Variables: ThemeVariables{
			// Page & Layout
			"--doclang-page-bg":        "#ffffff",
			"--doclang-page-max-width": "210mm",
			"--doclang-page-padding":   "25mm 30mm", // Márgenes más amplios
			"--doclang-page-margin":    "0 auto",

			// Typography - Serif fonts para estilo académico
			"--doclang-font-main":      "'Georgia', 'Times New Roman', 'Times', serif",
			"--doclang-font-heading":   "'Georgia', 'Times New Roman', serif",
			"--doclang-font-code":      "'Courier New', 'Courier', monospace",
			"--doclang-font-size-base": "11pt",
			"--doclang-line-height":    "1.8", // Más espaciado para lectura

			// Headings - Classic black con numeración romana
			"--doclang-h1-color":        "#000000",
			"--doclang-h1-size":         "2.2em",
			"--doclang-h1-weight":       "700",
			"--doclang-h1-border":       "2px solid",
			"--doclang-h1-border-color": "#000000",
			"--doclang-h2-color":        "#1a1a1a",
			"--doclang-h2-size":         "1.8em",
			"--doclang-h3-color":        "#333333",
			"--doclang-h3-size":         "1.4em",
			"--doclang-h4-color":        "#4a4a4a",
			"--doclang-h4-size":         "1.2em",

			// Text - Alto contraste
			"--doclang-text-color":       "#333333",
			"--doclang-text-light":       "#666666",
			"--doclang-text-muted":       "#999999",
			"--doclang-link-color":       "#0066cc",
			"--doclang-link-hover-color": "#0052a3",

			// Code - Estilo clásico
			"--doclang-code-bg":           "#f5f5f5",
			"--doclang-code-color":        "#000000",
			"--doclang-code-inline-bg":    "#eeeeee",
			"--doclang-code-inline-color": "#c7254e",
			"--doclang-code-border":       "#cccccc",
			"--doclang-code-font":         "'Courier New', monospace",

			// Tables - Estilo minimalista
			"--doclang-table-header-bg":    "#333333",
			"--doclang-table-header-color": "#ffffff",
			"--doclang-table-border":       "#cccccc",
			"--doclang-table-hover-bg":     "#f5f5f5",
			"--doclang-table-stripe-bg":    "#fafafa",

			// Blockquotes - Clásico con borde izquierdo
			"--doclang-quote-bg":     "#f9f9f9",
			"--doclang-quote-border": "#666666",
			"--doclang-quote-color":  "#555555",

			// Alerts - Colores sutiles
			"--doclang-alert-info-bg":        "#e7f3ff",
			"--doclang-alert-info-border":    "#0066cc",
			"--doclang-alert-info-color":     "#004080",
			"--doclang-alert-success-bg":     "#e8f5e9",
			"--doclang-alert-success-border": "#2e7d32",
			"--doclang-alert-warning-bg":     "#fff3e0",
			"--doclang-alert-warning-border": "#ef6c00",
			"--doclang-alert-danger-bg":      "#ffebee",
			"--doclang-alert-danger-border":  "#c62828",

			// TOC - Academic formal style
			"--doclang-toc-bg":               "#f5f5f5",
			"--doclang-toc-border":           "#000000",
			"--doclang-toc-color":            "#333333",
			"--doclang-toc-title-color":      "#000000",
			"--doclang-toc-link-color":       "#1a1a1a",
			"--doclang-toc-link-hover":       "#0066cc",
			"--doclang-toc-accent":           "#666666",
			"--doclang-toc-subsection-color": "#666666",
			"--doclang-toc-border-nested":    "#cccccc",

			// Shadows - Mínimas
			"--doclang-shadow-sm":     "none",
			"--doclang-shadow-md":     "0 1px 2px rgba(0,0,0,0.1)",
			"--doclang-shadow-lg":     "0 2px 4px rgba(0,0,0,0.15)",
			"--doclang-border-radius": "2px",

			// Page View (no aplicables)
			"--doclang-page-shadow":       "none",
			"--doclang-page-break-margin": "0",
			"--doclang-page-number-bg":    "#ffffff",
			"--doclang-header-footer-bg":  "#fafafa",
			"--doclang-header-height":     "0",
			"--doclang-footer-height":     "0",
		},
	}
}

// GetTechnicalTheme retorna el tema técnico minimalista
func GetTechnicalTheme() Theme {
	return Theme{
		Name:        "technical",
		Description: "Technical documentation with monospace aesthetic and terminal-like style",
		Author:      "DocLang",
		Version:     "1.0.0",
		Variables: ThemeVariables{
			// Page & Layout
			"--doclang-page-bg":        "#fafafa",
			"--doclang-page-max-width": "210mm",
			"--doclang-page-padding":   "15mm 20mm", // Márgenes reducidos
			"--doclang-page-margin":    "0 auto",

			// Typography - Monospace everywhere
			"--doclang-font-main":      "'Courier New', 'Monaco', 'Menlo', 'Consolas', monospace",
			"--doclang-font-heading":   "'Courier New', monospace",
			"--doclang-font-code":      "'Courier New', monospace",
			"--doclang-font-size-base": "11pt",
			"--doclang-line-height":    "1.6",

			// Headings - Minimal style
			"--doclang-h1-color":        "#000000",
			"--doclang-h1-size":         "1.8em",
			"--doclang-h1-weight":       "700",
			"--doclang-h1-border":       "1px solid",
			"--doclang-h1-border-color": "#000000",
			"--doclang-h2-color":        "#1a1a1a",
			"--doclang-h2-size":         "1.5em",
			"--doclang-h3-color":        "#333333",
			"--doclang-h3-size":         "1.3em",
			"--doclang-h4-color":        "#4a4a4a",
			"--doclang-h4-size":         "1.1em",

			// Text - Monochrome
			"--doclang-text-color":       "#1a1a1a",
			"--doclang-text-light":       "#4a4a4a",
			"--doclang-text-muted":       "#666666",
			"--doclang-link-color":       "#0000ee", // Classic blue link
			"--doclang-link-hover-color": "#0000cc",

			// Code - Same as text (todo es código)
			"--doclang-code-bg":           "#f0f0f0",
			"--doclang-code-color":        "#000000",
			"--doclang-code-inline-bg":    "#e8e8e8",
			"--doclang-code-inline-color": "#000000",
			"--doclang-code-border":       "#cccccc",
			"--doclang-code-font":         "'Courier New', monospace",

			// Tables - Terminal style
			"--doclang-table-header-bg":    "#000000",
			"--doclang-table-header-color": "#00ff00", // Green terminal text
			"--doclang-table-border":       "#333333",
			"--doclang-table-hover-bg":     "#f5f5f5",
			"--doclang-table-stripe-bg":    "#f8f8f8",

			// Blockquotes - Minimal
			"--doclang-quote-bg":     "#f5f5f5",
			"--doclang-quote-border": "#666666",
			"--doclang-quote-color":  "#333333",

			// Alerts - Simple borders
			"--doclang-alert-info-bg":        "#f0f0f0",
			"--doclang-alert-info-border":    "#666666",
			"--doclang-alert-info-color":     "#1a1a1a",
			"--doclang-alert-success-bg":     "#f0f0f0",
			"--doclang-alert-success-border": "#00aa00",
			"--doclang-alert-warning-bg":     "#f0f0f0",
			"--doclang-alert-warning-border": "#ff8800",
			"--doclang-alert-danger-bg":      "#f0f0f0",
			"--doclang-alert-danger-border":  "#cc0000",

			// TOC - Technical monospace style
			"--doclang-toc-bg":               "#f0f0f0",
			"--doclang-toc-border":           "#000000",
			"--doclang-toc-color":            "#000000",
			"--doclang-toc-title-color":      "#000000",
			"--doclang-toc-link-color":       "#1a1a1a",
			"--doclang-toc-link-hover":       "#0000ee",
			"--doclang-toc-accent":           "#000000",
			"--doclang-toc-subsection-color": "#4a4a4a",
			"--doclang-toc-border-nested":    "#333333",

			// No shadows - flat design
			"--doclang-shadow-sm":     "none",
			"--doclang-shadow-md":     "none",
			"--doclang-shadow-lg":     "none",
			"--doclang-border-radius": "0", // Sharp corners

			// Page View (no aplicables)
			"--doclang-page-shadow":       "none",
			"--doclang-page-break-margin": "0",
			"--doclang-page-number-bg":    "#fafafa",
			"--doclang-header-footer-bg":  "#f0f0f0",
			"--doclang-header-height":     "0",
			"--doclang-footer-height":     "0",
		},
	}
}

// GetPageViewTheme retorna el tema con vista por páginas (estilo Word/Google Docs)
func GetPageViewTheme() Theme {
	return Theme{
		Name:        "page-view",
		Description: "Document with visible page breaks and headers/footers (Word/Google Docs style)",
		Author:      "DocLang",
		Version:     "1.0.0",
		Variables: ThemeVariables{
			// Page Layout - Visual pages con fondo gris
			"--doclang-page-bg":        "#f5f5f5",             // Gris de fondo (canvas)
			"--doclang-page-max-width": "210mm",               // A4 width
			"--doclang-page-padding":   "25mm 30mm 25mm 30mm", // Top Right Bottom Left
			"--doclang-page-margin":    "20px auto",           // Separación entre páginas

			// Page Visual Effects
			"--doclang-page-shadow":       "0 2px 8px rgba(0,0,0,0.15), 0 4px 16px rgba(0,0,0,0.08)",
			"--doclang-page-break-margin": "40px", // Espacio visual entre páginas
			"--doclang-page-number-bg":    "#ffffff",

			// Header/Footer (visible in page view)
			"--doclang-header-footer-bg": "#fafafa",
			"--doclang-header-height":    "15mm",
			"--doclang-footer-height":    "15mm",

			// Typography - Similar a Word (Calibri-like)
			"--doclang-font-main":      "'Calibri', 'Segoe UI', 'Arial', sans-serif",
			"--doclang-font-code":      "'Consolas', 'Courier New', monospace",
			"--doclang-font-heading":   "'Calibri', 'Segoe UI', sans-serif",
			"--doclang-font-size-base": "11pt",
			"--doclang-line-height":    "1.5", // Word default

			// Headings - Word style blue
			"--doclang-h1-color":        "#1f4788", // Word blue
			"--doclang-h1-size":         "2.2em",
			"--doclang-h1-weight":       "600",
			"--doclang-h1-border":       "none",
			"--doclang-h1-border-color": "transparent",
			"--doclang-h2-color":        "#2e5d9e",
			"--doclang-h2-size":         "1.8em",
			"--doclang-h3-color":        "#4472c4",
			"--doclang-h3-size":         "1.5em",
			"--doclang-h4-color":        "#5b9bd5",
			"--doclang-h4-size":         "1.2em",

			// Text - Black como Word
			"--doclang-text-color":       "#000000",
			"--doclang-text-light":       "#595959",
			"--doclang-text-muted":       "#808080",
			"--doclang-link-color":       "#0563c1", // Word link blue
			"--doclang-link-hover-color": "#0344a0",

			// Code - Word style
			"--doclang-code-bg":           "#f2f2f2",
			"--doclang-code-color":        "#000000",
			"--doclang-code-inline-bg":    "#f2f2f2",
			"--doclang-code-inline-color": "#c7254e",
			"--doclang-code-border":       "#d4d4d4",
			"--doclang-code-font":         "'Consolas', monospace",

			// Tables - Office style
			"--doclang-table-header-bg":    "#4472c4", // Word table blue
			"--doclang-table-header-color": "#ffffff",
			"--doclang-table-border":       "#d4d4d4",
			"--doclang-table-hover-bg":     "#f2f2f2",
			"--doclang-table-stripe-bg":    "#fafafa",

			// Blockquotes - Subtle
			"--doclang-quote-bg":     "#f9f9f9",
			"--doclang-quote-border": "#0563c1",
			"--doclang-quote-color":  "#595959",

			// Alerts - Office colors
			"--doclang-alert-info-bg":        "#d9edf7",
			"--doclang-alert-info-border":    "#0563c1",
			"--doclang-alert-info-color":     "#31708f",
			"--doclang-alert-success-bg":     "#dff0d8",
			"--doclang-alert-success-border": "#70ad47",
			"--doclang-alert-warning-bg":     "#fcf8e3",
			"--doclang-alert-warning-border": "#ffc000",
			"--doclang-alert-danger-bg":      "#f2dede",
			"--doclang-alert-danger-border":  "#c00000",

			// TOC - Office style
			"--doclang-toc-bg":               "#fafafa",
			"--doclang-toc-border":           "#4472c4",
			"--doclang-toc-color":            "#000000",
			"--doclang-toc-title-color":      "#1f4788",
			"--doclang-toc-link-color":       "#000000",
			"--doclang-toc-link-hover":       "#0563c1",
			"--doclang-toc-accent":           "#4472c4",
			"--doclang-toc-subsection-color": "#595959",
			"--doclang-toc-border-nested":    "#d4d4d4",

			// Shadows - Subtle paper effect
			"--doclang-shadow-sm":     "0 1px 2px rgba(0,0,0,0.1)",
			"--doclang-shadow-md":     "0 2px 4px rgba(0,0,0,0.12)",
			"--doclang-shadow-lg":     "0 4px 8px rgba(0,0,0,0.15)",
			"--doclang-border-radius": "0", // Word tiene esquinas rectas
		},
	}
}

// EmbeddedThemes contiene todos los temas embebidos
var EmbeddedThemes = map[string]Theme{
	"professional": GetProfessionalTheme(),
	"academic":     GetAcademicTheme(),
	"technical":    GetTechnicalTheme(),
	"page-view":    GetPageViewTheme(),
}

// GetTheme retorna un tema por nombre, o professional si no existe
func GetTheme(name string) Theme {
	if theme, exists := EmbeddedThemes[name]; exists {
		return theme
	}
	return GetProfessionalTheme()
}

// GetThemeNames retorna los nombres de todos los temas embebidos
func GetThemeNames() []string {
	return []string{"professional", "academic", "technical", "page-view"}
}
