// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/internal/anchors"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// referenceAnchorPlan es la primera versión de planDocumentAnchors: quita
// cada `id:` del autor y vuelve a simular el documento entero, hasta que no
// sobre ninguno. Es cuadrática (revisión de #267, P1-a), pero directa: sirve
// de referencia para la pasada única.
func referenceAnchorPlan(doc *ast.AST) documentAnchorPlan {
	var headings []planHeading
	anchors.WalkDocument(doc, nil, func(el ast.Element) {
		h, ok := asDocumentHeading(el)
		if !ok {
			return
		}
		ph := planHeading{el: el, id: h.id, base: anchors.DocumentBase(renderer.DeriveAnchor(h.text))}
		switch e := el.(type) {
		case *ast.TextElement:
			ph.declared = e.HeadingAnchorDeclared
		case *ast.HeadingElement:
			ph.declared = e.AnchorDeclared
		}
		headings = append(headings, ph)
	})
	sectionBases := renderer.DocumentSectionAnchorBases(doc, true)

	declare := make([]bool, len(headings))
	for i, h := range headings {
		// Un id vacío es el de un encabezado sin caracteres de un AST
		// anterior a #267: el reparseo le da el de respaldo.
		declare[i] = h.id != "" && (h.declared || !inAnchorFamily(h.id, h.base))
	}

	// simulate devuelve, con los declarados de declare, el anchor que el
	// parser le daría a cada encabezado y si su id ya lo tenía algo
	// anterior.
	simulate := func(declare []bool) (got []string, taken []bool) {
		var set anchors.Set
		for i, h := range headings {
			if declare[i] {
				set.Reserve(h.id)
			}
		}
		got = make([]string, len(headings))
		taken = make([]bool, len(headings))
		earlier := map[string]bool{}
		next := 0
		anchors.WalkDocument(doc,
			func(i int, _ string) { earlier[set.Unique(sectionBases[i])] = true },
			func(el ast.Element) {
				if next >= len(headings) || headings[next].el != el {
					return
				}
				i := next
				next++
				if declare[i] {
					got[i] = headings[i].id
				} else {
					got[i] = set.Unique(headings[i].base)
				}
				taken[i] = earlier[headings[i].id]
				earlier[got[i]] = true
			})
		return got, taken
	}

	for changed := true; changed; {
		changed = false
		got, taken := simulate(declare)
		for i, h := range headings {
			if !declare[i] && h.id != "" && got[i] != h.id && !taken[i] {
				declare[i] = true
				changed = true
			}
		}
	}

	same := func(a, b []string) bool {
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	for changed := true; changed; {
		changed = false
		want, _ := simulate(declare)
		for i, h := range headings {
			if !declare[i] || !h.declared || !inAnchorFamily(h.id, h.base) {
				continue
			}
			declare[i] = false
			if got, _ := simulate(declare); same(got, want) {
				changed = true
				want = got
				continue
			}
			declare[i] = true
		}
	}

	plan := documentAnchorPlan{declare: map[ast.Element]bool{}}
	for i, h := range headings {
		if declare[i] {
			plan.declare[h.el] = true
		}
	}
	return plan
}

func planOrder(doc *ast.AST, plan documentAnchorPlan) []bool {
	var out []bool
	anchors.WalkDocument(doc, nil, func(el ast.Element) {
		if _, ok := asDocumentHeading(el); ok {
			out = append(out, plan.mustDeclare(el))
		}
	})
	return out
}

func samePlan(t *testing.T, src string, doc *ast.AST) {
	t.Helper()
	got, want := planOrder(doc, planDocumentAnchors(doc)), planOrder(doc, referenceAnchorPlan(doc))
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("plan = %v, referencia %v\n%s", got, want, src)
	}
}

// La pasada única de planDocumentAnchors decide lo mismo que la referencia
// cuadrática: en documentos strict generados (los de
// TestFormatDocumentStrictRoundTripGenerated), en flex con bloques ::: y en
// ASTs llegados por JSON con ids cambiados a mano.
func TestPlanDocumentAnchorsMatchesReference(t *testing.T) {
	texts := []string{"A", "A 2", "Notas", "Notas 2", "x", "概要"}
	ids := []string{"a", "a-2", "notas", "notas-2", "otro"}
	seed := uint32(500)
	next := func(n int) int {
		seed = seed*1664525 + 1013904223
		return int(seed>>16) % n
	}
	parse := func(src string) *ast.AST {
		doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "")
		return doc
	}
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
		samePlan(t, src, parse(src))
	}
	for round := 0; round < 500; round++ {
		var b strings.Builder
		for i, n := 0, 1+next(7); i < n; i++ {
			text := texts[next(len(texts))]
			switch k := next(4); {
			case i == 0 || k == 0:
				b.WriteString("# " + text + "\n\n")
			case k == 1:
				b.WriteString("## " + text + "\n\n")
			case k == 2:
				b.WriteString(":::note\n### " + text + "\n\ncuerpo\n:::\n\n")
			default:
				b.WriteString("### " + text + "\n\n")
			}
		}
		src := b.String()
		doc := parse(src)
		samePlan(t, src, doc)
		// Por JSON, sin las marcas en memoria y con algunos ids cambiados.
		anchors.WalkDocument(doc, nil, func(el ast.Element) {
			if h, ok := el.(*ast.TextElement); ok && next(3) == 0 {
				h.Content = strings.Replace(h.Content, `id="`+h.HeadingAnchor+`"`, `id="`+ids[next(len(ids))]+`"`, 1)
			}
		})
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := ast.DecodeAST(raw)
		if err != nil {
			t.Fatal(err)
		}
		samePlan(t, src+"(por JSON)", decoded)
	}
}

