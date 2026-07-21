// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"regexp"
	"sort"
	"strings"
)

// ThemeVariables contains all CSS variables for themes
type ThemeVariables map[string]string

// Theme represents a complete theme definition
type Theme struct {
	Name        string
	Description string
	Variables   ThemeVariables
	Author      string
	Version     string
	IsExternal  bool // Indicates if theme is loaded from external source
}

// GetDefaultTheme returns the default modern theme
func GetDefaultTheme() Theme {
	return Theme{
		Name:        "default",
		Description: "Modern professional theme with clean design and vibrant accents",
		Author:      "SlideLang",
		Version:     "2.0.0",
		Variables: ThemeVariables{
			// Core Brand Colors - Modern 2025 palette
			"--primary-color":   "#2563eb", // Modern blue
			"--secondary-color": "#1e293b", // Slate dark
			"--accent-color":    "#06b6d4", // Cyan accent
			"--success-color":   "#10b981", // Emerald
			"--warning-color":   "#f59e0b", // Amber
			"--danger-color":    "#ef4444", // Red
			"--info-color":      "#3b82f6", // Blue
			"--tip-color":       "#8b5cf6", // Violet

			// Modern Gradients with depth
			"--gradient-bg":     "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
			"--title-gradient":  "linear-gradient(135deg, #2563eb 0%, #1d4ed8 50%, #1e40af 100%)",
			"--accent-gradient": "linear-gradient(135deg, #06b6d4 0%, #0891b2 100%)",

			// Typography - Modern font stack
			"--font-main":    "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
			"--font-code":    "'JetBrains Mono', 'Fira Code', 'SF Mono', Consolas, monospace",
			"--font-heading": "'Inter', -apple-system, BlinkMacSystemFont, sans-serif",

			// Modern spacing and shapes
			"--border-radius":    "0.75rem", // 12px
			"--border-radius-lg": "1.5rem",  // 24px
			"--border-radius-sm": "0.5rem",  // 8px

			// Enhanced shadows with modern depth
			"--shadow-main": "0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)",
			"--shadow-lg":   "0 25px 50px -12px rgba(0, 0, 0, 0.25)",
			"--shadow-xl":   "0 35px 60px -12px rgba(0, 0, 0, 0.3)",

			// Refined transitions
			"--transition":      "all 0.3s cubic-bezier(0.4, 0, 0.2, 1)",
			"--transition-fast": "all 0.15s cubic-bezier(0.4, 0, 0.2, 1)",

			// Text colors with improved contrast
			"--text-color":      "#0f172a", // Slate 900
			"--text-light":      "#64748b", // Slate 500
			"--text-muted":      "#94a3b8", // Slate 400
			"--text-on-primary": "#ffffff",
			"--text-on-accent":  "#ffffff",
			"--text-on-dark":    "#ffffff",

			// Background colors - Modern neutral palette
			"--bg-white":    "#ffffff",
			"--bg-gray-50":  "#f8fafc", // Slate 50
			"--bg-gray-100": "#f1f5f9", // Slate 100
			"--bg-code":     "#1e293b", // Slate 800 - Asegurar que sea oscuro
			"--bg-light":    "#f8fafc",

			// Semantic backgrounds with modern colors
			"--bg-info":    "linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)",
			"--bg-success": "linear-gradient(135deg, #ecfdf5 0%, #d1fae5 100%)",
			"--bg-warning": "linear-gradient(135deg, #fffbeb 0%, #fef3c7 100%)",
			"--bg-danger":  "linear-gradient(135deg, #fef2f2 0%, #fecaca 100%)",
			"--bg-tip":     "linear-gradient(135deg, #f5f3ff 0%, #ede9fe 100%)",

			// Enhanced state backgrounds
			"--bg-note":          "#f8fafc",
			"--bg-success-light": "#ecfdf5",
			"--bg-warning-light": "#fffbeb",
			"--bg-danger-light":  "#fef2f2",
			"--bg-info-light":    "#eff6ff",
			"--bg-placeholder":   "rgba(37, 99, 235, 0.05)",

			// Modern borders
			"--border-color": "#e2e8f0", // Slate 200

			// State text colors with better accessibility
			"--note-color":           "#64748b",
			"--note-text-color":      "#334155",
			"--details-border-color": "#d1d5db",
			"--details-text-color":   "#374151",
			"--success-text-color":   "#065f46",
			"--warning-text-color":   "#92400e",
			"--danger-text-color":    "#991b1b",
			"--info-text-color":      "#1e40af",

			// Highlight/mark colors for text emphasis
			"--highlight-bg":   "#fef08a", // Yellow 200 - bright highlight
			"--highlight-text": "#713f12", // Yellow 900 - dark text on highlight

			// Link colors for better visibility
			"--link-color":       "#2563eb",                 // Blue 600 - clearly distinct link color
			"--link-bg":          "rgba(37, 99, 235, 0.03)", // Very subtle background for links
			"--link-hover-color": "#1d4ed8",                 // Blue 700 - darker on hover
			"--link-hover-bg":    "rgba(37, 99, 235, 0.08)", // Blue 600 with low opacity for hover

			// Slide-specific backgrounds
			"--bg-title-slide":   "var(--title-gradient)",
			"--bg-section-slide": "var(--gradient-bg)",
			"--bg-content-slide": "var(--bg-white)",
			"--bg-end-slide":     "var(--accent-gradient)",
			"--bg-closing-slide": "var(--title-gradient)", // Similar to title but for closing

			// Modern shadow variations
			"--shadow-text":   "rgba(0, 0, 0, 0.25)",
			"--shadow-light":  "rgba(0, 0, 0, 0.05)",
			"--shadow-medium": "rgba(0, 0, 0, 0.15)",

			// Enhanced syntax highlighting
			"--syntax-comment":  "#6b7280", // Gray 500
			"--syntax-keyword":  "#2563eb", // Blue 600
			"--syntax-string":   "#059669", // Emerald 600
			"--syntax-number":   "#d97706", // Amber 600
			"--syntax-operator": "#dc2626", // Red 600
			"--syntax-function": "#7c3aed", // Violet 600
			"--syntax-variable": "#dc2626", // Red 600
			"--syntax-type":     "#0d9488", // Teal 600

			// Interactive elements with modern styling
			"--bg-quote":             "#f8fafc",
			"--bg-hover":             "#f1f5f9",
			"--bg-progress":          "#e2e8f0",
			"--bg-copy-button":       "rgba(255,255,255,0.15)",
			"--border-copy-button":   "rgba(255,255,255,0.25)",
			"--bg-copy-button-hover": "rgba(255,255,255,0.25)",
			"--bg-line-highlight":    "rgba(255,255,255,0.1)",
			"--bg-diff-added":        "rgba(16, 185, 129, 0.1)",
			"--bg-diff-removed":      "rgba(239, 68, 68, 0.1)",
			"--bg-image-overlay":     "rgba(15, 23, 42, 0.6)",
			"--bg-comparison-label":  "rgba(15, 23, 42, 0.9)",
			"--bg-lightbox":          "rgba(15, 23, 42, 0.95)",
		},
	}
}

