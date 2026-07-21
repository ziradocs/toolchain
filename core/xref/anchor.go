// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package xref

import (
	"regexp"
	"strings"
)

var (
	unsafeAnchorChars = regexp.MustCompile(`[^a-z0-9\-]+`)
	repeatedDashes    = regexp.MustCompile(`-+`)
)

// AnchorID convierte un label de usuario (p. ej. "Fig: Arquitectura!") en un
// id de HTML seguro y estable ("fig-arquitectura"): minúsculas, todo lo que
// no sea [a-z0-9-] colapsa a "-", corridas de "-" (incluidas las que ya
// venían en el label, p. ej. "a---b") se colapsan a una sola, sin "-" al
// principio/final. Debe producir el MISMO resultado en el sitio que emite
// `id="..."` (renderer) y en el que emite `href="#..."` (resolución de
// \ref) — por eso vive acá, importado por ambos lados, en vez de duplicarse.
func AnchorID(label string) string {
	slug := unsafeAnchorChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(label)), "-")
	slug = repeatedDashes.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}
