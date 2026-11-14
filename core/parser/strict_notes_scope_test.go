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

// En strict, el cuerpo de un `@notes` de varias líneas termina antes de un
// elemento hermano escrito a la sangría de `@notes` (o a menos), además de en
// los terminadores de siempre (línea en blanco, `@`, `---`, `#`). Antes se
// llevaba los elementos que le seguían como texto de las notas. Una línea más
// sangrada que `@notes` sigue siendo nota, y la prosa a su sangría también.

// notesScopeShape describe los elementos del primer contenedor: el tipo de
// cada uno y, para una directiva @notes, su contenido entre corchetes.
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

func TestStrictNotesEndBeforeSiblingElements(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []string
		want string
		// slideOnly: SECTION "…" solo es un elemento dentro de un SLIDE.
		slideOnly bool
	}{
		{
			name: "body indented under @notes, siblings at its indentation",
			body: []string{"  @notes", "    uno", "    dos", "  TEXT", "    Hermano", "  POINTS", "    - a"},
			want: "@notes[uno|dos] text points",
		},
		{
			name: "@notes: with its body at its own indentation",
			body: []string{"  @notes:", "  uno", "  - punto", "  clave: valor", "  TEXT", "    Hermano"},
			want: "@notes[uno|- punto|clave: valor] text",
		},
		{
			name: "sibling indented less than @notes",
			body: []string{"    @notes", "      uno", "  TEXT", "    Hermano"},
			want: "@notes[uno] text",
		},
		{
			name: "prose at the @notes indentation after an indented body is still a note",
			body: []string{"  @notes", "    uno", "  dos", "  CHECKLIST", "    - [ ] a"},
			want: "@notes[uno|dos] checklist",
		},
		{
			name: "a line deeper than @notes is a note even if it looks like an element",
			body: []string{"  @notes", "    TEXT", "    POINTS", "  TEXT", "    Hermano"},
			want: "@notes[TEXT|POINTS] text",
		},
		{
			name: "code-group and grid after the notes",
			body: []string{"  @notes", "    uno", "  :::code-group", "  ```js [a]", "  x()", "  ```", "  :::", "  <<grid>>", "  <<column>>", "    TEXT", "      En columna", "  <<end>>"},
			want: "@notes[uno] code_group grid()",
		},
		{
			name: "callout, table, image, quote, code, diagram after the notes",
			body: []string{"  @notes", "    uno", "  :::note", "    Ojo", "  :::", "  @notes", "    dos", "  | A | B |", "  | --- | --- |", "  | 1 | 2 |", "  @notes", "    tres", `  IMAGE "foto.png" "alt"`, "  @notes", "    cuatro", "  QUOTE", "    Cita", "  @notes", "    cinco", "  CODE python", "    x = 1", "  @notes", "    seis", "  <<mermaid>>", "    graph TD", "    A --> B"},
			want: "@notes[uno] special_block @notes[dos] table @notes[tres] image @notes[cuatro] quote @notes[cinco] code @notes[seis] mermaid",
		},
		{
			name: "slide heading after the notes",
			body: []string{"  @notes", "    uno", `  SECTION "Sub"`, "  TEXT", "    Hermano"},
			want: "@notes[uno] text text", slideOnly: true,
		},
		{
			name: "a word that only starts with a keyword is a note",
			body: []string{"  @notes", "    uno", "  TEXTO de apoyo", "  TEXT hola", "  TEXT", "    Hermano"},
			want: "@notes[uno|TEXTO de apoyo|TEXT hola] text",
		},
		{
			name: "typed column",
			body: []string{"  <<grid>>", "  <<column typed>>", "    @notes", "      uno", "    TEXT", "      En columna", "  <<end>>"},
			want: "grid(@notes[uno] text)",
		},
	} {
		for _, slide := range []bool{true, false} {
			if tc.slideOnly && !slide {
				continue
			}
			name := tc.name + "/section"
			header := "SECTION \"S\"\n"
			if slide {
				name = tc.name + "/slide"
				header = "SLIDE content\n  title: \"S\"\n"
			}
			t.Run(name, func(t *testing.T) {
				src := noDropDocHeader + "\n" + header + strings.Join(tc.body, "\n") + "\n"
				doc, diags := parseNotesScope(t, src, slide)
				for _, d := range diags {
					if d.IsError() {
						t.Errorf("unexpected error: %s %s", d.RuleID, d.Message)
					}
				}
				if doc == nil || len(doc.ContentBlocks) != 1 {
					t.Fatalf("want one block, got %+v\n%s", doc, src)
				}
				if got := notesScopeShape(doc.ContentBlocks[0].Elements); got != tc.want {
					t.Errorf("elements:\n got %s\nwant %s\n%s", got, tc.want, src)
				}
			})
		}
	}
}

