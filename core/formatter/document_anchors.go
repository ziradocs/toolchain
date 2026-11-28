// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"regexp"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/internal/anchors"
	"go.ziradocs.com/core/v2/renderer"
)

// documentAnchorPlan dice qué encabezados de nivel 2-6 de un documento
// tienen que declarar su anchor para que el documento formateado se vuelva a
// leer con los mismos ids (issue #267). Strict les escribe `id:`; flex, que
// no tiene sintaxis para declararlo, los rechaza. Los demás llevan el anchor
// que el parser les va a derivar en su posición, contando los repetidos: el
// segundo `## Notas` lleva notas-2 y eso no se declara.
type documentAnchorPlan struct {
	declare map[ast.Element]bool
}

// mustDeclare reporta si el anchor de el no se deriva solo.
func (p documentAnchorPlan) mustDeclare(el ast.Element) bool {
	return p.declare[el]
}

type planHeading struct {
	el       ast.Element
	id       string
	base     string
	declared bool
}

// planDocumentAnchors arma el plan repitiendo la cuenta del parser
// (parser.assignDocumentAnchors): los anchors declarados se reservan antes y
// los demás se reparten en orden de documento, secciones incluidas.
//
//  1. Se declara, de entrada, lo que el autor declaró con `id:`
//     (HeadingAnchorDeclared, que el parser deja en memoria) y todo anchor
//     que no es de la forma base o base-N del texto, porque ninguna cuenta lo
//     derivaría.
//  2. Un encabezado que la cuenta no reproduce también se declara, salvo que
//     su anchor ya lo tenga un encabezado o una sección anterior: es el id
//     repetido de un AST anterior a #267 (llegado por JSON), y se deja que el
//     reparseo lo desduplique, como hace el flex legado.
//  3. Un `id:` del autor que la cuenta derivaría igual en su lugar se omite,
//     como siempre hizo `doclang fmt` con un `id:` derivable.
//
// Todo eso sale de UNA pasada en orden de documento (documentAnchorPass),
// lineal en el tamaño del documento. Para un AST que salió del parser, la
// salida conserva cada `id:` del autor donde estaba (o lo omite si es
// derivable), no agrega ninguno, el documento reparseado tiene los mismos
// ids y una segunda pasada de fmt no cambia nada.
func planDocumentAnchors(doc *ast.AST) documentAnchorPlan {
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
	declare, _ := documentAnchorPass(doc, headings, renderer.DocumentSectionAnchorBases(doc, true))
	plan := documentAnchorPlan{declare: map[ast.Element]bool{}}
	for i, h := range headings {
		if declare[i] {
			plan.declare[h.el] = true
		}
	}
	return plan
}

// documentAnchorPass decide, en una sola pasada, qué encabezados declaran su
// anchor (ver planDocumentAnchors). Lleva la misma cuenta que anchors.Set
// (primer sufijo -N libre, con el mismo atajo por id base) y, además, qué
// anchors reservados de antemano alguien encontró ocupados antes de que les
// tocara (contested): quitar esa reserva cambiaría a ese alguien.
//
// Quitar el `id:` de un encabezado h con anchor a no cambia nada si y solo
// si (1) ningún anchor anterior a h encontró a ocupado solo por la reserva,
// y (2) en la posición de h, sin la reserva, la cuenta le da a. Con eso, lo
// anterior a h queda igual, h se lleva a, y lo posterior ve a ocupado igual
// que antes. Para un AST salido del parser, decidirlo en orden da el mismo
// resultado que quitar y volver a simular uno por uno hasta que no sobre
// ninguno: si un `id:` posterior se pudiera quitar solo porque se quitó otro,
// el primero habría encontrado el anchor del segundo ocupado, y entonces ese
// segundo no se quita.
//
// Un AST que ya trae ids repetidos (llegado por JSON, de antes de #267) no
// cumple esa equivalencia: la pasada decide el paso 2 en orden, así que el
// primero de dos encabezados con el mismo id se declara y el segundo se deja
// desduplicar, mientras que volver a simular podía declarar los dos y dejar
// el id repetido. Con `# A`, un `#### Notas` y un `## Notas` que traen los
// dos notas-3, y `# Notas`, sale `id: notas-3` solo en el primero, y el
// reparseo da notas-3, notas y la sección notas-2.
//
// steps cuenta los candidatos probados, para la prueba de escala.
func documentAnchorPass(doc *ast.AST, headings []planHeading, sectionBases []string) (declare []bool, steps int) {
	declare = make([]bool, len(headings))
	t := trackedAnchors{used: map[string]int{}, next: map[string]int{}, pending: map[string]int{}, contested: map[string]bool{}}
	for i, h := range headings {
		// Un id vacío es el de un encabezado sin caracteres de un AST
		// anterior a #267: el reparseo le da el de respaldo.
		declare[i] = h.id != "" && (h.declared || !inAnchorFamily(h.id, h.base))
		if declare[i] {
			t.used[h.id]++
			t.pending[h.id]++
		}
	}
	assigned := map[string]bool{}
	next := 0
	anchors.WalkDocument(doc,
		func(i int, _ string) { assigned[t.unique(sectionBases[i])] = true },
		func(el ast.Element) {
			if next >= len(headings) || headings[next].el != el {
				return
			}
			i := next
			next++
			h := headings[i]
			if declare[i] {
				t.pending[h.id]--
				if h.declared && inAnchorFamily(h.id, h.base) && !t.contested[h.id] && t.used[h.id] == 1 {
					t.used[h.id] = 0
					if t.derive(h.base, false) == h.id {
						declare[i] = false
						t.unique(h.base)
					} else {
						t.used[h.id] = 1
					}
				}
				assigned[h.id] = true
				return
			}
			if h.id == "" || t.derive(h.base, false) == h.id || assigned[h.id] {
				assigned[t.unique(h.base)] = true
				return
			}
			declare[i] = true
			t.used[h.id]++
			assigned[h.id] = true
		})
	return declare, t.steps
}