// Revisión de #267, P1-a: la primera versión simulaba el documento entero
// por cada `id:` del autor, y con 8000 tardaba 32 s. La pasada única prueba
// un número de candidatos proporcional al documento. Se cuentan los
// candidatos (no el reloj, que falla con la máquina cargada): con N
// encabezados, a lo más unos pocos por encabezado (ocho), y la proporción no
// crece con N.
func TestPlanDocumentAnchorsScalesLinearly(t *testing.T) {
	for _, kind := range []string{"derivables", "necesarios", "mitad", "repetidos"} {
		var perHeading []float64
		for _, n := range []int{1000, 4000, 8000} {
			var b strings.Builder
			b.WriteString("---\nmode: strict\n---\n\nSECTION \"Doc\"\n\n")
			for i := 0; i < n; i++ {
				id := fmt.Sprintf("h%d", i)
				title := fmt.Sprintf("H%d", i)
				switch {
				case kind == "necesarios", kind == "mitad" && i%2 == 0:
					id = fmt.Sprintf("x%d", i)
				case kind == "repetidos":
					title = "Notas"
					id = "notas"
					if i > 0 {
						id = fmt.Sprintf("notas-%d", i+1)
					}
				}
				fmt.Fprintf(&b, "SECTION %q\n  level: 2\n  id: %s\n\n", title, id)
			}
			doc, _ := parser.New(util.NewNoop()).ParseDocument(b.String(), "")
			var headings []planHeading
			anchors.WalkDocument(doc, nil, func(el ast.Element) {
				h, _ := asDocumentHeading(el)
				ph := planHeading{el: el, id: h.id, base: anchors.DocumentBase(renderer.DeriveAnchor(h.text))}
				ph.declared = el.(*ast.TextElement).HeadingAnchorDeclared
				headings = append(headings, ph)
			})
			if len(headings) != n {
				t.Fatalf("%s: %d encabezados, want %d", kind, len(headings), n)
			}
			_, steps := documentAnchorPass(doc, headings, renderer.DocumentSectionAnchorBases(doc, true))
			perHeading = append(perHeading, float64(steps)/float64(n))
		}
		t.Logf("%s: candidatos por encabezado con N=1000, 4000, 8000: %v", kind, perHeading)
		for _, p := range perHeading {
			if p > 8 {
				t.Errorf("%s: %.2f candidatos por encabezado (%v)", kind, p, perHeading)
			}
		}
		if perHeading[2] > 1.5*perHeading[0] {
			t.Errorf("%s: la proporción crece con N: %v", kind, perHeading)
		}
	}
}

// Un AST que ya trae ids repetidos (por JSON, de antes de #267) es donde la
// pasada única deja de coincidir con la referencia: declara el primero de
// los dos encabezados con el mismo id y deja desduplicar el segundo, como
// pide la regla, en vez de declarar los dos y dejar el id repetido. Revisión
// de #267, tercera pasada.
func TestPlanDocumentAnchorsDecodedRepeatedIDs(t *testing.T) {
	doc, _ := parser.New(util.NewNoop()).ParseDocument("# A\n\n#### Notas\n\n## Notas\n\n# Notas\n", "")
	for _, el := range doc.ContentBlocks[0].Elements {
		h := el.(*ast.TextElement)
		h.Content = strings.Replace(h.Content, `id="`+h.HeadingAnchor+`"`, `id="notas-3"`, 1)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ast.DecodeAST(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(planOrder(decoded, planDocumentAnchors(decoded))); got != "[true false]" {
		t.Errorf("plan = %s, want [true false]", got)
	}
	if got := fmt.Sprint(planOrder(decoded, referenceAnchorPlan(decoded))); got != "[true true]" {
		t.Errorf("la referencia cambió: %s, want [true true]", got)
	}
	out, err := FormatDocumentStrict(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "id: notas-3") != 1 || strings.Count(out, "id:") != 1 {
		t.Errorf("id: escritos:\n%s", out)
	}
	reparsed, _ := parser.New(util.NewNoop()).ParseDocument(out, "")
	sections := renderer.DocumentSectionAnchors(reparsed, true)
	var ids []string
	anchors.WalkDocument(reparsed,
		func(i int, _ string) { ids = append(ids, sections[i]) },
		func(el ast.Element) {
			id, _ := anchors.HeadingID(el)
			ids = append(ids, id)
		})
	if got := strings.Join(ids, " "); got != "a notas-3 notas notas-2" {
		t.Errorf("ids tras formatear = %s, want a notas-3 notas notas-2\n%s", got, out)
	}
}
