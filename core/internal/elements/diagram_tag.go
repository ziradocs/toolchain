// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"strings"
)

// parseDiagramTag reconoce la apertura `<<tag>>` de un diagrama y su forma
// con pie, `<<tag title="Texto">>` (o con comillas simples). El pie llena el
// Title que MermaidElement/PlantUMLElement ya tenían y que el renderer ya
// dibuja, pero que ningún parser llenaba: el autor terminaba escribiendo el
// pie como un TEXT hermano, sin relación con la figura.
//
// matched dice si la línea es una apertura de este tag; err, si lo es pero
// sus atributos no son válidos. Un atributo desconocido, repetido o mal
// entrecomillado es un error y no texto: antes esa línea caía a prosa sin
// ningún diagnóstico.
func parseDiagramTag(trimmed, tag string) (matched bool, title string, err error) {
	rest, ok := strings.CutPrefix(trimmed, "<<"+tag)
	if !ok {
		return false, "", nil
	}
	if rest == ">>" {
		return true, "", nil
	}
	if !strings.HasPrefix(rest, " ") || !strings.HasSuffix(rest, ">>") {
		return false, "", nil
	}
	attrs := strings.TrimSpace(strings.TrimSuffix(rest, ">>"))
	if !closesInlineTagOnce(rest) {
		return true, "", fmt.Errorf("<<%s>> attributes cannot contain '>>'", tag)
	}
	seen := false
	for attrs != "" {
		key, after, found := strings.Cut(attrs, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" || len(after) == 0 || (after[0] != '"' && after[0] != '\'') {
			return true, "", fmt.Errorf("<<%s>> attributes must be key=\"value\"; got %q", tag, attrs)
		}
		q := after[0]
		end := strings.IndexByte(after[1:], q)
		if end < 0 {
			return true, "", fmt.Errorf("<<%s>> attribute %s is missing its closing quote", tag, key)
		}
		value := after[1 : 1+end]
		attrs = strings.TrimSpace(after[2+end:])
		if key != "title" {
			return true, "", fmt.Errorf("unknown <<%s>> attribute %q; only title is supported", tag, key)
		}
		if seen {
			return true, "", fmt.Errorf("duplicate <<%s>> attribute title", tag)
		}
		seen = true
		if strings.TrimSpace(value) == "" {
			return true, "", fmt.Errorf("<<%s>> title cannot be empty", tag)
		}
		title = value
	}
	return true, title, nil
}
