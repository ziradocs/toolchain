// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// SlideLangConfig represents the main configuration file
type SlideLangConfig struct {
	Theme  ThemeConfig            `yaml:"theme"`
	Themes map[string]ThemeConfig `yaml:"themes,omitempty"`
	Build  BuildConfig            `yaml:"build,omitempty"`
	Server ServerConfig           `yaml:"server,omitempty"`
}

// ThemeConfig represents theme configuration
type ThemeConfig struct {
	Default       string            `yaml:"default,omitempty"`
	ExternalPaths []string          `yaml:"external_paths,omitempty"`
	Cache         bool              `yaml:"cache,omitempty"`
	Validation    string            `yaml:"validation,omitempty"` // strict, normal
	Path          string            `yaml:"path,omitempty"`
	Variables     map[string]string `yaml:"variables,omitempty"`
}

// BuildConfig represents build configuration
type BuildConfig struct {
	OutputDir string `yaml:"output_dir,omitempty"`
	Format    string `yaml:"format,omitempty"`
	LogLevel  string `yaml:"log_level,omitempty"`
	// EnableNormalization is the canonical key (decisión 2 del plan OSS: el
	// normalizador base es determinista/sin red, "AI" se reserva para el
	// llm-kit). EnableAI is a deprecated alias — LoadConfig ORs it into
	// EnableNormalization so config files written before this rename keep
	// working. Kept as a separate field (not removed) so SaveConfig doesn't
	// silently drop the old key from configs that still set it.
	EnableNormalization bool `yaml:"enable_normalization,omitempty"`
	EnableAI            bool `yaml:"enable_ai,omitempty"` // Deprecated: usa EnableNormalization / enable_normalization.
	MinifyCSS           bool `yaml:"minify_css,omitempty"`
	MinifyHTML          bool `yaml:"minify_html,omitempty"`
	IncludeAssets       bool `yaml:"include_assets,omitempty"`
	// Filters lista rutas a binarios de filtro externo (issue #240, decisión C)
	// que transforman el AST entre parse y lint — estilo Lua filters de
	// Pandoc, un proceso por filtro, JSON por stdin/stdout (ver
	// core/transform). Deliberadamente NO configurable desde el
	// frontmatter del propio documento (contenido no confiable — mismo
	// principio que getThemeName marca el theme de frontmatter como
	// untrusted): declarar binarios ejecutables solo desde flag/config del
	// operador.
	Filters     []string          `yaml:"filters,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port       int    `yaml:"port,omitempty"`
	Host       string `yaml:"host,omitempty"`
	AutoOpen   bool   `yaml:"auto_open,omitempty"`
	LiveReload bool   `yaml:"live_reload,omitempty"`
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*SlideLangConfig, error) {
	// If no specific path provided, look for default config files
	if configPath == "" {
		configPath = findConfigFile()
	}

	// If still no config file found, return default config
	if configPath == "" {
		return GetDefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Parse YAML
	var config SlideLangConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Alias deprecado: un config file con el viejo "enable_ai: true" (y sin
	// el nuevo "enable_normalization") sigue habilitando normalización.
	if config.Build.EnableAI && !config.Build.EnableNormalization {
		config.Build.EnableNormalization = true
	}

	// Apply defaults
	config = *applyDefaults(&config)

	return &config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *SlideLangConfig, configPath string) error {
	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetDefaultConfig returns default configuration
func GetDefaultConfig() *SlideLangConfig {
	return &SlideLangConfig{
		Theme: ThemeConfig{
			Default: "default",
			ExternalPaths: []string{
				"./themes",
				"~/.slidelang/themes",
			},
			Cache:      true,
			Validation: "normal",
		},
		Build: BuildConfig{
			OutputDir:     "./output",
			Format:        "html",
			MinifyCSS:     false,
			MinifyHTML:    false,
			IncludeAssets: true,
		},
		Server: ServerConfig{
			Port:       8080,
			Host:       "localhost",
			AutoOpen:   true,
			LiveReload: true,
		},
	}
}

// findConfigFile looks for configuration files in standard locations
func findConfigFile() string {
	// Look for config files in current directory and parent directories
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	configFiles := []string{
		".slidelang.yaml",
		".slidelang.yml",
		"slidelang.yaml",
		"slidelang.yml",
	}

	for {
		for _, configFile := range configFiles {
			configPath := filepath.Join(dir, configFile)
			if _, err := os.Stat(configPath); err == nil {
				return configPath
			}
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached root directory
		}
		dir = parent
	}

	return ""
}

// applyDefaults applies default values to configuration
func applyDefaults(config *SlideLangConfig) *SlideLangConfig {
	defaults := GetDefaultConfig()

	// Theme defaults
	if config.Theme.Default == "" {
		config.Theme.Default = defaults.Theme.Default
	}
	if len(config.Theme.ExternalPaths) == 0 {
		config.Theme.ExternalPaths = defaults.Theme.ExternalPaths
	}
	if config.Theme.Validation == "" {
		config.Theme.Validation = defaults.Theme.Validation
	}

	// Build defaults
	if config.Build.OutputDir == "" {
		config.Build.OutputDir = defaults.Build.OutputDir
	}
	if config.Build.Format == "" {
		config.Build.Format = defaults.Build.Format
	}

	// Server defaults
	if config.Server.Port == 0 {
		config.Server.Port = defaults.Server.Port
	}
	if config.Server.Host == "" {
		config.Server.Host = defaults.Server.Host
	}

	return config
}

// validBuildFormats lists the output formats slidelang's build command
// accepts, shared between config-file validation (ValidateConfig) and CLI
// --format flag validation so the two don't drift independently. Unexported
// so an importer can't mutate this shared validation state at runtime; use
// IsValidBuildFormat.
var validBuildFormats = map[string]bool{"html": true, "pdf": true, "json": true, "pptx": true}

// IsValidBuildFormat indica si format (ya en minúsculas, sin espacios) es uno
// de los formatos de salida reconocidos por el build command.
func IsValidBuildFormat(format string) bool {
	return validBuildFormats[format]
}

// ValidateConfig validates the configuration
func ValidateConfig(config *SlideLangConfig) error {
	// Validate theme configuration
	if config.Theme.Validation != "" &&
		config.Theme.Validation != "strict" &&
		config.Theme.Validation != "normal" {
		return fmt.Errorf("invalid theme validation mode: %s (must be 'strict' or 'normal')", config.Theme.Validation)
	}

	// Validate build configuration. Acepta una lista separada por comas
	// (p. ej. "html,json") para builds que emiten múltiples formatos a la vez.
	// Case-insensitive, igual que parseFormats (build.go) para el --format CLI flag.
	if config.Build.Format != "" {
		for _, f := range strings.Split(config.Build.Format, ",") {
			f = strings.ToLower(strings.TrimSpace(f))
			if f == "" {
				// Empty entries (trailing/doubled commas) are tolerated here too,
				// matching parseFormats' behavior for the --format CLI flag.
				continue
			}
			if !IsValidBuildFormat(f) {
				return fmt.Errorf("invalid build format: %s (must be one of 'html', 'pdf', 'json', or a comma-separated combination)", f)
			}
		}
	}

	// Validate server configuration
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d (must be 1-65535)", config.Server.Port)
	}

	return nil
}

// GetThemeConfig returns theme configuration for a specific theme
func (c *SlideLangConfig) GetThemeConfig(themeName string) ThemeConfig {
	// Check if there's a specific config for this theme
	if themeConfig, exists := c.Themes[themeName]; exists {
		return themeConfig
	}

	// Return default theme config
	return c.Theme
}

// SetThemeConfig sets theme configuration for a specific theme
func (c *SlideLangConfig) SetThemeConfig(themeName string, config ThemeConfig) {
	if c.Themes == nil {
		c.Themes = make(map[string]ThemeConfig)
	}
	c.Themes[themeName] = config
}

// GetThemePaths returns all theme search paths
func (c *SlideLangConfig) GetThemePaths() []string {
	return c.Theme.ExternalPaths
}

// AddThemePath adds a theme search path
func (c *SlideLangConfig) AddThemePath(path string) {
	// Check if path already exists
	for _, existing := range c.Theme.ExternalPaths {
		if existing == path {
			return
		}
	}

	c.Theme.ExternalPaths = append(c.Theme.ExternalPaths, path)
}

// RemoveThemePath removes a theme search path
func (c *SlideLangConfig) RemoveThemePath(path string) {
	for i, existing := range c.Theme.ExternalPaths {
		if existing == path {
			c.Theme.ExternalPaths = append(c.Theme.ExternalPaths[:i], c.Theme.ExternalPaths[i+1:]...)
			break
		}
	}
}

// Example configuration file template
const ExampleConfig = `# SlideLang Configuration File
# This file configures default settings for SlideLang CLI

theme:
  # Default theme to use when none is specified
  default: "default"
  
  # Directories to search for external themes
  external_paths:
    - "./themes"
    - "~/.slidelang/themes"
  
  # Enable theme caching for faster loading
  cache: true
  
  # Theme validation mode: "strict" or "normal"
  validation: "normal"

# Build configuration
build:
  # Default output directory
  output_dir: "./output"
  
  # Default output format: "html" or "pdf"
  format: "html"
  
  # Minify generated CSS
  minify_css: false
  
  # Minify generated HTML
  minify_html: false
  
  # Include theme assets in output
  include_assets: true

# Development server configuration
server:
  # Server port
  port: 8080
  
  # Server host
  host: "localhost"
  
  # Automatically open browser
  auto_open: true
  
  # Enable live reload
  live_reload: true

# Per-theme configuration
themes:
  corporate:
    path: "./themes/corporate.json"
    variables:
      "--primary-color": "#0066cc"
      "--secondary-color": "#003d7a"
  
  minimal:
    variables:
      "--font-size-base": "1.1rem"
      "--line-height-base": "1.6"
`
