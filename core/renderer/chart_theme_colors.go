// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

// Sin build tag a propósito: RenderContext (context.go) y
// GenerateChartConfigWithTheme (html.go) usan este tipo y compilan en
// todos los targets, GOOS=js incluido (slidelang/cmd/wasm). Vivía en
// native_chart.go, que es //go:build !js, y eso rompía el build WASM.

// ChartThemeColors agrupa los tokens chart-* NO categóricos de
// motor-temas-v2.md §2.2 que el camino offline/PDF puede honrar de verdad.
// Es un struct por valor y su zero value significa "sin tema": cada campo
// vacío deja intacto el color por defecto de go-analyze/charts, así que un
// caller que no lo llene reproduce el render de antes byte por byte (el
// mismo contrato de ChartCategoricalColors, ver RenderContext).
//
// Deliberadamente NO cubre dos grupos del contrato §2.2, y no es un olvido
// sino una imposibilidad estructural del render offline:
//
//   - chart-tooltip-bg: un tooltip solo existe bajo hover. El camino nativo
//     produce un PNG estático y el camino Chromium toma el screenshot sin
//     puntero, así que ese color no se vería jamás. Emitirlo sería cablear
//     algo que no puede pintar un pixel.
//   - chart-seq-1..5: ni GenerateChartConfigWithMode ni este rasterizador
//     consumen una rampa secuencial — todas las paletas de ambos son
//     categóricas. No hay destinatario al cual dárselos.
//
// Ambos siguen siendo válidos en el navegador (PR #228, vía el blob de
// metadata que consume charts.js), que es donde sí hay hover; acá quedan
// documentados como browser-only para que nadie los busque después.
type ChartThemeColors struct {
	Surface string // chart-surface: fondo del lienzo del chart
	Grid    string // chart-grid: líneas de rejilla
	Axis    string // chart-axis: línea del eje y el texto de sus marcas
	Label   string // chart-label: texto de leyenda, título y etiquetas de valor
}

// IsZero reporta si no hay ningún color de tema puesto — el caso de todo
// caller que no resolvió un tema. Exportado porque chromium.ChartFetcher
// (otro paquete) lo necesita para decidir si el tema entra al cache key.
func (c ChartThemeColors) IsZero() bool {
	return c == ChartThemeColors{}
}
