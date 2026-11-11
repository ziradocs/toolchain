// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/fence"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// OpenBlock es un bloque que un archivo incluido abre y deja abierto al
// terminar (ver ExpandWithOptions con KeepIncludesInCode).
type OpenBlock struct {
	// Line es la línea de Text (1-based) de la primera línea de cierre que
	// se agregó, o 0 si el bloque no se pudo cerrar.
	Line int
	// Closers son las líneas de cierre que se agregaron después del archivo,
	// en orden (vacío si no se pudo cerrar). Su origen en Sources es
	// Directive.
	Closers []string
	// Directive es la línea del @include que trajo el archivo, en el archivo
	// que la tiene (File "" para el documento raíz), y Path la ruta como
	// está escrita en ella.
	Directive Origin
	Path      string
	// Opener es la línea que abre el bloque, en el archivo incluido; Opening
	// su texto sin espacios alrededor (```js, $$, :::code-group), y
	// OpenerLine su línea en Text (1-based).
	Opener     Origin
	Opening    string
	OpenerLine int
}

// OpenAtEndDiagnostics es un aviso por cada bloque de OpenAtEnd, sin regla
// (el llamador le pone la suya), en líneas de Text, para llevarlos a su origen
// con Sources.MapDiagnostics como los del parser. Uno cerrado va en su primera
// línea de cierre, cuyo origen es la línea del @include, y nombra el archivo
// incluido y la línea que lo abre. Uno que no se pudo cerrar va en la línea
// que lo abre, en el archivo incluido, y nombra el @include. Los archivos se
// nombran como Diagnostic.File: relativos a docDir, con "/" (ver
// SourceMap.MapDiagnostics).
func (x Expansion) OpenAtEndDiagnostics(docDir string) []diagnostics.Diagnostic {
	out := make([]diagnostics.Diagnostic, 0, len(x.OpenAtEnd))
	for _, b := range x.OpenAtEnd {
		if b.Line > 0 {
			file := displayPath(docDir, b.Opener.File)
			out = append(out, diagnostics.NewWarning(
				fmt.Sprintf("%s leaves %q open (%s:%d): an included file is a container, so it is closed where that file ends and the lines after the @include are not inside it. Close it in %s", file, b.Opening, file, b.Opener.Line, file),
				diagnostics.NewPosition(b.Line, 1), "include"))
			continue
		}
		where := fmt.Sprintf("line %d", b.Directive.Line)
		if b.Directive.File != "" {
			where = fmt.Sprintf("%s:%d", displayPath(docDir, b.Directive.File), b.Directive.Line)
		}
		out = append(out, diagnostics.NewWarning(
			fmt.Sprintf("%q is left open at the end of this file (included by \"@include %s\" at %s), and it could not be closed there: the lines after the @include are read inside it. Close it in this file", b.Opening, b.Path, where),
			diagnostics.NewPosition(b.OpenerLine, 1), "include"))
	}
	return out
}

// maxContextRounds es cuántas veces, como mucho, se parsea el texto expandido
// para comprobar en contexto los cierres que se agregaron
// (checkClosersInContext). Cada vuelta corrige, como mucho, un archivo cuyos
// cierres cambian lo que sigue: lo normal es una o dos. Si se acaban, los cierres que quedan sin comprobar se
// quitan. Es variable para las pruebas.
var maxContextRounds = 8

// onContextParse, si no es nil, se llama en cada parse del texto expandido de
// checkClosersInContext: las pruebas cuentan con él cuántas vueltas hubo.
var onContextParse func()

// openInfo es lo que keepExpander guarda de cada bloque de e.open para
// comprobarlo en contexto: dónde van sus cierres en e.out (end: la posición
// después de la última línea del archivo), las líneas del archivo y si sus
// cierres los dio gate sin parsear (pending).
type openInfo struct {
	end     int
	file    string
	lines   []string
	glued   string
	pending bool
	// gone: en contexto no deja nada abierto; sale de OpenAtEnd al final.
	gone bool
}

