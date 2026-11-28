// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/internal/anchors"
	"go.ziradocs.com/core/v2/internal/elements"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// documentIDs devuelve, en orden de documento, el id de cada encabezado que
// DocLang dibuja: el de cada sección (renderer.DocumentSectionAnchors) y el
// de cada encabezado de nivel 2-6 que el parser dejó en el AST.
func documentIDs(t *testing.T, src string) ([]string, *ast.AST) {
	t.Helper()
	doc, diags := New(util.NewNoop()).ParseDocument(src, "")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("parse error: %v", d)
		}
	}
	sections := renderer.DocumentSectionAnchors(doc, true)
	var ids []string
	anchors.WalkDocument(doc,
		func(i int, _ string) { ids = append(ids, sections[i]) },
		func(el ast.Element) {
			id, ok := anchors.HeadingID(el)
			if !ok {
				t.Fatalf("encabezado sin id: %#v", el)
			}
			ids = append(ids, id)
		})
	return ids, doc
}

// Issue #267: los ids de encabezado de un documento son únicos en orden de
// documento, contando las secciones (nivel 1) y los niveles 2-6. El primero
// conserva el id base, los siguientes llevan -2, -3, …
func TestDocumentHeadingIDsAreUnique(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want []string
	}{
		{"sección y subsección", "# Setup\n\nx\n\n## Setup\n\ny\n", []string{"setup", "setup-2"}},
		{"con título en el front matter", "---\ntitle: Manual\n---\n\n# Overview\n\nx\n\n# More\n\ny\n\n## Overview\n\nz\n",
			[]string{"overview", "more", "overview-2"}},
		{"tres iguales", "# Intro\n\nx\n\n## Intro\n\ny\n\n## Intro\n\nz\n", []string{"intro", "intro-2", "intro-3"}},
		{"secciones repetidas", "# Doc\n\nx\n\n## A\n\ny\n\n# Doc\n\nz\n\n## A\n\nw\n", []string{"doc", "a", "doc-2", "a-2"}},
		{"subsecciones en secciones distintas", "# A\n\nx\n\n## Notas\n\ny\n\n# B\n\n## Notas\n\nz\n", []string{"a", "notas", "b", "notas-2"}},
		{"sin caracteres para un id", "# 概要\n\nx\n\n## 日本語\n\ny\n", []string{"seccion", "seccion-2"}},
		// Regla de colisión (anchors.Set): el sufijo se prueba contra los
		// ids base reales, así que "Notas 2" (id base notas-2, ya tomado)
		// queda notas-2-2.
		{"sufijo contra un id base real", "# Doc\n\n## Notas\n\n## Notas\n\n## Notas 2\n", []string{"doc", "notas", "notas-2", "notas-2-2"}},
		{"id base real primero", "# Doc\n\n## Notas 2\n\n## Notas\n\n## Notas\n", []string{"doc", "notas-2", "notas", "notas-3"}},
		// Un encabezado dentro de un bloque ::: también se dibuja con id, así
		// que entra en la cuenta en su lugar.
		{"dentro de un bloque", "# Doc\n\n## Notas\n\n::: info\n### Notas\n\ntexto\n:::\n\n## Notas\n",
			[]string{"doc", "notas", "notas-2", "notas-3"}},
		{"sin repetidos", "# Uno\n\n## Dos\n\n### Tres\n\n# Cuatro\n", []string{"uno", "dos", "tres", "cuatro"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := documentIDs(t, tc.src)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ids = %v, want %v", got, tc.want)
			}
		})
	}
}

