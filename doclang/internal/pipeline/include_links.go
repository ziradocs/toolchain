// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/linter"
)

// linkFindingsDelta es lo que el rebase de las imágenes de los archivos
// incluidos le cambió a LINK001: los hallazgos que escondió (hidden) y los
// que agregó (added), los dos con la posición del contenido expandido.
type linkFindingsDelta struct {
	hidden, added []diagnostics.Diagnostic
}

// rebaseIncludedAssetsKeepingLinks es rebaseIncludedAssets más lo que eso
// le cambia a LINK001 (core/linter.LinkDestinationRule), para que el lint
// diga lo mismo que diría del texto que escribió el autor.
//
// El rebase escribe entre <...> una ruta nueva con espacios o paréntesis.
// Si el autor escribió la ruta suelta con espacios (`![](a b.png)`), el
// linter ya no ve la ruta suelta y su aviso "has spaces outside <...>" se
// pierde, aunque en el documento sí sale. Al revés, una ruta que solo gana
// espacios por la carpeta del archivo incluido no lo tiene que disparar. La
// regla corre sobre el AST justo antes y justo después del rebase, que solo
// cambia destinos de imagen, así que la diferencia es exactamente lo que el
// rebase cambió; los transforms y filtros que corren después no se mezclan.
// Correr la regla sobre el texto anterior al rebase en lugar de reconstruir
// el aviso en doclang deja el mensaje y la posición en manos de core.
func rebaseIncludedAssetsKeepingLinks(doc *ast.AST, lines []string, origins include.LineOrigins, sourceDir string) linkFindingsDelta {
	if doc == nil || sourceDir == "" || !hasIncludedLines(origins) {
		rebaseIncludedAssets(doc, lines, origins, sourceDir)
		return linkFindingsDelta{}
	}
	rule := &linter.LinkDestinationRule{}
	before := rule.Check(doc)
	rebaseIncludedAssets(doc, lines, origins, sourceDir)
	after := rule.Check(doc)
	return linkFindingsDelta{hidden: subtractDiagnostics(before, after), added: subtractDiagnostics(after, before)}
}

// apply devuelve lintDiags (lo que dio el linter sobre el AST ya
// rebaseado, en la misma forma) sin los hallazgos que agregó el rebase y con
// los que escondió.
func (d linkFindingsDelta) apply(lintDiags []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	if len(d.hidden) == 0 && len(d.added) == 0 {
		return lintDiags
	}
	return append(subtractDiagnostics(lintDiags, d.added), d.hidden...)
}

// hasIncludedLines dice si alguna línea vino de un archivo incluido.
func hasIncludedLines(origins include.LineOrigins) bool {
	for _, o := range origins {
		if o != "" {
			return true
		}
	}
	return false
}

// subtractDiagnostics es a menos b como multiconjuntos: de cada diagnóstico
// de b se quita una copia idéntica de a. El orden de a se conserva.
func subtractDiagnostics(a, b []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	if len(b) == 0 {
		return a
	}
	pending := map[diagnosticKey]int{}
	for _, d := range b {
		pending[keyOfDiagnostic(d)]++
	}
	out := make([]diagnostics.Diagnostic, 0, len(a))
	for _, d := range a {
		if k := keyOfDiagnostic(d); pending[k] > 0 {
			pending[k]--
			continue
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
