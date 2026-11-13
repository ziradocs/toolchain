// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"reflect"
	"strings"
	"sync"
	"unicode"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/internal/elements"
	"go.ziradocs.com/core/v2/util"
)

// emptyTextCut decide si un TEXT strict vacío suelta las líneas que tomaría
// como cuerpo, para que se lean como elementos. Solo las suelta si leerlas
// así no pierde ni reinterpreta nada de lo que main conservaba como el
// párrafo del TEXT.
//
// La prueba va de un elemento a la vez (step): lee el elemento que abre en
// una línea, lo contrasta con sus líneas (accountsFor) y sigue desde la
// línea siguiente a su misma sangría. El resultado de cada línea se guarda,
// así que cada una se prueba una vez y el costo es lineal.
//
// Leer un elemento a prueba puede leer otro TEXT vacío, que a su vez pide su
// propia prueba. En vez de recursión, esa pregunta se anota en missing y la
// responde el loop de keeps antes de repetir el paso, así que la profundidad
// de la pila no depende del número de elementos.
type emptyTextCut struct {
	root    *strictBody
	results map[int]bool
	busy    bool
	missing []int
	// anchors es la capa de anchors de los pasos de una misma prueba, sobre
	// los del cuerpo: cada paso ve lo que registraron los anteriores (un
	// id repetido entre dos encabezados leídos a prueba es un error, como en
	// el parse real), y la capa se descarta al terminar la prueba.
	anchors elements.HeadingAnchors
}

// emptyTextCutKeepsText es ParseContext.EmptyTextCutKeepsText: start es un
// índice de p.lines, y el rango llega hasta la primera línea con menos
// sangría (end es siempre el final de p.lines).
func (p *strictBody) emptyTextCutKeepsText(start, end int) bool {
	if start < 0 || start >= len(p.lines) || end != len(p.lines) {
		return false
	}
	if p.emptyText == nil {
		p.emptyText = &emptyTextCut{root: p, results: map[int]bool{}}
	}
	return p.emptyText.keeps(p.emptyTextBase + start)
}

// hasNodeID es ParseContext.HasNodeID.
func (p *strictBody) hasNodeID(index int) bool {
	return p.nodeIDLines[p.position(index).Line]
}

// keeps dice si las líneas desde root.lines[k] se leen como elementos sin
// perder nada.
func (c *emptyTextCut) keeps(k int) bool {
	if r, ok := c.results[k]; ok {
		return r
	}
	if c.busy {
		// Lo pide un TEXT vacío leído a prueba: keeps lo responde y repite
		// el paso que lo pidió. Mientras tanto dice que sí, para que ese TEXT
		// corte ahí y el paso termine enseguida; su resultado se descarta.
		c.missing = append(c.missing, k)
		return true
	}
	c.anchors = c.root.headingAnchors.Overlay()
	work := []int{k}
	for len(work) > 0 {
		top := work[len(work)-1]
		if _, ok := c.results[top]; ok {
			work = work[:len(work)-1]
			continue
		}
		c.busy = true
		c.missing = c.missing[:0]
		mark := c.anchors.Mark()
		ok, next := c.step(top)
		c.busy = false
		if len(c.missing) > 0 {
			// El paso se repite: lo que registró no cuenta.
			c.anchors.Rollback(mark)
			for _, m := range c.missing {
				if _, done := c.results[m]; !done {
					work = append(work, m)
				}
			}
			continue
		}
		switch {
		case !ok:
			c.results[top] = false
		case next < 0:
			c.results[top] = true
		default:
			if r, done := c.results[next]; done {
				c.results[top] = r
			} else {
				work = append(work, next)
				continue
			}
		}
		work = work[:len(work)-1]
	}
	return c.results[k]
}

