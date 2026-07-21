// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	if config == nil {
		t.Fatal("GetDefaultConfig() returned nil")
	}

	if config.Theme.Default != "default" {
		t.Errorf("Default theme = %s, want default", config.Theme.Default)
	}

	if config.Build.OutputDir != "./output" {
		t.Errorf("Default output dir = %s, want ./output", config.Build.OutputDir)
	}

	if config.Server.Port != 8080 {
		t.Errorf("Default port = %d, want 8080", config.Server.Port)
	}
}

func TestLoadConfig_Default(t *testing.T) {
	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config == nil {
		t.Fatal("LoadConfig() returned nil")
	}

	// Should return default config when no file is found
	defaultConfig := GetDefaultConfig()
	if config.Theme.Default != defaultConfig.Theme.Default {
		t.Errorf("Theme.Default = %s, want %s", config.Theme.Default, defaultConfig.Theme.Default)
	}
}

func TestLoadConfig_FromFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "slidelang.yaml")

	configContent := `theme:
  default: "custom"
  cache: false
build:
  output_dir: "./dist"
  format: "pdf"
server:
  port: 9000
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Theme.Default != "custom" {
		t.Errorf("Theme.Default = %s, want custom", config.Theme.Default)
	}

	if config.Build.OutputDir != "./dist" {
		t.Errorf("Build.OutputDir = %s, want ./dist", config.Build.OutputDir)
	}

	if config.Server.Port != 9000 {
		t.Errorf("Server.Port = %d, want 9000", config.Server.Port)
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "slidelang.yaml")

	config := GetDefaultConfig()
	config.Theme.Default = "test-theme"
	config.Server.Port = 9090

	if err := SaveConfig(config, configPath); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("SaveConfig() did not create file")
	}

	// Load it back and verify
	loadedConfig, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loadedConfig.Theme.Default != "test-theme" {
		t.Errorf("Loaded Theme.Default = %s, want test-theme", loadedConfig.Theme.Default)
	}

	if loadedConfig.Server.Port != 9090 {
		t.Errorf("Loaded Server.Port = %d, want 9090", loadedConfig.Server.Port)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *SlideLangConfig
		expectErr bool
	}{
		{
			name:      "Valid config",
			config:    GetDefaultConfig(),
			expectErr: false,
		},
		{
			name: "Invalid validation mode",
			config: &SlideLangConfig{
				Theme: ThemeConfig{
					Validation: "invalid",
				},
				Server: ServerConfig{Port: 8080},
			},
			expectErr: true,
		},
		{
			name: "Invalid build format",
			config: &SlideLangConfig{
				Build: BuildConfig{
					Format: "invalid",
				},
				Server: ServerConfig{Port: 8080},
			},
			expectErr: true,
		},
		{
			name: "Valid json format",
			config: &SlideLangConfig{
				Build: BuildConfig{
					Format: "json",
				},
				Server: ServerConfig{Port: 8080},
			},
			expectErr: false,
		},
		{
			name: "Valid comma-separated multi-format",
			config: &SlideLangConfig{
				Build: BuildConfig{
					Format: "html,json",
				},
				Server: ServerConfig{Port: 8080},
			},
			expectErr: false,
		},
		{
			name: "Invalid entry within comma-separated format list",
			config: &SlideLangConfig{
				Build: BuildConfig{
					Format: "html,bogus",
				},
				Server: ServerConfig{Port: 8080},
			},
			expectErr: true,
		},
		{
			// Empty entries from trailing/doubled commas are tolerated, matching
			// the CLI --format flag's parseFormats behavior.
			name: "Trailing comma in format list is tolerated",
			config: &SlideLangConfig{
				Build: BuildConfig{
					Format: "html,",
				},
				Server: ServerConfig{Port: 8080},
			},
			expectErr: false,
		},
		{
			name: "Invalid port (too low)",
			config: &SlideLangConfig{
				Server: ServerConfig{
					Port: 0,
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid port (too high)",
			config: &SlideLangConfig{
				Server: ServerConfig{
					Port: 70000,
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateConfig() error = %v, expectErr = %v", err, tt.expectErr)
			}
		})
	}
}

func TestGetThemeConfig(t *testing.T) {
	config := GetDefaultConfig()
	config.SetThemeConfig("custom", ThemeConfig{
		Default: "custom-theme",
		Path:    "./themes/custom.json",
	})

	// Test getting specific theme config
	themeConfig := config.GetThemeConfig("custom")
	if themeConfig.Default != "custom-theme" {
		t.Errorf("GetThemeConfig() = %s, want custom-theme", themeConfig.Default)
	}

	// Test getting non-existent theme (should return default)
	defaultTheme := config.GetThemeConfig("non-existent")
	if defaultTheme.Default != config.Theme.Default {
		t.Error("GetThemeConfig() for non-existent theme should return default theme config")
	}
}

func TestSetThemeConfig(t *testing.T) {
	config := GetDefaultConfig()

	themeConfig := ThemeConfig{
		Default: "test-theme",
		Path:    "./themes/test.json",
	}

	config.SetThemeConfig("test", themeConfig)

	if config.Themes == nil {
		t.Fatal("SetThemeConfig() did not initialize Themes map")
	}

	storedConfig, exists := config.Themes["test"]
	if !exists {
		t.Fatal("SetThemeConfig() did not store theme config")
	}

	if storedConfig.Default != "test-theme" {
		t.Errorf("Stored theme config Default = %s, want test-theme", storedConfig.Default)
	}
}

func TestGetThemePaths(t *testing.T) {
	config := GetDefaultConfig()

	paths := config.GetThemePaths()
	if len(paths) == 0 {
		t.Error("GetThemePaths() returned empty slice")
	}
}

func TestAddThemePath(t *testing.T) {
	config := GetDefaultConfig()
	initialCount := len(config.Theme.ExternalPaths)

	config.AddThemePath("./new/path")

	if len(config.Theme.ExternalPaths) != initialCount+1 {
		t.Errorf("AddThemePath() count = %d, want %d", len(config.Theme.ExternalPaths), initialCount+1)
	}

	// Adding the same path again should not duplicate
	config.AddThemePath("./new/path")
	if len(config.Theme.ExternalPaths) != initialCount+1 {
		t.Error("AddThemePath() should not add duplicate paths")
	}
}

func TestRemoveThemePath(t *testing.T) {
	config := GetDefaultConfig()
	testPath := "./test/path"

	config.AddThemePath(testPath)
	initialCount := len(config.Theme.ExternalPaths)

	config.RemoveThemePath(testPath)

	if len(config.Theme.ExternalPaths) != initialCount-1 {
		t.Errorf("RemoveThemePath() count = %d, want %d", len(config.Theme.ExternalPaths), initialCount-1)
	}

	// Verify path was actually removed
	for _, path := range config.Theme.ExternalPaths {
		if path == testPath {
			t.Error("RemoveThemePath() did not remove the path")
		}
	}
}

func TestApplyDefaults(t *testing.T) {
	config := &SlideLangConfig{
		Theme: ThemeConfig{
			Default: "custom",
		},
		Build: BuildConfig{
			OutputDir: "./custom-output",
		},
	}

	config = applyDefaults(config)

	// Custom values should be preserved
	if config.Theme.Default != "custom" {
		t.Errorf("applyDefaults() changed custom Theme.Default to %s", config.Theme.Default)
	}

	if config.Build.OutputDir != "./custom-output" {
		t.Errorf("applyDefaults() changed custom Build.OutputDir to %s", config.Build.OutputDir)
	}

	// Missing values should be filled with defaults
	if config.Server.Port != 8080 {
		t.Errorf("applyDefaults() Server.Port = %d, want 8080", config.Server.Port)
	}

	if config.Server.Host != "localhost" {
		t.Errorf("applyDefaults() Server.Host = %s, want localhost", config.Server.Host)
	}
}

func TestExampleConfig(t *testing.T) {
	if ExampleConfig == "" {
		t.Error("ExampleConfig should not be empty")
	}

	// Verify it contains key sections
	if !contains(ExampleConfig, "theme:") {
		t.Error("ExampleConfig should contain theme section")
	}

	if !contains(ExampleConfig, "build:") {
		t.Error("ExampleConfig should contain build section")
	}

	if !contains(ExampleConfig, "server:") {
		t.Error("ExampleConfig should contain server section")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != "" && substr != "" &&
		(len(s) >= len(substr)) && (s == substr || findInString(s, substr))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
