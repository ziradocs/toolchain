// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"fmt"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// ContrastPair nombra dos variables CSS del tema (resueltas por la CLI, ver
// ThemeAware) que ThemeContrastRule debe comparar: un color de primer plano
// (texto) y uno de fondo. core no sabe qué variable corresponde a qué tipo
// de elemento — esa asociación varía entre slidelang ("--text-color") y
// doclang ("--doclang-h1-color") y la decide quien construye la regla.
type ContrastPair struct {
	Label      string // descripción legible, p. ej. "body text on content background"
	FgVariable string // nombre de variable CSS del color de texto
	BgVariable string // nombre de variable CSS del color de fondo
	LargeText  bool   // true si aplica el umbral WCAG de "texto grande" (>=18pt, o >=14pt negrita)
}

// ThemeContrastRule es la regla de referencia para el seam de contraste
// (issue #30): recibe el mapa de variables del tema activo vía ThemeAware
// y, por cada ContrastPair configurado, calcula el ratio WCAG y emite un
// diagnóstico si no alcanza el umbral AA. Un par cuya variable no está en
// el mapa del tema, o cuyo valor no es un hex parseable (gradiente,
// rgba(), var() sin resolver), se SALTA en silencio — no es una condición
// de error, es una limitación conocida de este seam (ver core/a11y).
//
// Deliberadamente NO está en DefaultRules(): habilitarla por defecto
// cambiaría la salida de lint de cualquier usuario existente y arriesga
// falsos positivos sobre temas con mapas de variables parciales. Existe
// para probar el seam de punta a punta y como plantilla para el rulepack
// externo que consume este mecanismo — un consumidor real la agrega
// explícitamente vía linter.AddRule(linter.NewThemeContrastRule(pairs)).
type ThemeContrastRule struct {
	pairs          []ContrastPair
	themeVariables map[string]string
}

// NewThemeContrastRule construye una ThemeContrastRule para los pares dados.
func NewThemeContrastRule(pairs []ContrastPair) *ThemeContrastRule {
	return &ThemeContrastRule{pairs: pairs}
}

// SetThemeVariables implementa ThemeAware.
func (r *ThemeContrastRule) SetThemeVariables(vars map[string]string) {
	r.themeVariables = vars
}

// Check corre una sola vez, a nivel de documento (*ast.AST) — el tema es
// una propiedad del documento completo, no de un slide/sección individual,
// así que repetir la comprobación en cada ContentBlock (como hacen las
// reglas per-slide) produciría el mismo diagnóstico duplicado por slide.
func (r *ThemeContrastRule) Check(node ast.Node) []diagnostics.Diagnostic {
	astNode, ok := node.(*ast.AST)
	if !ok {
		return nil
	}
	if len(r.themeVariables) == 0 {
		return nil
	}

	var diags []diagnostics.Diagnostic
	for _, pair := range r.pairs {
		fg, fgOK := r.themeVariables[pair.FgVariable]
		bg, bgOK := r.themeVariables[pair.BgVariable]
		if !fgOK || !bgOK {
			continue
		}

		ratio, ok := a11y.ContrastRatio(fg, bg)
		if !ok {
			continue
		}

		if !a11y.MeetsAA(ratio, pair.LargeText) {
			threshold := a11y.AANormalText
			if pair.LargeText {
				threshold = a11y.AALargeText
			}
			diags = append(diags, diagnostics.NewWarning(
				fmt.Sprintf("%s contrast is %s, below WCAG 2.2 AA (%.1f:1 required)",
					pair.Label, a11y.FormatRatio(ratio), threshold),
				astNode.GetPosition(), "contrast-rule").WithRuleID("CONTRAST001"))
		}
	}
	return diags
}
