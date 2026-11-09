// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package inks nombra las tintas de estado del documento ([x]{.danger},
// .info, .success, .warning, .accent) y las superficies con fondo propio que
// pueden fijar las suyas. El CSS del documento
// (core/renderer/document_html.go, documentInkSurfaceCSS) lee
// --doclang-<superficie>-ink-<tinta> dentro de cada superficie; el nombre de
// esas variables es contrato de temas, estas listas no.
package inks

// Names son las tintas, en el orden de las clases .slidelang-text-<tinta>.
var Names = []string{"danger", "info", "success", "warning", "accent"}

// Surfaces son las superficies con fondo propio que fijan sus tintas.
var Surfaces = []string{
	"table", "table-stripe", "table-header", "panel", "blockquote",
	"callout-info", "callout-warning", "callout-danger", "callout-success", "callout-tip",
	"quiz",
}

// Variable es el nombre de la tinta ink dentro de la superficie surface.
func Variable(surface, ink string) string {
	return "--doclang-" + surface + "-ink-" + ink
}

// Variables lista Variable para cada superficie y tinta.
func Variables() []string {
	out := make([]string, 0, len(Surfaces)*len(Names))
	for _, s := range Surfaces {
		for _, ink := range Names {
			out = append(out, Variable(s, ink))
		}
	}
	return out
}
