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
	"strconv"
	"strings"
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

// fullyOpaqueAlpha es el único valor de canal alfa que ParseColor acepta
// para #rgba/#rrggbbaa — ver su doc comment.
const fullyOpaqueAlpha = 0xff

// ParseColor interpreta un color hexadecimal CSS: #rgb, #rgba, #rrggbb o
// #rrggbbaa. Formas no soportadas devuelven ok=false en vez de un valor
// adivinado: nombres de color CSS ("white", "tomato") y funciones
// (linear-gradient(...), rgba(...), var(...) sin resolver) no son hex, y
// una variable de tema real puede legítimamente contener cualquiera de
// esas — un rulepack de contraste debe poder saltarse ese par en vez de
// recibir un ratio inventado.
//
// Un canal alfa presente (#rgba/#rrggbbaa) que NO sea totalmente opaco
// también devuelve ok=false, por la misma razón: el contraste solo tiene
// sentido calculado sobre el color YA COMPUESTO contra su fondo real, y
// este paquete no tiene ese fondo — tratar cualquier alfa como si fuera
// 100% opaco (comportamiento previo, ahora corregido) le daría a un color
// translúcido/transparente el mismo trato que a rgba(), que si es
// correctamente rechazado por CanParse: dos formas del mismo color
// obtendrían resultados opuestos. Un alfa totalmente opaco (ff) es
// indistinguible de la forma sin alfa, así que se acepta igual.
func ParseColor(s string) (r, g, b uint8, ok bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		return 0, 0, 0, false
	}
	hex := s[1:]

	expand := func(c byte) string { return string([]byte{c, c}) }

	var rs, gs, bs, as string
	switch len(hex) {
	case 3: // #rgb
		rs, gs, bs = expand(hex[0]), expand(hex[1]), expand(hex[2])
	case 4: // #rgba
		rs, gs, bs, as = expand(hex[0]), expand(hex[1]), expand(hex[2]), expand(hex[3])
	case 6: // #rrggbb
		rs, gs, bs = hex[0:2], hex[2:4], hex[4:6]
	case 8: // #rrggbbaa
		rs, gs, bs, as = hex[0:2], hex[2:4], hex[4:6], hex[6:8]
	default:
		return 0, 0, 0, false
	}

	if as != "" {
		av, err := strconv.ParseUint(as, 16, 8)
		if err != nil || av != fullyOpaqueAlpha {
			return 0, 0, 0, false
		}
	}

	rv, err1 := strconv.ParseUint(rs, 16, 8)
	gv, err2 := strconv.ParseUint(gs, 16, 8)
	bv, err3 := strconv.ParseUint(bs, 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	return uint8(rv), uint8(gv), uint8(bv), true
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
// colores hex, ver ParseColor). El ratio siempre es >= 1 (el mayor de los
// dos luminance sobre el menor, +0.05 en cada uno). ok=false si cualquiera
// de los dos colores no es un hex parseable.
func ContrastRatio(fg, bg string) (ratio float64, ok bool) {
	fr, fgc, fb, fok := ParseColor(fg)
	if !fok {
		return 0, false
	}
	br, bgc, bb, bok := ParseColor(bg)
	if !bok {
		return 0, false
	}

	l1 := RelativeLuminance(fr, fgc, fb) + 0.05
	l2 := RelativeLuminance(br, bgc, bb) + 0.05
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return l1 / l2, true
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