// checkClosersInContext comprueba los cierres en el texto expandido, con un
// parse para todos los archivos: después de cada archivo cerrado va una línea
// de prosa marcada antes de sus cierres, que tiene que caer dentro del cuerpo
// abierto, y otra después, que tiene que caer en prosa. Lo que rodea al
// archivo puede leer distinto su apertura (un párrafo del padre en la línea de
// al lado se traga un $$): si la primera cae en prosa, el archivo no deja nada
// abierto ahí y se le quitan los cierres. Si cae en el cuerpo pero la segunda
// no cae en prosa, los cierres no sirven: si eran de gate, se buscan con el
// parser sobre el archivo solo (parsedClosureOf) y se comprueban en el parse
// siguiente; si no, se quitan y el bloque queda como uno que no se pudo
// cerrar. Solo hay parse si algún archivo se cerró.
func (e *keepExpander) checkClosersInContext() {
	defer e.dropGone()
	verified := map[int]bool{}
	for round := 0; ; round++ {
		var closed []int
		for k, b := range e.open {
			if b.Line > 0 {
				closed = append(closed, k)
			}
		}
		if len(closed) == 0 {
			return
		}
		if round == maxContextRounds {
			drop := map[int][]string{}
			for _, k := range closed {
				if !verified[k] {
					drop[k] = nil
				}
			}
			if len(drop) > 0 {
				e.setClosers(drop)
			}
			return
		}
		tagPrefix := includeTag(strings.Join(e.out, "\n"))
		before := map[int][]int{} // posición en out → bloques cuya marca va antes
		for _, k := range closed {
			before[e.info[k].end] = append(before[e.info[k].end], k)
		}
		after := map[int][]int{} // índice en out de la última línea de cierre → bloques
		for _, k := range closed {
			last := e.open[k].Line - 1 + len(e.open[k].Closers) - 1
			after[last] = append(after[last], k)
		}
		text := make([]string, 0, len(e.out)+4*len(closed))
		textLine := make([]int, len(e.out)) // índice en out → línea en text
		markLine := map[int]int{}
		for j, line := range e.out {
			for _, k := range before[j] {
				text = append(text, "", fmt.Sprintf("%s%de", tagPrefix, 2*k))
				markLine[2*k] = len(text)
			}
			text = append(text, line)
			textLine[j] = len(text)
			for _, k := range after[j] {
				text = append(text, "", fmt.Sprintf("%s%de", tagPrefix, 2*k+1))
			}
		}
		if onContextParse != nil {
			onContextParse()
		}
		doc, _ := parser.New(util.NewNoop()).ParseDocument(strings.Join(text, "\n"), "")
		got, starts := sentinelLandings(doc, tagPrefix)
		at := func(n int) where {
			if l, ok := got[n]; ok {
				return l.where
			}
			return nowhere
		}
		// ownBody: la marca de antes de los cierres de k cayó en el cuerpo que
		// abre la línea de apertura de k, y no en otro (uno que abrió algo
		// antes del archivo). Si la normalización corrió las líneas, vale que
		// la línea en la que empieza el cuerpo diga lo mismo que la apertura.
		ownBody := func(k int) bool {
			start := got[2*k].start
			if at(2*k) == nowhere {
				i := sort.SearchInts(starts, markLine[2*k]) - 1
				if i < 0 {
					return false
				}
				start = starts[i]
			}
			opener := textLine[e.open[k].OpenerLine-1]
			return start == opener || start >= 1 && start <= len(text) && strings.TrimSpace(text[start-1]) == e.open[k].Opening
		}
		// Los cierres de un archivo cambian cómo se lee lo que sigue: en esta
		// vuelta se decide hasta el primer archivo cuyos cierres hay que
		// cambiar, y los de después se vuelven a comprobar en la siguiente.
		// Quitar los cierres de un archivo que no deja nada abierto, cuando
		// después de ellos se sigue en prosa (eran texto), no cambia lo que
		// sigue: esa vuelta sigue.
		change := map[int][]string{}
		var notOpen []int
		stop := false
		for _, k := range closed {
			if stop {
				break
			}
			if _, done := change[k]; done {
				continue
			}
			switch {
			case at(2*k) == inProse || !ownBody(k):
				// En contexto el archivo no deja abierto lo que abre: o no
				// se traga nada, o lo de después cae en un cuerpo que abrió
				// otro.
				change[k] = nil
				notOpen = append(notOpen, k)
				stop = at(2*k+1) != inProse
			case at(2*k+1) == inProse:
				verified[k] = true
			case e.info[k].pending:
				stop = true
				prefix := []string(nil)
				if e.info[k].glued != "" {
					prefix = []string{e.info[k].glued}
				}
				c := parsedClosureOf(prefix, e.info[k].lines)
				// El mismo archivo, pegado a lo mismo, se cierra igual en
				// todas sus copias: se corrigen todas en esta vuelta.
				for k2, info := range e.info {
					same := k2 == k || info.pending && info.file == e.info[k].file && info.glued == e.info[k].glued &&
						e.open[k2].Line > 0 && slices.Equal(e.open[k2].Closers, e.open[k].Closers)
					if !same {
						continue
					}
					e.info[k2].pending = false
					change[k2] = c.closers
					if c.opener < 0 {
						notOpen = append(notOpen, k2)
					}
				}
			default:
				stop = true
				change[k] = nil
			}
		}
		if len(change) == 0 {
			return
		}
		e.setClosers(change)
		for _, k := range notOpen {
			e.info[k].gone = true
		}
		// Si solo se quitaron cierres que eran texto, lo demás ya se
		// comprobó en esta vuelta: no hace falta otro parse.
		if !stop {
			return
		}
	}
}

