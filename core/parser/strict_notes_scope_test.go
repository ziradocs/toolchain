// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// En strict, el cuerpo de un `@notes` de varias líneas se lee como siempre:
// hasta una línea en blanco o una que empieza con `@`, `---` o `#`. Si toma
// una línea a la sangría de `@notes` (o a menos) que abre un elemento bien
// escrito, avisa con DIRECTIVE005, igual que TEXT002 para un TEXT sin cuerpo.
// La única lectura nueva es que un SLIDE o SECTION en la columna 0 termina las
// notas.

// notesScopeShape describe los elementos del primer contenedor: el tipo de
// cada uno y, para una directiva, su contenido entre corchetes.
func notesScopeShape(elements []ast.Element) string {
	var parts []string
	for _, el := range elements {
		switch e := el.(type) {
		case *ast.DirectiveNode:
			content, _ := e.Parameters["content"].(string)
			parts = append(parts, fmt.Sprintf("@%s[%s]", e.Name, strings.ReplaceAll(content, "\n", "|")))
		case *ast.GridElement:
			var cols []string
			for _, c := range e.Columns {
				cols = append(cols, "("+notesScopeShape(c.Elements)+")")
			}
			parts = append(parts, "grid"+strings.Join(cols, ""))
		default:
			parts = append(parts, string(el.GetType()))
		}
	}
	return strings.Join(parts, " ")
}

func parseNotesScope(t *testing.T, src string, slide bool) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	p := New(util.NewNoop())
	if slide {
		return p.Parse(src, "notes.slidelang")
	}
	return p.ParseDocument(src, "notes.doclang")
}

func notesDirective005(diags []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	for _, d := range diags {
		if d.RuleID == "DIRECTIVE005" {
			out = append(out, d)
		}
	}
	return out
}

