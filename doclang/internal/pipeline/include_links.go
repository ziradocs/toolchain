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

// destRewrites son los destinos inline que reescribió rebaseIncludedAssets.
// Un mismo archivo escrito de dos formas en un campo (`a b.png` y
// `<a b.png>`, o `a%20b.png`) se reescribe al mismo destino nuevo, así que
// cada destino nuevo lleva, en orden, el escrito de cada una de sus
// ocurrencias: el mismo orden en que walkAssetFields las vuelve a recorrer.
// relevant dice si alguno puede cambiar lo que dicen LINK001 o IMG004 (ver
// record); si ninguno, linkFindingsUnderRebase no corre nada.
type destRewrites struct {
	queues   map[destRewrite][]string
	relevant bool
}

// record anota que la ocurrencia escrita written quedó como key. relevant
// es si puede cambiar lo que dice el lint: una ruta suelta con espacios, que
// avisa en la fuente y no una vez envuelta, o el destino de una imagen con
// un bloque {…}, cuyo IMG004 nombra la ruta. Una ruta sin espacios o ya
// escrita entre <...> dice lo mismo antes y después; se anota igual, para
// que las ocurrencias de un mismo destino nuevo se deshagan cada una con la
// suya, pero sola no hace correr la regla.
func (w *destRewrites) record(key destRewrite, written string, relevant bool) {
	if w.queues == nil {
		w.queues = map[destRewrite][]string{}
	}
	w.queues[key] = append(w.queues[key], written)
	w.relevant = w.relevant || relevant
}

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
	if doc == nil || !rewrites.relevant {
		return linkFindingsDelta{}
	}
	u := newRewriteUndoer(doc, rewrites)
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

// rewriteUndoer vuelve cada ocurrencia de un destino reescrito que sigue
// tal cual a la que escribió el autor, sacándolas de su cola en orden, y
// guarda en orden el valor de cada campo inline que recorre para que
// fieldRestorer lo ponga de vuelta.
//
// Un destino nuevo cuyas ocurrencias se escribieron de formas distintas solo
// se deshace si en doc quedan tantas ocurrencias como se reescribieron: si un
// filtro borró o agregó alguna, ya no se sabe cuál es cuál y se deja como
// está (dice lo que diga el AST reescrito). Si todas se escribieron igual, da
// lo mismo cuál se saca, y se deshacen las que alcancen.
type rewriteUndoer struct {
	queues  map[destRewrite][]string
	trusted map[destRewrite]bool
	next    map[destRewrite]int
	saved   []string
	changed bool
}

func newRewriteUndoer(doc *ast.AST, rewrites destRewrites) *rewriteUndoer {
	c := &occurrenceCounter{queues: rewrites.queues, counts: map[destRewrite]int{}}
	walkAssetFields(doc, c)
	trusted := map[destRewrite]bool{}
	for key, q := range rewrites.queues {
		trusted[key] = c.counts[key] == len(q) || allEqual(q)
	}
	return &rewriteUndoer{queues: rewrites.queues, trusted: trusted, next: map[destRewrite]int{}}
}

func (u *rewriteUndoer) inlineField(start int) func(string) string {
	return func(text string) string {
		u.saved = append(u.saved, text)
		return generator.RewriteInlineImageDestinations(text, func(dest string) string {
			key := destRewrite{start: start, dest: dest}
			q := u.queues[key]
			if !u.trusted[key] || u.next[key] >= len(q) {
				return dest
			}
			written := q[u.next[key]]
			u.next[key]++
			if written != dest {
				u.changed = true
			}
			return written
		})
	}
}

// occurrenceCounter cuenta, sin cambiar nada, cuántas ocurrencias de cada
// destino reescrito quedan en doc.
type occurrenceCounter struct {
	queues map[destRewrite][]string
	counts map[destRewrite]int
}

func (c *occurrenceCounter) inlineField(start int) func(string) string {
	return func(text string) string {
		generator.RewriteInlineImageDestinations(text, func(dest string) string {
			if key := (destRewrite{start: start, dest: dest}); c.queues[key] != nil {
				c.counts[key]++
			}
			return dest
		})
		return text
	}
}

func (c *occurrenceCounter) sourceField(int) func(string) string {
	return func(s string) string { return s }
}

func (c *occurrenceCounter) headingHTML(html string, _ int) string { return html }

// allEqual dice si todos los elementos de q son iguales.
func allEqual(q []string) bool {
	for _, s := range q[1:] {
		if s != q[0] {
			return false
		}
	}
	return true
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
