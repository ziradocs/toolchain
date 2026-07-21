// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

// ChromiumBrand agrupa el "branding" del pipeline de Chromium (issue #92): el
// segmento de directorio usado para instalar/cachear Chromium localmente (p.
// ej. ~/.doclang/chromium, ~/Library/Caches/doclang) y el bloque de ejemplos
// que EnsureChromium muestra cuando Chromium no se encuentra.
//
// Se inyecta por instancia vía NewChromiumRendererWithBrand — NO como estado
// global mutable. El diseño anterior (dos `var` a nivel de paquete, mutadas
// por el CLI justo antes de construir el renderer) fue señalado en code
// review de PR #122 como un antipatrón de Go: rompe encapsulamiento y no es
// thread-safe si en algún momento se renderiza en paralelo, se corren tests
// concurrentes, o un mismo proceso necesita gestionar dos brands distintos —
// los valores colisionarían causando bugs intermitentes difíciles de rastrear.
type ChromiumBrand struct {
	Name        string
	InstallHint string
}

// DefaultChromiumBrand reproduce el comportamiento histórico de doclang
// byte-por-byte. Es el valor que usan NewChromiumRenderer/NewChromiumManager/
// NewChromiumInstaller (las firmas "clásicas", sin "WithBrand") para que
// doclang no requiera ningún cambio.
var DefaultChromiumBrand = ChromiumBrand{
	Name: "doclang",
	InstallHint: `Examples:
  doclang build doc.doclang --format=pdf --install-chromium
  doclang build doc.doclang --format=pdf --chromium-path=/usr/bin/chromium
  doclang build doc.doclang --format=pdf --chromium-path="/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"`,
}
