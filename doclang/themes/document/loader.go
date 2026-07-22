// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.ziradocs.com/core/v2/util"
)

// ThemeLoader maneja la carga de temas embebidos y externos
type ThemeLoader struct {
	searchPaths []string
}

// NewThemeLoader crea un nuevo loader con las rutas de búsqueda por defecto
func NewThemeLoader() *ThemeLoader {
	homeDir, _ := os.UserHomeDir()
	return &ThemeLoader{
		searchPaths: []string{
			filepath.Join(homeDir, ".doclang", "themes"),
			"/usr/local/share/doclang/themes",
			"./themes", // Ruta local relativa al proyecto
		},
	}
}

// NewThemeLoaderWithPaths crea un loader con rutas personalizadas
func NewThemeLoaderWithPaths(paths []string) *ThemeLoader {
	return &ThemeLoader{
		searchPaths: paths,
	}
}

// LoadTheme carga un tema por nombre (embebido o externo).
// Prioridad: 1. Embebido, 2. Externo, 3. Fallback a professional.
//
// trusted indica el origen de name: true si viene del flag --theme del
// operador (confiable, igual que su propio acceso de shell); false si viene
// del frontmatter del documento — contenido que, bajo el threat model de
// este repo, controla el atacante. Un name no confiable debe ser un token
// opaco (sin "/", "\" ni "..", no absoluto); de lo contrario se rechaza
// antes de tocar el filesystem, en vez de dejar que filepath.Join+Clean
// colapse un ".." fuera de los searchPaths. Ver
// docs/SECURITY_AUDIT_2026-07.md, ME-2.
func (l *ThemeLoader) LoadTheme(name string, trusted bool) (Theme, error) {
	// 1. Intentar cargar tema embebido
	if theme, exists := EmbeddedThemes[name]; exists {
		return theme, nil
	}

	if !trusted && !util.IsOpaquePathToken(name) {
		return GetProfessionalTheme(), fmt.Errorf("invalid theme name %q: must not contain path separators or '..', using professional as fallback", name)
	}

	// 2. Intentar cargar tema externo
	for _, searchPath := range l.searchPaths {
		themePath := filepath.Join(searchPath, name+".json")
		if theme, err := l.loadExternalTheme(themePath); err == nil {
			return theme, nil
		}
	}

	// 3. Fallback a professional
	return GetProfessionalTheme(), fmt.Errorf("theme '%s' not found, using professional as fallback", name)
}

// loadExternalTheme carga un tema desde archivo JSON
func (l *ThemeLoader) loadExternalTheme(path string) (Theme, error) {
	// Verificar que el archivo existe
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Theme{}, err
	}

	// Leer archivo
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, fmt.Errorf("failed to read theme file: %w", err)
	}

	// Parse JSON
	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return Theme{}, fmt.Errorf("failed to parse theme JSON: %w", err)
	}

	// Marcar como externo
	theme.IsExternal = true

	// Validar tema
	if err := l.validateTheme(theme); err != nil {
		return Theme{}, fmt.Errorf("theme validation failed: %w", err)
	}

	return theme, nil
}

// validateTheme valida que un tema tenga todas las variables requeridas
func (l *ThemeLoader) validateTheme(theme Theme) error {
	missing := []string{}

	for _, required := range RequiredVariables {
		if _, exists := theme.Variables[required]; !exists {
			missing = append(missing, required)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

// GetSearchPaths retorna las rutas de búsqueda actuales
func (l *ThemeLoader) GetSearchPaths() []string {
	return l.searchPaths
}

// AddSearchPath añade una ruta de búsqueda
func (l *ThemeLoader) AddSearchPath(path string) {
	l.searchPaths = append(l.searchPaths, path)
}

// ListAvailableThemes lista todos los temas disponibles (embebidos + externos)
func (l *ThemeLoader) ListAvailableThemes() []Theme {
	themes := []Theme{}

	// Añadir temas embebidos
	for name := range EmbeddedThemes {
		themes = append(themes, GetTheme(name))
	}

	// Buscar temas externos en todas las rutas
	for _, searchPath := range l.searchPaths {
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}

		files, err := os.ReadDir(searchPath)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			themePath := filepath.Join(searchPath, file.Name())
			if theme, err := l.loadExternalTheme(themePath); err == nil {
				themes = append(themes, theme)
			}
		}
	}

	return themes
}

// ExportThemeToJSON exporta un tema a formato JSON
func ExportThemeToJSON(theme Theme, outputPath string) error {
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal theme: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write theme file: %w", err)
	}

	return nil
}

// InstallTheme instala un tema externo en la carpeta del usuario
func InstallTheme(sourcePath string) error {
	// Crear directorio de themes si no existe
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	themesDir := filepath.Join(homeDir, ".doclang", "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		return fmt.Errorf("failed to create themes directory: %w", err)
	}

	// Validar que el tema sea válido
	loader := NewThemeLoader()
	theme, err := loader.loadExternalTheme(sourcePath)
	if err != nil {
		return fmt.Errorf("invalid theme file: %w", err)
	}

	// Copiar archivo
	destPath := filepath.Join(themesDir, filepath.Base(sourcePath))
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write theme file: %w", err)
	}

	fmt.Printf("Theme '%s' installed successfully to: %s\n", theme.Name, destPath)
	return nil
}

// UninstallTheme desinstala un tema externo
func UninstallTheme(name string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	themePath := filepath.Join(homeDir, ".doclang", "themes", name+".json")

	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		return fmt.Errorf("theme '%s' is not installed", name)
	}

	if err := os.Remove(themePath); err != nil {
		return fmt.Errorf("failed to remove theme file: %w", err)
	}

	fmt.Printf("Theme '%s' uninstalled successfully\n", name)
	return nil
}
