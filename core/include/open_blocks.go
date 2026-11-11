// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
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
// que lo abre, en el archivo incluido, y nombra el @include.
func (x Expansion) OpenAtEndDiagnostics() []diagnostics.Diagnostic {
	out := make([]diagnostics.Diagnostic, 0, len(x.OpenAtEnd))
	for _, b := range x.OpenAtEnd {
		if b.Line > 0 {
			out = append(out, diagnostics.NewWarning(
				fmt.Sprintf("%s leaves %q open (line %d of %s): an included file is a container, so it is closed where that file ends and the lines after the @include are not inside it. Close it in %s", b.Path, b.Opening, b.Opener.Line, b.Path, b.Path),
				diagnostics.NewPosition(b.Line, 1), "include"))
			continue
		}
		where := "the document"
		if b.Directive.File != "" {
			where = filepath.Base(b.Directive.File)
		}
		out = append(out, diagnostics.NewWarning(
			fmt.Sprintf("%q is left open at the end of this file (included by \"@include %s\", %s line %d), and it could not be closed there: the lines after the @include are read inside it. Close it in this file", b.Opening, b.Path, where, b.Directive.Line),
			diagnostics.NewPosition(b.OpenerLine, 1), "include"))
	}
	return out
}

// maxContextRounds es cuántas veces, como mucho, se parsea el texto expandido
// para comprobar en contexto los cierres que se agregaron
// (checkClosersInContext).
const maxContextRounds = 3

// checkClosersInContext comprueba los cierres en el texto expandido: cada
// archivo se cierra según lo que lee solo, pero lo que lo rodea puede leer
// distinto su apertura (un párrafo del padre en la línea de al lado se traga
// un $$, y el cierre $$ abre entonces otro bloque). Con una línea de prosa
// marcada después de los cierres de cada archivo, un parse del texto entero
// dice si siguen cayendo en prosa; los que no, se quitan, y el bloque queda
// como uno que no se pudo cerrar. Solo hay parse si algún archivo se cerró, y
// otro solo si hubo que quitar alguno.
func (e *keepExpander) checkClosersInContext() {
	for round := 0; round < maxContextRounds; round++ {
		var closed []int
		for k, b := range e.open {
			if b.Line > 0 {
				closed = append(closed, k)
			}
		}
		if len(closed) == 0 {
			return
		}
		tagPrefix := includeTag(strings.Join(e.out, "\n"))
		after := map[int]int{} // índice en out después del que va la marca → bloque
		for _, k := range closed {
			b := e.open[k]
			after[b.Line-1+len(b.Closers)-1] = k
		}
		text := make([]string, 0, len(e.out)+2*len(closed))
		for j, line := range e.out {
			text = append(text, line)
			if k, ok := after[j]; ok {
				text = append(text, "", fmt.Sprintf("%s%de", tagPrefix, k))
			}
		}
		doc, _ := parser.New(util.NewNoop()).ParseDocument(strings.Join(text, "\n"), "")
		landed := sentinelLandings(doc, tagPrefix)
		var drop []int
		for _, k := range closed {
			if landed[k] != inProse {
				drop = append(drop, k)
			}
		}
		if len(drop) == 0 {
			return
		}
		e.dropClosers(drop)
	}
}

// dropClosers quita del resultado las líneas de cierre de los bloques drop
// (índices en e.open), que quedan como no cerrados, y corre las líneas de los
// demás.
func (e *keepExpander) dropClosers(drop []int) {
	remove := map[int]bool{}
	for _, k := range drop {
		b := &e.open[k]
		for n := range b.Closers {
			remove[b.Line-1+n] = true
		}
		b.Line, b.Closers = 0, nil
	}
	newIndex := make([]int, len(e.out))
	out := e.out[:0:0]
	origins := e.origins[:0:0]
	for j := range e.out {
		newIndex[j] = len(out)
		if !remove[j] {
			out, origins = append(out, e.out[j]), append(origins, e.origins[j])
		}
	}
	for k, n := range e.kept {
		e.kept[k] = newIndex[n-1] + 1
	}
	for k := range e.open {
		b := &e.open[k]
		b.OpenerLine = newIndex[b.OpenerLine-1] + 1
		if b.Line > 0 {
			b.Line = newIndex[b.Line-1] + 1
		}
	}
	e.out, e.origins = out, origins
}

// sentinelLandings dice dónde cayó, en doc, cada línea marcada tagPrefix+Ne
// (por N): en prosa, en un cuerpo literal o, si no está, en ninguna parte.
func sentinelLandings(doc *ast.AST, tagPrefix string) map[int]where {
	tag := regexp.MustCompile(regexp.QuoteMeta(tagPrefix) + `(\d+)e`)
	out := map[int]where{}
	scan := func(content string, w where) {
		if !strings.Contains(content, tagPrefix) {
			return
		}
		for _, m := range tag.FindAllStringSubmatch(content, -1) {
			if k, err := strconv.Atoi(m[1]); err == nil {
				// inBody gana: una marca que también aparece en prosa (el
				// texto crudo de un contenedor) está en el cuerpo.
				if cur, ok := out[k]; !ok || w == inBody || cur == nowhere {
					out[k] = w
				}
			}
		}
	}
	forEachText(doc, func(n ast.Element, w where, _ bodyKind, contents ...string) {
		for _, c := range contents {
			scan(c, w)
		}
	})
	return out
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
	kind   bodyKind
	opener int    // índice de la línea que lo abre
	open   string // secuencia de apertura de una cerca de math, o "$$"
	tab    int    // backticks de la pestaña abierta de un code-group, o 0
	items  int    // :::code-item abiertos dentro de un code-group
}