// dropGone quita de e.open los bloques que en contexto no dejan nada abierto:
// no se avisan.
func (e *keepExpander) dropGone() {
	open, info := e.open[:0:0], e.info[:0:0]
	for k := range e.open {
		if !e.info[k].gone {
			open, info = append(open, e.open[k]), append(info, e.info[k])
		}
	}
	e.open, e.info = open, info
}

// setClosers cambia las líneas de cierre de los bloques de change (índice en
// e.open → cierres nuevos, nil para quitarlas) y corre las demás líneas, las
// de KeptInCode y las de cada bloque.
func (e *keepExpander) setClosers(change map[int][]string) {
	closerOf := make([]int, len(e.out))
	for j := range closerOf {
		closerOf[j] = -1
	}
	for k, b := range e.open {
		for n := range b.Closers {
			closerOf[b.Line-1+n] = k
		}
	}
	insertAt := map[int][]int{}
	for k := range change {
		insertAt[e.info[k].end] = append(insertAt[e.info[k].end], k)
	}
	for _, ks := range insertAt {
		slices.Sort(ks)
	}
	newIndex := make([]int, len(e.out)+1)
	out := make([]string, 0, len(e.out))
	origins := make([]Origin, 0, len(e.origins))
	newLine := map[int]int{}
	insert := func(j int) {
		for _, k := range insertAt[j] {
			e.info[k].end = len(out)
			if len(change[k]) > 0 {
				newLine[k] = len(out) + 1
			}
			for _, c := range change[k] {
				out, origins = append(out, c), append(origins, e.open[k].Directive)
			}
		}
	}
	for j := range e.out {
		insert(j)
		newIndex[j] = len(out)
		if k := closerOf[j]; k >= 0 {
			if _, changed := change[k]; changed {
				continue
			}
			if j == e.open[k].Line-1 {
				newLine[k] = len(out) + 1
			}
		}
		out, origins = append(out, e.out[j]), append(origins, e.origins[j])
	}
	insert(len(e.out))
	newIndex[len(e.out)] = len(out)
	for k := range e.open {
		if _, changed := change[k]; !changed {
			e.info[k].end = newIndex[e.info[k].end]
		}
	}
	for k, n := range e.kept {
		e.kept[k] = newIndex[n-1] + 1
	}
	for k := range e.open {
		b := &e.open[k]
		b.OpenerLine = newIndex[b.OpenerLine-1] + 1
		if c, changed := change[k]; changed {
			b.Closers = c
		}
		b.Line = newLine[k]
	}
	e.out, e.origins = out, origins
}

// landed es dónde cayó una línea marcada: where y, si cayó en un cuerpo,
// start, la línea (1-based) en la que empieza ese cuerpo.
type landed struct {
	where where
	start int
}

// sentinelLandings dice dónde cayó, en doc, cada línea marcada tagPrefix+Ne
// (por N): en prosa, en un cuerpo literal o, si no está, en ninguna parte; y
// starts, las líneas en las que empieza cada cuerpo, en orden.
func sentinelLandings(doc *ast.AST, tagPrefix string) (map[int]landed, []int) {
	tag := regexp.MustCompile(regexp.QuoteMeta(tagPrefix) + `(\d+)e`)
	out := map[int]landed{}
	var starts []int
	forEachText(doc, func(n ast.Element, w where, _ bodyKind, contents ...string) {
		start := 0
		if w == inBody {
			start = n.GetPosition().Line
			starts = append(starts, start)
		}
		for _, c := range contents {
			if !strings.Contains(c, tagPrefix) {
				continue
			}
			for _, m := range tag.FindAllStringSubmatch(c, -1) {
				if k, err := strconv.Atoi(m[1]); err == nil {
					// inBody gana: una marca que también aparece en prosa (el
					// texto crudo de un contenedor) está en el cuerpo.
					if cur, ok := out[k]; !ok || w == inBody && cur.where != inBody || cur.where == nowhere {
						out[k] = landed{w, start}
					}
				}
			}
		}
	})
	slices.Sort(starts)
	return out, starts
}

