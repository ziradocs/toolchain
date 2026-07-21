// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"fmt"
	"regexp"
	"strings"
)

// ThemeValidator provides validation for external themes
type ThemeValidator struct {
	requiredVariables []string
	optionalVariables []string
	maxFileSize       int64
	allowedExtensions []string
	strictMode        bool
}

// ValidationResult contains the result of theme validation
type ValidationResult struct {
	IsValid  bool     `json:"is_valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// NewThemeValidator creates a new theme validator with default rules
func NewThemeValidator() *ThemeValidator {
	return &ThemeValidator{
		requiredVariables: []string{
			"--slidelang-primary-color",
			"--slidelang-secondary-color",
			"--slidelang-font-main",
			"--slidelang-font-size-base",
			"--slidelang-line-height-base",
			"--slidelang-background-color",
			"--slidelang-text-color",
		},
		optionalVariables: []string{
			"--slidelang-accent-color",
			"--slidelang-success-color",
			"--slidelang-warning-color",
			"--slidelang-danger-color",
			"--slidelang-info-color",
			"--slidelang-border-radius",
			"--slidelang-border-width",
			"--slidelang-box-shadow",
			"--slidelang-transition",
			"--slidelang-gradient-bg",
			"--slidelang-title-gradient",
			"--slidelang-bg-white",
			"--slidelang-bg-code",
			"--slidelang-bg-light",
			"--slidelang-bg-title-slide",
			"--slidelang-bg-section-slide",
			"--slidelang-bg-content-slide",
			"--slidelang-bg-end-slide",
			"--slidelang-shadow-text",
			"--slidelang-shadow-light",
			"--slidelang-shadow-medium",
			// Block and badge variables
			"--slidelang-bg-note",
			"--slidelang-bg-success-light",
			"--slidelang-bg-warning-light",
			"--slidelang-bg-danger-light",
			"--slidelang-bg-info-light",
			"--slidelang-note-color",
			"--slidelang-note-text-color",
			"--slidelang-details-border-color",
			"--slidelang-details-text-color",
			"--slidelang-success-text-color",
			"--slidelang-warning-text-color",
			"--slidelang-danger-text-color",
			"--slidelang-info-text-color",
			"--slidelang-text-muted",
		},
		maxFileSize:       10 * 1024 * 1024, // 10MB
		allowedExtensions: []string{".json", ".theme"},
		strictMode:        false,
	}
}

// NewStrictThemeValidator creates a validator with strict validation rules
func NewStrictThemeValidator() *ThemeValidator {
	validator := NewThemeValidator()
	validator.strictMode = true
	return validator
}

// ValidateTheme validates an external theme
func (tv *ThemeValidator) ValidateTheme(theme *ExternalTheme) error {
	result := tv.ValidateThemeDetailed(theme)

	if !result.IsValid {
		return fmt.Errorf("theme validation failed: %v", strings.Join(result.Errors, "; "))
	}

	return nil
}

// ValidateThemeDetailed performs detailed validation and returns full results
func (tv *ThemeValidator) ValidateThemeDetailed(theme *ExternalTheme) *ValidationResult {
	result := &ValidationResult{IsValid: true}

	// Validate manifest
	tv.validateManifest(&theme.Manifest, result)

	// Validate variables
	tv.validateVariables(theme.Variables, result)

	// Validate CSS syntax (if custom styles provided)
	tv.validateCSS(theme.Styles, result)

	// Validate assets
	tv.validateAssets(theme.Assets, result)

	// Validate compatibility
	tv.validateCompatibility(&theme.Manifest.Compatibility, result)

	// Set overall validity
	result.IsValid = len(result.Errors) == 0

	return result
}

// validateManifest validates the theme manifest
func (tv *ThemeValidator) validateManifest(manifest *ThemeManifest, result *ValidationResult) {
	// Required fields
	if manifest.Name == "" {
		result.Errors = append(result.Errors, "theme name is required")
	} else if !tv.isValidThemeName(manifest.Name) {
		result.Errors = append(result.Errors, "theme name contains invalid characters")
	}

	if manifest.Version == "" {
		result.Errors = append(result.Errors, "theme version is required")
	} else if !tv.isValidVersion(manifest.Version) {
		result.Errors = append(result.Errors, "theme version format is invalid")
	}

	if manifest.Description == "" {
		result.Warnings = append(result.Warnings, "theme description is recommended")
	}

	if manifest.Author == "" {
		result.Warnings = append(result.Warnings, "theme author is recommended")
	}

	// Validate keywords/tags
	if len(manifest.Keywords) > 10 {
		result.Warnings = append(result.Warnings, "too many keywords (max 10 recommended)")
	}

	if len(manifest.Metadata.Tags) > 15 {
		result.Warnings = append(result.Warnings, "too many tags (max 15 recommended)")
	}
}

// validateVariables validates CSS variables
func (tv *ThemeValidator) validateVariables(variables map[string]string, result *ValidationResult) {
	// Check required variables
	for _, required := range tv.requiredVariables {
		if value, exists := variables[required]; !exists {
			result.Errors = append(result.Errors, fmt.Sprintf("required variable %s is missing", required))
		} else if value == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("required variable %s is empty", required))
		} else {
			// Validate variable value based on type
			tv.validateVariableValue(required, value, result)
		}
	}

	// Check for unknown variables in strict mode
	if tv.strictMode {
		knownVars := append(tv.requiredVariables, tv.optionalVariables...)
		for varName := range variables {
			if !tv.isKnownVariable(varName, knownVars) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("unknown variable %s", varName))
			}
		}
	}

	// Validate all variable names
	for varName := range variables {
		if !strings.HasPrefix(varName, "--") {
			result.Errors = append(result.Errors, fmt.Sprintf("variable %s must start with --", varName))
		}
	}
}

// validateVariableValue validates a specific CSS variable value
func (tv *ThemeValidator) validateVariableValue(name, value string, result *ValidationResult) {
	switch {
	case strings.Contains(name, "color"):
		if !tv.isValidColor(value) {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid color value for %s: %s", name, value))
		}
	case strings.Contains(name, "font-size"):
		if !tv.isValidSize(value) {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid size value for %s: %s", name, value))
		}
	case strings.Contains(name, "font-family"):
		if !tv.isValidFontFamily(value) {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid font family for %s: %s", name, value))
		}
	case strings.Contains(name, "line-height"):
		if !tv.isValidLineHeight(value) {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid line height for %s: %s", name, value))
		}
	}
}

// validateCSS validates custom CSS styles
func (tv *ThemeValidator) validateCSS(styles map[string]string, result *ValidationResult) {
	for section, css := range styles {
		if css == "" {
			continue
		}

		// Basic CSS syntax validation
		if !tv.isValidCSS(css) {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid CSS syntax in %s section", section))
		}

		// Check for dangerous CSS
		if tv.containsDangerousCSS(css) {
			result.Errors = append(result.Errors, fmt.Sprintf("potentially dangerous CSS detected in %s section", section))
		}
	}
}

// validateAssets validates theme assets
func (tv *ThemeValidator) validateAssets(assets []ThemeAsset, result *ValidationResult) {
	totalSize := int64(0)

	for _, asset := range assets {
		// Check asset size
		if asset.Size > 0 {
			totalSize += asset.Size

			// Check individual asset size limits
			maxAssetSize := tv.getMaxAssetSize(asset.Type)
			if asset.Size > maxAssetSize {
				result.Errors = append(result.Errors, fmt.Sprintf("asset %s exceeds size limit (%d bytes)", asset.Name, maxAssetSize))
			}
		}

		// Validate asset paths
		if asset.Path != "" && !tv.isValidAssetPath(asset.Path) {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid asset path: %s", asset.Path))
		}

		// Validate URLs
		if asset.URL != "" && !tv.isValidURL(asset.URL) {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid asset URL: %s", asset.URL))
		}
	}

	// Check total assets size
	if totalSize > tv.maxFileSize {
		result.Errors = append(result.Errors, fmt.Sprintf("total assets size exceeds limit (%d bytes)", tv.maxFileSize))
	}
}

// validateCompatibility validates version compatibility
func (tv *ThemeValidator) validateCompatibility(compat *ThemeCompatibility, result *ValidationResult) {
	if compat.MinSlideLangVersion == "" {
		result.Warnings = append(result.Warnings, "minimum SlideLang version not specified")
		return
	}

	if !tv.isValidVersion(compat.MinSlideLangVersion) {
		result.Errors = append(result.Errors, "invalid minimum SlideLang version format")
	}

	if compat.MaxSlideLangVersion != "" && !tv.isValidVersion(compat.MaxSlideLangVersion) {
		result.Errors = append(result.Errors, "invalid maximum SlideLang version format")
	}
}

// Helper validation functions

func (tv *ThemeValidator) isValidThemeName(name string) bool {
	// Allow alphanumeric, hyphens, underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	return matched
}

func (tv *ThemeValidator) isValidVersion(version string) bool {
	// Basic semantic version validation
	matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+(-[a-zA-Z0-9_.-]+)?$`, version)
	return matched
}

