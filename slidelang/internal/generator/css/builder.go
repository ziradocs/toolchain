// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package css

import (
	"fmt"
	"strings"

	"go.ziradocs.com/slidelang/internal/generator/css/themes"
)

// CSSBuilder builds CSS from modular components
type CSSBuilder struct {
	Theme             string
	CustomCSS         string
	ResponsiveMode    bool
	ModularResponsive bool // Whether responsive CSS should be generated as separate file
	themeLoader       *themes.ThemeLoader
	fileLoader        *CSSFileLoader // CSS file loader for namespaced files
	// Modular configuration
	RequiredElements []string // Elements actually used in the presentation
	RequiredLayouts  []string // Layouts actually used in the presentation
	EnableNavigation bool     // Whether to include navigation CSS
	EnableUtilities  bool     // Whether to include utilities CSS (future use)
}

// NewCSSBuilder creates a new CSS builder with defaults
func NewCSSBuilder() *CSSBuilder {
	return &CSSBuilder{
		Theme:             "default",
		ResponsiveMode:    true,
		ModularResponsive: false, // Default to including responsive in main CSS
		themeLoader:       themes.NewThemeLoader(),
		fileLoader:        NewCSSFileLoader(),
		RequiredElements:  []string{"text"}, // Default to just text elements
		RequiredLayouts:   []string{},       // No layouts by default
		EnableNavigation:  true,             // Default enable navigation
		EnableUtilities:   true,             // Default enable utilities
	}
}

// WithTheme sets the theme to use
func (cb *CSSBuilder) WithTheme(theme string) *CSSBuilder {
	cb.Theme = theme
	return cb
}

// WithCustomCSS adds custom CSS
func (cb *CSSBuilder) WithCustomCSS(css string) *CSSBuilder {
	cb.CustomCSS = css
	return cb
}

// WithResponsive enables/disables responsive CSS
func (cb *CSSBuilder) WithResponsive(enabled bool) *CSSBuilder {
	cb.ResponsiveMode = enabled
	return cb
}

// WithModularResponsive enables/disables modular responsive CSS generation
func (cb *CSSBuilder) WithModularResponsive(enabled bool) *CSSBuilder {
	cb.ModularResponsive = enabled
	return cb
}

// WithRequiredElements sets the elements that are actually used in the presentation
func (cb *CSSBuilder) WithRequiredElements(elements []string) *CSSBuilder {
	cb.RequiredElements = elements
	return cb
}

// WithRequiredLayouts sets the layouts that are actually used in the presentation
func (cb *CSSBuilder) WithRequiredLayouts(layouts []string) *CSSBuilder {
	cb.RequiredLayouts = layouts
	return cb
}

// WithNavigation enables/disables navigation CSS
func (cb *CSSBuilder) WithNavigation(enabled bool) *CSSBuilder {
	cb.EnableNavigation = enabled
	return cb
}

// WithUtilities enables/disables utilities CSS
func (cb *CSSBuilder) WithUtilities(enabled bool) *CSSBuilder {
	cb.EnableUtilities = enabled
	return cb
}

// Build generates the complete CSS
func (cb *CSSBuilder) Build() string {
	var css strings.Builder

	// 1. Theme variables - Use ThemeLoader to support external themes
	var theme themes.Theme
	// cb.Theme never comes from untrusted document content in this codebase
	// (only the literal "default" or an operator-set value via the
	// create_theme scaffolding command), so it's trusted.
	loadedTheme, err := cb.themeLoader.LoadTheme(cb.Theme, true)
	if err != nil {
		// Fallback to hardcoded themes if external loading fails
		theme = themes.GetTheme(cb.Theme)
	} else {
		theme = *loadedTheme
	}

	css.WriteString("/* === THEME VARIABLES === */\n")
	css.WriteString(themes.GenerateThemeCSS(theme))
	css.WriteString("\n")

	// 1.5. External theme custom CSS (if available)
	if theme.IsExternal && err == nil {
		// Get the original external theme to access custom CSS
		if externalTheme, found := cb.themeLoader.GetExternalTheme(cb.Theme); found {
			if mainCSS, exists := externalTheme.Styles["main"]; exists && mainCSS != "" {
				css.WriteString("/* === EXTERNAL THEME CSS === */\n")
				css.WriteString(mainCSS)
				css.WriteString("\n")
			}
		}
	}

	// 2. Base CSS
	css.WriteString("/* === BASE STYLES === */\n")
	baseCSS, _ := cb.fileLoader.LoadBaseCSSWithoutReset()
	css.WriteString(baseCSS)
	css.WriteString("\n")

	// 3. Element modules - ONLY REQUIRED ELEMENTS
	if len(cb.RequiredElements) > 0 {
		css.WriteString("/* === ELEMENT MODULES === */\n")
		elementCSS, err := cb.fileLoader.LoadElementCSS(cb.RequiredElements)
		if err != nil {
			css.WriteString(fmt.Sprintf("/* ERROR loading element CSS: %v */\n", err))
		}
		css.WriteString(elementCSS)
		css.WriteString("\n")
	}

	// 4. Layout modules - ONLY REQUIRED LAYOUTS
	if len(cb.RequiredLayouts) > 0 {
		css.WriteString("/* === LAYOUT MODULES === */\n")
		layoutCSS, _ := cb.fileLoader.LoadLayoutCSS(cb.RequiredLayouts)
		css.WriteString(layoutCSS)
		css.WriteString("\n")
	}

	// 5. Theme-specific CSS
	if themeCSS, err := cb.fileLoader.LoadThemeCSS(cb.Theme); err == nil {
		css.WriteString("/* === THEME SPECIFIC === */\n")
		css.WriteString(themeCSS)
		css.WriteString("\n")
	}

	// 5.5. Navigation module CSS. EnableNavigation ya existía como campo pero
	// Build() nunca lo consultaba: el HTML embebido (--embed-assets, y ahora
	// --format pdf, issue #128) nunca incluía navigation.css en absoluto —
	// solo se escribía como archivo separado en modo no-embebido
	// (NavigationModuleGenerator.GenerateAssets). El contador/ayuda de
	// navegación y el menú flotante quedaban sin ningún CSS (ni
	// position:fixed ni las reglas de @media print que los ocultan),
	// visibles como texto plano suelto en cualquier salida embebida.
	if cb.EnableNavigation {
		if navCSS, err := cb.fileLoader.LoadModuleCSS([]string{"navigation"}); err == nil {
			css.WriteString("/* === NAVIGATION MODULE === */\n")
			css.WriteString(navCSS)
			css.WriteString("\n")
		}
	}

	// 6. Responsive CSS (only if not generating modular responsive files)
	if cb.ResponsiveMode && !cb.ModularResponsive {
		css.WriteString("/* === RESPONSIVE STYLES === */\n")
		css.WriteString(cb.getResponsiveCSS())
		css.WriteString("\n")
	}

	// 7. Custom CSS
	if cb.CustomCSS != "" {
		css.WriteString("/* === CUSTOM STYLES === */\n")
		css.WriteString(cb.CustomCSS)
		css.WriteString("\n")
	}

	return css.String()
}

