// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateCSPNonce returns a fresh, cryptographically random base64-encoded
// nonce for use as both a Content-Security-Policy 'nonce-...' source and
// the matching nonce="..." attribute on every inline <script>/<style> tag
// in the same document. A new nonce must be generated per build — reusing
// one across documents/builds would let an attacker who can inject markup
// into ANY one of them replay that nonce to authorize a script in another,
// defeating the point of using one.
func GenerateCSPNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate CSP nonce: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

// BuildDefaultOutputCSP builds the Content-Security-Policy value for the
// default (non-offline, browser-mode) HTML output of both slidelang and
// doclang. Ver docs/SECURITY_AUDIT_2026-07.md, BA-10.
//
// script-src es estricto: solo el nonce de este build y los dos hosts de
// CDN que el output realmente carga (cdn.jsdelivr.net para mermaid/
// chart.js, unpkg.com para leaflet) — cerrar la ejecución de script
// arbitrario es el objetivo real del hallazgo.
//
// style-src usa 'unsafe-inline' en vez de nonce: verificado en vivo
// (headless Chrome) que Mermaid inyecta su CSS de tema en runtime vía un
// <style> DENTRO del SVG renderizado, sin nonce y sin ninguna opción de
// configuración para asignarle uno — bajo un style-src con nonce ese
// <style> se bloquea silenciosamente (sin violation logueada en consola) y
// las figuras del diagrama caen al fill negro por defecto de SVG. Esto no
// reabre BA-11 (inyección CSS en variables de tema): esa vulnerabilidad se
// cierra en el string en sí, vía SanitizeCSSCustomProperty (rechaza
// `{}<>;`/CR-LF antes de interpolar) — la CSP para estilos era solo
// defensa en profundidad sobre una vulnerabilidad ya cerrada en la fuente,
// a diferencia de script-src, donde SÍ es la defensa real contra ejecución
// de JS.
//
// img-src/object-src/connect-src son permisivos, SIN restringir scheme: un
// documento puede referenciar cualquier imagen (markdown, logos de header/
// footer, iconos de marcador de mapa por documento — ver elements/map.go),
// cualquier proveedor de tiles de mapa configurado, y un `--plantuml-server`
// custom cuyo host termina directamente en un <object data=...>/<img
// src=...> del HTML. Ninguno de estos se puede acotar a una allowlist fija
// sin romper personalización legítima ya soportada, y forzar https:
// específicamente rompería un servidor PlantUML interno/self-hosted o un
// proveedor de tiles corriendo en http:// (un caso legítimo, no distinto al
// de --plantuml-server sobre HTTP que la propia SSRF fix de plantuml_fetcher.go
// trata como confiable) — antes de esta CSP ningún scheme estaba
// restringido para estos tres vectores, así que dejarlos abiertos aquí no
// reabre nada: es exactamente la misma superficie que ya existía.
func BuildDefaultOutputCSP(nonce string) string {
	return fmt.Sprintf(
		"default-src 'self'; "+
			"script-src 'self' 'nonce-%s' https://cdn.jsdelivr.net https://unpkg.com; "+
			"style-src 'self' 'unsafe-inline' https://unpkg.com; "+
			"img-src 'self' * data:; "+
			"object-src *; "+
			"connect-src 'self' *; "+
			"base-uri 'self';",
		nonce,
	)
}
