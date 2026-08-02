// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package a11y

import "regexp"

// bcp47Pattern valida la FORMA de la producción `langtag` de BCP 47
// (RFC 5646 §2.1): subtag primario alfabético de 2-8 caracteres, seguido de
// cero o más subtags alfanuméricos de 2-8 caracteres separados por guion
// (script, región, variantes, extensiones — p.ej. "es", "es-MX",
// "zh-Hans-CN"). Es sintáctico, no semántico — igual que ParseColor con
// colores hex: valida ESTRUCTURA, no verifica el tag contra el registro
// IANA de subtags de idioma. "xx-YY" pasa aunque "xx" no sea un código de
// idioma real; ese nivel de validación (contra un registro que cambia) no
// es responsabilidad de este paquete, que deliberadamente no lo carga ni
// lo actualiza — mismo principio que ParseColor no adivinando nombres de
// color CSS.
//
// Alcance deliberadamente NO cubierto (code review de este cambio: la
// primera versión de este regex aceptaba "en-a", un singleton de extensión
// colgado sin valor — subtags subsecuentes de un solo carácter solo son
// válidos como inicio de una extensión de la forma "-x-yyyy", nunca solos,
// así que el mínimo de 2 caracteres por subtag los excluye sin necesidad de
// modelar la gramática de extensión completa):
//   - `privateuse` standalone ("x-...") — un tag que empieza en "x" pasa
//     este regex como si "x" fuera un subtag de idioma normal, lo cual da
//     el resultado correcto (BCP 47 exige 2-8 letras para el primario, así
//     que "x" solo nunca matchea), pero un privateuse *válido* como
//     "x-private" tampoco matchea nada más — se rechaza. Es un
//     falso-rechazo conocido, no un bug: ver IsValidLangTag.
//   - `grandfathered` — los ~26 tags irregulares congelados en el registro
//     IANA (p.ej. "i-klingon", "sgn-BE-FR") tampoco matchean esta gramática
//     y se rechazan igual. Ninguno es un caso realista de `lang:` en un
//     documento de este toolchain.
//
// Las clases van explícitas en mayúsculas Y minúsculas ([a-zA-Z], no
// [a-z] con el flag (?i)): RE2 aplica Unicode simple case folding bajo
// (?i), que acepta homoglifos fuera de ASCII como U+017F ("ſ", folds con
// "s") o U+212A ("K" signo Kelvin, folds con "k") — un tag como "eſ"
// pasaría como si fuera "es" (code review de este cambio). BCP 47 es
// ASCII puro; las clases explícitas cierran esa puerta sin necesitar el
// flag.
var bcp47Pattern = regexp.MustCompile(`^[a-zA-Z]{2,8}(-[a-zA-Z0-9]{2,8})*$`)

// IsValidLangTag reporta si tag tiene la forma de la producción `langtag`
// de un tag de idioma BCP 47 (RFC 5646 §2.1: language-script-region-variant-
// extension). NO reconoce las formas `privateuse` standalone ("x-...") ni
// `grandfathered` ("i-...", "sgn-BE-FR", etc.) — ver el comentario de
// bcp47Pattern para el porqué. Quien necesite ese subconjunto adicional
// debe validarlo aparte; para el caso de uso de este paquete (un `lang:`
// de frontmatter que un autor humano declara) ese subconjunto no es
// alcanzable en la práctica.
//
// Una cadena vacía es inválida — "sin idioma declarado" y "idioma
// declarado vacío" son casos distintos, y quien llama (una regla de
// linter, un renderer) decide qué hacer con cada uno por separado.
func IsValidLangTag(tag string) bool {
	return tag != "" && bcp47Pattern.MatchString(tag)
}
