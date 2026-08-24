// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package css

import (
	"embed"
	"fmt"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/generator/css/themes"
)

//go:embed assets/css/base/*.css
var baseCSSFiles embed.FS

//go:embed assets/css/elements/*.css
var elementCSSFiles embed.FS

//go:embed assets/css/layouts/*.css
var layoutCSSFiles embed.FS

//go:embed assets/css/themes/*.css
var themeCSSFiles embed.FS

//go:embed assets/css/modules/*.css
var moduleCSSFiles embed.FS

// CSSFileLoader handles loading and processing CSS files with namespacing
type CSSFileLoader struct {
	Prefix         string
	ExcludeClasses []string
}

// NewCSSFileLoader creates a new CSS file loader with namespacing.
// ExcludeClasses now holds only a small local set of pseudo-selector names
// (defensive — classRegex requires a literal "." before a name, so it can
// never actually match a bare ":hover"/"::before" pseudo-class/element;
// this only guards a class LITERALLY named e.g. "hover"). Classes the
// engine's own generated HTML legitimately emits without the slidelang-
// prefix are no longer copied into this list — ApplyNamespacing checks
// themes.IsKnownUnprefixedClass directly (same function
// UnprefixedClassSelectors, the strict validator, uses) so a name-based
// entry and a prefix-based entry (e.g. "language-") are honored identically
// by both instead of ExcludeClasses's plain equality check silently
// missing the prefix half (fourth-round finding on PR #223: .language-go
// passed --strict but ApplyNamespacing still rewrote it to
// .slidelang-language-go). This list used to also carry "html", "body",
// "*", "h1"-"h6", "p", "a", "img", "ul", "ol", "li", "table", "tr", "td",
// "th", "button", "input", "textarea", "select", "form" — removed
// (code-review finding on PR #223): since classRegex can never match a
// bare tag selector in the first place, the only real effect of those
// entries was silently blocking a genuine CLASS that happens to share a
// tag's name, confirmed for ".table" (the engine's own slidelang-table
// wrapper class, css/assets/css/elements/tables.css) — every table rule
// in startup-tech was left unable to match anything.
func NewCSSFileLoader() *CSSFileLoader {
	return &CSSFileLoader{
		Prefix:         "slidelang-",
		ExcludeClasses: []string{"root", "before", "after", "hover", "focus", "visited"},
	}
}

// LoadBaseCSS loads all base CSS files with namespacing applied
func (loader *CSSFileLoader) LoadBaseCSS() (string, error) {
	var css strings.Builder

	// Load reset.css
	if resetCSS, err := loader.LoadCSSWithVariableNamespacing("assets/css/base/reset.css", baseCSSFiles); err == nil {
		css.WriteString("/* === BASE RESET === */\n")
		css.WriteString(resetCSS)
		css.WriteString("\n\n")
	}

	// Load slides.css
	if slidesCSS, err := loader.LoadCSSWithVariableNamespacing("assets/css/base/slides.css", baseCSSFiles); err == nil {
		css.WriteString("/* === BASE SLIDES === */\n")
		css.WriteString(slidesCSS)
		css.WriteString("\n\n")
	}

	// Load layout.css
	if layoutCSS, err := loader.LoadCSSWithVariableNamespacing("assets/css/base/layout.css", baseCSSFiles); err == nil {
		css.WriteString("/* === BASE LAYOUT === */\n")
		css.WriteString(layoutCSS)
		css.WriteString("\n\n")
	}

	return css.String(), nil
}

// LoadResetCSS loads only the reset CSS file with namespacing applied
func (loader *CSSFileLoader) LoadResetCSS() (string, error) {
	if resetCSS, err := loader.LoadCSSWithVariableNamespacing("assets/css/base/reset.css", baseCSSFiles); err == nil {
		var css strings.Builder
		css.WriteString("/* === RESET STYLES === */\n")
		css.WriteString(resetCSS)
		css.WriteString("\n")
		return css.String(), nil
	} else {
		return "", err
	}
}

// LoadBaseCSSWithoutReset loads base CSS files excluding reset
func (loader *CSSFileLoader) LoadBaseCSSWithoutReset() (string, error) {
	var css strings.Builder

	// Load slides.css
	if slidesCSS, err := loader.LoadCSSWithVariableNamespacing("assets/css/base/slides.css", baseCSSFiles); err == nil {
		css.WriteString("/* === BASE SLIDES === */\n")
		css.WriteString(slidesCSS)
		css.WriteString("\n\n")
	}

	// Load layout.css
	if layoutCSS, err := loader.LoadCSSWithVariableNamespacing("assets/css/base/layout.css", baseCSSFiles); err == nil {
		css.WriteString("/* === BASE LAYOUT === */\n")
		css.WriteString(layoutCSS)
		css.WriteString("\n\n")
	}

	return css.String(), nil
}

// LoadElementCSS loads specific element CSS files with namespacing applied
func (loader *CSSFileLoader) LoadElementCSS(elements []string) (string, error) {
	var css strings.Builder

	for _, element := range elements {
		filename := fmt.Sprintf("assets/css/elements/%s.css", element)
		if elementCSS, err := loader.LoadCSSWithVariableNamespacing(filename, elementCSSFiles); err == nil {
			fmt.Fprintf(&css, "/* === ELEMENT: %s === */\n", strings.ToUpper(element))
			css.WriteString(elementCSS)
			css.WriteString("\n\n")
		} else {
			// Un módulo requerido sin archivo CSS deja ese tipo de elemento sin
			// estilos en el HTML generado, sin que el build falle ni avise —
			// visibilizar el gap en vez de tragarlo en silencio.
			util.Warn("CSS: módulo de elemento '%s' requerido pero sin archivo (%s): %v", element, filename, err)
		}
	}

	return css.String(), nil
}

