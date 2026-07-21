// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package themes provides external theme loading and management functionality
package themes

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExternalTheme represents a theme loaded from external source
type ExternalTheme struct {
	Manifest  ThemeManifest     `json:"manifest"`
	Variables map[string]string `json:"variables"`
	Styles    map[string]string `json:"styles,omitempty"`
	Assets    []ThemeAsset      `json:"assets,omitempty"`
	Path      string            `json:"path,omitempty"`
	IsValid   bool              `json:"is_valid"`
	LoadTime  time.Time         `json:"load_time,omitempty"`
}

// ThemeManifest contains metadata and configuration for external themes
type ThemeManifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	License     string   `json:"license,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`

	// Theme variables (CSS custom properties)
	Variables map[string]string `json:"variables,omitempty"`

	// Compatibility constraints
	Compatibility ThemeCompatibility `json:"compatibility"`

	// Theme assets and overrides
	Styles  ThemeStyles  `json:"styles,omitempty"`
	Assets  ThemeAssets  `json:"assets,omitempty"`
	Layouts ThemeLayouts `json:"layouts,omitempty"`

	// Metadata
	Metadata ThemeMetadata `json:"metadata"`
}

// ThemeCompatibility defines version constraints
type ThemeCompatibility struct {
	MinSlideLangVersion string   `json:"minSlideLangVersion"`
	MaxSlideLangVersion string   `json:"maxSlideLangVersion,omitempty"`
	RequiredFeatures    []string `json:"requiredFeatures,omitempty"`
}

// ThemeStyles contains CSS overrides
type ThemeStyles struct {
	Overrides map[string]string `json:"overrides,omitempty"`
	Custom    string            `json:"custom,omitempty"`
}

// ThemeAssets defines external assets
type ThemeAssets struct {
	Fonts  []ThemeFont  `json:"fonts,omitempty"`
	Images []ThemeImage `json:"images,omitempty"`
	Icons  []ThemeIcon  `json:"icons,omitempty"`
}

// ThemeFont represents a font asset
type ThemeFont struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	Local   string `json:"local,omitempty"`
	Weight  string `json:"weight,omitempty"`
	Style   string `json:"style,omitempty"`
	Display string `json:"display,omitempty"`
}

// ThemeImage represents an image asset
type ThemeImage struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Usage  string `json:"usage"` // background, icon, decoration
	Alt    string `json:"alt,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// ThemeIcon represents an icon asset
type ThemeIcon struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // svg, png, icon-font
	Size string `json:"size,omitempty"`
}

// ThemeLayouts defines supported layouts
type ThemeLayouts struct {
	Supported []string          `json:"supported,omitempty"`
	Enhanced  []string          `json:"enhanced,omitempty"`
	Custom    map[string]string `json:"custom,omitempty"`
}

// ThemeMetadata contains additional theme information
type ThemeMetadata struct {
	Tags        []string  `json:"tags,omitempty"`
	Category    string    `json:"category,omitempty"`
	Preview     string    `json:"preview,omitempty"`
	Screenshots []string  `json:"screenshots,omitempty"`
	Created     time.Time `json:"created,omitempty"`
	Updated     time.Time `json:"updated,omitempty"`
	Downloads   int       `json:"downloads,omitempty"`
	Rating      float64   `json:"rating,omitempty"`
}

// ThemeAsset represents any theme asset
type ThemeAsset struct {
	Type     string `json:"type"` // font, image, icon, css, js
	Name     string `json:"name"`
	Path     string `json:"path"`
	URL      string `json:"url,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

// LoadExternalTheme loads a theme from a JSON file
func LoadExternalTheme(path string) (*ExternalTheme, error) {
	// Read theme file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read theme file %s: %w", path, err)
	}

	// Parse JSON manifest
	var manifest ThemeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse theme manifest %s: %w", path, err)
	}

	// Create external theme
	theme := &ExternalTheme{
		Manifest:  manifest,
		Variables: make(map[string]string),
		Styles:    make(map[string]string),
		Path:      path,
		LoadTime:  time.Now(),
	}

	// Extract variables from manifest
	if err := theme.extractVariables(); err != nil {
		return nil, fmt.Errorf("failed to extract variables: %w", err)
	}

	// Load additional assets
	if err := theme.loadAssets(); err != nil {
		return nil, fmt.Errorf("failed to load assets: %w", err)
	}

	return theme, nil
}

