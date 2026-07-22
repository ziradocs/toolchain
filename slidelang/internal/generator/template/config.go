// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"go.ziradocs.com/slidelang/v2/internal/generator/css"
)

// PresentationConfig define la configuración esencial para una presentación
type PresentationConfig struct {
	// Theme configuration
	Theme string

	// Custom styles and scripts
	CustomCSS string
	CustomJS  string

	// Generation options
	Responsive bool
	Minify     bool
}

// DefaultPresentationConfig returns the default configuration
func DefaultPresentationConfig() PresentationConfig {
	return PresentationConfig{
		Theme:      "default",
		Responsive: true,
		Minify:     false,
	}
}

// WithTheme sets the theme
func (pc PresentationConfig) WithTheme(theme string) PresentationConfig {
	pc.Theme = theme
	return pc
}

// WithDarkMode enables dark theme
func (pc PresentationConfig) WithDarkMode() PresentationConfig {
	pc.Theme = "dark"
	return pc
}

// WithMinimalTheme enables minimal theme
func (pc PresentationConfig) WithMinimalTheme() PresentationConfig {
	pc.Theme = "minimal"
	return pc
}

// WithCustomCSS adds custom CSS
func (pc PresentationConfig) WithCustomCSS(css string) PresentationConfig {
	pc.CustomCSS = css
	return pc
}

// WithCustomJS adds custom JavaScript
func (pc PresentationConfig) WithCustomJS(js string) PresentationConfig {
	pc.CustomJS = js
	return pc
}

// WithResponsive enables/disables responsive design
func (pc PresentationConfig) WithResponsive(enabled bool) PresentationConfig {
	pc.Responsive = enabled
	return pc
}

// WithMinify enables/disables minification
func (pc PresentationConfig) WithMinify(enabled bool) PresentationConfig {
	pc.Minify = enabled
	return pc
}

// ToTemplateBuilder converts config to TemplateBuilder
func (pc PresentationConfig) ToTemplateBuilder() *TemplateBuilder {
	return NewTemplateBuilder().
		WithTheme(pc.Theme).
		WithCustomCSS(pc.CustomCSS).
		WithCustomJS(pc.CustomJS).
		WithResponsive(pc.Responsive).
		WithMinify(pc.Minify)
}

// ToCSSConfig converts to CSS configuration
func (pc PresentationConfig) ToCSSConfig() css.CSSConfig {
	return css.CSSConfig{
		Theme:      pc.Theme,
		CustomCSS:  pc.CustomCSS,
		Responsive: pc.Responsive,
		Minify:     pc.Minify,
	}
}

// GetAvailablePresets returns predefined configuration presets
func GetAvailablePresets() map[string]PresentationConfig {
	return map[string]PresentationConfig{
		"default": DefaultPresentationConfig(),
		"dark":    DefaultPresentationConfig().WithDarkMode(),
		"minimal": DefaultPresentationConfig().WithMinimalTheme(),
	}
}