// step lee a prueba el elemento que abre en root.lines[k]. Devuelve si sus
// líneas se leen sin perder nada y dónde sigue: -1 si ahí termina lo que el
// TEXT tomaba como cuerpo, o el índice de la línea siguiente, que tiene que
// abrir otro elemento a la misma sangría.
func (c *emptyTextCut) step(k int) (bool, int) {
	root := c.root
	lines := root.lines[k:]
	sub := &strictBody{
		emptyText:       c,
		emptyTextBase:   k,
		nodeIDLines:     root.nodeIDLines,
		nestedListTypes: root.nestedListTypes,
		listStart:       root.listStart,
		mathSource:      root.mathSource,
		contentMarkdown: root.contentMarkdown,
		slideHeadings:   root.slideHeadings,
		// Lo que el elemento de prueba registre queda en la capa de la
		// prueba, que ven los pasos siguientes y se descarta con ella.
		headingAnchors: c.anchors,
		lines:          lines,
		logger:         util.NewNoop(),
		lineOffset:     root.lineOffset + k,
	}
	scratch := ast.NewContentBlock(sub.position(0), "content")
	reinterpreted := false
	reached := 0
	sub.parseIndentedElements(scratch,
		func(string, string) { reinterpreted = true },
		// Solo el primer elemento: el loop para en la siguiente línea con
		// texto que llega a despachar.
		func(string) bool { reached++; return reached > 1 },
		func(string) { reinterpreted = true })
	next := sub.currentLine
	if reinterpreted || next == 0 || len(c.missing) > 0 {
		return false, 0
	}
	for _, d := range sub.diagnostics {
		if d.RuleID != elements.EmptyTextRuleID {
			return false, 0
		}
	}
	first := strings.TrimSpace(lines[0])
	indent := elements.CalculateIndentLevel(lines[0])
	switch len(scratch.Elements) {
	case 0:
		// Otro TEXT vacío: no da elemento, solo TEXT001, y no pierde nada.
		if first != "TEXT" {
			return false, 0
		}
	case 1:
		el := scratch.Elements[0]
		// Una línea a la misma sangría dentro del elemento es suya solo si
		// el elemento se escribe así (las filas de una tabla pipe, el cuerpo
		// y el cierre de un ":::" o de un "<<…>>", la prosa de un TEXT). Una
		// QUOTE o un POINTS que la tomaran le cambiarían el sentido.
		if !ownsSameIndentLines(el) {
			for _, line := range lines[1:next] {
				if strings.TrimSpace(line) != "" && elements.CalculateIndentLevel(line) == indent {
					return false, 0
				}
			}
		}
		if !accountsFor(lines[:next], el) {
			return false, 0
		}
	default:
		return false, 0
	}
	if next >= len(lines) {
		return true, -1
	}
	// Una línea con menos sangría termina lo que el TEXT tomaba como cuerpo.
	// Una con más sangría que el elemento no tomó quedaría en posición de
	// elemento, y lo que sigue a su misma sangría tiene que abrir otro
	// elemento con las mismas reglas que la primera línea.
	line := lines[next]
	switch lineIndent := elements.CalculateIndentLevel(line); {
	case lineIndent < indent:
		return true, -1
	case lineIndent > indent:
		return false, 0
	}
	if !elements.OpensElementAfterEmptyText(sub.parseContext(), strings.TrimSpace(line)) {
		return false, 0
	}
	return true, k + next
}

// ownsSameIndentLines dice si el elemento strict el se escribe con líneas a
// la misma sangría de su apertura: un TEXT (su cuerpo a la misma sangría),
// una tabla pipe, un bloque ":::" o un bloque "<<…>>".
func ownsSameIndentLines(el ast.Element) bool {
	switch el.(type) {
	case *ast.TextElement, *ast.TableElement, *ast.SpecialBlockElement, *ast.CodeGroupElement,
		*ast.MermaidElement, *ast.PlantUMLElement, *ast.ChartElement, *ast.MapElement,
		*ast.MathElement, *ast.GridElement, *ast.QuizElement, *ast.PollElement,
		*ast.MetricElement:
		return true
	}
	return false
}

// accountsFor dice si el elemento el da cuenta de cada línea no vacía de
// lines, las que tomó. Cada una tiene que ser:
//
//   - la de apertura (la primera): sus tokens, salvo la palabra clave o
//     etiqueta con que abre, están en el elemento;
//   - una de cierre del contenedor (closesElement): "<<end>>", ":::", el
//     separador de una tabla pipe…;
//   - una propiedad que el elemento guardó: "Clave: valor" cuenta como
//     propiedad solo si el elemento tiene un campo con ese nombre cuyo valor
//     trae ese valor; cada campo da cuenta de una sola línea, y sus tokens
//     salen del mismo conteo que los de las demás líneas;
//   - o una línea cuyo contenido el elemento tomó: cada uno de sus tokens
//     (también los de un carácter y signos como € o %) está en el texto del
//     elemento, contando repeticiones, y una línea sin tokens (solo signos de
//     la sintaxis, como "---" o "**") tiene que estar tal cual en él.
//
// Una línea que no es ninguna de esas no está a cuenta, y entonces el corte
// no va. Un token es un tramo de letras y dígitos, de cualquier largo, o un
// signo que no es sintaxis strict. De los campos del elemento no cuentan los
// de tipo o enumeración (type, listType, context…) ni los derivados (*HTML,
// cells, id), que repetirían el texto.
func accountsFor(lines []string, el ast.Element) bool {
	have := map[string]int{}
	var texts []string
	collectElementTokens(reflect.ValueOf(el), have, &texts)
	consume := func(toks []string) bool {
		for _, tok := range toks {
			if have[tok] == 0 {
				return false
			}
			have[tok]--
		}
		return true
	}
	usedFields := map[string]bool{}
	for i, line := range lines {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			continue
		case i == 0:
			if !consume(openerTokens(t)) {
				return false
			}
			continue
		case closesElement(t, el):
			continue
		}
		body := withoutListMarker(t)
		if key, value, ok := splitProperty(body); ok {
			if field := propertyField(el, key, value); field != "" && !usedFields[field] {
				usedFields[field] = true
				// El valor sale del mismo conteo que el resto de las líneas,
				// para que otra línea no dé cuenta de él. Un campo de
				// enumeración ("context: hero") no está en ese conteo.
				if derivedOrEnumField[field] || strings.HasSuffix(field, "HTML") || consume(tokens(value)) {
					continue
				}
				return false
			}
		}
		toks := tokens(body)
		if len(toks) == 0 {
			if !consumeVerbatim(texts, body) {
				return false
			}
			continue
		}
		if !consume(toks) {
			return false
		}
	}
	return true
}