// LoadExternalThemeFromBytes builds an *ExternalTheme from in-memory manifest
// and stylesheet bytes instead of reading them from disk. LoadExternalTheme's
// disk-based search (loader.go's "./themes"/"~/.slidelang/themes" paths) has
// no filesystem to search against in a browser/WASM context — this is the
// entry point for a caller that already has the theme's bytes some other way
// (e.g. compiled in via go:embed, as cmd/wasm does with
// slidelang/themes.FS).
func LoadExternalThemeFromBytes(manifestJSON, stylesCSS []byte) (*ExternalTheme, error) {
	var manifest ThemeManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse theme manifest: %w", err)
	}

	theme := &ExternalTheme{
		Manifest:  manifest,
		Variables: make(map[string]string),
		Styles:    make(map[string]string),
		LoadTime:  time.Now(),
	}

	// Populate Styles["main"] before extractVariables (unlike
	// LoadExternalTheme, which calls extractVariables before loadAssets sets
	// Styles["main"] — extractVariables's "also extract from external CSS
	// files" branch can never actually fire there): styles.css's :root
	// variables should be picked up here.
	if len(stylesCSS) > 0 {
		theme.Styles["main"] = string(stylesCSS)
	}

	if err := theme.extractVariables(); err != nil {
		return nil, fmt.Errorf("failed to extract variables: %w", err)
	}

	return theme, nil
}

// extractVariables extracts CSS variables from the manifest
func (et *ExternalTheme) extractVariables() error {
	// First, add variables directly from the manifest.json
	if et.Manifest.Variables != nil {
		for key, value := range et.Manifest.Variables {
			et.Variables[key] = value
		}
	}

	// Add default variables that every theme should have (only if not already defined)
	defaultVars := map[string]string{
		"--slidelang-primary-color":    "#007bff",
		"--slidelang-secondary-color":  "#6c757d",
		"--slidelang-success-color":    "#28a745",
		"--slidelang-danger-color":     "#dc3545",
		"--slidelang-warning-color":    "#ffc107",
		"--slidelang-info-color":       "#17a2b8",
		"--slidelang-light-color":      "#f8f9fa",
		"--slidelang-dark-color":       "#343a40",
		"--slidelang-font-main":        "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif",
		"--slidelang-font-size-base":   "1rem",
		"--slidelang-line-height-base": "1.5",
		"--slidelang-border-radius":    "0.375rem",
		"--slidelang-border-width":     "1px",
		"--slidelang-box-shadow":       "0 0.5rem 1rem rgba(0, 0, 0, 0.15)",
		"--slidelang-transition":       "all 0.15s ease-in-out",
	}

	// Copy default variables only if they don't exist
	for key, value := range defaultVars {
		if _, exists := et.Variables[key]; !exists {
			et.Variables[key] = value
		}
	}

	// Also extract variables from external CSS files
	if mainCSS, exists := et.Styles["main"]; exists && mainCSS != "" {
		if err := et.extractCSSVariables(mainCSS); err != nil {
			// Don't fail on CSS extraction errors, just log
			fmt.Printf("⚠️  WARNING: Failed to extract CSS variables: %v\n", err)
		}
	}

	return nil
}

// extractCSSVariables extracts CSS variables from raw CSS content
func (et *ExternalTheme) extractCSSVariables(cssContent string) error {
	// Simple regex to extract CSS variables from :root {} blocks
	// This is a basic implementation - could be improved with proper CSS parsing

	lines := strings.Split(cssContent, "\n")
	inRootBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if we're entering a :root block
		if strings.Contains(trimmed, ":root") && strings.Contains(trimmed, "{") {
			inRootBlock = true
			continue
		}

		// Check if we're exiting the root block
		if inRootBlock && strings.Contains(trimmed, "}") {
			inRootBlock = false
			continue
		}

		// Extract variables if we're in a root block
		if inRootBlock && strings.HasPrefix(trimmed, "--") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				varName := strings.TrimSpace(parts[0])
				varValue := strings.TrimSpace(parts[1])
				// Remove trailing semicolon
				varValue = strings.TrimSuffix(varValue, ";")

				et.Variables[varName] = varValue
			}
		}
	}

	return nil
}