// El <hN> del AST, su HeadingAnchor y su HeadingContent cambian juntos, y
// HeadingSource no cambia: el renderer y el formatter siguen pudiendo volver
// a armar el encabezado desde la fuente.
func TestDocumentHeadingIDRewriteKeepsFieldsTogether(t *testing.T) {
	_, doc := documentIDs(t, "# A\n\n## Notas **ya**\n\n# B\n\n## Notas **ya**\n")
	second := doc.ContentBlocks[1].Elements[0].(*ast.TextElement)
	if second.HeadingAnchor != "notas-ya-2" {
		t.Fatalf("HeadingAnchor = %q", second.HeadingAnchor)
	}
	want := renderer.HeadingHTML(2, "Notas **ya**", "notas-ya-2")
	if second.Content != want || second.HeadingContent != want || second.HeadingSource != "Notas **ya**" {
		t.Errorf("Content=%q HeadingContent=%q HeadingSource=%q, want Content=%q", second.Content, second.HeadingContent, second.HeadingSource, want)
	}
}

// Un documento sin repetidos ni ids vacíos queda igual que antes: el <hN> es
// el que BuildHeadingElement arma con el id base.
func TestDocumentHeadingIDsUnchangedWithoutRepeats(t *testing.T) {
	_, doc := documentIDs(t, "# Uno\n\n## Dos *ya*\n\n# Tres\n\n## Cuatro\n")
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			h := el.(*ast.TextElement)
			fresh := elements.BuildHeadingElement(h.HeadingSource, h.Level, h.Position, "")
			if h.Content != fresh.Content || h.HeadingAnchor != fresh.HeadingAnchor {
				t.Errorf("%q cambió: %q, antes %q", h.HeadingSource, h.Content, fresh.Content)
			}
		}
	}
}