// forEachText llama a f con el texto de cada elemento de doc que puede
// guardar una línea: los cuerpos que se leen al pie de la letra (inBody, con
// su clase) y la prosa (inProse). Un bloque ::: o una columna con elementos
// adentro no cuentan como prosa: su Content es el texto crudo de todos ellos,
// también el de un code-group.
func forEachText(doc *ast.AST, f func(n ast.Element, w where, k bodyKind, contents ...string)) {
	marshal := func(v any) string {
		data, _ := json.Marshal(v)
		return string(data)
	}
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch c := n.(type) {
		case *ast.CodeElement:
			f(c, inBody, bodyFence, c.Content)
		case *ast.CodeGroupElement:
			contents := make([]string, len(c.CodeBlocks))
			for k, b := range c.CodeBlocks {
				contents[k] = b.Content
			}
			f(c, inBody, bodyCodeGroup, contents...)
		case *ast.MermaidElement:
			f(c, inBody, bodyFence, c.Content)
		case *ast.PlantUMLElement:
			f(c, inBody, bodyFence, c.Content)
		case *ast.ChartElement:
			f(c, inBody, bodyFence, marshal(c))
		case *ast.MapElement:
			f(c, inBody, bodyFence, marshal(c))
		case *ast.MathElement:
			f(c, inBody, bodyMath, c.Content)
		case *ast.TextElement:
			f(c, inProse, bodyNone, c.Content)
		case *ast.QuoteElement:
			f(c, inProse, bodyNone, c.Content)
		case *ast.SpecialBlockElement:
			if len(c.Elements) == 0 {
				f(c, inProse, bodyNone, c.Content)
			}
		case *ast.ColumnElement:
			if len(c.Elements) == 0 {
				f(c, inProse, bodyNone, c.Content)
			}
		case *ast.GridElement:
			f(c, inProse, bodyNone, c.Content)
		case *ast.PointItem:
			f(nil, inProse, bodyNone, c.Content)
		case *ast.ChecklistItem:
			f(nil, inProse, bodyNone, c.Content)
		}
		return nil
	})
}

// closure es lo que un archivo incluido deja abierto al terminar, leído solo:
// closers cierra el bloque que abre su línea opener (índice en sus líneas), o
// failed si ningún cierre lo cierra. opener -1: no deja nada abierto.
type closure struct {
	closers []string
	opener  int
	failed  bool
	// pending: gate encontró algo abierto y está seguro de cómo se cierra, y
	// no se parseó; lo comprueba checkClosersInContext en el texto expandido.
	pending bool
}

// bodyKind es qué cuerpo, de los que se leen al pie de la letra, abre una
// línea.
type bodyKind int

const (
	bodyNone bodyKind = iota
	bodyFence
	bodyMath
	bodyCodeGroup
)

// openState es lo que gate encuentra abierto al final de unas líneas.
type openState struct {
	kind bodyKind
	// diagram: una cerca de diagrama, chart o mapa, que cierra también con
	// ```` (las de código, solo con ```); uml: un @startuml, que cierra con
	// @enduml.
	diagram bool
	uml     bool
	opener  int    // índice de la línea que lo abre
	open    string // secuencia de apertura de una cerca de math, o "$$"
	tab     int    // backticks de la pestaña abierta de un code-group, o 0
	items   int    // :::code-item abiertos dentro de un code-group
}

