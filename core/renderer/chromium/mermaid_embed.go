// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	_ "embed"
	"strings"
)

// embeddedMermaidJS es el mismo build de mermaid que carga el HTML en modo
// browser (renderer.MermaidCDNScriptTag: mermaid@10.9.6/dist/mermaid.min.js),
// copiado tal cual de jsDelivr. TestEmbeddedMermaid_MatchesCDNIntegrity
// verifica que su sha384 sea el mismo hash SRI de ese tag, así que un bump de
// versión en un lado sin el otro rompe el test en vez de divergir en
// silencio. Licencia MIT: third_party/mermaid/LICENSE y NOTICE.
//
// Vive en este paquete (//go:build !js) y no en renderer porque solo lo usan
// las páginas que rasteriza Chromium; el build WASM del playground no carga
// estos ~3.3 MB.
//
//go:embed third_party/mermaid/mermaid.min.js
var embeddedMermaidJS string

// embeddedMermaidScriptTag inyecta el build embebido como <script> inline,
// para que rasterizar mermaid (DOCX, PDF y HTML offline-*) no necesite red.
// El bundle no contiene "</script" (lo verifica un test); el reemplazo es
// solo una defensa por si un bump futuro lo trajera, y es inocuo dentro de
// JS ("<\/" y "</" son el mismo texto en strings y regex).
var embeddedMermaidScriptTag = inlineScriptTag(embeddedMermaidJS)

// inlineScriptTag envuelve un bundle embebido en un <script> inline. Los
// bundles no contienen "</script" (lo verifican los tests de cada embed); el
// reemplazo es solo una defensa por si un bump futuro lo trajera.
func inlineScriptTag(js string) string {
	return "<script>" + strings.ReplaceAll(js, "</script", `<\/script`) + "</script>"
}

// mermaidRenderCSP es la CSP de las páginas de mermaid (y, con otro nombre,
// de chart y math): sin cdn.jsdelivr.net, porque el script ya va inline. Sin
// ningún origen de red permitido, la página no puede pedir nada afuera
// aunque quisiera.
const mermaidRenderCSP = "default-src 'none'; script-src 'unsafe-inline'; " +
	"style-src 'unsafe-inline'; img-src data:; connect-src 'none'; object-src 'none'; base-uri 'none';"
