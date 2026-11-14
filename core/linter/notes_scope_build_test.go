// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// En strict, el cuerpo de un `@notes` de varias líneas se lee como siempre,
// salvo que un SLIDE o SECTION en la columna 0 lo termina, y avisa
// (DIRECTIVE005) cuando toma a un elemento hermano. Ningún documento que
// compilaba puede dejar de compilar. El diferencial de abajo corre el build
// como el CLI (buildErrors) sobre un generador de @notes seguidos de hermanos
// de todo tipo, también líneas sueltas que solo abren un elemento (`TEXT`,
// `TABLE`, `:::note` sin cierre), para compararlo con la base.

// notesScopeSiblings son los hermanos que pueden seguir a las notas, con la
// sangría relativa de cada línea. Los que llevan slideOnly existen solo en un
// SLIDE; noColumn, fuera de una columna tipada (un grid no se anida).
var notesScopeSiblings = []struct {
	lines     []string
	slideOnly bool
	noColumn  bool
}{
	{lines: nil},
	{lines: []string{"TEXT", "  Hermano"}},
	{lines: []string{"POINTS", "  - a", "  - b"}},
	{lines: []string{"CHECKLIST", "  - [ ] a"}},
	{lines: []string{"QUOTE", "  Cita", "  AUTHOR: Ana"}},
	{lines: []string{"CODE python", "  x = 1"}},
	{lines: []string{`IMAGE "foto.png" "alt"`}},
	{lines: []string{"TABLE", "  headers: [A, B]", "  rows:", "    [1, 2]"}},
	{lines: []string{"| A | B |", "| --- | --- |", "| 1 | 2 |"}},
	{lines: []string{":::note", "  Ojo", ":::"}},
	{lines: []string{"<<mermaid>>", "  graph TD", "  A --> B"}},
	{lines: []string{"<<grid>>", "<<column>>", "  TEXT", "    Col", "<<end>>"}, noColumn: true},
	{lines: []string{`SECTION "Sub"`}, slideOnly: true},
	{lines: []string{"@center"}},
	// Líneas que solo abren un elemento, sin el resto.
	{lines: []string{"TEXT"}},
	{lines: []string{"POINTS"}},
	{lines: []string{"CHECKLIST"}},
	{lines: []string{"QUOTE"}},
	{lines: []string{"TABLE"}},
	{lines: []string{"TABLE:"}},
	{lines: []string{"CODE"}},
	{lines: []string{"CODE python main.py"}},
	{lines: []string{"IMAGE foto.png"}},
	{lines: []string{":::note"}},
	{lines: []string{":::details Más"}},
	{lines: []string{"<<chart:bar>>"}},
	{lines: []string{"<<mermaid>>"}},
	{lines: []string{"<<math>>"}},
	{lines: []string{"<<quiz>>"}},
	{lines: []string{"<<grid>>"}, noColumn: true},
	{lines: []string{"| a |"}},
	{lines: []string{"| a | b |", "| c |"}},
	// Hermanos que el parser lee sin aviso y que caen en una regla del
	// linter que es error.
	{lines: []string{"IMAGE"}},
	{lines: []string{"IMAGE:"}},
	{lines: []string{":::code-group", ":::"}},
	{lines: []string{`:::code-item{title="a"}`, "  x", ":::"}},
	{lines: []string{"<<quiz>>", "  question: ¿Qué?", "  options: [a, b]", "<<end>>"}},
	{lines: []string{"<<quiz>>", "  question: ¿Qué?", "  options: [a, b]", "  answer: 0", "  results: [50]", "<<end>>"}},
	{lines: []string{"<<poll>>", "  question: ¿Qué?", "  options: [a, b]", "  results: [50]", "<<end>>"}},
	{lines: []string{"<<poll>>", "  question: ¿Qué?", "  options: [a, b]", "  results: [50, 150]", "<<end>>"}},
	{lines: []string{"TABLE", "  headers: [A, B]", "  rows:", "    [1]"}},
	{lines: []string{"| A | B |", "| --- | --- |", "| 1 |"}},
	// Hermanos con una línea en blanco adentro: cruzan el final de antes de
	// las notas.
	{lines: []string{"CODE python", "  a = 1", "", "  b = 2"}},
	{lines: []string{":::note", "  Uno", "", "  Dos", ":::"}},
	{lines: []string{"<<grid>>", "<<column>>", "  TEXT", "    A", "", "<<column>>", "  TEXT", "    B", "<<end>>"}, noColumn: true},
	{lines: []string{"TEXT", "  Uno", "", "  Dos"}},
	{lines: []string{"POINTS", "  - a", "", "  - b"}},
	{lines: []string{"QUOTE", "  Uno", "", "  Dos"}},
	{lines: []string{"<<mermaid>>", "  graph TD", "", "  A --> B"}},
	{lines: []string{":::note", "  Uno", ""}},
}

// notesScopeBody son las líneas que puede llevar el cuerpo de las notas.
var notesScopeBody = []string{
	"nota uno", "- punto", "clave: valor", "Tip: algo", "TEXTO extra", "TEXT hola",
	"1. paso", "| a | b |", "x", "title: \"Otro\"", "**negrita**", "POINTS de apoyo",
}