// Cuando lo que dejaría de ser nota no se lee limpio como elementos, o las
// notas empiezan después de una línea en blanco, el cuerpo queda como siempre
// y DIRECTIVE005 avisa desde la línea de @notes hasta la última nota, citando
// la línea del hermano. El aviso no sale cuando el hermano sí sale.
func TestStrictNotesWarnWhenTheyKeepASibling(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []string
		want string
		// quote es la línea citada; endLine, la última línea del aviso
		// (0-based dentro de body).
		quote   string
		endLine int
		reason  string
	}{
		{
			name: "unclosed callout", body: []string{"  @notes", "    uno", "  :::note", "  Ojo"},
			want: "@notes[uno|:::note|Ojo]", quote: `":::note"`, endLine: 3, reason: "would give a diagnostic",
		},
		{
			name: "prose after the sibling", body: []string{"  @notes", "    uno", "  TEXT", "    Hermano", "  clave: valor"},
			want: "@notes[uno|TEXT|Hermano|clave: valor]", quote: `"TEXT"`, endLine: 4, reason: "would give a diagnostic",
		},
		{
			name: "ragged table", body: []string{"  @notes", "    uno", "  | a | b |", "  | c |"},
			want: "@notes[uno|| a | b ||| c |]", quote: `"| a | b |"`, endLine: 3, reason: "would give a diagnostic",
		},
		{
			name: "image without source", body: []string{"  @notes", "    uno", "  IMAGE"},
			want: "@notes[uno|IMAGE]", quote: `"IMAGE"`, endLine: 2, reason: "would give a diagnostic",
		},
		{
			name: "blank line before the body", body: []string{"  @notes", "", "    uno", "  TEXT"},
			want: "@notes[uno|TEXT]", quote: `"TEXT"`, endLine: 3, reason: "start after a blank line",
		},
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
				first := strings.Count(prefix, "\n") + 1 // línea de @notes
				src := prefix + strings.Join(tc.body, "\n") + "\n"
				doc, diags := parseNotesScope(t, src, slide)
				if doc == nil || len(doc.ContentBlocks) != 1 {
					t.Fatalf("want one block, got %+v", doc)
				}
				got := notesScopeShape(doc.ContentBlocks[0].Elements)
				// Con la línea en blanco, el conteo de siempre vuelve a leer
				// la última nota después de @notes.
				if !strings.HasPrefix(got, tc.want) {
					t.Errorf("elements:\n got %s\nwant %s…", got, tc.want)
				}
				var warn []diagnostics.Diagnostic
				for _, d := range diags {
					if d.RuleID == "DIRECTIVE005" {
						warn = append(warn, d)
					}
				}
				if len(warn) != 1 {
					t.Fatalf("want one DIRECTIVE005, got %+v", diags)
				}
				w := warn[0]
				if w.Severity != diagnostics.Warning || w.Position.Line != first || w.EndPosition == nil ||
					w.EndPosition.Line != first+tc.endLine || !strings.Contains(w.Message, tc.quote) || !strings.Contains(w.Message, tc.reason) {
					t.Errorf("DIRECTIVE005: %+v (end %+v), want lines %d-%d quoting %s", w, w.EndPosition, first, first+tc.endLine, tc.quote)
				}
			})
		}
	}
	// Sin aviso cuando el hermano sale.
	src := noDropDocHeader + "\nSECTION \"S\"\n  @notes\n    uno\n  TEXT\n    Hermano\n"
	_, diags := parseNotesScope(t, src, false)
	for _, d := range diags {
		if d.RuleID == "DIRECTIVE005" {
			t.Errorf("unexpected DIRECTIVE005: %s", d.Message)
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

// Lo que sigue igual: el cuerpo de un `@notes` termina en la línea en blanco y
// en los terminadores de siempre, una línea sin la sangría del contenedor sigue
// siendo nota (sacarla la dejaría sin dueño y, en un SECTION, sería un error),
// y la forma de una línea no cambia.
func TestStrictNotesKeepTheirLegacyBody(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []string
		want string
	}{
		{"one-line form", []string{`  @notes "Recordar"`, "  TEXT", "    Hermano"}, "@notes[Recordar] text"},
		{"blank line ends the body", []string{"  @notes", "    uno", "", "  dos"}, "@notes[uno]"},
		{"another directive ends the body", []string{"  @notes", "    uno", "  @center", "  TEXT", "    Hermano"}, "@notes[uno] @center[] text"},
		{"a line without the container's indentation is a note", []string{"  @notes", "    uno", "TEXT", "  POINTS", "    - a"}, "@notes[uno|TEXT] points"},
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
				// En "blank line ends the body", "dos" no es de nadie: lo
				// reporta el contenedor, como antes.
				if got := notesScopeShape(doc.ContentBlocks[0].Elements); got != tc.want {
					t.Errorf("elements:\n got %s\nwant %s", got, tc.want)
				}
			})
		}
	}
}