// Las notas se llevan al hermano como siempre, y DIRECTIVE005 lo dice: cubre
// desde @notes hasta la última nota, cita la línea del hermano y dice cómo
// arreglarlo.
func TestStrictNotesWarnWhenTheyTakeInASibling(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []string
		// want es el comienzo de la forma de los elementos: con una línea en
		// blanco antes del cuerpo, el conteo de siempre vuelve a leer la
		// última nota después de @notes.
		want string
		// quote es la línea citada; endLine, la última línea del aviso
		// (0-based dentro de body).
		quote     string
		endLine   int
		slideOnly bool
	}{
		{name: "TEXT and POINTS at the notes indentation", body: []string{"  @notes", "    uno", "    dos", "  TEXT", "    Hermano", "  POINTS", "    - a"},
			want: "@notes[uno|dos|TEXT|Hermano|POINTS|- a]", quote: `"TEXT"`, endLine: 6},
		{name: "@notes: with its body at its own indentation", body: []string{"  @notes:", "  uno", "  - punto", "  clave: valor", "  TEXT", "    Hermano"},
			want: "@notes[uno|- punto|clave: valor|TEXT|Hermano]", quote: `"TEXT"`, endLine: 5},
		{name: "sibling indented less than @notes", body: []string{"    @notes", "      uno", "  TEXT", "    Hermano"},
			want: "@notes[uno|TEXT|Hermano]", quote: `"TEXT"`, endLine: 3},
		{name: "code-group", body: []string{"  @notes", "    uno", "  :::code-group", "  ```js [a]", "  x()", "  ```", "  :::"},
			want: "@notes[uno|:::code-group|```js [a]|x()|```|:::]", quote: `":::code-group"`, endLine: 6},
		{name: "grid", body: []string{"  @notes", "    uno", "  <<grid>>", "  <<column>>", "    TEXT", "      Col", "  <<end>>"},
			want: "@notes[uno|<<grid>>|<<column>>|TEXT|Col|<<end>>]", quote: `"<<grid>>"`, endLine: 6},
		{name: "table row", body: []string{"  @notes", "    uno", "  | a | b |", "  | 1 | 2 |"},
			want: "@notes[uno|| a | b ||| 1 | 2 |]", quote: `"| a | b |"`, endLine: 3},
		{name: "image", body: []string{"  @notes", "    uno", `  IMAGE "foto.png" "alt"`},
			want: `@notes[uno|IMAGE "foto.png" "alt"]`, quote: `IMAGE \"foto.png\"`, endLine: 2},
		{name: "slide heading", body: []string{"  @notes", "    uno", `  SECTION "Sub"`},
			want: `@notes[uno|SECTION "Sub"]`, quote: `SECTION \"Sub\"`, endLine: 2, slideOnly: true},
		// TEXT y QUOTE leen el texto de su línea: `TEXT hola` es un TEXT.
		{name: "TEXT with text on its line", body: []string{"  @notes", "    uno", "  TEXT hola"},
			want: "@notes[uno|TEXT hola]", quote: `"TEXT hola"`, endLine: 2},
		{name: "blank line before the body", body: []string{"  @notes", "", "    uno", "  TEXT"},
			want: "@notes[uno|TEXT]", quote: `"TEXT"`, endLine: 3},
		{name: "typed column", body: []string{"  <<grid>>", "  <<column typed>>", "    @notes", "      uno", "    TEXT", "      En columna", "  <<end>>"},
			want: "grid(@notes[uno|TEXT|En columna])", quote: `"TEXT"`, endLine: 5},
	} {
		for _, slide := range []bool{true, false} {
			if tc.slideOnly && !slide {
				continue
			}
			header := "SECTION \"S\"\n"
			name := tc.name + "/section"
			if slide {
				header = "SLIDE content\n  title: \"S\"\n"
				name = tc.name + "/slide"
			}
			t.Run(name, func(t *testing.T) {
				prefix := noDropDocHeader + "\n" + header
				src := prefix + strings.Join(tc.body, "\n") + "\n"
				doc, diags := parseNotesScope(t, src, slide)
				for _, d := range diags {
					if d.IsError() {
						t.Errorf("unexpected error: %s", d.Message)
					}
				}
				if doc == nil || len(doc.ContentBlocks) != 1 {
					t.Fatalf("want one block, got %+v", doc)
				}
				if got := notesScopeShape(doc.ContentBlocks[0].Elements); !strings.HasPrefix(got, tc.want) {
					t.Errorf("elements:\n got %s\nwant %s…", got, tc.want)
				}
				first := strings.Count(prefix, "\n") + 1 // primera línea de body
				notesLine := first
				for i, l := range tc.body {
					if strings.Contains(l, "@notes") {
						notesLine = first + i
						break
					}
				}
				warn := notesDirective005(diags)
				if len(warn) != 1 {
					t.Fatalf("want one DIRECTIVE005, got %+v", diags)
				}
				w := warn[0]
				end := first + tc.endLine
				if w.Severity != diagnostics.Warning || w.Position.Line != notesLine || w.EndPosition == nil || w.EndPosition.Line != end ||
					!strings.Contains(w.Message, tc.quote) || !strings.Contains(w.Message, "indent them under @notes") ||
					!strings.Contains(w.Message, "leave a blank line") {
					t.Errorf("DIRECTIVE005: %+v (end %+v), want lines %d-%d quoting %s", w, w.EndPosition, notesLine, end, tc.quote)
				}
			})
		}
	}
}

