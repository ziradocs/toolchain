// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/a11y"
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

// sanitizeThemeFont recorre el stack CSS y devuelve la primera familia que no
// sea genérica: sin comillas, de letras ASCII, dígitos, espacio y guion (1 a
// 64 caracteres). Las familias genéricas de CSS (sans-serif, system-ui, ...)
// no son una fuente instalable en Word y se saltan. Si una familia con nombre
// no cumple el formato (por ejemplo trae símbolos o letras fuera de ASCII), se
// descarta todo el stack: no se sigue buscando después de un valor sospechoso.
func sanitizeThemeFont(value string) (string, bool) {
	for _, family := range splitCSSFamilies(value) {
		family = strings.TrimSpace(family)
		if len(family) >= 2 && (family[0] == '\'' || family[0] == '"') && family[len(family)-1] == family[0] {
			family = strings.TrimSpace(family[1 : len(family)-1])
		}
		if cssGenericFamily[strings.ToLower(family)] {
			continue
		}
		if !themeFontNameRe.MatchString(family) {
			return "", false
		}
		return family, true
	}
	return "", false
}

// splitCSSFamilies parte un stack CSS en sus familias, respetando las comillas.
func splitCSSFamilies(stack string) []string {
	var (
		out   []string
		quote rune
		start int
	)
	for i, r := range stack {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ',':
			out = append(out, stack[start:i])
			start = i + 1
		}
	}
	return append(out, stack[start:])
}

// Contraste mínimo (WCAG) de cada color de texto del DOCX. El papel es blanco:
// el DOCX no pinta el fondo de página (Word no lo imprime por defecto).
const (
	docxPaperColor    = "#FFFFFF"
	docxMinBodyRatio  = 4.5 // texto, enlaces, código inline, y texto sobre un fondo propio
	docxMinLargeRatio = 3.0 // texto claro y títulos
)

func contrastAtLeast(fg, bg string, min float64) bool {
	ratio, ok := a11y.ContrastRatio("#"+fg, bg)
	return ok && ratio >= min
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

	// Texto sobre papel blanco: un color sin contraste suficiente se descarta
	// y queda el de professional. Un tema oscuro (texto claro, fondo oscuro)
	// no se puede pintar en un papel que el DOCX no oscurece.
	withContrast := func(dst *string, min float64, name string) {
		prev := *dst
		color(dst, name)
		if !contrastAtLeast(*dst, docxPaperColor, min) {
			*dst = prev
		}
	}
	withContrast(&style.H1Color, docxMinLargeRatio, "--doclang-h1-color")
	withContrast(&style.H2Color, docxMinLargeRatio, "--doclang-h2-color")
	withContrast(&style.H3Color, docxMinLargeRatio, "--doclang-h3-color")
	withContrast(&style.H4Color, docxMinLargeRatio, "--doclang-h4-color")
	withContrast(&style.TextColor, docxMinBodyRatio, "--doclang-text-color")
	withContrast(&style.TextLightColor, docxMinLargeRatio, "--doclang-text-light")
	withContrast(&style.LinkColor, docxMinBodyRatio, "--doclang-link-color")
	withContrast(&style.CodeInlineColor, docxMinBodyRatio, "--doclang-code-inline-color")

	// Pares fondo/texto: se toman los dos del tema o los dos de professional.
	// El texto se mide contra el fondo ya resuelto.
	pair := func(bgDst, fgDst *string, bgVar, fgVar string) {
		bg, okBg := sanitizeThemeColor(vars[bgVar])
		fg, okFg := sanitizeThemeColor(vars[fgVar])
		if okBg && okFg && contrastAtLeast(fg, "#"+bg, docxMinBodyRatio) {
			*bgDst, *fgDst = bg, fg
		}
	}
	pair(&style.CodeBlockBg, &style.CodeBlockColor, "--doclang-code-bg", "--doclang-code-color")
	pair(&style.TableHeaderBg, &style.TableHeaderColor, "--doclang-table-header-bg", "--doclang-table-header-color")

	// Tablas: borde y filas alternas. La fila alterna debe dejar legible el
	// texto del cuerpo.
	color(&style.TableBorderColor, "--doclang-table-border")
	if stripe, ok := sanitizeThemeColor(vars["--doclang-table-stripe-bg"]); ok && contrastAtLeast(style.TextColor, "#"+stripe, docxMinBodyRatio) {
		style.TableStripeBg = stripe
	}

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
	if usesThemeVariables(opts) {
		return styleFromThemeVariables(opts.ThemeVariables, logger)
	}
	return GetStyleForTheme(themeName, logger)
}

// usesThemeVariables dice si el DOCX se estiliza desde las variables del tema.
func usesThemeVariables(opts GeneratorOptions) bool {
	return opts.ThemeExternal && len(opts.ThemeVariables) > 0
}

// themeBlockColor es el color del título de un bloque especial: el del estilo
// para ese tipo (un tipo desconocido se trata como info).
func (g *DOCXGenerator) themeBlockColor(blockType string) string {
	switch blockType {
	case "warning":
		return g.style.WarningColor
	case "danger":
		return g.style.DangerColor
	case "success":
		return g.style.SuccessColor
	case "tip":
		return g.style.TipColor
	default:
		return g.style.InfoColor
	}
}