// trackedAnchors es la cuenta de anchors.Set más el registro de reservas
// encontradas ocupadas (contested) que usa documentAnchorPass.
type trackedAnchors struct {
	used      map[string]int
	next      map[string]int
	pending   map[string]int
	contested map[string]bool
	steps     int
}

// derive devuelve el anchor que unique asignaría a base. Con track, anota
// cada candidato ocupado solo por una reserva pendiente.
func (t *trackedAnchors) derive(base string, track bool) string {
	occupied := func(c string) bool {
		t.steps++
		if t.used[c] == 0 {
			return false
		}
		if track && t.pending[c] > 0 {
			t.contested[c] = true
		}
		return true
	}
	if !occupied(base) {
		return base
	}
	n := max(2, t.next[base])
	candidate := base + "-" + strconv.Itoa(n)
	for occupied(candidate) {
		n++
		candidate = base + "-" + strconv.Itoa(n)
	}
	return candidate
}

func (t *trackedAnchors) unique(base string) string {
	candidate := t.derive(base, true)
	t.used[candidate]++
	if candidate != base {
		n, _ := strconv.Atoi(strings.TrimPrefix(candidate, base+"-"))
		t.next[base] = n
	}
	return candidate
}

// inAnchorFamily reporta si id es base o base-N (N >= 2): un anchor que la
// cuenta de repetidos puede derivar de un texto con ese id base.
func inAnchorFamily(id, base string) bool {
	if id == base {
		return true
	}
	suffix, ok := strings.CutPrefix(id, base+"-")
	if !ok || suffix == "" || suffix[0] == '0' {
		return false
	}
	n, err := strconv.Atoi(suffix)
	return err == nil && n >= 2
}

var headingOpenIDPattern = regexp.MustCompile(`^(<h[1-6][^>]*\bid=")[^"]*(")`)

// maskHeadingAnchors devuelve una copia de els en la que el anchor de cada
// encabezado (también dentro de bloques ::: y columnas) queda vacío, salvo
// los que keep conserva. Sirve para comparar un bloque releído aislado, cuyo
// anchor depende del documento que lo rodea y no del bloque.
func maskHeadingAnchors(els []ast.Element, keep func(ast.Element) bool) []ast.Element {
	if els == nil {
		return nil
	}
	out := make([]ast.Element, len(els))
	for i, el := range els {
		switch e := el.(type) {
		case *ast.TextElement:
			if e.IsRawHTML && e.Level > 0 && !keep(e) {
				c := *e
				c.Content = headingOpenIDPattern.ReplaceAllString(c.Content, "$1$2")
				c.ContentHTML = headingOpenIDPattern.ReplaceAllString(c.ContentHTML, "$1$2")
				out[i] = &c
				continue
			}
		case *ast.HeadingElement:
			if !keep(e) {
				c := *e
				c.Anchor = ""
				out[i] = &c
				continue
			}
		case *ast.SpecialBlockElement:
			c := *e
			c.Elements = maskHeadingAnchors(e.Elements, keep)
			out[i] = &c
			continue
		case *ast.GridElement:
			c := *e
			c.Columns = make([]ast.ColumnElement, len(e.Columns))
			for j, col := range e.Columns {
				col.Elements = maskHeadingAnchors(col.Elements, keep)
				c.Columns[j] = col
			}
			out[i] = &c
			continue
		}
		out[i] = el
	}
	return out
}