// GetDarkTheme returns the sophisticated dark theme with high impact
func GetDarkTheme() Theme {
	return Theme{
		Name:        "dark",
		Description: "Premium dark theme with striking gradients, vibrant accents and cinematic depth",
		Author:      "SlideLang",
		Version:     "2.0.0",
		Variables: ThemeVariables{
			// Core Colors - Vibrant dark palette with electric accents
			"--primary-color":   "#3b82f6", // Blue 500 - vibrant primary
			"--secondary-color": "#1e293b", // Slate 800 - deep secondary
			"--accent-color":    "#06d6a0", // Electric teal - striking accent
			"--success-color":   "#10b981", // Emerald 500 - clear success
			"--warning-color":   "#f59e0b", // Amber 500 - attention-grabbing
			"--danger-color":    "#ef4444", // Red 500 - vibrant danger
			"--info-color":      "#06b6d4", // Cyan 500 - informative
			"--tip-color":       "#8b5cf6", // Violet 500 - distinctive

			// Dramatic gradients with cinematic depth
			"--gradient-bg":     "linear-gradient(135deg, #0c0a09 0%, #1c1917 25%, #292524 50%, #1c1917 75%, #0c0a09 100%)",
			"--title-gradient":  "linear-gradient(135deg, #1e1b4b 0%, #312e81 25%, #3730a3 50%, #312e81 75%, #1e1b4b 100%)",
			"--accent-gradient": "linear-gradient(135deg, #065f46 0%, #047857 25%, #059669 50%, #06d6a0 75%, #34d399 100%)",

			// Typography optimized for dark mode elegance
			"--font-main":    "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
			"--font-code":    "'JetBrains Mono', 'Fira Code', 'SF Mono', Consolas, monospace",
			"--font-heading": "'Inter', -apple-system, BlinkMacSystemFont, sans-serif",

			// Modern spacing with dramatic appeal
			"--border-radius":    "0.75rem", // 12px - balanced elegance
			"--border-radius-lg": "1.5rem",  // 24px - dramatic curves
			"--border-radius-sm": "0.5rem",  // 8px - subtle rounds

			// Cinematic shadows with depth and glow
			"--shadow-main": "0 8px 25px 0 rgba(0, 0, 0, 0.5), 0 4px 12px 0 rgba(6, 214, 160, 0.1)",
			"--shadow-lg":   "0 25px 50px 0 rgba(0, 0, 0, 0.7), 0 12px 25px 0 rgba(6, 214, 160, 0.15)",
			"--shadow-xl":   "0 40px 80px 0 rgba(0, 0, 0, 0.8), 0 20px 40px 0 rgba(6, 214, 160, 0.2)",

			// Smooth cinematic transitions
			"--transition":      "all 0.4s cubic-bezier(0.23, 1, 0.32, 1)",
			"--transition-fast": "all 0.2s cubic-bezier(0.23, 1, 0.32, 1)",

			// Dark mode text with electric contrast and high visibility
			"--text-color":      "#ffffff", // Pure white - maximum readability
			"--text-light":      "#e2e8f0", // Slate 200 - bright secondary
			"--text-muted":      "#cbd5e1", // Slate 300 - visible tertiary
			"--text-on-primary": "#ffffff",
			"--text-on-accent":  "#000000", // Black text on bright accent
			"--text-on-dark":    "#ffffff",

			// Dark backgrounds with dramatic depth
			"--bg-white":    "#0c0a09", // Stone 950 - deep main background
			"--bg-gray-50":  "#1c1917", // Stone 900 - subtle contrast
			"--bg-gray-100": "#292524", // Stone 800 - light backgrounds
			"--bg-code":     "#0a0a0a", // Almost black - code background
			"--bg-light":    "#1c1917",

			// Vibrant semantic backgrounds with glow effects
			"--bg-info":    "linear-gradient(135deg, rgba(6, 182, 212, 0.15) 0%, rgba(14, 165, 233, 0.2) 100%)",
			"--bg-success": "linear-gradient(135deg, rgba(16, 185, 129, 0.15) 0%, rgba(52, 211, 153, 0.2) 100%)",
			"--bg-warning": "linear-gradient(135deg, rgba(245, 158, 11, 0.15) 0%, rgba(251, 191, 36, 0.2) 100%)",
			"--bg-danger":  "linear-gradient(135deg, rgba(239, 68, 68, 0.15) 0%, rgba(248, 113, 113, 0.2) 100%)",
			"--bg-tip":     "linear-gradient(135deg, rgba(139, 92, 246, 0.15) 0%, rgba(167, 139, 250, 0.2) 100%)",

			// Dark state backgrounds with subtle illumination
			"--bg-note":          "#1c1917",
			"--bg-success-light": "rgba(16, 185, 129, 0.12)",
			"--bg-warning-light": "rgba(245, 158, 11, 0.12)",
			"--bg-danger-light":  "rgba(239, 68, 68, 0.12)",
			"--bg-info-light":    "rgba(6, 182, 212, 0.12)",
			"--bg-placeholder":   "rgba(6, 214, 160, 0.08)",

			// Glowing borders
			"--border-color": "#404040", // Neutral 700 - visible in dark

			// Electric state text colors with high contrast
			"--note-color":           "#e2e8f0", // Brighter note color
			"--note-text-color":      "#ffffff", // Pure white for notes
			"--details-border-color": "#525252",
			"--details-text-color":   "#ffffff", // Pure white for details
			"--success-text-color":   "#34d399",
			"--warning-text-color":   "#fbbf24",
			"--danger-text-color":    "#f87171",
			"--info-text-color":      "#06b6d4",

			// Highlight/mark colors for dark theme - electric glow
			"--highlight-bg":   "#fbbf24", // Amber 400 - bright against dark
			"--highlight-text": "#1f2937", // Gray 800 - dark text on bright bg

			// Link colors for dark theme - electric blue accents
			"--link-color":       "#60a5fa",                  // Blue 400 - bright and visible against dark
			"--link-bg":          "rgba(96, 165, 250, 0.05)", // Very subtle blue glow for links
			"--link-hover-color": "#34d399",                  // Emerald 400 - electric accent on hover
			"--link-hover-bg":    "rgba(96, 165, 250, 0.15)", // Blue 400 with glow effect

			// Dramatic slide backgrounds
			"--bg-title-slide":   "var(--title-gradient)",
			"--bg-section-slide": "var(--gradient-bg)",
			"--bg-content-slide": "var(--bg-white)",
			"--bg-end-slide":     "var(--accent-gradient)",
			"--bg-closing-slide": "var(--title-gradient)", // Similar to title but for closing

			// Cinematic shadows with accent glow
			"--shadow-text":   "rgba(0, 0, 0, 0.9)",
			"--shadow-light":  "rgba(0, 0, 0, 0.4)",
			"--shadow-medium": "rgba(0, 0, 0, 0.6)",

			// Electric syntax highlighting
			"--syntax-comment":  "#94a3b8", // Slate 400 - muted
			"--syntax-keyword":  "#3b82f6", // Blue 500 - vibrant
			"--syntax-string":   "#10b981", // Emerald 500 - distinct
			"--syntax-number":   "#f59e0b", // Amber 500 - bright
			"--syntax-operator": "#ef4444", // Red 500 - striking
			"--syntax-function": "#8b5cf6", // Violet 500 - electric
			"--syntax-variable": "#06d6a0", // Teal accent - signature
			"--syntax-type":     "#06b6d4", // Cyan 500 - info

			// Interactive elements with glow effects
			"--bg-quote":             "#1c1917",
			"--bg-hover":             "#292524",
			"--bg-progress":          "#404040",
			"--bg-copy-button":       "rgba(6,214,160,0.15)",
			"--border-copy-button":   "rgba(6,214,160,0.3)",
			"--bg-copy-button-hover": "rgba(6,214,160,0.25)",
			"--bg-line-highlight":    "rgba(6,214,160,0.1)",
			"--bg-diff-added":        "rgba(16, 185, 129, 0.2)",
			"--bg-diff-removed":      "rgba(239, 68, 68, 0.2)",
			"--bg-image-overlay":     "rgba(12, 10, 9, 0.8)",
			"--bg-comparison-label":  "rgba(12, 10, 9, 0.95)",
			"--bg-lightbox":          "rgba(12, 10, 9, 0.98)",
		},
	}
}