// closesElement dice si t es una línea de la sintaxis del contenedor el que
// no lleva texto: el cierre "<<end>>" de un bloque "<<…>>", los marcadores de
// columna de un grid, el cierre ":::" de un bloque, el cierre de una cerca en
// un code-group o el separador "|---|" de una tabla pipe.
func closesElement(t string, el ast.Element) bool {
	switch el.(type) {
	case *ast.MermaidElement, *ast.PlantUMLElement, *ast.ChartElement, *ast.MapElement,
		*ast.MathElement, *ast.QuizElement, *ast.PollElement, *ast.MetricElement:
		return t == "<<end>>"
	case *ast.GridElement:
		return t == "<<end>>" || t == "<<column>>" || t == "<<column typed>>"
	case *ast.SpecialBlockElement:
		return len(t) >= 3 && strings.Trim(t, ":") == ""
	case *ast.CodeGroupElement:
		return len(t) >= 3 && (strings.Trim(t, ":") == "" || strings.Trim(t, "`") == "")
	case *ast.TableElement:
		return strings.HasPrefix(t, "|") && strings.Contains(t, "-") && strings.Trim(t, "|-: \t") == ""
	}
	return false
}

// splitProperty parte "Clave: valor" (o "Clave:" sola) en clave y valor.
func splitProperty(t string) (string, string, bool) {
	rest := withoutPropertyKey(t)
	if rest == t {
		return "", "", false
	}
	key := strings.TrimSuffix(strings.TrimSpace(t[:len(t)-len(rest)]), ":")
	return key, strings.Trim(strings.TrimSpace(rest), `"`), true
}

// propertyField devuelve el nombre JSON del campo de el que guardó la
// propiedad key con valor value, o "" si el no la tiene: un campo con ese
// nombre (sin distinguir mayúsculas), no vacío, cuyos tokens traen los de
// value. Una clave sin valor ("rows:") vale si el campo no está vacío.
func propertyField(el ast.Element, key, value string) string {
	v := reflect.ValueOf(el)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if !f.IsExported() || f.Anonymous || name == "" || name == "-" || !strings.EqualFold(name, key) {
			continue
		}
		field := v.Field(i)
		if field.IsZero() {
			return ""
		}
		have := map[string]int{}
		collectValueTokens(field, have)
		for _, tok := range tokens(value) {
			if have[tok] == 0 {
				return ""
			}
			have[tok]--
		}
		return name
	}
	return ""
}

// consumeVerbatim dice si line está tal cual en alguna cadena de texts, y la
// tacha ahí para que no dé cuenta de otra línea igual.
func consumeVerbatim(texts []string, line string) bool {
	for i, text := range texts {
		if j := strings.Index(text, line); j >= 0 {
			texts[i] = text[:j] + "\x00" + text[j+len(line):]
			return true
		}
	}
	return false
}

// derivedOrEnumField son los campos del JSON de un elemento que no son texto
// del autor, o que repiten otro campo.
var derivedOrEnumField = map[string]bool{
	"type": true, "listType": true, "context": true, "scope": true, "icon": true,
	"nodeId": true, "id": true, "cells": true, "position": true, "endPosition": true,
	"rowPositions": true,
}

// collectElementTokens suma a have los tokens de cada cadena de v, un nodo
// del AST, recorriendo los campos que su JSON publica salvo los de
// derivedOrEnumField y los *HTML. Es lo mismo que collectTokens sobre su
// JSON, sin codificarlo.
func collectElementTokens(v reflect.Value, have map[string]int, texts *[]string) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			collectElementTokens(v.Elem(), have, texts)
		}
	case reflect.Struct:
		for _, i := range textFields(v.Type()) {
			collectElementTokens(v.Field(i), have, texts)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			collectElementTokens(v.Index(i), have, texts)
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			if k := iter.Key(); k.Kind() == reflect.String && (derivedOrEnumField[k.String()] || strings.HasSuffix(k.String(), "HTML")) {
				continue
			}
			collectElementTokens(iter.Value(), have, texts)
		}
	case reflect.String:
		if texts != nil {
			*texts = append(*texts, v.String())
		}
		for _, tok := range tokens(v.String()) {
			have[tok]++
		}
	}
}