// LoadLayoutCSS loads specific layout CSS files with namespacing applied
func (loader *CSSFileLoader) LoadLayoutCSS(layouts []string) (string, error) {
	var css strings.Builder

	for _, layout := range layouts {
		filename := fmt.Sprintf("assets/css/layouts/%s.css", layout)
		if layoutCSS, err := loader.LoadCSSWithVariableNamespacing(filename, layoutCSSFiles); err == nil {
			fmt.Fprintf(&css, "/* === LAYOUT: %s === */\n", strings.ToUpper(layout))
			css.WriteString(layoutCSS)
			css.WriteString("\n\n")
		}
	}

	return css.String(), nil
}

// LoadThemeCSS loads theme-specific CSS with namespacing applied
func (loader *CSSFileLoader) LoadThemeCSS(theme string) (string, error) {
	filename := fmt.Sprintf("assets/css/themes/%s.css", theme)
	if themeCSS, err := loader.LoadCSSWithVariableNamespacing(filename, themeCSSFiles); err == nil {
		return themeCSS, nil
	}
	return "", fmt.Errorf("theme CSS file not found: %s", theme)
}

// LoadModuleCSS loads specific module CSS files with namespacing applied
func (loader *CSSFileLoader) LoadModuleCSS(modules []string) (string, error) {
	var css strings.Builder

	for _, module := range modules {
		filename := fmt.Sprintf("assets/css/modules/%s.css", module)
		if moduleCSS, err := loader.LoadCSSWithVariableNamespacing(filename, moduleCSSFiles); err == nil {
			fmt.Fprintf(&css, "/* === MODULE: %s */\n", strings.ToUpper(module))
			css.WriteString(moduleCSS)
			css.WriteString("\n\n")
		}
	}

	return css.String(), nil
}

// LoadResponsiveCSS loads responsive CSS module with namespacing applied
func (loader *CSSFileLoader) LoadResponsiveCSS() (string, error) {
	filename := "assets/css/modules/responsive.css"
	if responsiveCSS, err := loader.LoadCSSWithVariableNamespacing(filename, moduleCSSFiles); err == nil {
		return responsiveCSS, nil
	}
	return "", fmt.Errorf("responsive CSS file not found")
}

// ApplyNamespacing applies CSS namespacing to a CSS string
func (loader *CSSFileLoader) ApplyNamespacing(css string) string {
	// Regex para encontrar selectores CSS (.class-name)
	classRegex := regexp.MustCompile(`\.([a-zA-Z][\w-]*)`)

	// themes.RewriteOutsideProtectedSpans: sin esto, un url("./Brand.woff2")
	// o content: "..." con un "." seguido de letras se reescribiría como si
	// fuera un selector — mismo hallazgo de code-review que motivó
	// protectedSpanRe en namespace.go, acá con más riesgo porque esta
	// función SÍ reescribe (no solo detecta), así que el bug corrompería la
	// ruta de un asset en vez de solo dar un falso positivo de validación.
	return themes.RewriteOutsideProtectedSpans(css, func(segment string) string {
		return classRegex.ReplaceAllStringFunc(segment, func(match string) string {
			className := match[1:] // Remover el punto

			// Excluir clases que no deben tener prefijo
			for _, exclude := range loader.ExcludeClasses {
				if className == exclude {
					return match
				}
			}
			if themes.IsKnownUnprefixedClass(className) {
				return match
			}

			// Evitar double-prefixing si ya tiene el prefijo
			if strings.HasPrefix(className, loader.Prefix) {
				return match
			}

			return "." + loader.Prefix + className
		})
	})
}

// ValidateNamespacedCSS validates that CSS has proper namespacing
func (loader *CSSFileLoader) ValidateNamespacedCSS(css string) []ValidationError {
	errors := []ValidationError{}

	// Clases principales que deberían tener prefijo
	requiredPrefixed := []string{
		"slide", "element", "presentation-container", "title-slide-container",
		"progress-bar", "floating-menu", "nav-button",
	}

	for _, class := range requiredPrefixed {
		// Buscar la clase sin prefijo (que no debería existir)
		nonNamespacedPattern := fmt.Sprintf(`\.%s(?:[^a-zA-Z-]|$)`, regexp.QuoteMeta(class))
		if matched, _ := regexp.MatchString(nonNamespacedPattern, css); matched {
			errors = append(errors, ValidationError{
				Type:    "missing-namespace",
				Class:   class,
				Message: fmt.Sprintf("Class .%s should be .%s%s", class, loader.Prefix, class),
			})
		}
	}

	return errors
}

// ProcessCSSVariables namespaces both var(--x) usages and --x: declarations
// in the given CSS to their --slidelang-x form. Delegates to
// themes.NamespaceStylesheet — the canonical implementation, shared with
// CSSBuilder.Build()'s external-theme CSS path (§2.1) — instead of keeping
// a second copy of the same regex/paren-matching logic here.
func (loader *CSSFileLoader) ProcessCSSVariables(css string) string {
	return themes.NamespaceStylesheet(css)
}

// LoadCSSWithVariableNamespacing loads CSS content and processes variables for namespacing
func (loader *CSSFileLoader) LoadCSSWithVariableNamespacing(filePath string, fs embed.FS) (string, error) {
	content, err := fs.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Process variables to add namespacing
	processedContent := loader.ProcessCSSVariables(string(content))
	return processedContent, nil
}

// ValidationError represents a CSS validation error
type ValidationError struct {
	Type    string
	Class   string
	Message string
}

// Error implements the error interface
func (e ValidationError) Error() string {
	return e.Message
}