// GetMinimalTheme returns the modern minimal theme with elegant contrast
func GetMinimalTheme() Theme {
	return Theme{
		Name:        "minimal",
		Description: "Elegant minimal theme with high contrast, refined typography and purposeful accents",
		Author:      "SlideLang",
		Version:     "2.0.0",
		Variables: ThemeVariables{
			// Core Colors - Sophisticated minimal palette with strong contrast
			"--primary-color":   "#1f2937", // Gray 800 - strong, elegant primary
			"--secondary-color": "#4b5563", // Gray 600 - balanced secondary
			"--accent-color":    "#dc2626", // Red 600 - bold minimal accent
			"--success-color":   "#16a34a", // Green 600 - clear success
			"--warning-color":   "#ea580c", // Orange 600 - attention-grabbing
			"--danger-color":    "#dc2626", // Red 600 - consistent with accent
			"--info-color":      "#2563eb", // Blue 600 - informative
			"--tip-color":       "#7c3aed", // Violet 600 - distinctive

			// Minimal gradients - subtle but purposeful
			"--gradient-bg":     "linear-gradient(135deg, #ffffff 0%, #f8fafc 100%)",
			"--title-gradient":  "linear-gradient(135deg, #1f2937 0%, #374151 100%)",
			"--accent-gradient": "linear-gradient(135deg, #dc2626 0%, #b91c1c 100%)",

			// Typography - Premium minimal font stack with strong hierarchy
			"--font-main":    "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
			"--font-code":    "'JetBrains Mono', 'SF Mono', 'Fira Code', Consolas, monospace",
			"--font-heading": "'Inter', -apple-system, BlinkMacSystemFont, sans-serif",

			// Minimal geometry - precise and intentional
			"--border-radius":    "0.5rem",  // 8px - balanced minimal feel
			"--border-radius-lg": "1rem",    // 16px
			"--border-radius-sm": "0.25rem", // 4px

			// Refined shadows - minimal but impactful
			"--shadow-main": "0 2px 8px 0 rgba(0, 0, 0, 0.12), 0 2px 4px 0 rgba(0, 0, 0, 0.08)",
			"--shadow-lg":   "0 12px 28px 0 rgba(0, 0, 0, 0.16), 0 4px 8px 0 rgba(0, 0, 0, 0.08)",
			"--shadow-xl":   "0 24px 48px 0 rgba(0, 0, 0, 0.20), 0 8px 16px 0 rgba(0, 0, 0, 0.12)",

			// Crisp minimal transitions
			"--transition":      "all 0.25s cubic-bezier(0.4, 0, 0.2, 1)",
			"--transition-fast": "all 0.15s cubic-bezier(0.4, 0, 0.2, 1)",

			// Text colors - maximum contrast and clarity
			"--text-color":      "#111827", // Gray 900 - maximum readability
			"--text-light":      "#6b7280", // Gray 500 - clear secondary text
			"--text-muted":      "#9ca3af", // Gray 400 - subtle tertiary
			"--text-on-primary": "#ffffff",
			"--text-on-accent":  "#ffffff",
			"--text-on-dark":    "#ffffff",

			// Clean backgrounds - pure with subtle depth
			"--bg-white":    "#ffffff",
			"--bg-gray-50":  "#f9fafb", // Gray 50 - subtle contrast
			"--bg-gray-100": "#f3f4f6", // Gray 100 - light backgrounds
			"--bg-code":     "#1e293b", // Slate 800 - Oscuro incluso en tema minimal
			"--bg-light":    "#f9fafb",

			// Minimal semantic backgrounds - clean and purposeful
			"--bg-info":    "linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)",
			"--bg-success": "linear-gradient(135deg, #f0fdf4 0%, #dcfce7 100%)",
			"--bg-warning": "linear-gradient(135deg, #fffbeb 0%, #fed7aa 100%)",
			"--bg-danger":  "linear-gradient(135deg, #fef2f2 0%, #fecaca 100%)",
			"--bg-tip":     "linear-gradient(135deg, #f5f3ff 0%, #ede9fe 100%)",

			// Minimal state backgrounds
			"--bg-note":          "#f8fafc",
			"--bg-success-light": "#f0fdf4",
			"--bg-warning-light": "#fffbeb",
			"--bg-danger-light":  "#fef2f2",
			"--bg-info-light":    "#eff6ff",
			"--bg-placeholder":   "rgba(220, 38, 38, 0.04)",

			// Clean borders
			"--border-color": "#e5e7eb", // Gray 200 - subtle but visible

			// Clear state text colors
			"--note-color":           "#6b7280",
			"--note-text-color":      "#374151",
			"--details-border-color": "#d1d5db",
			"--details-text-color":   "#1f2937",
			"--success-text-color":   "#166534",
			"--warning-text-color":   "#9a3412",
			"--danger-text-color":    "#991b1b",
			"--info-text-color":      "#1d4ed8",

			// Link colors for minimal theme - subtle but visible
			"--link-color":       "#2563eb",                 // Blue 600 - clear and readable
			"--link-bg":          "rgba(37, 99, 235, 0.02)", // Very minimal background
			"--link-hover-color": "#1d4ed8",                 // Blue 700 - slightly darker on hover
			"--link-hover-bg":    "rgba(37, 99, 235, 0.05)", // Minimal hover background

			// Strong slide backgrounds - more visual impact
			"--bg-title-slide":   "var(--title-gradient)",
			"--bg-section-slide": "var(--bg-white)",
			"--bg-content-slide": "var(--bg-white)",
			"--bg-end-slide":     "var(--accent-gradient)",
			"--bg-closing-slide": "var(--title-gradient)", // Similar to title but for closing

			// Purposeful shadows
			"--shadow-text":   "rgba(0, 0, 0, 0.25)",
			"--shadow-light":  "rgba(0, 0, 0, 0.06)",
			"--shadow-medium": "rgba(0, 0, 0, 0.12)",

			// Minimal syntax highlighting - clear and readable
			"--syntax-comment":  "#9ca3af", // Gray 400
			"--syntax-keyword":  "#1f2937", // Gray 800 - strong
			"--syntax-string":   "#16a34a", // Green 600 - distinctive
			"--syntax-number":   "#ea580c", // Orange 600 - clear
			"--syntax-operator": "#6b7280", // Gray 500
			"--syntax-function": "#7c3aed", // Violet 600 - purposeful
			"--syntax-variable": "#dc2626", // Red 600 - accent color
			"--syntax-type":     "#2563eb", // Blue 600 - info color

			// Clean interactive elements
			"--bg-quote":             "#f8fafc",
			"--bg-hover":             "#f3f4f6",
			"--bg-progress":          "#e5e7eb",
			"--bg-copy-button":       "rgba(255,255,255,0.9)",
			"--border-copy-button":   "rgba(107,114,128,0.3)",
			"--bg-copy-button-hover": "rgba(255,255,255,1)",
			"--bg-line-highlight":    "rgba(220,38,38,0.08)",
			"--bg-diff-added":        "rgba(22, 163, 74, 0.12)",
			"--bg-diff-removed":      "rgba(220, 38, 38, 0.12)",
			"--bg-image-overlay":     "rgba(31, 41, 55, 0.5)",
			"--bg-comparison-label":  "rgba(31, 41, 55, 0.9)",
			"--bg-lightbox":          "rgba(31, 41, 55, 0.95)",
		},
	}
}