// Con typed-headings-v1 el id deduplicado llega al campo anchor del
// HeadingElement.
func TestDocumentHeadingIDsTypedHeadings(t *testing.T) {
	ids, doc := documentIDs(t, "---\nast_capabilities: [typed-headings-v1]\n---\n# A\n\n## Notas\n\n# B\n\n## Notas\n")
	if want := []string{"a", "notas", "b", "notas-2"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	h, ok := doc.ContentBlocks[1].Elements[0].(*ast.HeadingElement)
	if !ok || h.Anchor != "notas-2" || h.Text != "Notas" {
		t.Errorf("segundo encabezado = %#v", doc.ContentBlocks[1].Elements[0])
	}
}

const strictAnchorsFrontMatter = "---\nmode: strict\n---\n\n"

func strictSection(title string, props ...string) string {
	var b strings.Builder
	b.WriteString("SECTION \"" + title + "\"\n")
	for _, p := range props {
		b.WriteString("  " + p + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// En strict, un `id:` del autor gana: no se renombra y ningún encabezado
// derivado, antes o después, se queda con él. Dos `id:` iguales del autor se
// quedan como están y sin diagnóstico, igual que antes de #267.
func TestDocumentHeadingIDsStrictExplicit(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"secciones repetidas",
			strictSection("Doc") + strictSection("Notas", "level: 2") + strictSection("Doc") + strictSection("Notas", "level: 2"),
			[]string{"doc", "notas", "doc-2", "notas-2"}},
		{"explícito después de un derivado",
			strictSection("Doc") + strictSection("Notas", "level: 2") + strictSection("Otra", "level: 2", "id: notas"),
			[]string{"doc", "notas-2", "notas"}},
		{"explícito antes de un derivado",
			strictSection("Doc") + strictSection("Otra", "level: 2", "id: notas") + strictSection("Notas", "level: 2"),
			[]string{"doc", "notas", "notas-2"}},
		{"explícito contra una sección",
			strictSection("Notas") + strictSection("Otra", "level: 2", "id: notas"),
			[]string{"notas-2", "notas"}},
		{"dos explícitos iguales",
			strictSection("Doc") + strictSection("Uno", "level: 2", "id: mismo") + strictSection("Dos", "level: 2", "id: mismo"),
			[]string{"doc", "mismo", "mismo"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := documentIDs(t, strictAnchorsFrontMatter+tc.body)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ids = %v, want %v", got, tc.want)
			}
		})
	}
}

// Las posiciones no se mueven: la pasada solo cambia el id.
func TestDocumentHeadingIDsKeepPositions(t *testing.T) {
	src := "# A\n\n## Notas\n\n# B\n\n## Notas\n"
	_, doc := documentIDs(t, src)
	second := doc.ContentBlocks[1].Elements[0].(*ast.TextElement)
	if second.Position.Line != 7 {
		t.Errorf("línea = %d, want 7", second.Position.Line)
	}
}

// El renderer calcula el id de cada sección por su cuenta
// (renderer.DocumentSectionAnchors: reserva los ids de nivel 2-6 y después
// reparte los títulos). Para cualquier mezcla tiene que dar lo mismo que la
// regla aplicada directo: reservar los `id:` del autor y repartir todo lo
// demás junto, en orden de documento. Se compara contra esa referencia en
// documentos generados, flex y strict, con preámbulo antes del primer `#`,
// bloques ::: con encabezados (flex), `id:` del autor (strict),
// {{variables}} en los títulos y textos que chocan entre sí y con sus
// sufijos.
func TestDocumentSectionAnchorsMatchDocumentOrder(t *testing.T) {
	texts := []string{"A", "A 2", "A-2", "A 2 2", "B", "概要", "{{p}}", "{{p}} 2"}
	ids := []string{"a", "a-2", "b", "otro"}
	vars := map[string]string{"{{p}}": "A", "{{p}} 2": "A 2"}
	base := func(text string, section bool) string {
		if v, ok := vars[text]; ok && section {
			text = v
		}
		return anchors.DocumentBase(renderer.DeriveAnchor(text))
	}
	seed := uint32(267)
	next := func(n int) int {
		seed = seed*1664525 + 1013904223
		return int(seed>>16) % n
	}
	type entry struct {
		base     string
		declared string
	}
	for round := 0; round < 1000; round++ {
		strict := next(2) == 0
		var b strings.Builder
		var entries []entry
		if strict {
			b.WriteString("---\nmode: strict\nvariables:\n  p: A\n---\n\n")
		} else {
			b.WriteString("---\nvariables:\n  p: A\n---\n\n")
			if next(3) == 0 {
				b.WriteString("Preámbulo.\n\n")
				if next(2) == 0 {
					text := texts[next(len(texts))]
					b.WriteString("## " + text + "\n\n")
					entries = append(entries, entry{base: base(text, false)})
				}
			}
		}
		for i, n := 0, 1+next(8); i < n; i++ {
			text := texts[next(len(texts))]
			level := 1
			if i > 0 {
				level = 1 + next(3)
			}
			if strict {
				fmt.Fprintf(&b, "SECTION %q\n", text)
				e := entry{base: base(text, level == 1)}
				if level > 1 {
					fmt.Fprintf(&b, "  level: %d\n", level)
					if next(3) == 0 {
						e.declared = ids[next(len(ids))]
						fmt.Fprintf(&b, "  id: %s\n", e.declared)
					}
				}
				entries = append(entries, e)
				if next(4) == 0 {
					// En strict documental el cuerpo de un ::: es texto: el
					// ### de adentro no es un encabezado y no cuenta.
					fmt.Fprintf(&b, "\n  :::note\n  ### %s\n\n  cuerpo\n  :::\n", texts[next(len(texts))])
				}
				b.WriteString("\n")
				continue
			}
			b.WriteString(strings.Repeat("#", level) + " " + text + "\n\n")
			entries = append(entries, entry{base: base(text, level == 1)})
			if next(4) == 0 {
				inner := texts[next(len(texts))]
				b.WriteString(":::note\n### " + inner + "\n\ncuerpo\n:::\n\n")
				entries = append(entries, entry{base: base(inner, false)})
			}
		}

		var ref anchors.Set
		for _, e := range entries {
			if e.declared != "" {
				ref.Reserve(e.declared)
			}
		}
		var want []string
		for _, e := range entries {
			if e.declared != "" {
				want = append(want, e.declared)
				continue
			}
			want = append(want, ref.Unique(e.base))
		}
		got, _ := documentIDs(t, b.String())
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("documento:\n%s\nids = %v, want %v", b.String(), got, want)
		}
	}
}
