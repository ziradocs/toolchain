// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.ziradocs.com/core/v2/diagnostics"
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
		column := 1
		if n >= 1 && n <= len(lines) {
			column = len(lines[n-1]) - len(strings.TrimLeft(lines[n-1], " \t")) + 1
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

// DocumentDir es el directorio del documento inputFile en la misma forma que
// includeRoot, para PrepareOptions.DocDir: el contrato de
// include.SourceMap.MapDiagnostics pide que los dos estén en la misma forma
// canónica. Los archivos incluidos salen de filepath.Abs sobre includeRoot,
// así que si includeRoot ya tiene los symlinks resueltos (--base-dir,
// --include-root), docDir también; si no, los dos son el filepath.Abs de
// siempre. Vacío si inputFile está vacío (un documento sin ruta en disco).
func DocumentDir(inputFile, includeRoot string) string {
	if inputFile == "" {
		return ""
	}
	dir, err := filepath.Abs(filepath.Dir(inputFile))
	if err != nil {
		return ""
	}
	if includeRoot == "" {
		return dir
	}
	if real, err := filepath.EvalSymlinks(includeRoot); err == nil && real == filepath.Clean(includeRoot) {
		if realDir, err := filepath.EvalSymlinks(dir); err == nil {
			return realDir
		}
	}
	return dir
}
