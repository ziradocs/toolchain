// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/formatter"
	"go.ziradocs.com/core/v2/internal/anchors"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// documentAnchors devuelve, en orden de documento, el id de cada sección y
// de cada encabezado de nivel 2-6, también los de un bloque :::.
func documentAnchors(doc *ast.AST) []string {
	sections := renderer.DocumentSectionAnchors(doc, true)
	var out []string
	anchors.WalkDocument(doc,
		func(i int, _ string) { out = append(out, sections[i]) },
		func(el ast.Element) {
			id, _ := anchors.HeadingID(el)
			out = append(out, id)
		})
	return out
}

func parseForFormat(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %s\n%s", d.String(), src)
		}
	}
	return doc
}

func headingAnchorsOf(t *testing.T, src string) []string {
	t.Helper()
	return documentAnchors(parseForFormat(t, src))
}

var strictIDLine = regexp.MustCompile(`(?m)^\s*id: (\S+)$`)

// declaredIDs son los `id:` que una salida strict escribe, en orden.
func declaredIDs(out string) []string {
	var ids []string
	for _, m := range strictIDLine.FindAllStringSubmatch(out, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// formatTwice formatea src, reparsea, vuelve a formatear y exige que la
// segunda salida sea igual a la primera y que los ids no cambien en ninguna
// de las dos vueltas.
func formatTwice(t *testing.T, src string, format func(*ast.AST) (string, error)) string {
	t.Helper()
	want := headingAnchorsOf(t, src)
	out, err := format(parseForFormat(t, src))
	if err != nil {
		t.Fatalf("formatear: %v\n%s", err, src)
	}
	if got := headingAnchorsOf(t, out); !reflect.DeepEqual(got, want) {
		t.Fatalf("ids tras formatear = %v, want %v\nfuente:\n%s\nsalida:\n%s", got, want, src, out)
	}
	again, err := format(parseForFormat(t, out))
	if err != nil {
		t.Fatalf("segunda vuelta: %v\n%s", err, out)
	}
	if again != out {
		t.Fatalf("fmt no es idempotente:\nprimera:\n%s\nsegunda:\n%s", out, again)
	}
	return out
}

// Issue #267: el id de un encabezado repetido (notas-2) es el que el parser
// deriva en esa posición, así que el formatter no lo trata como declarado.
// Strict no escribe un `id:` por él, flex con typed-headings-v1 lo puede
// escribir, y el documento reparseado conserva los mismos ids.
func TestFormatDocumentRepeatedHeadingsRoundTrip(t *testing.T) {
	const legacy = "# A\n\n## Notas\n\n# B\n\n## Notas\n\n# 概要\n\n## 日本語\n"
	const typed = "---\nast_capabilities: [typed-headings-v1]\n---\n" + legacy
	want := []string{"a", "notas", "b", "notas-2", "seccion", "seccion-2"}

	for name, src := range map[string]string{"legado": legacy, "typed": typed} {
		t.Run(name, func(t *testing.T) {
			if got := headingAnchorsOf(t, src); !reflect.DeepEqual(got, want) {
				t.Fatalf("ids del original = %v, want %v", got, want)
			}
			strict := formatTwice(t, src, formatter.FormatDocumentStrict)
			if ids := declaredIDs(strict); len(ids) > 0 {
				t.Errorf("strict escribió id: derivables %v:\n%s", ids, strict)
			}
			formatTwice(t, src, formatter.FormatDocument)
		})
	}
}

// Lo que el autor escribió con `id:` se conserva donde lo escribió, nada
// más, y fmt no cambia los ids ni la salida en una segunda vuelta (revisión
// de #267, P2).
func TestFormatDocumentStrictKeepsAuthorIDsWhereWritten(t *testing.T) {
	const fm = "---\nmode: strict\n---\n\n"
	sec := func(title string, props ...string) string {
		s := "SECTION \"" + title + "\"\n"
		for _, p := range props {
			s += "  " + p + "\n"
		}
		return s + "\n"
	}
	for _, tc := range []struct {
		name     string
		src      string
		ids      []string
		declared []string
	}{
		{"id del autor después de un derivado",
			fm + sec("Doc") + sec("Notas", "level: 2") + sec("Otra", "level: 2", "id: notas"),
			[]string{"doc", "notas-2", "notas"}, []string{"notas"}},
		{"id del autor en el segundo de dos iguales",
			fm + sec("Doc") + sec("Notas", "level: 2") + sec("Notas", "level: 2", "id: notas"),
			[]string{"doc", "notas-2", "notas"}, []string{"notas"}},
		{"dos id del autor iguales",
			fm + sec("Doc") + sec("Notas", "level: 2", "id: notas") + sec("Notas", "level: 2", "id: notas"),
			[]string{"doc", "notas", "notas"}, []string{"notas", "notas"}},
		{"id del autor derivable",
			fm + sec("Doc") + sec("Notas", "level: 2", "id: notas"),
			[]string{"doc", "notas"}, nil},
		{"id del autor no derivable",
			fm + sec("Doc") + sec("Instalación", "level: 2", "id: install"),
			[]string{"doc", "install"}, []string{"install"}},
		{"id del autor contra una sección",
			fm + sec("Notas") + sec("Otra", "level: 2", "id: notas"),
			[]string{"notas-2", "notas"}, []string{"notas"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := headingAnchorsOf(t, tc.src); !reflect.DeepEqual(got, tc.ids) {
				t.Fatalf("ids del original = %v, want %v", got, tc.ids)
			}
			out := formatTwice(t, tc.src, formatter.FormatDocumentStrict)
			if got := declaredIDs(out); !reflect.DeepEqual(got, tc.declared) {
				t.Errorf("id: escritos = %v, want %v\n%s", got, tc.declared, out)
			}
		})
	}
}

// Un AST de antes de #267, que llega por JSON con dos `notas`, se formatea
// como en flex: los dos encabezados se vuelven a derivar y el reparseo los
// desduplica. Un anchor que no se deriva del texto sí se declara.
func TestFormatDocumentStrictDecodedPre267AST(t *testing.T) {
	old := parseForFormat(t, "# A\n\n## Notas\n\n# B\n\n## Notas\n\n## Instalación\n")
	// Así lo dejaba el CLI viejo: el segundo encabezado con el id del
	// primero, y uno con un id que no sale de su texto.
	retarget := func(el ast.Element, id string) {
		h := el.(*ast.TextElement)
		h.Content = strings.Replace(h.Content, `id="`+h.HeadingAnchor+`"`, `id="`+id+`"`, 1)
	}
	retarget(old.ContentBlocks[1].Elements[0], "notas")
	retarget(old.ContentBlocks[1].Elements[1], "install")
	raw, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ast.DecodeAST(raw)
	if err != nil {
		t.Fatal(err)
	}
	for name, format := range map[string]func(*ast.AST) (string, error){
		"strict": formatter.FormatDocumentStrict,
		"flex":   formatter.FormatDocument,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := format(decoded)
			if err != nil {
				t.Fatalf("formatear: %v", err)
			}
			want := []string{"a", "notas", "b", "notas-2", "instalacion"}
			if name == "strict" {
				want[4] = "install"
				if got := declaredIDs(out); !reflect.DeepEqual(got, []string{"install"}) {
					t.Errorf("id: escritos = %v\n%s", got, out)
				}
			}
			if got := headingAnchorsOf(t, out); !reflect.DeepEqual(got, want) {
				t.Errorf("ids tras formatear = %v, want %v\n%s", got, want, out)
			}
		})
	}
}

