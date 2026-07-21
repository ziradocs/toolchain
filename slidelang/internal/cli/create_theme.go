// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.ziradocs.com/slidelang/internal/generator/css"
	"go.ziradocs.com/core/util"
	"github.com/spf13/cobra"
)

// CreateThemeCmd creates a new theme command
func CreateThemeCmd() *cobra.Command {
	var (
		templateType      string
		interactive       bool
		outputPath        string
		author            string
		description       string
		fullCSS           bool
		includeNavigation bool
		modules           string
		layouts           string
		force             bool
	)

	cmd := &cobra.Command{
		Use:   "create <theme-name>",
		Short: "Create a new theme from template",
		Long: `Create a new SlideLang theme from a predefined template.

Available templates:
  - business: Professional business presentation theme
  - academic: Clean academic/research presentation theme  
  - creative: Modern creative/artistic theme
  - minimal: Ultra-minimal clean theme

CSS Export Options:
  --full-css: Export complete CSS system (not just theme styles)
  --include-navigation: Include navigation CSS in full export
  --modules: Specify element modules to include (comma-separated)
  --layouts: Specify layout modules to include (comma-separated)

Examples:
  slidelang themes create my-theme --template business
  slidelang themes create my-theme --full-css --include-navigation
  slidelang themes create my-theme --full-css --modules="text,code,images" --layouts="specialized"
  slidelang themes create custom-theme --template academic --interactive`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			themeName := args[0]

			// Hallazgo de security-review de PR #147: themeName alimenta
			// filepath.Join("themes", themeName) cuando no se pasa --output
			// (ver createTheme abajo) — sin validar, era exactamente el mismo
			// vector de traversal que este PR cierra en doclang init.go, solo
			// que en el archivo hermano. Mismo guard, mismo tratamiento que
			// ya usan los theme loaders (css/themes/loader.go,
			// doclang/themes/document/loader.go).
			if !util.IsOpaquePathToken(themeName) {
				return fmt.Errorf("invalid theme name %q: must not contain path separators, \"..\", or be an absolute path", themeName)
			}

			if interactive {
				return createThemeInteractive(themeName, outputPath)
			}

			return createTheme(themeName, templateType, outputPath, author, description, fullCSS, includeNavigation, modules, layouts, force)
		},
	}

	cmd.Flags().StringVarP(&templateType, "template", "t", "business", "Template type (business, academic, creative, minimal)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive theme creation mode")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output directory (default: ./themes/<theme-name>)")
	cmd.Flags().StringVarP(&author, "author", "a", "", "Theme author name")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Theme description")
	cmd.Flags().BoolVar(&fullCSS, "full-css", false, "Export complete CSS system (not just theme styles)")
	cmd.Flags().BoolVar(&includeNavigation, "include-navigation", true, "Include navigation CSS in full export")
	cmd.Flags().StringVar(&modules, "modules", "", "Element modules to include (comma-separated: text,code,images,tables,blocks,quotes,checklists,maps,headers_footers)")
	cmd.Flags().StringVar(&layouts, "layouts", "", "Layout modules to include (comma-separated: specialized,infographics)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite the output directory if it already exists and is not empty")

	return cmd
}

// ThemeTemplate represents a theme template
type ThemeTemplate struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Variables   map[string]string `json:"variables"`
	BaseCSS     string            `json:"base_css"`
	Assets      []string          `json:"assets,omitempty"`
}

