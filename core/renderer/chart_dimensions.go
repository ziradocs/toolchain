// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "go.ziradocs.com/core/ast"

// ChartDimensions resuelve las dimensiones de render de elem — 800x600 por
// default, o elem.Width/elem.Height si el autor los especificó. Única
// fuente de verdad reusada por html.go (renderChartElement, el render
// real) y por el gate de issue #164 (tryBuildNativeContext, que necesita
// intentar el mismo render con las mismas dimensiones para PROBAR éxito
// —no solo elegibilidad de tipo— y sembrar esos bytes en el fetcher) — si
// alguna vez divergieran, el gate podría aprobar/sembrar un chart con
// dimensiones distintas a las que el render real termina pidiendo.
//
// Vive en su propio archivo, SIN el `//go:build !js` de native_chart.go: es
// aritmética pura sobre el AST, sin dependencia de go-analyze/charts, y
// html.go (que sí se compila para wasm) la llama. Mientras estuvo dentro de
// native_chart.go, `GOOS=js GOARCH=wasm go build ./cmd/wasm` fallaba con
// "undefined: ChartDimensions" — es decir, el build del playground.
func ChartDimensions(elem *ast.ChartElement) (width, height int) {
	width, height = 800, 600
	if elem.Width > 0 {
		width = elem.Width
	}
	if elem.Height > 0 {
		height = elem.Height
	}
	return width, height
}
