// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.ziradocs.com/core/v2/util"
)

// ThemeLoader manages loading and caching of both embedded and external themes
type ThemeLoader struct {
	embeddedThemes map[string]Theme
	externalPaths  []string
	cache          map[string]*ExternalTheme
	validator      *ThemeValidator
	mutex          sync.RWMutex
}

// NewThemeLoader creates a new theme loader with default configuration
func NewThemeLoader() *ThemeLoader {
	loader := &ThemeLoader{
		embeddedThemes: make(map[string]Theme),
		externalPaths:  getDefaultThemePaths(),
		cache:          make(map[string]*ExternalTheme),
		validator:      NewThemeValidator(),
	}

	// Load embedded themes
	loader.loadEmbeddedThemes()

	return loader
}

// NewThemeLoaderWithPaths creates a theme loader with custom paths
func NewThemeLoaderWithPaths(paths []string) *ThemeLoader {
	loader := &ThemeLoader{
		embeddedThemes: make(map[string]Theme),
		externalPaths:  paths,
		cache:          make(map[string]*ExternalTheme),
		validator:      NewThemeValidator(),
	}

	// Load embedded themes
	loader.loadEmbeddedThemes()

	return loader
}

// getDefaultThemePaths returns default paths to search for external themes
func getDefaultThemePaths() []string {
	paths := []string{
		"./themes",            // Current directory themes folder
		"~/.slidelang/themes", // User home themes folder
	}

	// Add system-wide theme path on Unix-like systems
	if os.Getenv("SLIDELANG_THEMES_PATH") != "" {
		paths = append(paths, os.Getenv("SLIDELANG_THEMES_PATH"))
	}

	return paths
}

// loadEmbeddedThemes loads all built-in themes
func (tl *ThemeLoader) loadEmbeddedThemes() {
	// Load default theme
	tl.embeddedThemes["default"] = GetDefaultTheme()

	// Load dark theme
	tl.embeddedThemes["dark"] = GetDarkTheme()

	// Load minimal theme
	tl.embeddedThemes["minimal"] = GetMinimalTheme()
}

// LoadTheme loads a theme by name, checking embedded themes first, then
// external.
//
// trusted indicates the origin of name: true for the operator's own CLI
// invocation (e.g. `slidelang themes info <name>`, `--theme`,
// `preview-theme <path>`); false for a name sourced from a `.slidelang`
// document's frontmatter, which under this repo's threat model is
// attacker-controlled content. An untrusted name that isn't an opaque token
// (contains "/", "\", ".." or is absolute) is rejected before touching the
// filesystem, and the raw-file-path shortcut in findAndLoadExternalTheme is
// skipped entirely — both the theme JSON itself and its sibling styles.css
// (loaded by ExternalTheme.loadAssets from the same directory) would
// otherwise let a crafted frontmatter theme name read arbitrary files. See
// docs/SECURITY_AUDIT_2026-07.md, ME-2.
func (tl *ThemeLoader) LoadTheme(name string, trusted bool) (*Theme, error) {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()

	// 1. Check embedded themes first
	if theme, exists := tl.embeddedThemes[name]; exists {
		return &theme, nil
	}

	if !trusted && !util.IsOpaquePathToken(name) {
		defaultTheme := tl.embeddedThemes["default"]
		return &defaultTheme, fmt.Errorf("invalid theme name %q: must not contain path separators or '..', using default", name)
	}

	// 2. Check cache for external themes
	if external, exists := tl.cache[name]; exists {
		if external.IsValid {
			converted := external.ToTheme()
			return &converted, nil
		}
	}

	// 3. Look for external themes
	external, err := tl.findAndLoadExternalTheme(name, trusted)
	if err != nil {
		// 4. Fallback to default if not found
		defaultTheme := tl.embeddedThemes["default"]
		return &defaultTheme, fmt.Errorf("theme '%s' not found, using default: %w", name, err)
	}

	// Validate external theme
	if err := tl.validator.ValidateTheme(external); err != nil {
		// Fallback to default if validation fails
		defaultTheme := tl.embeddedThemes["default"]
		return &defaultTheme, fmt.Errorf("theme '%s' validation failed, using default: %w", name, err)
	}

	// Cache the validated theme
	external.IsValid = true
	tl.cache[name] = external

	// Convert to internal Theme format
	converted := external.ToTheme()
	return &converted, nil
}

// findAndLoadExternalTheme searches for and loads an external theme.
// trusted must already have been validated by the caller (LoadTheme) — an
// untrusted, non-opaque name never reaches this function to begin with.
func (tl *ThemeLoader) findAndLoadExternalTheme(name string, trusted bool) (*ExternalTheme, error) {
	// Check if name is already a file path. Only the operator's own trusted
	// input may be treated as a raw path (see docs/SECURITY_AUDIT_2026-07.md,
	// ME-2) — for untrusted input this shortcut is skipped and name is
	// resolved only via the confined searchPaths loop below.
	if trusted && (strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.HasSuffix(name, ".json")) {
		if _, err := os.Stat(name); err == nil {
			return LoadExternalTheme(name)
		}
	}

	// Search in configured paths
	for _, searchPath := range tl.externalPaths {
		// Expand home directory
		if strings.HasPrefix(searchPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				searchPath = filepath.Join(homeDir, searchPath[2:])
			}
		}

		// Check if search path exists
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}

		// Try different filename patterns
		patterns := []string{
			filepath.Join(searchPath, name+".json"),
			filepath.Join(searchPath, name+".theme"),
			filepath.Join(searchPath, name, "theme.json"),
			filepath.Join(searchPath, name, "manifest.json"),
		}

		for _, pattern := range patterns {
			if _, err := os.Stat(pattern); err == nil {
				return LoadExternalTheme(pattern)
			}
		}
	}

	return nil, fmt.Errorf("external theme '%s' not found in any search path", name)
}

