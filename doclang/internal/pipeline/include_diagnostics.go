// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/include"
)

// KeptIncludeRuleID es el aviso de un `@include` escrito dentro de un bloque
// de código: es código, queda como está y no se expande (issue #487).
const KeptIncludeRuleID = "INCLUDE001"

// keptIncludeDiagnostics es un INCLUDE001 por cada línea de lines (el
// contenido expandido) que include.ExpandWithOptions dejó sin expandir por
// estar en código (Expansion.KeptInCode, 1-based).
func keptIncludeDiagnostics(lines []string, kept []int) []diagnostics.Diagnostic {
	out := make([]diagnostics.Diagnostic, 0, len(kept))
	for _, n := range kept {
		directive := ""
		if n >= 1 && n <= len(lines) {
			directive = strings.TrimSpace(lines[n-1])
		}
		// La columna del `@`: core reconoce la directiva con
		// strings.TrimSpace, así que delante puede haber cualquier espacio
		// Unicode (U+00A0, U+3000), no solo espacios y tabs. En bytes, como
		// las demás posiciones de core.
		column := 1
		if n >= 1 && n <= len(lines) {
			column = len(lines[n-1]) - len(strings.TrimLeftFunc(lines[n-1], unicode.IsSpace)) + 1
		}
		out = append(out, diagnostics.NewWarning(
			fmt.Sprintf("%q is inside a code block, so it is code: it is kept as written and not expanded. To include the file, write the @include outside the code block", directive),
			diagnostics.NewPosition(n, column), "include").WithRuleID(KeptIncludeRuleID))
	}
	return out
}

// Where es dónde está d para un mensaje: "line N" en el documento, o
// "archivo:N" en un archivo incluido (Diagnostic.File, relativo al
// documento).
func Where(d diagnostics.Diagnostic) string {
	if d.File == "" {
		return fmt.Sprintf("line %d", d.Position.Line)
	}
	return fmt.Sprintf("%s:%d", d.File, d.Position.Line)
}

// DocumentDir es el directorio del documento inputFile para
// PrepareOptions.DocDir: filepath.Abs de su directorio, sin resolver
// symlinks, en la misma forma que la ruta que el reporte pone en
// document.path (core une las dos con path.Join, léxico). Vacío si inputFile
// está vacío (un documento sin ruta en disco).
func DocumentDir(inputFile string) string {
	if inputFile == "" {
		return ""
	}
	dir, err := filepath.Abs(filepath.Dir(inputFile))
	if err != nil {
		return ""
	}
	return dir
}

// sourcesFromDocDir devuelve sources con la ruta de cada archivo incluido
// escrita desde docDir (DocumentDir), para que
// include.SourceMap.MapDiagnostics la nombre relativa a él y
// path.Join(dir de document.path, file) sea ese archivo.
//
// Las rutas de los archivos incluidos salen de la raíz de includes, que con
// --base-dir o --include-root tiene los symlinks resueltos, y docDir no: con
// la entrada por un directorio enlazado, un archivo relativo a la forma
// resuelta del directorio y unido a la forma con symlink no existe. Si el
// archivo se alcanza desde docDir con su ruta relativa a la forma resuelta
// (el caso común, el archivo debajo del documento), se usa esa: es la más
// corta y la de siempre. Si no, se deja la ruta tal cual, y su relativa a
// docDir es léxica, que es como core la une.
func sourcesFromDocDir(sources include.SourceMap, docDir string) include.SourceMap {
	if docDir == "" {
		return sources
	}
	realDir, err := filepath.EvalSymlinks(docDir)
	if err != nil || realDir == docDir {
		return sources
	}
	rewritten := map[string]string{}
	lines := make([]include.Origin, len(sources.Lines))
	for i, o := range sources.Lines {
		if o.File != "" {
			file, ok := rewritten[o.File]
			if !ok {
				file = o.File
				if rel, err := filepath.Rel(realDir, o.File); err == nil {
					if candidate := filepath.Join(docDir, rel); sameFile(candidate, o.File) {
						file = candidate
					}
				}
				rewritten[o.File] = file
			}
			o.File = file
		}
		lines[i] = o
	}
	return include.SourceMap{Lines: lines, RootLines: sources.RootLines}
}

// sameFile dice si a y b son el mismo archivo en disco.
func sameFile(a, b string) bool {
	ia, err := os.Stat(a)
	if err != nil {
		return false
	}
	ib, err := os.Stat(b)
	return err == nil && os.SameFile(ia, ib)
}

// MapToSource lleva diags al archivo y la línea que los escribieron
// (include.SourceMap.MapDiagnostics) y quita las copias que dejó otra
// expansión del mismo archivo incluido (DedupeIncluded).
func MapToSource(sources include.SourceMap, diags []diagnostics.Diagnostic, docDir string) []diagnostics.Diagnostic {
	return DedupeIncluded(diags, sources.MapDiagnostics(diags, docDir))
}

// DedupeIncluded quita de mapped (raw llevado a su archivo de origen, índice
// por índice, como lo devuelve include.SourceMap.MapDiagnostics) las copias
// de un hallazgo de un archivo incluido que vienen de otra expansión de ese
// archivo, y conserva las demás en su orden.
//
// Un archivo incluido dos veces se lintea dos veces, y las dos expansiones
// dan el mismo hallazgo en el mismo archivo y la misma línea. Pero una sola
// expansión también puede dar varios hallazgos idénticos (dos enlaces
// javascript: en una línea se reportan con la misma posición y el mismo
// mensaje), y esos se quedan todos, como en el documento. La expansión se
// distingue por la línea del contenido expandido (la de raw): una línea de
// un archivo incluido cae en una línea expandida distinta en cada expansión,
// también con includes anidados. De cada hallazgo quedan tantas copias como
// dio la expansión que más dio. Un hallazgo del documento (File vacío) no se
// toca.
func DedupeIncluded(raw, mapped []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	type key = diagnosticKey
	keyOf := keyOfDiagnostic
	type expansion struct {
		k    key
		line int
	}
	perExpansion := map[expansion]int{}
	allowed := map[key]int{}
	for i, d := range mapped {
		if d.File == "" {
			continue
		}
		e := expansion{keyOf(d), i}
		if i < len(raw) {
			e.line = raw[i].Position.Line
		}
		perExpansion[e]++
		if perExpansion[e] > allowed[e.k] {
			allowed[e.k] = perExpansion[e]
		}
	}
	kept := map[key]int{}
	out := make([]diagnostics.Diagnostic, 0, len(mapped))
	for _, d := range mapped {
		if d.File != "" {
			k := keyOf(d)
			if kept[k] >= allowed[k] {
				continue
			}
			kept[k]++
		}
		out = append(out, d)
	}
	return out
}