// gate lee lines con las reglas de apertura y cierre del parser flex para los
// cuerpos que se leen al pie de la letra (una cerca de código, diagrama, chart
// o mapa; una de math, ```math, ````math o ~~~math; un bloque $$; un
// :::code-group con sus pestañas y sus :::code-item) y dice qué queda abierto
// al final. No parsea: es lo que decide, sin costo, que un archivo no deja
// nada abierto. Cuando no puede seguir al parser (una etiqueta <<...>>, una
// cerca con sangría, que puede ser texto de un ítem de lista) devuelve
// sure=false y el archivo se parsea.
func gate(lines []string, afterProse bool) (st openState, sure bool) {
	st, sure = openState{kind: bodyNone, opener: -1}, true
	// prose: la línea anterior es texto de un párrafo (afterProse: la que va
	// antes del archivo en el documento); blocks: bloques ::: de prosa
	// abiertos. Un $$ ahí es texto para el parser, no math.
	prose, blocks := afterProse, 0
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			prose = false
			continue
		}
		afterProse := prose
		prose = false
		indented := t != line && (line[0] == ' ' || line[0] == '\t')
		switch st.kind {
		case bodyFence:
			if st.uml && strings.HasPrefix(t, "@enduml") || !st.uml && (t == "```" || st.diagram && t == "````") {
				st = openState{kind: bodyNone, opener: -1}
			}
			continue
		case bodyMath:
			if st.open == "$$" {
				// Como parseDollarForm: cierra la línea que termina en $$, y
				// una línea --- termina el bloque.
				if strings.HasSuffix(t, "$$") || t == "---" {
					st = openState{kind: bodyNone, opener: -1}
				}
			} else if len(t) >= len(st.open) && strings.Trim(t, st.open[:1]) == "" {
				st = openState{kind: bodyNone, opener: -1}
			}
			continue
		case bodyCodeGroup:
			switch {
			case st.tab > 0:
				if fence.Closes(t, st.tab) {
					st.tab = 0
				}
			case fence.Open(t) > 0:
				st.tab = fence.Open(t)
			case strings.HasPrefix(t, ":::code-item"):
				st.items++
			case t == ":::" || t == "::::":
				if st.items > 0 {
					st.items--
				} else {
					st = openState{kind: bodyNone, opener: -1}
				}
			}
			continue
		}
		switch {
		case strings.HasPrefix(t, "<<"):
			sure = false
		case strings.HasPrefix(t, "@startuml"):
			// Como el parser de PlantUML: hasta la línea que empieza con
			// @enduml.
			st = openState{kind: bodyFence, opener: i, uml: true}
		case strings.HasPrefix(t, "::::code-group") || strings.HasPrefix(t, ":::code-group"):
			st = openState{kind: bodyCodeGroup, opener: i}
		case t == ":::" || t == "::::":
			blocks = max(blocks-1, 0)
			continue
		case strings.HasPrefix(t, ":::"):
			blocks++
			continue
		case mathOpening(t) != "":
			// Como parseFenceForm: es math solo si cierra más adelante; si no,
			// una cerca de backticks es de código (cierra con ``` exacto) y
			// una de ~~~ es texto. Dentro de un bloque ::: de prosa el parser
			// la lee distinto: gate no está seguro.
			open := mathOpening(t)
			if blocks > 0 {
				sure = false
			}
			switch {
			case mathFenceCloses(lines[i+1:], open):
				st = openState{kind: bodyMath, opener: i, open: open}
			case open[0] == '`':
				st = openState{kind: bodyFence, opener: i}
			default:
				prose = true
				continue
			}
		case strings.HasPrefix(t, "$$"):
			rest := strings.TrimSpace(strings.TrimPrefix(t, "$$"))
			if !strings.HasSuffix(rest, "$$") || rest == "$$" {
				st = openState{kind: bodyMath, opener: i, open: "$$"}
			}
			if afterProse || blocks > 0 {
				sure = false
			}
		case fence.Open(t) > 0:
			st = openState{kind: bodyFence, opener: i, diagram: diagramFence(t)}
		default:
			prose = !strings.HasPrefix(t, "#")
			continue
		}
		if indented {
			sure = false
		}
	}
	return st, sure
}

// mathFenceCloses dice si alguna de lines cierra una cerca de math abierta con
// open: el mismo carácter, al menos tantas veces, y nada más.
func mathFenceCloses(lines []string, open string) bool {
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if len(t) >= len(open) && strings.Trim(t, open[:1]) == "" {
			return true
		}
	}
	return false
}

// diagramFence dice si trimmed abre una cerca de diagrama, chart o mapa, con
// la regla de sus parsers (```mermaid…, ```plantuml…, ```chart, ```map, con
// tres o cuatro backticks): cierran en ``` o en ````.
func diagramFence(trimmed string) bool {
	info := strings.TrimPrefix(strings.TrimPrefix(trimmed, "````"), "```")
	if info == trimmed || strings.HasPrefix(info, "`") {
		return false
	}
	return strings.HasPrefix(info, "mermaid") || strings.HasPrefix(info, "plantuml") || info == "chart" || info == "map"
}

