// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package findings

import (
	"path/filepath"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/include"
)

// Llevar un diagnóstico al archivo y la línea que lo escribieron es
// include.SourceMap.MapDiagnostics, de core. Lo de este archivo es lo que
// doclang le agrega alrededor (doclang/internal/pipeline/include_diagnostics.go:
// DocumentDir, MapToSource y DedupeIncluded), con el mismo código: los dos
// CLIs solo comparten core, y esto tiene que compilar contra el core
// publicado, que todavía no lo tiene.

// DocumentDir es el directorio del documento inputFile para MapToSource:
// filepath.Abs de su directorio, sin resolver symlinks, en la misma forma que
// la ruta que el reporte pone en document.path (core une las dos con
// path.Join, léxico). Vacío si inputFile está vacío (un documento sin ruta en
// disco).
//
// A diferencia de doclang, slidelang no resuelve symlinks en la raíz de
// includes (filepath.Abs de --include-root o del directorio del documento),
// así que las rutas de los archivos incluidos ya están en la forma de este
// directorio y no hace falta reescribirlas.
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

// MapToSource lleva diags al archivo y la línea que los escribieron
// (include.SourceMap.MapDiagnostics) y quita las copias que dejó otra
// expansión del mismo archivo incluido (DedupeIncluded).
//
// Un diagnóstico de un archivo incluido dice ese archivo (Diagnostic.File,
// relativo a docDir y con "/") y su línea ahí; uno del documento queda sin
// File y con su línea en el documento. Una posición final en otro archivo que
// la inicial se descarta: un rango no cruza archivos.
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
// expansión también puede dar varios hallazgos idénticos, y esos se quedan
// todos, como en el documento. La expansión se distingue por la línea del
// contenido expandido (la de raw): una línea de un archivo incluido cae en
// una línea expandida distinta en cada expansión, también con includes
// anidados. De cada hallazgo quedan tantas copias como dio la expansión que
// más dio. Un hallazgo del documento (File vacío) no se toca.
func DedupeIncluded(raw, mapped []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	// Sin raw alineado con mapped no se sabe de qué expansión es cada
	// hallazgo: no se quita nada.
	if len(raw) != len(mapped) {
		return mapped
	}
	type expansion struct {
		k    diagnosticKey
		line int
	}
	perExpansion := map[expansion]int{}
	allowed := map[diagnosticKey]int{}
	for i, d := range mapped {
		if d.File == "" {
			continue
		}
		e := expansion{keyOfDiagnostic(d), raw[i].Position.Line}
		perExpansion[e]++
		if perExpansion[e] > allowed[e.k] {
			allowed[e.k] = perExpansion[e]
		}
	}
	kept := map[diagnosticKey]int{}
	out := make([]diagnostics.Diagnostic, 0, len(mapped))
	for _, d := range mapped {
		if d.File != "" {
			k := keyOfDiagnostic(d)
			if kept[k] >= allowed[k] {
				continue
			}
			kept[k]++
		}
		out = append(out, d)
	}
	return out
}

// diagnosticKey identifica un diagnóstico por todos sus campos, con
// EndPosition por valor.
type diagnosticKey struct {
	d      diagnostics.Diagnostic
	end    diagnostics.Position
	hasEnd bool
}

func keyOfDiagnostic(d diagnostics.Diagnostic) diagnosticKey {
	k := diagnosticKey{d: d}
	if d.EndPosition != nil {
		k.end, k.hasEnd = *d.EndPosition, true
	}
	k.d.EndPosition = nil
	return k
}