type notesScopeDoc struct {
	src      string
	document bool
}

// genNotesScopeDocs arma n documentos strict con semilla fija: un SLIDE o un
// SECTION (o una columna tipada dentro de uno), con uno o dos @notes cuyo
// cuerpo va más sangrado, a la sangría de @notes o en la columna 0, seguidos
// de un hermano a la sangría de @notes o a menos, con o sin línea en blanco
// en medio, y a veces de prosa después.
func genNotesScopeDocs(seed int64, n int) []notesScopeDoc {
	rng := rand.New(rand.NewSource(seed))
	pick := func(xs []string) string { return xs[rng.Intn(len(xs))] }
	var out []notesScopeDoc
	for len(out) < n {
		document := rng.Intn(2) == 0
		column := rng.Intn(4) == 0
		var b strings.Builder
		b.WriteString("---\nmode: strict\ntitle: \"T\"\n---\n\n")
		if document {
			b.WriteString("SECTION \"S\"\n")
		} else {
			b.WriteString("SLIDE content\n  title: \"S\"\n")
		}
		base := 2
		if column {
			b.WriteString("  <<grid>>\n  <<column typed>>\n")
			base = 4
		}
		for seg := 1 + rng.Intn(2); seg > 0; seg-- {
			// Las notas van a la sangría del cuerpo o, a veces, más adentro.
			notes := base
			if rng.Intn(4) == 0 {
				notes = base + 2
			}
			pad := func(k int) string {
				if k < 0 {
					k = 0
				}
				return strings.Repeat(" ", k)
			}
			b.WriteString(pad(notes) + pick([]string{"@notes", "@notes:"}) + "\n")
			if rng.Intn(5) == 0 {
				b.WriteString("\n")
			}
			for k := rng.Intn(4); k > 0; k-- {
				indent := []int{notes + 2, notes + 2, notes, notes + 4, 0}[rng.Intn(5)]
				b.WriteString(pad(indent) + pick(notesScopeBody) + "\n")
			}
			if rng.Intn(4) == 0 {
				b.WriteString("\n")
			}
			var sib []string
			for {
				s := notesScopeSiblings[rng.Intn(len(notesScopeSiblings))]
				if s.slideOnly && document || s.noColumn && column {
					continue
				}
				sib = s.lines
				break
			}
			at := notes
			if notes > base && rng.Intn(2) == 0 {
				at = base
			}
			for _, l := range sib {
				b.WriteString(pad(at) + l + "\n")
			}
			if rng.Intn(3) == 0 {
				b.WriteString(pad(at) + pick(notesScopeBody) + "\n")
			}
		}
		if column {
			b.WriteString("  <<end>>\n")
		}
		if rng.Intn(3) == 0 {
			// Otro bloque justo después, sin línea en blanco.
			if document {
				b.WriteString("SECTION \"Otra\"\n  TEXT\n    Fin\n")
			} else {
				b.WriteString("SLIDE content\n  title: \"Otra\"\n  TEXT\n    Fin\n")
			}
		}
		out = append(out, notesScopeDoc{src: b.String(), document: document})
	}
	return out
}

const notesScopeSeed, notesScopeCount = 525, 4000

// notesScopeSummary resume el AST de d para el diferencial: el hash del JSON,
// cuántos elementos hay en total y el texto de cada @notes.
func notesScopeSummary(d notesScopeDoc) string {
	p := parser.New(util.NewNoop())
	var doc *ast.AST
	if d.document {
		doc, _ = p.ParseDocument(d.src, "t.doclang")
	} else {
		doc, _ = p.Parse(d.src, "t.slidelang")
	}
	raw, _ := json.Marshal(doc)
	elements := 0
	var notes []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		if el, ok := n.(ast.Element); ok {
			elements++
			if dn, ok := el.(*ast.DirectiveNode); ok && dn.Name == "notes" {
				c, _ := dn.Parameters["content"].(string)
				notes = append(notes, c)
			}
		}
		return nil
	})
	return fmt.Sprintf("%x\t%d\t%q", sha256.Sum256(raw), elements, strings.Join(notes, "\x00"))
}

// TestStrictNotesScopeBuildDifferential, con NOTES_SCOPE_DUMP=<archivo>,
// escribe una línea por documento generado con su número y sus errores de
// build. Corriendo la misma prueba en la base y en la rama, ningún documento
// pasa de 0 errores a alguno.
func TestStrictNotesScopeBuildDifferential(t *testing.T) {
	path := os.Getenv("NOTES_SCOPE_DUMP")
	if path == "" {
		t.Skip("NOTES_SCOPE_DUMP not set")
	}
	var b strings.Builder
	for i, d := range genNotesScopeDocs(notesScopeSeed, notesScopeCount) {
		errs := buildErrors(d.src, d.document)
		fmt.Fprintf(&b, "%d\t%d\t%q\t%s\n", i, len(errs), strings.Join(errs, " | "), notesScopeSummary(d))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
