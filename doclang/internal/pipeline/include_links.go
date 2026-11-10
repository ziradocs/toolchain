// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"

	"go.ziradocs.com/doclang/v2/internal/generator"
)

// destRewrite es un destino inline de imagen tal como quedó después de que
// rebaseIncludedAssets lo reescribió, en el campo que empieza en la línea
// start (del contenido expandido).
type destRewrite struct {
	start int
	dest  string
}

// destRewrites lleva cada destino reescrito al que escribió el autor.
type destRewrites map[destRewrite]string

// linkFindingsDelta es lo que la reescritura de destinos de los archivos
// incluidos le cambia a LINK001 (y a IMG004, que también nombra el destino)
// en el AST que llega al lint: los hallazgos que esconde (hidden) y los que
// agrega (added), con la posición del contenido expandido.
type linkFindingsDelta struct {
	hidden, added []diagnostics.Diagnostic
}

// linkFindingsUnderRebase calcula, sobre doc tal como llega al lint (después
// de los transforms y los filtros), lo que le cambian los destinos que
// reescribió rebaseIncludedAssets, para que el lint diga lo mismo que diría
// del texto que escribió el autor.
//
// El rebase escribe entre <...> una ruta nueva con espacios o paréntesis. Si
// el autor escribió la ruta suelta con espacios (`![](a b.png)`), el linter ya
// no la ve y su aviso "has spaces outside <...>" se pierde, aunque en el
// documento sí sale; y una ruta que solo gana espacios por la carpeta no lo
// tiene que disparar. La regla de core (linter.LinkDestinationRule) corre
// sobre doc y sobre doc con cada destino reescrito que sigue tal cual vuelto
// al que escribió el autor; la diferencia es lo que cambió el rebase. Un
// destino que un filtro cambió o borró ya no se deshace, así que tampoco
// cuenta. Así el mensaje y la posición siguen siendo los de core. doc se
// deshace en el lugar y se deja como estaba (walkAssetFields recorre los
// mismos campos en el mismo orden). Sin destinos reescritos no corre nada.
func linkFindingsUnderRebase(doc *ast.AST, rewrites destRewrites) linkFindingsDelta {
	if doc == nil || len(rewrites) == 0 {
		return linkFindingsDelta{}
	}
	u := &rewriteUndoer{rewrites: rewrites}
	walkAssetFields(doc, u)
	rule := &linter.LinkDestinationRule{}
	var written []diagnostics.Diagnostic
	func() {
		defer walkAssetFields(doc, &fieldRestorer{saved: u.saved})
		if u.changed {
			written = rule.Check(doc)
		}
	}()
	if !u.changed {
		return linkFindingsDelta{}
	}
	rewritten := rule.Check(doc)
	return linkFindingsDelta{hidden: subtractDiagnostics(written, rewritten), added: subtractDiagnostics(rewritten, written)}
}

// rewriteUndoer vuelve cada destino reescrito que sigue tal cual al que
// escribió el autor, y guarda en orden el valor de cada campo inline que
// recorre para que fieldRestorer lo ponga de vuelta.
type rewriteUndoer struct {
	rewrites destRewrites
	saved    []string
	changed  bool
}

func (u *rewriteUndoer) inlineField(start int) func(string) string {
	return func(text string) string {
		u.saved = append(u.saved, text)
		return generator.RewriteInlineImageDestinations(text, func(dest string) string {
			if written, ok := u.rewrites[destRewrite{start: start, dest: dest}]; ok {
				u.changed = true
				return written
			}
			return dest
		})
	}
}

func (u *rewriteUndoer) sourceField(int) func(string) string {
	return func(s string) string { return s }
}

func (u *rewriteUndoer) headingHTML(html string, _ int) string { return html }

// fieldRestorer pone en cada campo inline, en el orden de walkAssetFields,
// el valor que guardó rewriteUndoer.
type fieldRestorer struct {
	saved []string
	next  int
}

func (r *fieldRestorer) inlineField(int) func(string) string {
	return func(text string) string {
		if r.next >= len(r.saved) {
			return text
		}
		r.next++
		return r.saved[r.next-1]
	}
}

func (r *fieldRestorer) sourceField(int) func(string) string {
	return func(s string) string { return s }
}

func (r *fieldRestorer) headingHTML(html string, _ int) string { return html }

// apply devuelve lintDiags (lo que dio el linter sobre el AST ya
// rebaseado, en la misma forma) sin los hallazgos que agregó el rebase y con
// los que escondió.
func (d linkFindingsDelta) apply(lintDiags []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	if len(d.hidden) == 0 && len(d.added) == 0 {
		return lintDiags
	}
	return append(subtractDiagnostics(lintDiags, d.added), d.hidden...)
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
