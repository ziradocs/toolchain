// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"go.ziradocs.com/core/v2/util"
)

// DOCXStyle representa los estilos aplicables a documentos DOCX
type DOCXStyle struct {
	// Typography
	FontFamily     string
	FontSize       string // e.g., "20" (half-points, so 20 = 10pt)
	FontSizeBase   string
	FontSizeCode   string
	CodeFontFamily string

	// Headings
	H1Size        string
	H1Color       string
	H1Bold        bool
	H1SpaceBefore string // Espaciado antes en puntos (1pt = 20 twips)
	H1SpaceAfter  string // Espaciado después en puntos
	H2Size        string
	H2Color       string
	H2Bold        bool
	H2SpaceBefore string
	H2SpaceAfter  string
	H3Size        string
	H3Color       string
	H3Bold        bool
	H3SpaceBefore string
	H3SpaceAfter  string
	H4Size        string
	H4Color       string
	H4Bold        bool
	H4SpaceBefore string
	H4SpaceAfter  string

	// Text
	TextColor       string
	TextLightColor  string
	LinkColor       string
	CodeInlineColor string
	CodeBlockColor  string
	CodeBlockBg     string // Color de fondo para código
	TextSpaceAfter  string // Espaciado después de párrafos normales
	ListIndent      string // Indentación para listas (en twips, 720 = 0.5")

	// Table
	TableHeaderColor   string
	TableHeaderBg      string
	TableBorderColor   string
	TableStripeEnabled bool

	// Special Blocks
	InfoColor    string
	WarningColor string
	DangerColor  string
	SuccessColor string
	TipColor     string
}