// EmbeddedThemes contains all built-in themes
var EmbeddedThemes = map[string]Theme{
	"default": GetDefaultTheme(),
	"dark":    GetDarkTheme(),
	"minimal": GetMinimalTheme(),
}

// GenerateThemeCSS generates CSS variables for a theme with proper namespacing
func GenerateThemeCSS(theme Theme) string {
	names := make([]string, 0, len(theme.Variables))
	for variable := range theme.Variables {
		names = append(names, variable)
	}
	sort.Strings(names)

	css := ":root {\n"
	for _, variable := range names {
		value := theme.Variables[variable]
		// Add slidelang- prefix if not already present
		cssVar := variable
		if !strings.HasPrefix(variable, "--slidelang-") {
			if strings.HasPrefix(variable, "--") {
				cssVar = "--slidelang-" + variable[2:]
			} else {
				cssVar = "--slidelang-" + variable
			}
		}

		// Process the value to namespace any var() references
		namespacedValue := namespaceVariableReferences(value)

		css += "  " + cssVar + ": " + namespacedValue + ";\n"
	}
	css += "}\n"
	return css
}

// namespaceVariableReferences processes CSS values to namespace any var() references
func namespaceVariableReferences(value string) string {
	// This function handles nested var() calls by processing them recursively

	// Use a more sophisticated approach to handle nested var() calls
	result := value
	changed := true
	for changed {
		changed = false
		// Find var(--variable-name) or var(--variable-name, fallback-value)
		re := regexp.MustCompile(`var\(--([a-zA-Z0-9_-]+)(?:\s*,\s*([^)]*))?\)`)

		result = re.ReplaceAllStringFunc(result, func(match string) string {
			matches := re.FindStringSubmatch(match)
			varName := matches[1]
			fallback := ""
			if len(matches) > 2 && matches[2] != "" {
				fallback = matches[2]
			}

			// Don't namespace if already namespaced
			if strings.HasPrefix(varName, "slidelang-") {
				return match
			}

			// Process the variable name
			namespacedVar := "slidelang-" + varName
			changed = true

			// Reconstruct the var() function
			if fallback != "" {
				return "var(--" + namespacedVar + ", " + fallback + ")"
			} else {
				return "var(--" + namespacedVar + ")"
			}
		})
	}

	return result
}

// GetThemeNames returns all available embedded theme names
func GetThemeNames() []string {
	names := make([]string, 0, len(EmbeddedThemes))
	for name := range EmbeddedThemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetTheme returns a theme by name, or default if not found
func GetTheme(name string) Theme {
	if theme, exists := EmbeddedThemes[name]; exists {
		return theme
	}
	return GetDefaultTheme()
}

// ValidateTheme validates that a theme has all required variables
func ValidateTheme(theme Theme) []string {
	requiredVars := []string{
		"--slidelang-primary-color",
		"--slidelang-secondary-color",
		"--slidelang-accent-color",
		"--slidelang-text-color",
		"--slidelang-bg-white",
		"--slidelang-font-main",
		"--slidelang-border-radius",
		"--slidelang-transition",
	}

	var missing []string
	for _, varName := range requiredVars {
		if _, exists := theme.Variables[varName]; !exists {
			missing = append(missing, varName)
		}
	}

	return missing
}
