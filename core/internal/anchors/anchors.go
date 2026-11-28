// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package anchors reparte los ids (anchors) de los encabezados para que no se
// repitan dentro de un documento o de un deck: el primero conserva el id
// base, los siguientes llevan -2, -3, … en orden de aparición.
//
// Es la única implementación de esa cuenta en el toolchain. SlideLang la usa
// con el prefijo "heading-" (elements.HeadingAnchors, issue #194) y DocLang
// sin prefijo (issue #267), así que los dos resuelven igual las repeticiones
// y los choques con un id base real.
//
// El paquete no deriva anchors desde texto: recibe el id base ya derivado
// (renderer.DeriveAnchor). Así lo pueden usar el renderer y el parser sin un
// ciclo de imports.
package anchors

import (
	"regexp"
	"strconv"

	"go.ziradocs.com/core/v2/ast"
)

// Set lleva la cuenta de los anchors ya usados.
//
// Regla de colisión: un id base libre se usa tal cual; uno ocupado recibe el
// primer sufijo -N (N >= 2) que esté libre. Los sufijos se prueban contra
// todo lo usado, incluidos los ids base reales: con encabezados "Notas",
// "Notas" y "Notas 2" salen notas, notas-2 y notas-2-2 (el tercero tiene id
// base notas-2, que ya tomó el segundo).
type Set struct {
	used map[string]int
	// next es, por id base, el sufijo desde el que buscar el próximo libre.
	// Un anchor usado nunca se libera, así que todo sufijo por debajo del
	// último asignado sigue ocupado: empezar ahí da el mismo resultado que
	// empezar en 2, sin volver a probar uno por uno los ya tomados. Sin esto,
	// k encabezados con el mismo texto costaban O(k²).
	next map[string]int
}

// Derive devuelve el anchor que Unique asignaría a base, sin registrarlo.
func (s *Set) Derive(base string) string {
	candidate, _ := s.derive(base)
	return candidate
}

func (s *Set) derive(base string) (candidate string, n int) {
	candidate = base
	n = max(2, s.next[base])
	if s.used[candidate] == 0 {
		return candidate, 0
	}
	for candidate = base + "-" + strconv.Itoa(n); s.used[candidate] > 0; n++ {
		candidate = base + "-" + strconv.Itoa(n+1)
	}
	return candidate, n
}

// Unique asigna a base su anchor y lo registra.
func (s *Set) Unique(base string) string {
	candidate, n := s.derive(base)
	s.Reserve(candidate)
	if n > 0 {
		if s.next == nil {
			s.next = make(map[string]int)
		}
		s.next[base] = n
	}
	return candidate
}

// Reserve registra un anchor que no se renombra (un `id:` del autor, o uno
// ya asignado). Devuelve false si ya estaba usado.
func (s *Set) Reserve(anchor string) bool {
	if s.used == nil {
		s.used = make(map[string]int)
	}
	s.used[anchor]++
	return s.used[anchor] == 1
}

// DocumentFallback es el id base de un encabezado de documento cuyo texto no
// deja ningún carácter utilizable (otro alfabeto, solo emoji). Antes salía
// id="", que es HTML inválido y no se puede enlazar.
const DocumentFallback = "seccion"

// DocumentBase devuelve el id base de un encabezado de documento a partir del
// anchor derivado de su texto: el mismo, o DocumentFallback si quedó vacío.
func DocumentBase(derived string) string {
	if derived == "" {
		return DocumentFallback
	}
	return derived
}

// WalkDocument recorre, en orden de documento, todo lo que DocLang dibuja
// como encabezado: el título de cada bloque (el `#` de sección, nivel 1),
// con section, y cada encabezado de nivel 2-6, también los que van dentro de
// un bloque ::: o de una columna de grid, con heading. section recibe el
// índice del bloque y su título (ast.ContentBlock.SectionTitle); un bloque
// sin título no dibuja <h1> y no se visita. Cualquiera de los dos puede ser
// nil.
func WalkDocument(doc *ast.AST, section func(block int, title string), heading func(el ast.Element)) {
	if doc == nil {
		return
	}
	for i := range doc.ContentBlocks {
		block := &doc.ContentBlocks[i]
		if title, _ := block.SectionTitle(); title != "" && section != nil {
			section(i, title)
		}
		if heading != nil {
			walkHeadings(block.Elements, heading)
		}
	}
}

func walkHeadings(elements []ast.Element, heading func(el ast.Element)) {
	for _, el := range elements {
		switch e := el.(type) {
		case *ast.TextElement:
			if e.IsRawHTML && e.Level > 0 {
				heading(e)
			}
		case *ast.HeadingElement:
			heading(e)
		case *ast.SpecialBlockElement:
			walkHeadings(e.Elements, heading)
		case *ast.GridElement:
			for j := range e.Columns {
				walkHeadings(e.Columns[j].Elements, heading)
			}
		}
	}
}

// headingIDPattern extrae el id del <hN id="…"> de un encabezado legado.
var headingIDPattern = regexp.MustCompile(`^<h[1-6][^>]*\bid="([^"]*)"`)

// HeadingID devuelve el id que el AST le asignó a un encabezado de nivel 2-6:
// el Anchor de un HeadingElement o el id="…" del <hN> de un TextElement
// legado. ok es false si el elemento no es un encabezado o no trae id.
func HeadingID(el ast.Element) (id string, ok bool) {
	switch e := el.(type) {
	case *ast.HeadingElement:
		return e.Anchor, true
	case *ast.TextElement:
		if !e.IsRawHTML || e.Level == 0 {
			return "", false
		}
		if m := headingIDPattern.FindStringSubmatch(e.Content); m != nil {
			return m[1], true
		}
	}
	return "", false
}