// GetStyleForTheme retorna los estilos DOCX basados en el tema HTML
func GetStyleForTheme(themeName string, logger util.Logger) DOCXStyle {
	switch themeName {
	case "professional":
		return DOCXStyle{
			// Typography - Segoe UI modern
			FontFamily:     "Segoe UI",
			FontSize:       "22", // 11pt
			FontSizeBase:   "22", // 11pt for content
			FontSizeCode:   "20", // 10pt for code
			CodeFontFamily: "Consolas",

			// Headings - Modern blue theme
			H1Size:        "56", // 28pt
			H1Color:       "1A202C",
			H1Bold:        true,
			H1SpaceBefore: "480", // 24pt (24 * 20 twips)
			H1SpaceAfter:  "240", // 12pt (12 * 20 twips)
			H2Size:        "48",  // 24pt
			H2Color:       "2D3748",
			H2Bold:        true,
			H2SpaceBefore: "240", // 12pt
			H2SpaceAfter:  "120", // 6pt
			H3Size:        "40",  // 20pt
			H3Color:       "4A5568",
			H3Bold:        true,
			H3SpaceBefore: "240", // 12pt
			H3SpaceAfter:  "120", // 6pt
			H4Size:        "36",  // 18pt
			H4Color:       "718096",
			H4Bold:        true,
			H4SpaceBefore: "120", // 6pt
			H4SpaceAfter:  "120", // 6pt

			// Text
			TextColor:       "2C3E50",
			TextLightColor:  "718096",
			LinkColor:       "3498DB", // Professional blue
			TextSpaceAfter:  "120",    // 6pt después de párrafos
			ListIndent:      "720",    // 0.5" indentación para listas
			CodeInlineColor: "E83E8C",
			CodeBlockColor:  "D4D4D4",
			CodeBlockBg:     "1E1E1E",

			// Table - Gradient purple (use single color)
			TableHeaderColor:   "FFFFFF",
			TableHeaderBg:      "667EEA", // Purple from gradient
			TableBorderColor:   "E2E8F0",
			TableStripeEnabled: true,

			// Special Blocks - Modern colors
			InfoColor:    "2C5282", // Blue
			WarningColor: "ED8936", // Orange
			DangerColor:  "E53E3E", // Red
			SuccessColor: "38A169", // Green
			TipColor:     "805AD5", // Purple
		}

	case "academic":
		return DOCXStyle{
			// Typography - Classic serif
			FontFamily:     "Georgia",
			FontSize:       "22", // 11pt
			FontSizeBase:   "22", // 11pt for content
			FontSizeCode:   "20", // 10pt for code
			CodeFontFamily: "Courier New",

			// Headings - Classic black
			H1Size:        "44", // 22pt
			H1Color:       "000000",
			H1Bold:        true,
			H1SpaceBefore: "360", // 18pt
			H1SpaceAfter:  "180", // 9pt
			H2Size:        "36",  // 18pt
			H2Color:       "1A1A1A",
			H2Bold:        true,
			H2SpaceBefore: "240", // 12pt
			H2SpaceAfter:  "120", // 6pt
			H3Size:        "28",  // 14pt
			H3Color:       "333333",
			H3Bold:        true,
			H3SpaceBefore: "180", // 9pt
			H3SpaceAfter:  "100", // 5pt
			H4Size:        "24",  // 12pt
			H4Color:       "4A4A4A",
			H4Bold:        true,
			H4SpaceBefore: "120", // 6pt
			H4SpaceAfter:  "80",  // 4pt

			// Text - High contrast
			TextColor:       "333333",
			TextLightColor:  "666666",
			LinkColor:       "0066CC",
			TextSpaceAfter:  "120", // 6pt
			ListIndent:      "720", // 0.5"
			CodeInlineColor: "C7254E",
			CodeBlockColor:  "000000",
			CodeBlockBg:     "F5F5F5",

			// Table - Classic black
			TableHeaderColor:   "FFFFFF",
			TableHeaderBg:      "333333",
			TableBorderColor:   "CCCCCC",
			TableStripeEnabled: true,

			// Special Blocks - Subtle colors
			InfoColor:    "004080",
			WarningColor: "EF6C00",
			DangerColor:  "C62828",
			SuccessColor: "2E7D32",
			TipColor:     "666666",
		}

	case "technical":
		return DOCXStyle{
			// Typography - Monospace
			FontFamily:     "Courier New",
			FontSize:       "22", // 11pt
			FontSizeBase:   "22", // 11pt for content
			FontSizeCode:   "22", // 11pt for code (same)
			CodeFontFamily: "Courier New",

			// Headings - Minimal
			H1Size:        "36", // 18pt
			H1Color:       "000000",
			H1Bold:        true,
			H1SpaceBefore: "240", // 12pt
			H1SpaceAfter:  "120", // 6pt
			H2Size:        "30",  // 15pt
			H2Color:       "1A1A1A",
			H2Bold:        true,
			H2SpaceBefore: "180", // 9pt
			H2SpaceAfter:  "100", // 5pt
			H3Size:        "26",  // 13pt
			H3Color:       "333333",
			H3Bold:        true,
			H3SpaceBefore: "120", // 6pt
			H3SpaceAfter:  "80",  // 4pt
			H4Size:        "22",  // 11pt
			H4Color:       "4A4A4A",
			H4Bold:        true,
			H4SpaceBefore: "100", // 5pt
			H4SpaceAfter:  "60",  // 3pt

			// Text - Monochrome
			TextColor:       "1A1A1A",
			TextLightColor:  "4A4A4A",
			LinkColor:       "0000EE", // Classic link blue
			TextSpaceAfter:  "100",    // 5pt
			ListIndent:      "720",    // 0.5"
			CodeInlineColor: "000000",
			CodeBlockColor:  "000000",
			CodeBlockBg:     "F0F0F0",

			// Table - Terminal style
			TableHeaderColor:   "00FF00", // Green terminal
			TableHeaderBg:      "000000",
			TableBorderColor:   "333333",
			TableStripeEnabled: true,

			// Special Blocks - Simple
			InfoColor:    "666666",
			WarningColor: "FF8800",
			DangerColor:  "CC0000",
			SuccessColor: "00AA00",
			TipColor:     "000000",
		}

	case "page-view":
		return DOCXStyle{
			// Typography - Calibri (Word default)
			FontFamily:     "Calibri",
			FontSize:       "22", // 11pt
			FontSizeBase:   "22", // 11pt for content
			FontSizeCode:   "20", // 10pt for code
			CodeFontFamily: "Consolas",

			// Headings - Word blue
			H1Size:        "44",     // 22pt
			H1Color:       "1F4788", // Word blue
			H1Bold:        true,
			H1SpaceBefore: "360", // 18pt
			H1SpaceAfter:  "180", // 9pt
			H2Size:        "36",  // 18pt
			H2Color:       "2E5D9E",
			H2Bold:        true,
			H2SpaceBefore: "240", // 12pt
			H2SpaceAfter:  "120", // 6pt
			H3Size:        "30",  // 15pt
			H3Color:       "4472C4",
			H3Bold:        true,
			H3SpaceBefore: "180", // 9pt
			H3SpaceAfter:  "100", // 5pt
			H4Size:        "24",  // 12pt
			H4Color:       "5B9BD5",
			H4SpaceBefore: "120", // 6pt
			H4SpaceAfter:  "80",  // 4pt
			H4Bold:        true,

			// Text - Office style
			TextColor:       "000000",
			TextLightColor:  "595959",
			LinkColor:       "0563C1", // Word link blue
			TextSpaceAfter:  "120",    // 6pt
			ListIndent:      "720",    // 0.5"
			CodeInlineColor: "C7254E",
			CodeBlockColor:  "000000",
			CodeBlockBg:     "F2F2F2",

			// Table - Office blue
			TableHeaderColor:   "FFFFFF",
			TableHeaderBg:      "4472C4", // Office blue
			TableBorderColor:   "D4D4D4",
			TableStripeEnabled: true,

			// Special Blocks - Office colors
			InfoColor:    "31708F",
			WarningColor: "8A6D3B",
			DangerColor:  "A94442",
			SuccessColor: "3C763D",
			TipColor:     "31708F",
		}

	default:
		logger.Warn("DOCX_STYLES", "Unknown theme '%s', using professional", themeName)
		return GetStyleForTheme("professional", logger)
	}
}