// Revisión de #267, P1: un bloque ::: con un encabezado adentro se relee
// aislado (con un `# x` de envoltorio), y ahí el anchor del encabezado sale
// de otra cuenta que la del documento. fmt no debe rechazarlo por eso, con o
// sin repetidos, legado o typed. En strict documental el cuerpo de un :::
// es texto (un `### X` adentro no es un encabezado), así que ahí se prueba
// que un bloque así junto a encabezados repetidos y `id:` del autor tampoco
// se rechace; transpilar a strict un bloque de flex con un encabezado
// adentro se rechaza igual que antes de #267.
func TestFormatDocumentSpecialBlockWithHeading(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"sin repetidos", "# Intro\n\n:::note\n### X\n\ncuerpo\n:::\n"},
		{"repetido con el de afuera", "# A\n\n## Notas\n\n:::note\n### Notas\n\ncuerpo\n:::\n"},
		{"igual al título del envoltorio", "# Doc\n\n:::note\n### X\n\ncuerpo\n:::\n\n## X\n"},
		{"dos bloques", "# A\n\n:::note\n### Notas\n\nuno\n:::\n\n:::tip\n### Notas\n\ndos\n:::\n"},
		{"sin letras", "# 概要\n\n:::note\n### 日本語\n\ncuerpo\n:::\n"},
	} {
		for _, typed := range []bool{false, true} {
			src := tc.src
			name := tc.name
			if typed {
				src = "---\nast_capabilities: [typed-headings-v1]\n---\n" + src
				name += " typed"
			}
			t.Run(name+" flex", func(t *testing.T) {
				out := formatTwice(t, src, formatter.FormatDocument)
				if !typed && out != tc.src {
					t.Errorf("flex cambió el documento:\n%s\nwant:\n%s", out, tc.src)
				}
			})
		}
	}
	for _, tc := range []struct {
		name string
		body string
	}{
		{"sin repetidos", "SECTION \"Intro\"\n  :::note\n  ### X\n\n  cuerpo\n  :::\n"},
		{"repetido con el de afuera", "SECTION \"A\"\n\nSECTION \"Notas\"\n  level: 2\n\n  :::note\n  ### Notas\n\n  cuerpo\n  :::\n"},
		{"repetido con un id del autor", "SECTION \"A\"\n\nSECTION \"Otra\"\n  level: 2\n  id: notas\n\n  :::note\n  ### Notas\n\n  cuerpo\n  :::\n"},
	} {
		for _, typed := range []bool{false, true} {
			src := "---\nmode: strict\n---\n\n" + tc.body
			name := tc.name
			if typed {
				src = "---\nmode: strict\nast_capabilities: [typed-headings-v1]\n---\n\n" + tc.body
				name += " typed"
			}
			t.Run(name+" strict", func(t *testing.T) {
				formatTwice(t, src, formatter.FormatDocumentStrict)
			})
		}
	}
}