// proseLine dice si line es texto de un párrafo para gate: una línea con texto
// que no es un encabezado ni abre o cierra ninguno de los bloques que gate
// sigue. Un $$ en la línea de abajo es texto del párrafo.
func proseLine(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return false
	}
	for _, p := range []string{"```", "~~~", "$$", ":::", "<<", "@startuml"} {
		if strings.HasPrefix(t, p) {
			return false
		}
	}
	return true
}

// closers son las líneas que cierran lo que st tiene abierto: ``` una cerca de
// código, diagrama, chart o mapa; @enduml un @startuml; $$ un bloque $$; y un
// code-group, ``` su pestaña abierta, ::: cada :::code-item abierto y ::: el
// grupo, en ese orden. Una cerca de math nunca queda abierta: sin cierre más
// adelante, el parser la lee como código (```math, ````math: cierra con ```)
// o como texto (~~~math), ver gate.
func (st openState) closers() []string {
	switch st.kind {
	case bodyFence:
		if st.uml {
			return []string{"@enduml"}
		}
		return []string{"```"}
	case bodyMath:
		return []string{st.open}
	case bodyCodeGroup:
		var c []string
		if st.tab > 0 {
			c = append(c, "```")
		}
		for range st.items {
			c = append(c, ":::")
		}
		return append(c, ":::")
	}
	return nil
}

// closureOf decide qué deja abierto un archivo incluido (lines, leído con el
// dialecto del raíz: prefix) y con qué se cierra. Lo que importa es lo que
// lee el parser. gate decide sin parsear que un archivo no deja nada abierto
// o, si está seguro, qué deja abierto y con qué se cierra: eso lo comprueba un
// parse del texto expandido (checkClosersInContext), uno para todos los
// archivos. Si gate no está seguro, parsedClosureOf lo decide con el parser
// sobre el archivo solo. ~~~ solo cuenta como apertura de una cerca de math:
// el parser flex no tiene cercas de código con ~~~.
//
// Si glued, en el documento la línea anterior al archivo es texto de un
// párrafo (proseLine), sin línea vacía en medio: el archivo se lee después de
// ella, porque el párrafo puede tragarse su primera línea.
func closureOf(prefix, lines []string, glued string) closure {
	if glued != "" {
		prefix = append(slices.Clip(prefix), glued)
	}
	st, sure := gate(lines, glued != "")
	none := closure{opener: -1}
	if st.kind == bodyNone && sure {
		return none
	}
	if sure {
		return closure{closers: st.closers(), opener: st.opener, pending: true}
	}
	return parsedClosureOf(prefix, lines)
}

// parsedClosureOf es closureOf con el parser, para un archivo en el que gate no
// está seguro o cuyos cierres no sirvieron en el texto expandido: un parse sin
// cierres dice qué cuerpo se traga una línea puesta después del archivo y qué
// línea lo abre, y otro con los cierres comprueba que lo cierran.
func parsedClosureOf(prefix, lines []string) closure {
	none := closure{opener: -1}
	var st openState
	sentinel := includeTag(strings.Join(lines, "\n")) + "end"
	// Qué deja abierto, sin cierres: un cierre puede cambiar cómo se lee la
	// apertura (una cerca de math busca su cierre más adelante).
	where, kind, opener := findOpen(prefix, lines, sentinel)
	if where == inProse {
		return none
	}
	if opener < 0 {
		return closure{opener: lastOpener(lines), failed: true}
	}
	// Los cierres salen de lo que el parser dice que está abierto: una cerca
	// (de código, diagrama, chart o mapa) cierra con ```, aunque gate la lea
	// como math (dentro de un bloque ::: el parser no busca el cierre de math);
	// un @startuml con @enduml; un bloque de math con $$; un code-group con lo
	// que gate encuentra abierto adentro.
	opening := strings.TrimSpace(lines[opener])
	switch {
	case kind == bodyFence && strings.HasPrefix(opening, "```"):
		st = openState{kind: bodyFence}
	case kind == bodyFence && strings.HasPrefix(opening, "@startuml"):
		st = openState{kind: bodyFence, uml: true}
	case kind == bodyMath && strings.HasPrefix(opening, "$$"):
		st = openState{kind: bodyMath, open: "$$"}
	case kind == bodyCodeGroup:
		st, _ = gate(lines[opener:], false)
	default:
		st = openState{kind: bodyNone}
	}
	if st.kind != kind {
		return closure{opener: opener, failed: true}
	}
	c := st.closers()
	if _, closed := landing(prefix, lines, c, sentinel); closed != inProse {
		return closure{opener: opener, failed: true}
	}
	return closure{closers: c, opener: opener}
}

