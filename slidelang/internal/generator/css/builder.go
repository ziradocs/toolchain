// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package css

import (
	"fmt"
	"strings"

	"go.ziradocs.com/slidelang/v2/internal/generator/css/themes"
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
	IncludeReset     bool     // Whether to prepend reset.css (issue #164)
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
		IncludeReset:      true,             // Default enable reset
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

// WithReset enables/disables prepending reset.css (issue #164)
func (cb *CSSBuilder) WithReset(enabled bool) *CSSBuilder {
	cb.IncludeReset = enabled
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

	// 1.6. Reset CSS. Issue #164: Build() only ever loaded
	// LoadBaseCSSWithoutReset() below, so every EmbedAssets=true build
	// (--format pdf, --embed-assets, RenderHTMLPreview, wasm) shipped with
	// no `body { font-family }`, no `* { box-sizing: border-box }`, and no
	// UA-default margin reset — Chrome fell back to Times for the whole
	// deck. Gated behind IncludeReset because the non-embedded path
	// (BuildCSS(), template/base.go) writes reset.css as its own linked
	// file already; including it here too would just duplicate identical
	// bytes into presentation.css, same reasoning as EnableNavigation
	// below.
	if cb.IncludeReset {
		if resetCSS, err := cb.fileLoader.LoadResetCSS(); err == nil {
			css.WriteString("/* === RESET STYLES === */\n")
			css.WriteString(resetCSS)
			css.WriteString("\n")
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
			fmt.Fprintf(&css, "/* ERROR loading element CSS: %v */\n", err)
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

// getResponsiveCSS returns responsive media queries from the modular file
// (assets/css/modules/responsive.css, loaded via go:embed). There used to
// be a hand-copied fallback literal here for when the load failed — but
// moduleCSSFiles is a //go:embed of that exact path, so the read cannot
// fail at runtime for any binary that compiled at all, and the literal had
// already drifted out of sync with the real file (it still had the #163
// pagination bug fixed in the file this reads). A duplicate that can only
// go stale is worse than no duplicate — issue #163's exploration found two
// more of these upstream, one already diverged. If this load ever
// legitimately fails, an empty responsive block is the honest result, not
// a second copy nobody will remember to keep in sync.
func (cb *CSSBuilder) getResponsiveCSS() string {
	responsiveCSS, err := cb.fileLoader.LoadResponsiveCSS()
	if err != nil {
		return ""
	}
	return responsiveCSS
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
		"media",           // Audio/video elements (issue #21)
		"headers_footers", // Headers and footers (specialized use)
		"plantuml",        // PlantUML diagrams (issue #38)
		"math",            // Math equations (issue #38)
		"charts",          // Chart images (issue #166)
		"mermaid",         // Mermaid diagrams (issue #173)
		"watermark",       // Repeating overlay from frontmatter (issue #179)
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
