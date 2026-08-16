// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package a11y provides pure, theme-agnostic accessibility math — hoy solo
// contraste WCAG 2.2 AA/AAA (issue #30). Deliberadamente no sabe qué es un
// "tema": core nunca resuelve un nombre de tema a colores (eso lo hace cada
// CLI por separado, con su propio mapa de variables CSS y su propia
// convención de nombres — slidelang usa "--text-color", doclang usa
// "--doclang-h1-color", y no comparten esquema). Por eso este paquete no
// intenta mapear "tipo de elemento" → "nombre de variable de color": esa
// asociación es responsabilidad de quien construye la regla (ver
// core/linter/contrast_rule.go), no de esta librería. Este paquete solo
// sabe convertir un string de color en RGB y calcular un ratio; nada más.
package a11y

import (
	"fmt"
	"math"
	"strings"

	"github.com/mazznoer/csscolorparser"
)

// Umbrales WCAG 2.2, Success Criteria 1.4.3 (AA) y 1.4.6 (AAA). "Large
// text" es texto de al menos 18pt, o 14pt en negrita, según la definición
// WCAG.
const (
	AANormalText  = 4.5
	AALargeText   = 3.0
	AAANormalText = 7.0
	AAALargeText  = 4.5
)

// ParseColor interpreta un color CSS: nombres ("white", "rebeccapurple"),
// hex (#rgb, #rgba, #rrggbb, #rrggbbaa) y funciones (rgb(), rgba(), hsl(),
// hsla(), hwb(), lab(), lch(), oklab(), oklch()) — delega en
// github.com/mazznoer/csscolorparser (CSS Color Module Level 4), que ya
// viaja en el binario de los dos CLIs como dependencia indirecta de
// core/renderer (vía go-staticmaps, para mapas), así que esto no agrega
// peso nuevo. Formas no soportadas devuelven ok=false en vez de un valor
// adivinado: gradientes (linear-gradient(...)) y var(...) sin resolver no
// son un color, y una variable de tema real puede legítimamente contener
// cualquiera de esos — un rulepack de contraste debe poder saltarse ese
// par en vez de recibir un ratio inventado.
//
// Dos guardas se conservan sobre lo que la librería aceptaría de por sí:
//
//   - Un canal alfa presente que NO sea totalmente opaco devuelve
//     ok=false, por la misma razón de siempre: el contraste solo tiene
//     sentido calculado sobre el color YA COMPUESTO contra su fondo real,
//     y este paquete no tiene ese fondo. Tratar cualquier alfa como si
//     fuera 100% opaco le daría a un color translúcido/transparente el
//     mismo trato que a un rgba() correctamente rechazado: dos formas del
//     mismo color obtendrían resultados opuestos.
//   - Hex sigue exigiendo el prefijo "#". csscolorparser.Parse acepta hex
//     pelón como fallback ("ffffff" sin "#"), que no es un color CSS
//     válido dentro de una hoja de estilos — se rechaza explícitamente
//     en vez de heredar esa tolerancia extra.
func ParseColor(s string) (r, g, b uint8, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, 0, false
	}
	if !strings.HasPrefix(s, "#") && !strings.Contains(s, "(") {
		if _, named := csscolorparser.NamedColors[strings.ToLower(s)]; !named {
			return 0, 0, 0, false
		}
	}

	c, err := csscolorparser.Parse(s)
	if err != nil || c.A != 1.0 {
		return 0, 0, 0, false
	}
	// oklab()/oklch()/lab()/lch() cubren un gamut más amplio que sRGB: un
	// canal fuera de [0,1] (p. ej. "oklch(0.7 0.4 30)" da R≈1.11) es
	// perfectamente válido en esos espacios de color, pero Color.RGBA255()
	// no hace ningún clipping (uint8(c.R*255+0.5) sobre un valor negativo o
	// >1 produce un canal uint8 con truncamiento indefinido, no el valor
	// visualmente más cercano dentro de sRGB) -- hallazgo de code-review
	// sobre PR #157. Clamp() (que la librería expone justo para esto)
	// recorta cada canal a [0,1] antes de la conversión.
	rv, gv, bv, _ := c.Clamp().RGBA255()
	return rv, gv, bv, true
}