// Sin aviso cuando el cuerpo no toma ningún elemento a su sangría: las notas
// sangradas bajo @notes (lo que escribe fmt) con una línea en blanco antes
// del hermano, prosa, palabras que solo empiezan con una palabra clave, un
// elemento más sangrado que @notes, y una línea sin la sangría del
// contenedor.
func TestStrictNotesStayQuietWithoutASibling(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []string
		want string
	}{
		{"indented notes, blank line, sibling", []string{"  @notes", "    uno", "", "  TEXT", "    Hermano"}, "@notes[uno] text"},
		{"prose and items at the notes indentation", []string{"  @notes:", "  uno", "  - punto", "  clave: valor"}, "@notes[uno|- punto|clave: valor]"},
		// `POINTS uno` no es un POINTS bien escrito: POINTS no lee el texto de
		// su línea. `TEXT hola` sí es un TEXT (ver el caso que avisa).
		{"words that only start with a keyword", []string{"  @notes", "    uno", "  TEXTO de apoyo", "  POINTS uno"}, "@notes[uno|TEXTO de apoyo|POINTS uno]"},
		{"element deeper than @notes", []string{"  @notes", "    TEXT", "    POINTS"}, "@notes[TEXT|POINTS]"},
		{"one-line form", []string{`  @notes "Recordar"`, "  TEXT", "    Hermano"}, "@notes[Recordar] text"},
		{"another directive ends the body", []string{"  @notes", "    uno", "  @center", "  TEXT", "    Hermano"}, "@notes[uno] @center[] text"},
		{"line without the container's indentation", []string{"  @notes", "    uno", "TEXT"}, "@notes[uno|TEXT]"},
	} {
		for _, slide := range []bool{true, false} {
			header := "SECTION \"S\"\n"
			name := tc.name + "/section"
			if slide {
				header = "SLIDE content\n  title: \"S\"\n"
				name = tc.name + "/slide"
			}
			t.Run(name, func(t *testing.T) {
				src := noDropDocHeader + "\n" + header + strings.Join(tc.body, "\n") + "\n"
				doc, diags := parseNotesScope(t, src, slide)
				if doc == nil || len(doc.ContentBlocks) != 1 {
					t.Fatalf("want one block, got %+v", doc)
				}
				if got := notesScopeShape(doc.ContentBlocks[0].Elements); got != tc.want {
					t.Errorf("elements:\n got %s\nwant %s", got, tc.want)
				}
				if w := notesDirective005(diags); len(w) != 0 {
					t.Errorf("unexpected DIRECTIVE005: %s", w[0].Message)
				}
			})
		}
	}
}

// Un SLIDE o un SECTION en la columna 0 termina las notas aunque no haya
// línea en blanco antes. Antes las notas se lo llevaban y el parser cortaba
// el elemento con un error interno.
func TestStrictNotesEndAtTheNextBlock(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   string
		slide bool
	}{
		{"slide", noDropDocHeader + "\nSLIDE content\n  title: \"A\"\n  @notes\n    uno\nSLIDE content\n  title: \"B\"\n  TEXT\n    Hermano\n", true},
		{"section", noDropDocHeader + "\nSECTION \"A\"\n  @notes\n    uno\nSECTION \"B\"\n  TEXT\n    Hermano\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, diags := parseNotesScope(t, tc.src, tc.slide)
			for _, d := range diags {
				if d.IsError() {
					t.Errorf("unexpected error: %s", d.Message)
				}
			}
			if doc == nil || len(doc.ContentBlocks) != 2 {
				t.Fatalf("want two blocks, got %+v", doc)
			}
			if got := notesScopeShape(doc.ContentBlocks[0].Elements); got != "@notes[uno]" {
				t.Errorf("first block: %s", got)
			}
			if got := notesScopeShape(doc.ContentBlocks[1].Elements); got != "text" {
				t.Errorf("second block: %s", got)
			}
		})
	}
}

// Flex no cambia: el cuerpo de `@notes` sigue terminando solo en los
// terminadores de siempre, y no avisa.
func TestFlexNotesBodyUnchanged(t *testing.T) {
	src := "---\nmode: flex\ntitle: \"T\"\n---\n\n## Slide\n\n@notes:\n- uno\n- dos\n\nTexto\n"
	doc, diags := New(util.NewNoop()).Parse(src, "notes.slidelang")
	if doc == nil || len(doc.ContentBlocks) == 0 {
		t.Fatalf("no blocks")
	}
	found := false
	for _, el := range doc.ContentBlocks[0].Elements {
		if d, ok := el.(*ast.DirectiveNode); ok && d.Name == "notes" {
			found = true
			if d.Parameters["content"] != "- uno\n- dos" {
				t.Errorf("notes: %q", d.Parameters["content"])
			}
		}
	}
	if !found {
		t.Errorf("no @notes in %s", notesScopeShape(doc.ContentBlocks[0].Elements))
	}
	if w := notesDirective005(diags); len(w) != 0 {
		t.Errorf("unexpected DIRECTIVE005 in flex: %s", w[0].Message)
	}
}