// lastOpener es la última línea de lines que podría abrir un cuerpo literal,
// para nombrarla en el aviso de un bloque que no se pudo cerrar.
func lastOpener(lines []string) int {
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") || strings.HasPrefix(t, "$$") ||
			strings.HasPrefix(t, ":::") || strings.HasPrefix(t, "<<") {
			return i
		}
	}
	return 0
}

// where es dónde cae, en el AST, una línea de prosa puesta al final de un
// archivo: en prosa (un párrafo, una cita, un ítem, un bloque ::: de prosa),
// dentro de un cuerpo que se lee al pie de la letra, o en ninguna parte (un
// :::code-group sin pestaña abierta la descarta).
type where int

const (
	inProse where = iota
	inBody
	nowhere
)

// landing parsea, en un solo parse, prefix + lines, una línea vacía y una
// marca, closers, otra línea vacía y otra marca, y dice dónde cae cada marca:
// open es la de antes de los cierres (en prosa si lines no deja nada
// abierto) y closed la de después (en prosa si los cierres cierran lo
// abierto).
func landing(prefix, lines, closers []string, sentinel string) (open, closed where) {
	text := make([]string, 0, len(prefix)+len(lines)+len(closers)+4)
	text = append(text, prefix...)
	text = append(text, lines...)
	text = append(text, "", sentinel+"0e")
	text = append(text, closers...)
	text = append(text, "", sentinel+"1e")
	doc, _ := parser.New(util.NewNoop()).ParseDocument(strings.Join(text, "\n"), "")
	got, _ := sentinelLandings(doc, sentinel)
	where := func(k int) where {
		if l, ok := got[k]; ok {
			return l.where
		}
		return nowhere
	}
	return where(0), where(1)
}

// findOpen es landing sin cierres, y además qué cuerpo se traga a sentinel y
// cuál es su línea de apertura en lines (-1 si no es una línea de lines).
func findOpen(prefix, lines []string, sentinel string) (where, bodyKind, int) {
	return locate(prefix, lines, nil, sentinel)
}

func locate(prefix, lines, closers []string, sentinel string) (where, bodyKind, int) {
	text := make([]string, 0, len(prefix)+len(lines)+len(closers)+2)
	text = append(text, prefix...)
	text = append(text, lines...)
	text = append(text, closers...)
	text = append(text, "", sentinel)
	doc, _ := parser.New(util.NewNoop()).ParseDocument(strings.Join(text, "\n"), "")
	sentinelLine := len(text)
	toLines := func(line int) int {
		if i := line - 1 - len(prefix); i >= 0 && i < len(lines) {
			return i
		}
		return -1
	}
	result, kind, opener := nowhere, bodyNone, -1
	lastStart, lastKind := 0, bodyNone
	has := func(s string) bool { return strings.Contains(s, sentinel) }
	forEachText(doc, func(n ast.Element, w where, k bodyKind, contents ...string) {
		if w == inBody {
			line := n.GetPosition().Line
			if line < sentinelLine && line >= lastStart {
				lastStart, lastKind = line, k
			}
			if result != inBody && slices.ContainsFunc(contents, has) {
				result, kind, opener = inBody, k, toLines(line)
			}
			return
		}
		if result == nowhere && slices.ContainsFunc(contents, has) {
			result = inProse
		}
	})
	if result == nowhere {
		// La descartó el último cuerpo que empieza antes de ella.
		kind, opener = lastKind, toLines(lastStart)
	}
	return result, kind, opener
}

// mathOpening es la secuencia de apertura de una cerca de math (```math,
// ````math, ~~~math), con la regla del parser de math, o "".
func mathOpening(trimmed string) string {
	if trimmed == "" || (trimmed[0] != '`' && trimmed[0] != '~') {
		return ""
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == trimmed[0] {
		n++
	}
	if n < 3 || strings.TrimSpace(trimmed[n:]) != "math" {
		return ""
	}
	return trimmed[:n]
}