// Transpilar a strict un bloque ::: de flex con un encabezado adentro se
// rechaza, como antes de #267: strict no lee un encabezado dentro de un
// bloque sin `typed`.
func TestFormatDocumentStrictRefusesFlexBlockHeading(t *testing.T) {
	_, err := formatter.FormatDocumentStrict(parseForFormat(t, "# Intro\n\n:::note\n### X\n\ncuerpo\n:::\n"))
	if err == nil || !strings.Contains(err.Error(), "special_block") {
		t.Fatalf("err = %v", err)
	}
}

// Ida y vuelta por fmt de documentos flex generados: preámbulo, secciones y
// encabezados de nivel 2-3, bloques ::: con un ### adentro y textos que
// chocan entre sí y con sus sufijos. fmt no debe fallar, no debe cambiar
// ningún id y una segunda vuelta no debe cambiar nada.
func TestFormatDocumentRoundTripGenerated(t *testing.T) {
	texts := []string{"A", "A 2", "A-2", "Notas", "Notas 2", "x", "概要"}
	seed := uint32(1500)
	next := func(n int) int {
		seed = seed*1664525 + 1013904223
		return int(seed>>16) % n
	}
	failures := 0
	for round := 0; round < 1500; round++ {
		var b strings.Builder
		typed := next(4) == 0
		if typed {
			b.WriteString("---\nast_capabilities: [typed-headings-v1]\n---\n")
		}
		if next(3) == 0 {
			b.WriteString("Preámbulo.\n\n")
			if next(2) == 0 {
				b.WriteString("## " + texts[next(len(texts))] + "\n\n")
			}
		}
		for i, n := 0, 1+next(7); i < n; i++ {
			text := texts[next(len(texts))]
			switch k := next(5); {
			case i == 0 || k == 0:
				b.WriteString("# " + text + "\n\n")
			case k == 1:
				b.WriteString("## " + text + "\n\n")
			case k == 2:
				b.WriteString("### " + text + "\n\n")
			case k == 3:
				b.WriteString(":::note\n### " + text + "\n\ncuerpo\n:::\n\n")
			default:
				b.WriteString("Texto.\n\n")
			}
		}
		src := b.String()
		ok := t.Run(fmt.Sprintf("doc %d", round), func(t *testing.T) {
			formatTwice(t, src, formatter.FormatDocument)
			if !strings.Contains(src, ":::") {
				formatTwice(t, src, formatter.FormatDocumentStrict)
			}
		})
		if !ok {
			failures++
			if failures > 5 {
				t.Fatalf("demasiadas fallas")
			}
		}
	}
}

// Lo mismo en strict: secciones, encabezados con y sin `id:` del autor
// (repetidos, derivables y no) y bloques ::: cuyo cuerpo trae un `###`.
func TestFormatDocumentStrictRoundTripGenerated(t *testing.T) {
	texts := []string{"A", "A 2", "Notas", "Notas 2", "x", "概要"}
	ids := []string{"a", "a-2", "notas", "notas-2", "otro"}
	seed := uint32(500)
	next := func(n int) int {
		seed = seed*1664525 + 1013904223
		return int(seed>>16) % n
	}
	failures := 0
	for round := 0; round < 500; round++ {
		var b strings.Builder
		b.WriteString("---\nmode: strict\n")
		if next(4) == 0 {
			b.WriteString("ast_capabilities: [typed-headings-v1]\n")
		}
		b.WriteString("---\n\n")
		for i, n := 0, 1+next(7); i < n; i++ {
			text := texts[next(len(texts))]
			level := 1
			if i > 0 {
				level = 1 + next(3)
			}
			fmt.Fprintf(&b, "SECTION %q\n", text)
			if level > 1 {
				fmt.Fprintf(&b, "  level: %d\n", level)
				if next(3) == 0 {
					fmt.Fprintf(&b, "  id: %s\n", ids[next(len(ids))])
				}
			}
			if next(3) == 0 {
				fmt.Fprintf(&b, "\n  :::note\n  ### %s\n\n  cuerpo\n  :::\n", texts[next(len(texts))])
			}
			b.WriteString("\n")
		}
		src := b.String()
		ok := t.Run(fmt.Sprintf("doc %d", round), func(t *testing.T) {
			formatTwice(t, src, formatter.FormatDocumentStrict)
		})
		if !ok {
			failures++
			if failures > 5 {
				t.Fatalf("demasiadas fallas")
			}
		}
	}
}
