// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"regexp"
	"sort"
	"strconv"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// parserFindingRules son las reglas del linter que, para una parte de sus
// casos, detecta el parser y no el linter. LINK001 de una imagen de bloque
// con una ruta suelta con espacios (`![a](mi dir/a.png)`) es el caso: el AST
// guarda la ruta limpia, así que el linter ya no distingue `mi dir/a.png` de
// `<mi dir/a.png>`, y el aviso sale del parser. Como diagnóstico de parseo
// solo se imprimía por stderr: el reporte (--report json/sarif), que es lo
// que leen los editores, y la política de lint no lo veían, mientras que el
// mismo aviso sobre una imagen inline sí llegaba porque lo da el linter.
var parserFindingRules = map[string]bool{"LINK001": true}

// SplitParserFindings separa de los diagnósticos del parser los avisos de
// una regla del linter (ver parserFindingRules): rest son los diagnósticos
// de parseo de siempre y findings los que van con los del linter, a la
// política y al reporte. Los errores se quedan en rest: frenan el build en la
// etapa de parseo.
func SplitParserFindings(parseDiags []diagnostics.Diagnostic) (rest, findings []diagnostics.Diagnostic) {
	for _, d := range parseDiags {
		if !d.IsError() && parserFindingRules[d.RuleID] {
			findings = append(findings, d)
			continue
		}
		rest = append(rest, d)
	}
	return rest, findings
}

// MergeParserFindings agrega findings (de SplitParserFindings) a los
// diagnósticos del linter, sin duplicar: una imagen de bloque dentro de un
// :::bloque o de una columna de grid está también en el texto (Content) de su
// contenedor, y el linter, que lee ese texto como Markdown inline, ya avisa
// de ella, pero en la línea del contenedor. En ese caso se queda el aviso del
// parser, que lleva la línea de la imagen, y se quitan los del linter. El
// resultado va ordenado por línea.
func MergeParserFindings(doc *ast.AST, findings, lintDiags []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	if len(findings) == 0 {
		return lintDiags
	}
	containers := blockImageContainers(doc)
	dropped := make([]bool, len(lintDiags))
	for _, f := range findings {
		dest, ok := spacedDestination(f.Message)
		if !ok {
			continue
		}
		lines := containers[f.Position.Line]
		for i, d := range lintDiags {
			if dropped[i] || d.RuleID != f.RuleID || !lines[d.Position.Line] {
				continue
			}
			// Cada contenedor de la imagen (un :::note dentro de un
			// :::warning) avisa por su cuenta: se quitan todos.
			if other, ok := spacedDestination(d.Message); ok && other == dest {
				dropped[i] = true
			}
		}
	}
	out := make([]diagnostics.Diagnostic, 0, len(lintDiags)+len(findings))
	for i, d := range lintDiags {
		if !dropped[i] {
			out = append(out, d)
		}
	}
	out = append(out, findings...)
	// En orden de documento: el aviso del parser no queda al final.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Position.Line < out[j].Position.Line })
	return out
}

// spacedDestinationPattern saca la ruta del aviso LINK001 de ruta suelta con
// espacios, el del parser ("image destination %q has spaces ...") y el del
// linter ("Image destination %q has spaces ...").
var spacedDestinationPattern = regexp.MustCompile(`(?i)destination ("(?:[^"\\]|\\.)*") has spaces outside`)

func spacedDestination(msg string) (string, bool) {
	m := spacedDestinationPattern.FindStringSubmatch(msg)
	if m == nil {
		return "", false
	}
	dest, err := strconv.Unquote(m[1])
	return dest, err == nil
}

// blockImageContainers devuelve, por línea de cada imagen de bloque anidada,
// las líneas de los contenedores cuyo texto lee el linter (el :::bloque, la
// columna y el grid).
func blockImageContainers(doc *ast.AST) map[int]map[int]bool {
	out := map[int]map[int]bool{}
	if doc == nil {
		return out
	}
	var visit func(elements []ast.Element, outer []int)
	visit = func(elements []ast.Element, outer []int) {
		for _, el := range elements {
			switch e := el.(type) {
			case *ast.ImageElement:
				if len(outer) == 0 {
					continue
				}
				line := e.GetPosition().Line
				if out[line] == nil {
					out[line] = map[int]bool{}
				}
				for _, l := range outer {
					out[line][l] = true
				}
			case *ast.SpecialBlockElement:
				visit(e.Elements, append(outer[:len(outer):len(outer)], e.GetPosition().Line))
			case *ast.GridElement:
				grid := append(outer[:len(outer):len(outer)], e.GetPosition().Line)
				for _, col := range e.Columns {
					visit(col.Elements, append(grid[:len(grid):len(grid)], col.GetPosition().Line))
				}
			}
		}
	}
	for _, b := range doc.ContentBlocks {
		visit(b.Elements, nil)
	}
	return out
}
