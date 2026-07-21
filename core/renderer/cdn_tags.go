// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

// Tags CDN con Subresource Integrity (SRI), compartidos entre el HTML
// browser-mode (document_html.go, generateDocumentScripts) y las páginas
// temporales que renderer/chromium carga para rasterizar mermaid/chart/mapas
// (buildMermaidSVGHTML/buildMermaidPNGHTML/buildChartHTML/generateLeafletHTML)
// para que un bump de versión no deje una copia desactualizada con un hash
// que ya no coincide con el contenido real — el navegador bloquea el script
// en silencio, sin señal del lado servidor. Si se bumpea una versión, hay
// que recomputar el hash sha384 (base64) contra la nueva URL exacta (no vale
// una URL de versión flotante: el hash deja de ser estable). Distintos de
// los pins de slidelang/.../base.go, que fijan versiones propias para
// mermaid/chart.js. Exportadas porque renderer/chromium las consume desde
// otro paquete.
const (
	MermaidCDNScriptTag = `<script src="https://cdn.jsdelivr.net/npm/mermaid@10.9.6/dist/mermaid.min.js" integrity="sha384-qX9VvWkP79m/O121ZE6sOYp0nf/pldQgtvWDbkpzi+3mUo4Wn4Ix4cFzNPay3VaB" crossorigin="anonymous"></script>`
	ChartJSCDNScriptTag = `<script src="https://cdn.jsdelivr.net/npm/chart.js@4.5.1/dist/chart.umd.js" integrity="sha384-hfkuqrKeWFmnTMWN31VWyoe8xgdTADD11kgxmdpx2uyE6j5Az5uZq6u6AKYYmAOw" crossorigin="anonymous"></script>`
	LeafletCDNCSSTag    = `<link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css" integrity="sha384-sHL9NAb7lN7rfvG5lfHpm643Xkcjzp4jFvuavGOndn6pjVqS6ny56CAt3nsEVT4H" crossorigin="anonymous">`
	LeafletCDNScriptTag = `<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js" integrity="sha384-cxOPjt7s7Iz04uaHJceBmS+qpjv2JkIHNVcuOrM+YHwZOmJGBXI00mdUXEq65HTH" crossorigin="anonymous"></script>`
	// MathCDNScriptTag (issue #239-B) usa el bundle tex-svg.js de MathJax:
	// TeX de entrada + SVG autocontenido de salida (glifos embebidos como
	// <path>, sin web-fonts externas) — la razón por la que se eligió
	// MathJax-SVG sobre KaTeX (que depende de fuentes woff2 vía CDN;
	// renderer/csp.go no tiene font-src, las bloquearía en silencio).
	// Hash sha384 calculado y verificado 2026-07-19 contra el archivo real
	// (1,849,625 bytes; sha256 cruzado contra el hash publicado por la API
	// de jsdelivr para este mismo paquete/versión) — no un valor de
	// plantilla. MathJax 4.x (no 3.x): la última versión estable en el
	// momento de escribir esto; el bundle tex-svg.js existe igual en ambas
	// series mayores, la razón de diseño (SVG autocontenido) es
	// independiente de la versión.
	MathCDNScriptTag = `<script src="https://cdn.jsdelivr.net/npm/mathjax@4.1.3/tex-svg.js" integrity="sha384-my9P1jDckpHD+5LZsLQ0gaiCl/RMO32HaqwBtbo/25QIMVr6xXIUCg1jvdSRcvb4" crossorigin="anonymous"></script>`
)