// getThemeTemplates returns predefined theme templates
func getThemeTemplates() map[string]ThemeTemplate {
	return map[string]ThemeTemplate{
		"business": {
			Name:        "Business Professional",
			Description: "Professional business presentation theme with corporate styling",
			Variables: map[string]string{
				"--primary-color":    "#0066cc",
				"--secondary-color":  "#f8f9fa",
				"--accent-color":     "#17a2b8",
				"--text-color":       "#212529",
				"--background-color": "#ffffff",
				"--font-family":      "'Segoe UI', 'Arial', sans-serif",
				"--font-size-base":   "1rem",
				"--line-height-base": "1.6",
				"--border-radius":    "0.375rem",
				"--box-shadow":       "0 0.125rem 0.25rem rgba(0, 0, 0, 0.075)",
			},
			BaseCSS: businessThemeCSS,
		},
		"academic": {
			Name:        "Academic Research",
			Description: "Clean academic presentation theme optimized for research content",
			Variables: map[string]string{
				"--primary-color":    "#2c3e50",
				"--secondary-color":  "#ecf0f1",
				"--accent-color":     "#3498db",
				"--text-color":       "#2c3e50",
				"--background-color": "#ffffff",
				"--font-family":      "'Times New Roman', serif",
				"--font-size-base":   "1.1rem",
				"--line-height-base": "1.7",
				"--border-radius":    "0.25rem",
				"--box-shadow":       "0 0.0625rem 0.125rem rgba(0, 0, 0, 0.1)",
			},
			BaseCSS: academicThemeCSS,
		},
		"creative": {
			Name:        "Creative Modern",
			Description: "Modern creative theme with vibrant colors and artistic layouts",
			Variables: map[string]string{
				"--primary-color":    "#e74c3c",
				"--secondary-color":  "#f39c12",
				"--accent-color":     "#9b59b6",
				"--text-color":       "#2c3e50",
				"--background-color": "#ffffff",
				"--font-family":      "'Helvetica Neue', 'Arial', sans-serif",
				"--font-size-base":   "1rem",
				"--line-height-base": "1.5",
				"--border-radius":    "0.5rem",
				"--box-shadow":       "0 0.25rem 0.5rem rgba(0, 0, 0, 0.1)",
			},
			BaseCSS: creativeThemeCSS,
		},
		"minimal": {
			Name:        "Minimal Clean",
			Description: "Ultra-minimal theme with subtle accents and clean typography",
			Variables: map[string]string{
				"--primary-color":    "#333333",
				"--secondary-color":  "#f5f5f5",
				"--accent-color":     "#666666",
				"--text-color":       "#333333",
				"--background-color": "#ffffff",
				"--font-family":      "'Helvetica', 'Arial', sans-serif",
				"--font-size-base":   "1rem",
				"--line-height-base": "1.6",
				"--border-radius":    "0.125rem",
				"--box-shadow":       "none",
			},
			BaseCSS: minimalThemeCSS,
		},
	}
}