// Con líneas en blanco antes de la primera nota, el conteo es el de siempre:
// @notes y una línea por nota, sin esas líneas en blanco. Las últimas notas se
// vuelven a leer después de @notes, y lo que leen ahí queda en el AST como en
// la base (84e227e). Contar bien las líneas en blanco cambiaría el AST de esos
// documentos.
func TestStrictNotesAfterABlankLineKeepTheBaseCount(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []string
		want string
	}{
		{"one blank line", []string{"  @notes", "", "    uno", `  IMAGE "a.png" "alt"`}, `@notes[uno|IMAGE "a.png" "alt"] image`},
		{"two blank lines", []string{"  @notes", "", "", "    uno", "  TEXT", "    Hermano"}, "@notes[uno|TEXT|Hermano] text"},
		{"two blank lines, two siblings", []string{"  @notes", "", "", "    uno", `  IMAGE "a.png" "alt"`, `  IMAGE "b.png" "alt"`}, `@notes[uno|IMAGE "a.png" "alt"|IMAGE "b.png" "alt"] image image`},
		{"typed column", []string{"  <<grid>>", "  <<column typed>>", "    @notes", "", "", "      uno", "    TEXT", "      Hermano", "  <<end>>"}, "grid(@notes[uno|TEXT|Hermano] text)"},
	} {
		for _, slide := range []bool{true, false} {
			header := "SECTION \"S\"\n"
			name := tc.name + "/section"
			if slide {
				header = "SLIDE content\n  title: \"S\"\n"
				name = tc.name + "/slide"
			}
			t.Run(name, func(t *testing.T) {
				src := noDropDocHeader + "\n" + header + strings.Join(tc.body, "\n") + "\n"
				doc, _ := parseNotesScope(t, src, slide)
				if doc == nil || len(doc.ContentBlocks) != 1 {
					t.Fatalf("want one block, got %+v", doc)
				}
				if got := notesScopeShape(doc.ContentBlocks[0].Elements); got != tc.want {
					t.Errorf("elements:\n got %s\nwant %s\n%s", got, tc.want, src)
				}
			})
		}
	}
}

// DIRECTIVE005 termina en la última nota también cuando hay líneas en blanco
// antes de la primera: esas líneas no corren el final del aviso.
func TestStrictNotesWarningEndsAtTheLastNoteAfterBlankLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []string
		// last es el índice, en body, de la última nota.
		last int
	}{
		{"two blank lines", []string{"  @notes", "", "", "    uno", "  TEXT", "    Hermano"}, 5},
		{"three blank lines, sibling first", []string{"  @notes", "", "", "", "  TEXT", "    Hermano", "    sigue"}, 6},
		{"typed column", []string{"  <<grid>>", "  <<column typed>>", "    @notes", "", "", "      uno", "    TEXT", "  <<end>>"}, 6},
	} {
		for _, slide := range []bool{true, false} {
			header := "SECTION \"S\"\n"
			name := tc.name + "/section"
			if slide {
				header = "SLIDE content\n  title: \"S\"\n"
				name = tc.name + "/slide"
			}
			t.Run(name, func(t *testing.T) {
				prefix := noDropDocHeader + "\n" + header
				first := strings.Count(prefix, "\n") + 1
				src := prefix + strings.Join(tc.body, "\n") + "\n"
				_, diags := parseNotesScope(t, src, slide)
				warn := notesDirective005(diags)
				if len(warn) != 1 || warn[0].EndPosition == nil {
					t.Fatalf("want one DIRECTIVE005 with an end, got %+v", diags)
				}
				if got, want := warn[0].EndPosition.Line, first+tc.last; got != want {
					t.Errorf("DIRECTIVE005 ends on line %d, want %d (the last note)", got, want)
				}
			})
		}
	}
}
