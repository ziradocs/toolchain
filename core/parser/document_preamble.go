// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import "strings"

// normalizeDocumentStart prepara el inicio de un DOCUMENTO para que su
// frontmatter se reconozca aunque no esté en el primer byte:
//
//   - quita un BOM UTF-8 inicial (editores de Windows lo agregan);
//   - si el primer renglón no vacío es `---`, mueve las líneas en blanco que
//     lo preceden a DESPUÉS del `---` de cierre.
//
// Antes cualquiera de los dos dejaba el frontmatter sin reconocer y su YAML
// (título, autor…) acababa como un párrafo del cuerpo. Mover las líneas en
// blanco en vez de borrarlas conserva exactas las posiciones del cuerpo:
// sigue teniendo el mismo número de líneas antes que él. Solo las
// posiciones dentro del frontmatter se corren tantas líneas como blancos
// había. FrontMatterParser (compartido con slidelang, que exige `---` en la
// primera línea) no cambia: esto corre solo en las entradas de documentos.
func normalizeDocumentStart(content string) string {
	content = strings.TrimPrefix(content, "\xef\xbb\xbf")

	lines := strings.Split(content, "\n")
	lead := 0
	for lead < len(lines) && strings.TrimSpace(lines[lead]) == "" {
		lead++
	}
	if lead == 0 || lead >= len(lines) || strings.TrimSpace(lines[lead]) != "---" {
		return content
	}
	closing := -1
	for i := lead + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closing = i
			break
		}
	}
	if closing < 0 {
		return content
	}

	out := make([]string, 0, len(lines))
	out = append(out, lines[lead:closing+1]...)
	out = append(out, lines[:lead]...)
	out = append(out, lines[closing+1:]...)
	return strings.Join(out, "\n")
}