// Flex no cambia: el cuerpo de `@notes` sigue terminando solo en los
// terminadores de siempre.
func TestFlexNotesBodyUnchanged(t *testing.T) {
	src := "---\nmode: flex\ntitle: \"T\"\n---\n\n## Slide\n\n@notes:\n- uno\n- dos\n\nTexto\n"
	doc, _ := New(util.NewNoop()).Parse(src, "notes.slidelang")
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
}

// La prueba de lo que deja de ser nota (notesTailReadsClean) lee solo hasta
// donde terminaba el cuerpo de @notes: muchos @notes seguidos de un bloque sin
// cerrar no la hacen leer hasta el final del contenedor cada vez.
func TestStrictNotesScopeScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	for name, unit := range map[string]string{
		"unclosed callout": "  @notes\n    nota\n  :::note\n",
		"bare TEXT":        "  @notes\n    nota\n  TEXT\n",
		"unclosed diagram": "  @notes\n    nota\n  <<mermaid>>\n",
		"sibling TEXT":     "  @notes\n    nota\n  TEXT\n    Hermano\n",
	} {
		t.Run(name, func(t *testing.T) {
			assertLinear(t, func(n int) string {
				return noDropDocHeader + "SLIDE content\n  title: \"T\"\n" + strings.Repeat(unit, n)
			}, true)
		})
	}
}

// La prueba que decide si el hermano sale lee los encabezados con una copia de
// los anchors: un SECTION "…" que sale de las notas tiene el mismo anchor que
// sin las notas, y el siguiente encabezado igual no se corre.
func TestStrictNotesScopeKeepsHeadingAnchors(t *testing.T) {
	body := "  SECTION \"Sub\"\n  TEXT\n    Hermano\n  SECTION \"Sub\"\n  SECTION \"Otro\"\n    id: propio\n"
	with := noDropDocHeader + "\nSLIDE content\n  title: \"S\"\n  @notes\n    uno\n" + body
	without := noDropDocHeader + "\nSLIDE content\n  title: \"S\"\n" + body
	headings := func(src string) []string {
		doc, diags := parseNotesScope(t, src, true)
		for _, d := range diags {
			if d.IsError() {
				t.Errorf("unexpected error: %s", d.Message)
			}
		}
		var out []string
		for _, el := range doc.ContentBlocks[0].Elements {
			if te, ok := el.(*ast.TextElement); ok && strings.Contains(te.Content, "<h") {
				out = append(out, te.Content)
			}
		}
		return out
	}
	got, want := headings(with), headings(without)
	if len(want) != 3 || strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("headings with the notes:\n%v\nwithout:\n%v", got, want)
	}
}