// collectValueTokens suma a have los tokens de v, el valor de un campo, sin
// excluir nada: una propiedad puede guardarse en un campo de enumeración
// ("context: hero").
func collectValueTokens(v reflect.Value, have map[string]int) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			collectValueTokens(v.Elem(), have)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			collectValueTokens(v.Index(i), have)
		}
	case reflect.String:
		for _, tok := range tokens(v.String()) {
			have[tok]++
		}
	default:
		collectElementTokens(v, have, nil)
	}
}

// textFieldCache guarda, por tipo de struct, los índices de los campos que
// collectElementTokens recorre.
var textFieldCache sync.Map

// textFields son los campos exportados de t que su JSON publica, salvo los de
// derivedOrEnumField y los *HTML.
func textFields(t reflect.Type) []int {
	if cached, ok := textFieldCache.Load(t); ok {
		return cached.([]int)
	}
	var out []int
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" && !f.Anonymous {
			name = f.Name
		}
		if derivedOrEnumField[name] || strings.HasSuffix(name, "HTML") {
			continue
		}
		out = append(out, i)
	}
	textFieldCache.Store(t, out)
	return out
}

// openerTokens son los tokens de la línea de apertura que el elemento tiene
// que conservar: todos salvo la palabra clave, la etiqueta o el nombre con
// que abre ("POINTS", "<<math>>", ":::note", "@center", "SECTION") y la marca
// {verbatim} de CODE.
func openerTokens(t string) []string {
	toks := tokens(t)
	for i, tok := range toks {
		if isWordToken(tok) {
			toks = append(toks[:i:i], toks[i+1:]...)
			break
		}
	}
	if strings.HasPrefix(t, "CODE"+elements.CodeVerbatimMarker) {
		for i, tok := range toks {
			if tok == "verbatim" {
				toks = append(toks[:i:i], toks[i+1:]...)
				break
			}
		}
	}
	return toks
}

// withoutListMarker quita la marca de un ítem de lista ("- ", "* ", "+ ",
// "1. ", "1) ") y la de una casilla ("[ ]", "[x]") del principio de t.
func withoutListMarker(t string) string {
	switch {
	case len(t) >= 2 && strings.ContainsRune("-*+", rune(t[0])) && (t[1] == ' ' || t[1] == '\t'):
		t = strings.TrimLeft(t[1:], " \t")
	default:
		i := 0
		for i < len(t) && t[i] >= '0' && t[i] <= '9' {
			i++
		}
		if i > 0 && i+1 < len(t) && (t[i] == '.' || t[i] == ')') && (t[i+1] == ' ' || t[i+1] == '\t') {
			t = strings.TrimLeft(t[i+1:], " \t")
		}
	}
	if len(t) >= 3 && t[0] == '[' && t[2] == ']' && strings.ContainsRune(" xX", rune(t[1])) {
		t = strings.TrimLeft(t[3:], " \t")
	}
	return t
}

// withoutPropertyKey quita la clave de una propiedad ("alt:", "AUTHOR:") del
// principio de t: una palabra seguida de ":" y de un espacio o del final.
func withoutPropertyKey(t string) string {
	for i, r := range t {
		if r == ':' {
			if i > 0 && (i+1 == len(t) || t[i+1] == ' ' || t[i+1] == '\t') {
				return t[i+1:]
			}
			return t
		}
		if !(unicode.IsLetter(r) || (i > 0 && (unicode.IsDigit(r) || r == '-')) || r == '_') {
			return t
		}
	}
	return t
}

// strictSyntaxRune son los signos de la sintaxis strict, que no llegan al
// texto de un elemento.
const strictSyntaxRune = "-|[]:<>@{}*#`\"\\"

// tokens parte s en tramos de letras y dígitos y en signos sueltos que no son
// sintaxis strict.
func tokens(s string) []string {
	var out []string
	word := -1
	for i, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if word < 0 {
				word = i
			}
			continue
		}
		if word >= 0 {
			out = append(out, s[word:i])
			word = -1
		}
		if !unicode.IsSpace(r) && !strings.ContainsRune(strictSyntaxRune, r) {
			out = append(out, string(r))
		}
	}
	if word >= 0 {
		out = append(out, s[word:])
	}
	return out
}

func isWordToken(tok string) bool {
	for _, r := range tok {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	}
	return false
}
