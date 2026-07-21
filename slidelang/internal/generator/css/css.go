// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package css provides modular CSS generation for SlideLang presentations
// This package implements the Phase 1 refactoring of the theme system,
// organizing CSS into modular components while maintaining compatibility.
package css

import (
	"fmt"
	"strings"

	"go.ziradocs.com/slidelang/internal/generator/css/themes"
)

// CSSConfig holds the configuration for CSS generation
type CSSConfig struct {
	Theme             string
	CustomCSS         string
	Responsive        bool
	ModularResponsive bool // Whether responsive CSS should be generated as separate file
	Minify            bool
	// Modular configuration
	RequiredElements []string // Elements actually used in the presentation
	RequiredLayouts  []string // Layouts actually used in the presentation
	EnableNavigation bool     // Whether to include navigation CSS
	EnableUtilities  bool     // Whether to include utilities CSS
}

// DefaultCSSConfig returns the default CSS configuration
func DefaultCSSConfig() CSSConfig {
	return CSSConfig{
		Theme:             "default",
		CustomCSS:         "",
		Responsive:        true,
		ModularResponsive: false, // Default to including responsive in main CSS
		Minify:            false,
		RequiredElements:  []string{"text"}, // Default to text only
		RequiredLayouts:   []string{},       // No layouts by default
		EnableNavigation:  true,             // Default enable navigation
		EnableUtilities:   true,             // Default enable utilities
	}
}

// GenerateCSS generates complete CSS for a presentation with modular architecture
func GenerateCSS(config CSSConfig) (string, error) {
	// Validate theme exists (only for embedded themes, external themes are validated by CSSBuilder)
	if config.Theme != "" {
		availableThemes := themes.GetThemeNames()
		found := false
		for _, name := range availableThemes {
			if name == config.Theme {
				found = true
				break
			}
		}

		// Only fail if it's not an embedded theme AND appears to be invalid
		// Let CSSBuilder handle external theme validation
		if !found && len(availableThemes) > 0 {
			fmt.Printf("⚠️  WARNING: Theme '%s' not found in embedded themes. Trying as external theme...\n", config.Theme)
		}
	}

	// Build CSS with MODULAR configuration - only include what's needed
	builder := NewCSSBuilder().
		WithTheme(config.Theme).
		WithCustomCSS(config.CustomCSS).
		WithResponsive(config.Responsive).
		WithModularResponsive(config.ModularResponsive).
		WithRequiredElements(config.RequiredElements).
		WithRequiredLayouts(config.RequiredLayouts).
		WithNavigation(config.EnableNavigation).
		WithUtilities(config.EnableUtilities)

	css := builder.Build()

	// Minify if requested
	if config.Minify {
		css = minifyCSS(css)
	}

	return css, nil
}

// GetAvailableThemes returns all available themes (for documentation purposes)
func GetAvailableThemes() []string {
	return []string{"default", "dark", "minimal"}
}

// ValidateConfig validates a CSS configuration (simplified)
func ValidateConfig(config CSSConfig) error {
	// Only validate theme exists
	availableThemes := themes.GetThemeNames()
	found := false
	for _, name := range availableThemes {
		if name == config.Theme {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid theme '%s'. Available: %v", config.Theme, availableThemes)
	}

	return nil
}

// minifyCSS performs basic CSS minification
func minifyCSS(css string) string {
	// Basic minification - remove comments and extra whitespace
	// This is a simple implementation - in production you might want to use a proper CSS minifier
	lines := strings.Split(css, "\n")
	var minified []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "/*") {
			minified = append(minified, trimmed)
		}
	}

	return strings.Join(minified, " ")
}

// GetThemeInfo returns information about all available themes
func GetThemeInfo() map[string]themes.Theme {
	return themes.EmbeddedThemes
}

// GetCSSStats returns statistics about the generated CSS
type CSSStats struct {
	TotalLines int
	TotalSize  int
	ThemeVars  int
}

// GetCSSStats analyzes generated CSS and returns statistics
func GetCSSStats(css string, config CSSConfig) CSSStats {
	lines := strings.Split(css, "\n")

	// Count theme variables
	theme := themes.GetTheme(config.Theme)

	return CSSStats{
		TotalLines: len(lines),
		TotalSize:  len(css),
		ThemeVars:  len(theme.Variables),
	}
}

// CreateModularCSSConfig creates a CSS config with automatic element detection
func CreateModularCSSConfig(theme string, elementTypes []string, enableNavigation, enableUtilities bool) CSSConfig {
	return CSSConfig{
		Theme:            theme,
		CustomCSS:        "",
		Responsive:       true,
		Minify:           false,
		RequiredElements: DetectRequiredElements(elementTypes),
		RequiredLayouts:  []string{}, // TODO: Implement layout detection
		EnableNavigation: enableNavigation,
		EnableUtilities:  enableUtilities,
	}
}

// DetectRequiredElements analyzes element types and returns required CSS modules
func DetectRequiredElements(elementTypes []string) []string {
	var required []string
	seen := make(map[string]bool)

	// Always include text as it's core
	required = append(required, "text")
	seen["text"] = true

	// Map element types to CSS modules
	for _, elementType := range elementTypes {
		var module string
		switch elementType {
		case "code":
			module = "code"
		case "image", "gallery":
			module = "images"
		case "table":
			module = "tables"
		case "info", "warning", "error", "success", "card":
			module = "blocks"
		case "quote":
			module = "quotes"
		case "checklist":
			module = "checklists"
		case "map":
			module = "maps"
		case "header", "footer":
			module = "headers_footers"
		}

		if module != "" && !seen[module] {
			required = append(required, module)
			seen[module] = true
		}
	}

	return required
}
