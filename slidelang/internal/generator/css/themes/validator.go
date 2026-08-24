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

	// motor-temas-v2.md §2.3: a font declared in a stack but not backed by
	// any assets.fonts entry silently falls back to the reader's installed
	// fonts — the exact failure mode this whole feature exists to surface.
	tv.validateFontFamilyBacking(theme, result)

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
	case hasFontStackSuffix(name):
		// Previously matched on strings.Contains(name, "font-family"), which
		// no real variable name contains — every theme uses -font-main/
		// -font-code/-font-heading, so this branch never fired and
		// isValidFontFamily was dead code.
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

		// Namespacing contract (motor-temas-v2.md §2.1) — strict mode
		// only. ThemeLoader.LoadTheme uses the NON-strict validator and
		// falls back to the "default" theme on any validation error, so
		// promoting these to unconditional errors would make every theme
		// shipped today that predates this contract (modern-blue,
		// startup-tech, startup-tech-solid — none of them prefix their
		// selectors, and the first two don't prefix their variables
		// either) stop loading entirely instead of just rendering with
		// broken decorative CSS, which is the regression these rules
		// exist to prevent, not cause. The motor deliberately does not
		// rewrite a third-party theme's CSS to fix this for it (that's
		// what NamespaceStylesheet does for the toolchain's OWN base
		// CSS) — an author finds out via `themes validate --strict`.
		if tv.strictMode {
			if unprefixed := UnprefixedVarNames(css); len(unprefixed) > 0 {
				result.Errors = append(result.Errors, fmt.Sprintf(
					"%s section: %d CSS variable(s) without the --slidelang- prefix (e.g. var(--%s)) — they will not resolve against this theme's own :root block",
					section, len(unprefixed), unprefixed[0]))
			}
			if unprefixed := UnprefixedClassSelectors(css); len(unprefixed) > 0 {
				result.Errors = append(result.Errors, fmt.Sprintf(
					"%s section: %d class selector(s) without the .slidelang- prefix (e.g. .%s) — they will not match the toolchain's namespaced markup",
					section, len(unprefixed), unprefixed[0]))
			}
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

		// motor-temas-v2.md §2.3: fonts are always self-hosted, so "local"
		// is required and "url" is rejected outright — not just validated
		// as a well-formed URL above. A font declared only via "url" would
		// need a build-time download to actually self-host it, reintroducing
		// the exact network dependency this decision removes, plus a
		// typeface-redistribution question this repo doesn't want to own.
		if asset.Type == "font" {
			if asset.Path == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("font asset %q requires 'local' — fonts are always self-hosted with the theme", asset.Name))
			}
			if asset.URL != "" {
				result.Errors = append(result.Errors, fmt.Sprintf("font asset %q must not declare 'url' — self-hosted fonts only ('local')", asset.Name))
			}
		}
	}

	// Check total assets size
	if totalSize > tv.maxFileSize {
		result.Errors = append(result.Errors, fmt.Sprintf("total assets size exceeds limit (%d bytes)", tv.maxFileSize))
	}
}

// fontStackVariableSuffixes are the theme variable names that carry a
// font-family stack, matched by suffix so both the --slidelang- prefixed
// form (external themes) and the unprefixed form (embedded Go themes, see
// variables.go) are covered.
var fontStackVariableSuffixes = []string{"-font-main", "-font-code", "-font-heading"}

// validateFontFamilyBacking warns when a font-family stack's first entry —
// the only one that needs backing; the rest are system fallbacks by design,
// see motor-temas-v2.md §2.3 — names a family that no assets.fonts entry
// declares. This is the exact failure mode elegant-minimal has today
// (Playfair Display/Crimson Text/Berkeley Mono, no assets section at all):
// the family silently falls back to whatever the reader has installed. A
// Warning, not an Error — many perfectly valid themes intentionally rely on
// system fonts for a body-text stack ("'Segoe UI', Tahoma, ... sans-serif"
// has no theme-shipped font at all) and that must keep loading.
func (tv *ThemeValidator) validateFontFamilyBacking(theme *ExternalTheme, result *ValidationResult) {
	knownFamilies := make(map[string]bool, len(theme.Manifest.Assets.Fonts))
	for _, font := range theme.Manifest.Assets.Fonts {
		if font.Name != "" {
			knownFamilies[strings.ToLower(font.Name)] = true
		}
	}

	for name, value := range theme.Variables {
		if !hasFontStackSuffix(name) {
			continue
		}
		entries := SplitFontStack(value)
		if len(entries) == 0 {
			continue
		}
		first := unquoteFontFamily(entries[0])
		if first == "" || genericFontFamilyKeywords[strings.ToLower(first)] {
			continue
		}
		if !knownFamilies[strings.ToLower(first)] {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"%s declares %q as its primary font, but no assets.fonts entry backs it — it will fall back to whatever the reader has installed", name, first))
		}
	}
}

func hasFontStackSuffix(varName string) bool {
	for _, suffix := range fontStackVariableSuffixes {
		if strings.HasSuffix(varName, suffix) {
			return true
		}
	}
	return false
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