func (tv *ThemeValidator) isValidColor(color string) bool {
	color = strings.TrimSpace(color)

	// Check for hex colors
	if matched, _ := regexp.MatchString(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`, color); matched {
		return true
	}

	// Check for rgb/rgba
	if matched, _ := regexp.MatchString(`^rgba?\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*(,\s*[\d.]+)?\s*\)$`, color); matched {
		return true
	}

	// Check for hsl/hsla
	if matched, _ := regexp.MatchString(`^hsla?\(\s*\d+\s*,\s*\d+%\s*,\s*\d+%\s*(,\s*[\d.]+)?\s*\)$`, color); matched {
		return true
	}

	// Check for CSS gradients (linear-gradient, radial-gradient, conic-gradient)
	if matched, _ := regexp.MatchString(`^(linear|radial|conic)-gradient\(.*\)$`, color); matched {
		return true
	}

	// Check for CSS color names (basic check)
	colorNames := []string{
		"black", "white", "red", "green", "blue", "yellow", "orange", "purple",
		"gray", "grey", "pink", "brown", "cyan", "magenta", "lime", "navy",
		"transparent", "inherit", "initial", "unset",
	}

	for _, colorName := range colorNames {
		if strings.EqualFold(color, colorName) {
			return true
		}
	}

	// Check for CSS variables
	if strings.HasPrefix(color, "var(") && strings.HasSuffix(color, ")") {
		return true
	}

	return false
}

func (tv *ThemeValidator) isValidSize(size string) bool {
	// Allow CSS size units
	matched, _ := regexp.MatchString(`^\d*\.?\d+(px|em|rem|%|vh|vw|pt|pc|in|cm|mm|ex|ch|lh)$`, size)
	return matched || size == "0"
}

func (tv *ThemeValidator) isValidFontFamily(fontFamily string) bool {
	// Basic font family validation
	return len(strings.TrimSpace(fontFamily)) > 0
}

func (tv *ThemeValidator) isValidLineHeight(lineHeight string) bool {
	// Allow numbers and CSS units
	if matched, _ := regexp.MatchString(`^\d*\.?\d+$`, lineHeight); matched {
		return true
	}
	return tv.isValidSize(lineHeight)
}

func (tv *ThemeValidator) isValidCSS(css string) bool {
	// Basic CSS validation - check for balanced braces
	openBraces := strings.Count(css, "{")
	closeBraces := strings.Count(css, "}")
	return openBraces == closeBraces
}

func (tv *ThemeValidator) containsDangerousCSS(css string) bool {
	// Check for potentially dangerous CSS
	dangerous := []string{
		"javascript:",
		"vbscript:",
		"expression(",
		"@import",
		"url(data:",
	}

	lowerCSS := strings.ToLower(css)
	for _, danger := range dangerous {
		if strings.Contains(lowerCSS, danger) {
			return true
		}
	}

	return false
}

func (tv *ThemeValidator) isKnownVariable(varName string, knownVars []string) bool {
	for _, known := range knownVars {
		if varName == known {
			return true
		}
	}
	return false
}

func (tv *ThemeValidator) getMaxAssetSize(assetType string) int64 {
	switch assetType {
	case "font":
		return 2 * 1024 * 1024 // 2MB for fonts
	case "image":
		return 5 * 1024 * 1024 // 5MB for images
	case "icon":
		return 100 * 1024 // 100KB for icons
	default:
		return 1 * 1024 * 1024 // 1MB for other assets
	}
}

func (tv *ThemeValidator) isValidAssetPath(path string) bool {
	// Basic path validation - no directory traversal
	return !strings.Contains(path, "..") && !strings.HasPrefix(path, "/")
}

func (tv *ThemeValidator) isValidURL(url string) bool {
	// Basic URL validation
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// SetStrictMode enables or disables strict validation
func (tv *ThemeValidator) SetStrictMode(strict bool) {
	tv.strictMode = strict
}

// AddRequiredVariable adds a required CSS variable
func (tv *ThemeValidator) AddRequiredVariable(variable string) {
	if !tv.isKnownVariable(variable, tv.requiredVariables) {
		tv.requiredVariables = append(tv.requiredVariables, variable)
	}
}

// RemoveRequiredVariable removes a required CSS variable
func (tv *ThemeValidator) RemoveRequiredVariable(variable string) {
	for i, v := range tv.requiredVariables {
		if v == variable {
			tv.requiredVariables = append(tv.requiredVariables[:i], tv.requiredVariables[i+1:]...)
			break
		}
	}
}

// GetRequiredVariables returns the list of required variables
func (tv *ThemeValidator) GetRequiredVariables() []string {
	return append([]string(nil), tv.requiredVariables...)
}
