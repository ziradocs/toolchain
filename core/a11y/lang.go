// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package a11y

import "regexp"

// bcp47Pattern valida la FORMA de un tag de idioma BCP 47 (RFC 5646):
// subtag primario alfabético de 2-8 caracteres, seguido de cero o más
// subtags alfanuméricos de 1-8 caracteres separados por guion (script,
// región, variantes, extensiones — p.ej. "es", "es-MX", "zh-Hans-CN"). Es
// sintáctico, no semántico — igual que ParseColor con colores hex: valida
// ESTRUCTURA, no verifica el tag contra el registro IANA de subtags de
// idioma. "xx-YY" pasa aunque "xx" no sea un código de idioma real; ese
// nivel de validación (contra un registro que cambia) no es responsabilidad
// de este paquete, que deliberadamente no lo carga ni lo actualiza — mismo
// principio que ParseColor no adivinando nombres de color CSS.
var bcp47Pattern = regexp.MustCompile(`(?i)^[a-z]{2,8}(-[a-z0-9]{1,8})*$`)

// IsValidLangTag reporta si tag tiene la forma de un tag de idioma BCP 47
// bien formado. Una cadena vacía es inválida — "sin idioma declarado" y
// "idioma declarado vacío" son casos distintos, y quien llama (una regla de
// linter, un renderer) decide qué hacer con cada uno por separado.
func IsValidLangTag(tag string) bool {
	return tag != "" && bcp47Pattern.MatchString(tag)
}