// createTheme creates a new theme from template
func createTheme(themeName, templateType, outputPath, author, description string, fullCSS, includeNavigation bool, modules, layouts string, force bool) error {
	templates := getThemeTemplates()

	template, exists := templates[templateType]
	if !exists {
		return fmt.Errorf("unknown template type: %s. Available: business, academic, creative, minimal", templateType)
	}

	// Determine output path
	if outputPath == "" {
		outputPath = filepath.Join("themes", themeName)
	}

	// Issue #47: manifestFile/styles.css/README.md se escriben más abajo con
	// os.Create/os.WriteFile, que truncan en silencio si ya existen — a
	// diferencia de los outputs de build (que sobrescriben por diseño en
	// cada rebuild), un comando de scaffolding no debería pisar un theme
	// existente sin que el usuario lo pida explícitamente.
	//
	// Lstat (no Stat) antes que nada, SIEMPRE (incluso con --force): si
	// outputPath ya existe como symlink, ReadDir/MkdirAll lo siguen de forma
	// transparente — un symlink plantado (p. ej. por un repo/working tree
	// no confiable) podía apuntar a otro directorio vacío en cualquier otro
	// lado, pasando el chequeo de "no vacío" y haciendo que las escrituras
	// de abajo aterricen fuera del árbol de themes/ pretendido, sin ningún
	// error (hallazgo de security-review de PR #147). --force está pensado
	// para "sí, pisa MI theme existente", no para seguir un symlink a otro
	// lado — por eso se rechaza en ambos casos.
	if info, err := os.Lstat(outputPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output path %q is a symlink, refusing to write through it", outputPath)
	}

	if !force {
		// Hallazgo de code-review de PR #147: la versión anterior trataba
		// CUALQUIER error de ReadDir como "seguro para proceder" (el chequeo
		// era `err == nil && len(entries) > 0`) — un directorio que sí existe
		// pero no se puede listar (permission-denied, algún error de I/O)
		// pasaba el guard en silencio en vez de fallar. Ahora se distingue:
		// no existe → nada que proteger; existe y no está vacío → rechazar;
		// existe pero falla por otra razón → superficie ese error real, no
		// asumir que es seguro sobrescribir.
		switch entries, err := os.ReadDir(outputPath); {
		case err == nil:
			if len(entries) > 0 {
				return fmt.Errorf("output directory %q already exists and is not empty; use --force to overwrite", outputPath)
			}
		case os.IsNotExist(err):
			// No existe todavía — nada que proteger, se crea abajo.
		default:
			return fmt.Errorf("failed to check output directory %q: %w", outputPath, err)
		}
	}
	// Limitación conocida y aceptada (code-review de PR #147, rated
	// PLAUSIBLE no CONFIRMED): hay una ventana TOCTOU entre este chequeo y
	// los os.Create/os.WriteFile de abajo — dos invocaciones concurrentes de
	// `themes create` sobre el mismo nombre podrían pisarse. Es un comando
	// de scaffolding interactivo, no una ruta con datos de terceros
	// concurrentes; agregar locking a nivel de archivo para este caso es
	// complejidad desproporcionada frente al riesgo real.

	// Create output directory
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Use provided values or defaults
	if author == "" {
		author = "Theme Author"
	}
	if description == "" {
		description = template.Description
	}

	// Create theme manifest
	manifest := map[string]interface{}{
		"name":        themeName,
		"description": description,
		"author":      author,
		"version":     "1.0.0",
		"variables":   template.Variables,
	}

	// Write manifest file
	manifestPath := filepath.Join(outputPath, "theme.json")
	manifestFile, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %v", err)
	}
	defer func() { _ = manifestFile.Close() }()

	encoder := json.NewEncoder(manifestFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}

	// Write CSS file(s)
	if fullCSS {
		// Generate complete CSS system with modular components
		if err := generateFullCSSExport(outputPath, template, includeNavigation, modules, layouts); err != nil {
			return fmt.Errorf("failed to generate full CSS: %v", err)
		}
	} else {
		// Write only theme styles (legacy behavior)
		cssPath := filepath.Join(outputPath, "styles.css")
		if err := os.WriteFile(cssPath, []byte(template.BaseCSS), 0644); err != nil {
			return fmt.Errorf("failed to write CSS file: %v", err)
		}
	}

	// Write README
	readmePath := filepath.Join(outputPath, "README.md")
	readme := generateThemeReadme(themeName, description, author, template)
	if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to write README: %v", err)
	}

	fmt.Printf("✅ Theme '%s' created successfully!\n", themeName)
	fmt.Printf("📁 Output: %s\n", outputPath)
	if !fullCSS {
		fmt.Printf("📄 Files created:\n")
		fmt.Printf("   - theme.json (manifest)\n")
		fmt.Printf("   - styles.css (theme styles)\n")
		fmt.Printf("   - README.md (documentation)\n")
	} else {
		fmt.Printf("📄 Full CSS system exported - see details above\n")
	}
	fmt.Printf("\n💡 Next steps:\n")
	fmt.Printf("   1. Customize variables in theme.json\n")
	if fullCSS {
		fmt.Printf("   2. Use presentation.css and navigation.css in your projects\n")
		fmt.Printf("   3. Customize theme styles in styles.css\n")
	} else {
		fmt.Printf("   2. Edit styles in styles.css\n")
		fmt.Printf("   3. Test: slidelang themes validate %s\n", outputPath)
		fmt.Printf("   4. Install: slidelang themes install %s\n", outputPath)
	}

	return nil
}

// createThemeInteractive creates a theme with interactive prompts
func createThemeInteractive(themeName, outputPath string) error {
	// TODO: Implement interactive mode with prompts
	fmt.Println("🔮 Interactive mode coming soon!")
	fmt.Println("For now, use: slidelang themes create <name> --template <type>")
	return nil
}

