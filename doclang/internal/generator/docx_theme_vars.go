// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/util"
)

// Estilos DOCX para temas externos.
//
// Los temas embebidos (professional, academic, technical, page-view) tienen un
// DOCXStyle cableado en docx_styles.go. Un tema externo (JSON cargado por
// themes/document/loader.go) solo trae variables CSS --doclang-*; aquí se
// traducen a un DOCXStyle partiendo de professional. Tamaños, espaciados y
// negritas no se derivan del tema: salen de professional.
//
// Los valores terminan interpolados en XML de Word, así que nada pasa sin
// sanearse: los colores solo se aceptan como #rgb o #rrggbb, y las fuentes
// como una familia de letras, dígitos, espacio y guion.

var (
	themeHexColorRe  = regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$`)
	themeFontNameRe  = regexp.MustCompile(`^[A-Za-z0-9 -]{1,64}$`)
	cssGenericFamily = map[string]bool{
		"serif": true, "sans-serif": true, "monospace": true, "cursive": true,
		"fantasy": true, "system-ui": true, "ui-serif": true, "ui-sans-serif": true,
		"ui-monospace": true, "ui-rounded": true, "emoji": true, "math": true,
		"fangsong": true, "inherit": true, "initial": true, "unset": true,
		"revert": true, "-apple-system": true, "blinkmacsystemfont": true,
	}
)

// sanitizeThemeColor convierte "#rgb" o "#rrggbb" a "RRGGBB" en mayúsculas.
// Cualquier otra cosa (rgb(), var(), nombres, gradientes, url(), #rrggbbaa)
// devuelve "", false.
func sanitizeThemeColor(value string) (string, bool) {
	m := themeHexColorRe.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil {
		return "", false
	}
	hex := strings.ToUpper(m[1])
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	return hex, true
}

// sanitizeThemeFont toma la primera familia de un stack CSS, le quita las
// comillas y la acepta solo si son letras, dígitos, espacio y guion (1 a 64
// caracteres). Las familias genéricas de CSS (sans-serif, system-ui, ...) no
// son una fuente instalable en Word, así que tampoco se aceptan.
func sanitizeThemeFont(value string) (string, bool) {
	first := firstCSSFamily(value)
	first = strings.TrimSpace(first)
	if len(first) >= 2 && (first[0] == '\'' || first[0] == '"') && first[len(first)-1] == first[0] {
		first = first[1 : len(first)-1]
	}
	first = strings.TrimSpace(first)
	if !themeFontNameRe.MatchString(first) || cssGenericFamily[strings.ToLower(first)] {
		return "", false
	}
	return first, true
}

// firstCSSFamily devuelve el texto hasta la primera coma que no esté entre comillas.
func firstCSSFamily(stack string) string {
	var quote rune
	for i, r := range stack {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ',':
			return stack[:i]
		}
	}
	return stack
}

// styleFromThemeVariables arma el DOCXStyle de un tema externo: parte de
// professional y reemplaza solo lo que el tema define con un valor válido.
func styleFromThemeVariables(vars map[string]string, logger util.Logger) DOCXStyle {
	style := GetStyleForTheme("professional", logger)

	color := func(dst *string, names ...string) {
		for _, name := range names {
			if v, ok := vars[name]; ok {
				if c, ok := sanitizeThemeColor(v); ok {
					*dst = c
					return
				}
			}
		}
	}
	font := func(dst *string, names ...string) {
		for _, name := range names {
			if v, ok := vars[name]; ok {
				if f, ok := sanitizeThemeFont(v); ok {
					*dst = f
					return
				}
			}
		}
	}

	// Tipografía. El HTML usa --doclang-code-font para el código;
	// --doclang-font-code es el nombre del contrato estándar.
	font(&style.FontFamily, "--doclang-font-main")
	font(&style.CodeFontFamily, "--doclang-code-font", "--doclang-font-code")

	// Encabezados y texto.
	color(&style.H1Color, "--doclang-h1-color")
	color(&style.H2Color, "--doclang-h2-color")
	color(&style.H3Color, "--doclang-h3-color")
	color(&style.H4Color, "--doclang-h4-color")
	color(&style.TextColor, "--doclang-text-color")
	color(&style.TextLightColor, "--doclang-text-light")
	color(&style.LinkColor, "--doclang-link-color")

	// Código: bloque (pre) e inline.
	color(&style.CodeBlockBg, "--doclang-code-bg")
	color(&style.CodeBlockColor, "--doclang-code-color")
	color(&style.CodeInlineColor, "--doclang-code-inline-color")

	// Tablas.
	color(&style.TableHeaderBg, "--doclang-table-header-bg")
	color(&style.TableHeaderColor, "--doclang-table-header-color")
	color(&style.TableBorderColor, "--doclang-table-border")
	color(&style.TableStripeBg, "--doclang-table-stripe-bg")

	// Bloques especiales: info usa el color de texto del bloque, el resto el
	// color del borde, igual que el DOCX de los temas embebidos.
	color(&style.InfoColor, "--doclang-alert-info-color")
	color(&style.WarningColor, "--doclang-alert-warning-border")
	color(&style.DangerColor, "--doclang-alert-danger-border")
	color(&style.SuccessColor, "--doclang-alert-success-border")
	color(&style.TipColor, "--doclang-alert-tip-border")

	return style
}

// resolveDOCXStyle elige el estilo: variables del tema para un tema externo,
// el estilo cableado por nombre para los embebidos.
func resolveDOCXStyle(themeName string, opts GeneratorOptions, logger util.Logger) DOCXStyle {
	if opts.ThemeExternal && len(opts.ThemeVariables) > 0 {
		return styleFromThemeVariables(opts.ThemeVariables, logger)
	}
	return GetStyleForTheme(themeName, logger)
}