// GetAvailableThemes returns a list of all available themes
func (tl *ThemeLoader) GetAvailableThemes() (map[string]Theme, error) {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()

	themes := make(map[string]Theme)

	// Add embedded themes
	for name, theme := range tl.embeddedThemes {
		themes[name] = theme
	}

	// Discover external themes
	externalThemes, err := tl.discoverExternalThemes()
	if err != nil {
		return themes, fmt.Errorf("error discovering external themes: %w", err)
	}

	// Add external themes
	for _, external := range externalThemes {
		if external.IsValid {
			themes[external.Manifest.Name] = external.ToTheme()
		}
	}

	return themes, nil
}

// discoverExternalThemes searches for external themes in all configured paths
func (tl *ThemeLoader) discoverExternalThemes() ([]*ExternalTheme, error) {
	var allThemes []*ExternalTheme

	for _, searchPath := range tl.externalPaths {
		// Expand home directory
		if strings.HasPrefix(searchPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				searchPath = filepath.Join(homeDir, searchPath[2:])
			}
		}

		// Check if path exists
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}

		// Discover themes in this path
		themes, err := DiscoverThemes(searchPath)
		if err != nil {
			continue // Skip paths with errors
		}

		// Validate discovered themes
		for _, theme := range themes {
			if err := tl.validator.ValidateTheme(theme); err == nil {
				theme.IsValid = true
				allThemes = append(allThemes, theme)
			}
		}
	}

	return allThemes, nil
}

// InstallTheme installs an external theme from a file
func (tl *ThemeLoader) InstallTheme(sourcePath string, installDir string) error {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	// Load theme to validate it
	theme, err := LoadExternalTheme(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to load theme: %w", err)
	}

	// Validate theme
	if err := tl.validator.ValidateTheme(theme); err != nil {
		return fmt.Errorf("theme validation failed: %w", err)
	}

	// Create install directory
	if installDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			installDir = "./themes"
		} else {
			installDir = filepath.Join(homeDir, ".slidelang", "themes")
		}
	}

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Install theme
	targetPath := filepath.Join(installDir, theme.Manifest.Name+".json")
	if err := theme.Save(targetPath); err != nil {
		return fmt.Errorf("failed to save theme: %w", err)
	}

	// Add to cache
	theme.IsValid = true
	tl.cache[theme.Manifest.Name] = theme

	return nil
}

// RemoveTheme removes an external theme
func (tl *ThemeLoader) RemoveTheme(name string) error {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	// Cannot remove embedded themes
	if _, exists := tl.embeddedThemes[name]; exists {
		return fmt.Errorf("cannot remove embedded theme '%s'", name)
	}

	// Find theme file. RemoveTheme is only ever invoked from the operator's
	// own CLI command (`slidelang themes remove <name>`), so name is trusted.
	external, err := tl.findAndLoadExternalTheme(name, true)
	if err != nil {
		return fmt.Errorf("theme '%s' not found: %w", name, err)
	}

	// Remove theme file
	if err := os.Remove(external.Path); err != nil {
		return fmt.Errorf("failed to remove theme file: %w", err)
	}

	// Remove from cache
	delete(tl.cache, name)

	return nil
}

// ClearCache clears the external theme cache
func (tl *ThemeLoader) ClearCache() {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	tl.cache = make(map[string]*ExternalTheme)
}

// AddPath adds a new search path for external themes
func (tl *ThemeLoader) AddPath(path string) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	// Check if path already exists
	for _, existing := range tl.externalPaths {
		if existing == path {
			return
		}
	}

	tl.externalPaths = append(tl.externalPaths, path)
}

// RemovePath removes a search path
func (tl *ThemeLoader) RemovePath(path string) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	for i, existing := range tl.externalPaths {
		if existing == path {
			tl.externalPaths = append(tl.externalPaths[:i], tl.externalPaths[i+1:]...)
			break
		}
	}
}

// GetPaths returns current search paths
func (tl *ThemeLoader) GetPaths() []string {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()

	// Return a copy to prevent external modification
	paths := make([]string, len(tl.externalPaths))
	copy(paths, tl.externalPaths)
	return paths
}

// ReloadThemes reloads all external themes (clears cache)
func (tl *ThemeLoader) ReloadThemes() error {
	tl.ClearCache()
	_, err := tl.GetAvailableThemes()
	return err
}

// GetExternalTheme returns the cached external theme if it exists
func (tl *ThemeLoader) GetExternalTheme(name string) (*ExternalTheme, bool) {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()

	external, exists := tl.cache[name]
	return external, exists
}