// generateThemeReadme generates a README.md for the new theme
func generateThemeReadme(name, description, author string, template ThemeTemplate) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s Theme\n\n", name))
	sb.WriteString(fmt.Sprintf("**Author:** %s  \n", author))
	sb.WriteString(fmt.Sprintf("**Description:** %s  \n\n", description))

	sb.WriteString("## Installation\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString(fmt.Sprintf("slidelang themes install %s\n", name))
	sb.WriteString("```\n\n")

	sb.WriteString("## Usage\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString(fmt.Sprintf("slidelang build presentation.slidelang --theme %s\n", name))
	sb.WriteString("```\n\n")

	sb.WriteString("## Customization\n\n")
	sb.WriteString("This theme supports the following CSS variables:\n\n")

	for variable, value := range template.Variables {
		sb.WriteString(fmt.Sprintf("- `%s`: %s\n", variable, value))
	}

	sb.WriteString("\n## Files\n\n")
	sb.WriteString("- `theme.json` - Theme manifest and variables\n")
	sb.WriteString("- `styles.css` - Custom CSS styles\n")
	sb.WriteString("- `README.md` - This documentation\n")

	return sb.String()
}

// generateFullCSSExport generates a complete CSS system export with all modular components
func generateFullCSSExport(outputPath string, template ThemeTemplate, includeNavigation bool, modules, layouts string) error {
	// Parse modules list
	var requiredElements []string
	if modules != "" {
		requiredElements = strings.Split(modules, ",")
		// Trim whitespace
		for i, element := range requiredElements {
			requiredElements[i] = strings.TrimSpace(element)
		}
	} else {
		// Use all available modules by default
		requiredElements = css.GetAvailableModules()
	}

	// Parse layouts list
	var requiredLayouts []string
	if layouts != "" {
		requiredLayouts = strings.Split(layouts, ",")
		// Trim whitespace
		for i, layout := range requiredLayouts {
			requiredLayouts[i] = strings.TrimSpace(layout)
		}
	} else {
		// Use all available layouts by default
		requiredLayouts = css.GetAvailableLayouts()
	}

	// Create CSS builder with modular configuration
	builder := css.NewCSSBuilder().
		WithTheme("custom"). // Use custom theme
		WithRequiredElements(requiredElements).
		WithRequiredLayouts(requiredLayouts).
		WithNavigation(includeNavigation).
		WithResponsive(true)

	// Generate main presentation CSS
	mainCSS := builder.Build()

	// Write main presentation CSS
	presentationPath := filepath.Join(outputPath, "presentation.css")
	if err := os.WriteFile(presentationPath, []byte(mainCSS), 0644); err != nil {
		return fmt.Errorf("failed to write presentation CSS: %v", err)
	}

	// Generate and write navigation CSS if enabled
	if includeNavigation {
		navCSS := css.GetNavigationCSS()
		navigationPath := filepath.Join(outputPath, "navigation.css")
		if err := os.WriteFile(navigationPath, []byte(navCSS), 0644); err != nil {
			return fmt.Errorf("failed to write navigation CSS: %v", err)
		}
	}

	// Write theme-specific styles.css (original theme CSS)
	stylesPath := filepath.Join(outputPath, "styles.css")
	themeCSS := fmt.Sprintf("/* %s Theme Styles */\n%s", template.Name, template.BaseCSS)
	if err := os.WriteFile(stylesPath, []byte(themeCSS), 0644); err != nil {
		return fmt.Errorf("failed to write theme styles: %v", err)
	}

	fmt.Printf("✅ Generated full CSS system with:\n")
	fmt.Printf("   📄 presentation.css - Complete modular CSS system\n")
	if includeNavigation {
		fmt.Printf("   🧭 navigation.css - Navigation controls\n")
	}
	fmt.Printf("   🎨 styles.css - Theme-specific styles\n")
	fmt.Printf("   📋 Elements: %s\n", strings.Join(requiredElements, ", "))
	if len(requiredLayouts) > 0 {
		fmt.Printf("   📐 Layouts: %s\n", strings.Join(requiredLayouts, ", "))
	}

	return nil
}