// gate lee lines con las reglas de apertura y cierre del parser flex para los
// cuerpos que se leen al pie de la letra (una cerca de código, diagrama, chart
// o mapa; una de math, ```math, ````math o ~~~math; un bloque $$; un
// :::code-group con sus pestañas y sus :::code-item) y dice qué queda abierto
// al final. No parsea: es lo que decide, sin costo, que un archivo no deja
// nada abierto. Cuando no puede seguir al parser (una etiqueta <<...>>, una
// cerca con sangría, que puede ser texto de un ítem de lista) devuelve
// sure=false y el archivo se parsea.
func gate(lines []string) (st openState, sure bool) {
	st, sure = openState{kind: bodyNone, opener: -1}, true
	// prose: la línea anterior es texto de un párrafo; blocks: bloques :::
	// de prosa abiertos. Un $$ ahí es texto para el parser, no math.
	prose, blocks := false, 0
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
			if t == "```" {
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
		case strings.HasPrefix(t, "::::code-group") || strings.HasPrefix(t, ":::code-group"):
			st = openState{kind: bodyCodeGroup, opener: i}
		case t == ":::" || t == "::::":
			blocks = max(blocks-1, 0)
			continue
		case strings.HasPrefix(t, ":::"):
			blocks++
			continue
		case mathOpening(t) != "":
			st = openState{kind: bodyMath, opener: i, open: mathOpening(t)}
		case strings.HasPrefix(t, "$$"):
			rest := strings.TrimSpace(strings.TrimPrefix(t, "$$"))
			if !strings.HasSuffix(rest, "$$") || rest == "$$" {
				st = openState{kind: bodyMath, opener: i, open: "$$"}
			}
			if afterProse || blocks > 0 {
				sure = false
			}
		case fence.Open(t) > 0:
			st = openState{kind: bodyFence, opener: i}
		default:
			prose = !strings.HasPrefix(t, "#")
			continue
		}
		if indented {
			sure = false
		}
	}
	// Una cerca ~~~math sin cierre el parser no la lee como math sino como
	// texto (no se traga nada), y lo que sigue a su apertura gate no lo leyó
	// como lo lee el parser.
	if st.kind == bodyMath && strings.HasPrefix(st.open, "~") {
		return openState{kind: bodyNone, opener: -1}, false
	}
	return st, sure
}

// closers son las líneas que cierran lo que st tiene abierto: ``` una cerca de
// código, diagrama, chart o mapa; su misma secuencia de apertura una de math
// (```, ````) y $$ un bloque $$; y un code-group, ``` su pestaña abierta, :::
// cada :::code-item abierto y ::: el grupo, en ese orden. ~~~ no hace falta
// nunca: una cerca ~~~math sin cierre no se traga nada (ver gate).
func (st openState) closers() []string {
	switch st.kind {
	case bodyFence:
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
// lee el parser, así que gate solo evita parsear un archivo que no deja nada
// abierto: si gate encuentra algo abierto, sus cierres se comprueban con un
// parse (landing: una línea de prosa puesta después de los cierres tiene que
// caer en prosa); si no está seguro, o el parse no confirma, se busca con el
// parser qué cuerpo se traga esa línea (findOpen) y se cierra eso,
// comprobándolo con otro parse. ~~~ solo cuenta como apertura de una cerca de
// math: el parser flex no tiene cercas de código con ~~~.
func closureOf(prefix, lines []string) closure {
	st, sure := gate(lines)
	none := closure{opener: -1}
	if st.kind == bodyNone && sure {
		return none
	}
	sentinel := includeTag(strings.Join(lines, "\n")) + "end"
	if st.kind != bodyNone {
		c := st.closers()
		switch open, closed := landing(prefix, lines, c, sentinel); {
		case open == inProse:
			// gate vio abierto algo que el parser no deja abierto.
			return none
		case closed == inProse:
			return closure{closers: c, opener: st.opener}
		}
	}
	// gate no supo o se equivocó: lo que diga el parser.
	where, kind, opener := findOpen(prefix, lines, sentinel)
	if where == inProse {
		return none
	}
	if opener < 0 {
		return closure{opener: lastOpener(lines), failed: true}
	}
	st, _ = gate(lines[opener:])
	// Una cerca de math sin cierre el parser puede leerla como código: gate
	// la ve como math, y se cierra como código.
	if kind == bodyFence && st.kind == bodyMath {
		st = openState{kind: bodyFence}
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
	landed := sentinelLandings(doc, sentinel)
	where := func(k int) where {
		if w, ok := landed[k]; ok {
			return w
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