// getResponsiveCSS returns responsive media queries from modular file
func (cb *CSSBuilder) getResponsiveCSS() string {
	// Try to load responsive CSS from modular file first
	if responsiveCSS, err := cb.fileLoader.LoadResponsiveCSS(); err == nil {
		return responsiveCSS
	}

	// Fallback to namespaced responsive CSS
	return cb.getNamespacedResponsiveCSS()
}

// getNamespacedResponsiveCSS returns responsive CSS with slidelang- namespacing
func (cb *CSSBuilder) getNamespacedResponsiveCSS() string {
	css := `/* Responsive breakpoints */
@media (max-width: 768px) {
    .slidelang-slide {
        width: 95vw;
        height: 85vh;
        padding: 30px;
    }
    
    .slidelang-slide.slidelang-title-slide h1 {
        font-size: 2.5rem;
    }
    
    .slidelang-slide.slidelang-content-slide h1 {
        font-size: 2rem;
    }
    
    .slidelang-element.slidelang-text {
        font-size: 1.1rem;
    }
    
    .slidelang-element.slidelang-points li {
        font-size: 1rem;
        padding-left: 1.5rem;
    }
    
    .slidelang-element.slidelang-table {
        font-size: 0.9rem;
    }
    
    .slidelang-element.slidelang-table th,
    .slidelang-element.slidelang-table td {
        padding: 0.5rem;
    }
}

@media (max-width: 480px) {
    .slidelang-slide {
        width: 98vw;
        height: 90vh;
        padding: 20px;
    }
    
    .slidelang-slide.slidelang-title-slide h1 {
        font-size: 2rem;
    }
    
    .slidelang-slide.slidelang-title-slide h2 {
        font-size: 1.3rem;
    }
    
    .slidelang-slide.slidelang-content-slide h1 {
        font-size: 1.5rem;
    }
    
    .slidelang-element.slidelang-text {
        font-size: 1rem;
    }
    
    .slidelang-cards-grid {
        grid-template-columns: 1fr;
    }
    
    .slidelang-element.slidelang-image-gallery {
        grid-template-columns: 1fr;
    }
}

@media print {
    /* El container es flex en pantalla (una fila, un solo slide visible a la
       vez); un flex container no fragmenta de forma confiable entre sus
       items al imprimir (soporte de page-break-after/break-after dentro de
       flexbox es inconsistente entre motores) - sin este reset, los N
       slides (ya todos display:block por la regla de abajo) se compactan
       como columnas de una sola fila en vez de apilarse uno por página
       (issue #128). */
    .slidelang-presentation-container {
        display: block !important;
    }

    .slidelang-slide {
        width: auto;
        height: auto;
        box-shadow: none;
        page-break-after: always;
        display: block !important;
    }

    /* Navigation print rules are handled in navigation.css */
}
`
	return css
}

// GetAvailableModules returns list of available CSS element modules
func GetAvailableModules() []string {
	return []string{
		"text",            // Text elements and typography (always recommended)
		"code",            // Code blocks and syntax highlighting
		"images",          // Images and galleries
		"tables",          // Tables and data grids
		"blocks",          // Special blocks and cards
		"quotes",          // Quote elements
		"checklists",      // Checklist elements
		"grids",           // Grid layouts and columns
		"maps",            // Map elements (Leaflet)
		"headers_footers", // Headers and footers (specialized use)
	}
}

// GetAvailableLayouts returns list of available layout modules
func GetAvailableLayouts() []string {
	return []string{
		"specialized",  // Specialized presentation layouts
		"infographics", // Infographic and data visualization layouts
	}
}

// GetCoreModules returns modules that are always recommended for basic presentations
func GetCoreModules() []string {
	return []string{"text"}
}