// RelativeLuminance calcula la luminancia relativa de un color sRGB según
// la fórmula WCAG 2.x: cada canal se convierte a su valor lineal (con el
// corte gamma estándar en 0.03928) y se combina con los pesos de
// percepción humana (0.2126/0.7152/0.0722 — el ojo es mucho más sensible
// al verde que al azul).
func RelativeLuminance(r, g, b uint8) float64 {
	linearize := func(c uint8) float64 {
		cs := float64(c) / 255.0
		if cs <= 0.03928 {
			return cs / 12.92
		}
		return math.Pow((cs+0.055)/1.055, 2.4)
	}
	rl := linearize(r)
	gl := linearize(g)
	bl := linearize(b)
	return 0.2126*rl + 0.7152*gl + 0.0722*bl
}

// ContrastRatio calcula el ratio de contraste WCAG entre fg y bg (ambos
// colores CSS parseables, ver ParseColor). El ratio siempre es >= 1 (el
// mayor de los dos luminance sobre el menor, +0.05 en cada uno). ok=false
// si cualquiera de los dos colores no es parseable — sin decir cuál; para
// distinguirlo, ver ContrastRatioDetail.
func ContrastRatio(fg, bg string) (ratio float64, ok bool) {
	ratio, status := ContrastRatioDetail(fg, bg)
	return ratio, status == ContrastOK
}

// ContrastStatus distingue por qué ContrastRatioDetail no pudo (o sí pudo)
// calcular un ratio — ver su doc comment.
type ContrastStatus int

const (
	// ContrastOK indica que ambos colores parsearon y el ratio es válido.
	ContrastOK ContrastStatus = iota
	// ContrastFgUnparseable indica que fg no es un color CSS parseable
	// (ver ParseColor) — bg puede o no serlo, no importa: se reporta el
	// primer color que falla.
	ContrastFgUnparseable
	// ContrastBgUnparseable indica que fg sí parseó pero bg no.
	ContrastBgUnparseable
)

// ContrastRatioDetail es ContrastRatio con el motivo de un fallo: a
// diferencia de ContrastRatio, que colapsa "no evaluable" (gradiente,
// var() sin resolver) y "evaluado y pasa" en el mismo booleano, esto le
// permite al caller (ver linter.ThemeContrastRule) distinguir un color que
// no se pudo evaluar de un color que sí se evaluó — y nombrar cuál de los
// dos fue el problema, en vez de fabricar un diagnóstico de contraste
// reprobado o quedarse callado sobre lo que nunca se revisó.
func ContrastRatioDetail(fg, bg string) (ratio float64, status ContrastStatus) {
	fr, fgc, fb, fok := ParseColor(fg)
	if !fok {
		return 0, ContrastFgUnparseable
	}
	br, bgc, bb, bok := ParseColor(bg)
	if !bok {
		return 0, ContrastBgUnparseable
	}

	l1 := RelativeLuminance(fr, fgc, fb) + 0.05
	l2 := RelativeLuminance(br, bgc, bb) + 0.05
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return l1 / l2, ContrastOK
}

// MeetsAA reporta si ratio cumple el umbral WCAG 2.2 AA (SC 1.4.3) para
// texto normal o grande.
func MeetsAA(ratio float64, largeText bool) bool {
	if largeText {
		return ratio >= AALargeText
	}
	return ratio >= AANormalText
}

// MeetsAAA reporta si ratio cumple el umbral WCAG 2.2 AAA (SC 1.4.6) para
// texto normal o grande.
func MeetsAAA(ratio float64, largeText bool) bool {
	if largeText {
		return ratio >= AAALargeText
	}
	return ratio >= AAANormalText
}

// FormatRatio da una representación legible de un ratio ("4.50:1"), útil
// para mensajes de diagnóstico. TRUNCA a 2 decimales (no redondea): un
// ratio que MeetsAA ya evaluó como reprobado (p. ej. 4.499888, contra el
// umbral 4.5) redondeado con %.2f mostraría "4.50:1" — un valor que
// aparenta IGUALAR el umbral en un mensaje que dice "below... 4.5:1
// required", contradictorio (code review de issue #30). Truncar en vez de
// redondear garantiza que el valor mostrado nunca "suba" hacia el umbral:
// como todos los umbrales WCAG (3.0, 4.5, 7.0) son exactamente
// representables a 2 decimales, un ratio reprobado siempre se ve
// estrictamente por debajo, y uno aprobado nunca se ve por debajo del
// umbral que sí alcanzó.
func FormatRatio(ratio float64) string {
	truncated := math.Trunc(ratio*100) / 100
	return fmt.Sprintf("%.2f:1", truncated)
}