// loadAssets loads theme assets (fonts, images, etc.)
func (et *ExternalTheme) loadAssets() error {
	// Get theme directory
	themeDir := filepath.Dir(et.Path)

	// Load styles.css if it exists
	stylesPath := filepath.Join(themeDir, "styles.css")
	if _, err := os.Stat(stylesPath); err == nil {
		cssContent, err := os.ReadFile(stylesPath)
		if err == nil {
			// Store the raw CSS content
			et.Styles["main"] = string(cssContent)

			// Extract CSS variables from the styles.css file
			if err := et.extractCSSVariables(string(cssContent)); err != nil {
				// Log warning but don't fail
				fmt.Printf("Warning: failed to extract CSS variables from %s: %v\n", stylesPath, err)
			}
		}
	}

	// Convert manifest assets to theme assets
	for _, font := range et.Manifest.Assets.Fonts {
		asset := ThemeAsset{
			Type: "font",
			Name: font.Name,
			Path: font.Local,
			URL:  font.URL,
		}

		// Check if local font file exists
		if font.Local != "" {
			fullPath := filepath.Join(themeDir, font.Local)
			if info, err := os.Stat(fullPath); err == nil {
				asset.Size = info.Size()
			}
		}

		et.Assets = append(et.Assets, asset)
	}

	for _, img := range et.Manifest.Assets.Images {
		asset := ThemeAsset{
			Type: "image",
			Name: img.Name,
			Path: img.Path,
		}

		// Check if image file exists
		fullPath := filepath.Join(themeDir, img.Path)
		if info, err := os.Stat(fullPath); err == nil {
			asset.Size = info.Size()
		}

		et.Assets = append(et.Assets, asset)
	}

	return nil
}

// GetVariable returns a CSS variable value
func (et *ExternalTheme) GetVariable(name string) (string, bool) {
	// Ensure variable name starts with --
	if !strings.HasPrefix(name, "--") {
		name = "--" + name
	}

	value, exists := et.Variables[name]
	return value, exists
}

// SetVariable sets a CSS variable value
func (et *ExternalTheme) SetVariable(name, value string) {
	// Ensure variable name starts with --
	if !strings.HasPrefix(name, "--") {
		name = "--" + name
	}

	if et.Variables == nil {
		et.Variables = make(map[string]string)
	}

	et.Variables[name] = value
}

// GetVariables returns all CSS variables
func (et *ExternalTheme) GetVariables() map[string]string {
	return et.Variables
}

// ToTheme converts ExternalTheme to internal Theme format
func (et *ExternalTheme) ToTheme() Theme {
	return Theme{
		Name:        et.Manifest.Name,
		Variables:   et.Variables,
		Description: et.Manifest.Description,
		Author:      et.Manifest.Author,
		Version:     et.Manifest.Version,
		IsExternal:  true,
	}
}

// Validate checks if the external theme is valid
func (et *ExternalTheme) Validate() error {
	// Basic validation
	if et.Manifest.Name == "" {
		return fmt.Errorf("theme name is required")
	}

	if et.Manifest.Version == "" {
		return fmt.Errorf("theme version is required")
	}

	// Check required variables exist
	requiredVars := []string{
		"--slidelang-primary-color",
		"--slidelang-font-main",
	}

	for _, reqVar := range requiredVars {
		if _, exists := et.GetVariable(reqVar); !exists {
			return fmt.Errorf("required variable %s is missing", reqVar)
		}
	}

	return nil
}

// Save saves the external theme to a file
func (et *ExternalTheme) Save(path string) error {
	// Update manifest with current variables
	et.Manifest.Metadata.Updated = time.Now()

	// Marshal to JSON
	data, err := json.MarshalIndent(et.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal theme: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write theme file: %w", err)
	}

	et.Path = path
	return nil
}

// DiscoverThemes finds all theme files in a directory
func DiscoverThemes(dir string) ([]*ExternalTheme, error) {
	var themes []*ExternalTheme

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Look for .json files that might be themes
		if !d.IsDir() && (strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".theme")) {
			if theme, loadErr := LoadExternalTheme(path); loadErr == nil {
				// Validate theme
				if validErr := theme.Validate(); validErr == nil {
					theme.IsValid = true
					themes = append(themes, theme)
				}
			}
		}

		return nil
	})

	return themes, err
}
