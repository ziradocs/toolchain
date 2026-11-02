// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	_ "embed"
)

// embeddedChartJS y embeddedChartJSTreemap son los mismos builds que carga el
// HTML en modo browser (renderer.ChartJSCDNScriptTag: chart.js@4.5.1
// dist/chart.umd.js, y renderer.ChartJSTreemapCDNScriptTag:
// chartjs-chart-treemap@4.2.0), copiados tal cual de jsDelivr. Mismo esquema
// que mermaid (mermaid_embed.go): TestEmbeddedChartJS_MatchesCDNIntegrity
// exige que su sha384 sea el hash SRI de esos tags, así que un bump de un
// lado sin el otro rompe el test. Licencia MIT de los dos:
// third_party/chartjs/LICENSE.md, third_party/chartjs-chart-treemap/LICENSE
// y NOTICE.
//
// Antes la página de captura cargaba los dos scripts de jsDelivr: sin red,
// `new Chart(...)` lanzaba "Chart is not defined", la señal de listo nunca
// llegaba y `doclang build -f pdf|docx` se colgaba con cualquier chart que
// no pudiera dibujarse nativo (p. ej. un ```chart con cuerpo JSON).
//
//go:embed third_party/chartjs/chart.umd.js
var embeddedChartJS string

//go:embed third_party/chartjs-chart-treemap/chartjs-chart-treemap.min.js
var embeddedChartJSTreemap string

// embeddedChartScriptTags inyecta los dos builds inline, en el orden que
// exige el treemap (se auto-registra contra el `Chart` global, ver
// renderer.ChartJSTreemapCDNScriptTag).
var embeddedChartScriptTags = inlineScriptTag(embeddedChartJS) + "\n    " + inlineScriptTag(embeddedChartJSTreemap)

// chartRenderCSP es la CSP de las páginas de chart: sin ningún origen de red,
// porque los scripts ya van inline.
const chartRenderCSP = mermaidRenderCSP
