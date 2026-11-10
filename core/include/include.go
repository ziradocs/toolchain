// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package include implementa la primitiva de transclusión del MVP OSS
// (issue #238, decisión 3 del plan): una línea `@include ruta` se reemplaza
// por el contenido del archivo que esa ruta resuelve, recursivamente.
//
// # Por qué expansión textual pre-parse, no una directiva AST
//
// La resolución de rutas relativas necesita un directorio base y acceso al
// filesystem — ninguno de los dos vive en el parser: slidelang.Parse recibe
// el path del archivo raíz solo para estampar AST.FilePath (no para resolver
// includes), y NewDocumentFlexParserWithNormalization (doclang) no recibe
// ningún path en absoluto. Además hay un build WASM sin filesystem real. Una
// directiva a nivel AST no puede resolver ninguna de estas dos cosas.
//
// Por eso Expand corre como paso de BUILD-TIME, fuera del parser, entre leer
// el archivo raíz y parsearlo (ver el wiring en cada build.go de los CLIs).
// Esto tiene una consecuencia deliberada: `fmt` (que solo invoca el parser,
// nunca Expand) preserva `@include` verbatim en vez de expandirlo — el
// artefacto auditable sigue declarando SUS includes, no su expansión.
package include

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// ReadFunc lee el contenido de una ruta ya confinada dentro de baseDir. Cada
// CLI inyecta os.ReadFile; un entorno sin filesystem (p. ej. WASM) puede
// inyectar una función que siempre error-ea, dejando el resto del pipeline
// intacto para documentos sin @include.
type ReadFunc func(path string) ([]byte, error)

// directiveKeyword es la única sintaxis soportada: una línea que, una vez
// recortada de espacios, es exactamente "@include" (sin ruta — error) o
// empieza con "@include " seguido de la ruta. No hay variante `!include` ni
// bloque — una sola forma, simple y grep-eable.
const directiveKeyword = "@include"

// MaxDepth es el límite de anidamiento de @include (a incluye b incluye
// c...) antes de abortar con error — protege contra árboles patológicamente
// profundos incluso sin un ciclo real (que ya se detecta aparte, ver
// visited más abajo).
const MaxDepth = 32

// Expand reemplaza cada línea `@include ruta` de content por el contenido
// (sin su propio frontmatter, si lo tiene) del archivo que ruta resuelve.
//
// baseDir es el directorio de confinamiento — el mismo para TODOS los
// niveles de anidamiento, no el directorio del archivo que contiene cada
// @include (mismo modelo de raíz única que --asset-root en doclang, más
// simple y consistente que resolver relativo-al-contenedor). Cada ruta pasa
// por util.ResolveConfinedPath(baseDir, ruta): rutas absolutas y cualquier
// intento de escapar baseDir (incl. vía symlink) se rechazan.
//
// Un ciclo (a incluye b incluye a) se detecta por RUTA ABSOLUTA RESUELTA —no
// por el texto literal de la ruta, que podría variar entre niveles— y aborta
// con error en vez de recursión infinita.
func Expand(content, baseDir string, read ReadFunc) (string, error) {
	expanded, _, err := ExpandWithOrigins(content, baseDir, read)
	return expanded, err
}

// LineOrigins dice de qué archivo viene cada línea del contenido que
// devuelve ExpandWithOrigins: el elemento i es la línea i+1. Una línea del
// contenido raíz (el que recibió Expand, que no tiene ruta propia) vale "";
// una que vino de un @include, la ruta absoluta del archivo incluido que la
// contiene directamente (el más profundo, si hay anidamiento).
type LineOrigins []string

// File devuelve el archivo de origen de la línea line (1-based), o "" si es
// del contenido raíz o está fuera de rango.
func (o LineOrigins) File(line int) string {
	if line < 1 || line > len(o) {
		return ""
	}
	return o[line-1]
}

// ExpandWithOrigins es Expand más el origen de cada línea expandida.
//
// La expansión es textual y corre antes del parser, así que el AST no sabe
// de qué archivo vino cada elemento; quien necesite resolver algo relativo
// al archivo que lo escribió (p. ej. una imagen dentro de un archivo
// incluido) cruza la línea de la posición del elemento con estos orígenes.
// Las líneas cuentan sobre el contenido ya expandido, con el frontmatter del
// contenido raíz incluido (el de cada archivo incluido se descarta antes de
// contar), que es lo mismo que cuentan las posiciones del parser.
//
// Un archivo incluido que es solo frontmatter, sin salto de línea final,
// deja una línea vacía que no está en él; su origen es el archivo que tiene
// el @include, no el incluido (ver SourceMap). Antes de ExpandWithSourceMap
// esa línea decía el archivo incluido.
//
// El texto expandido y los errores son exactamente los de Expand.
func ExpandWithOrigins(content, baseDir string, read ReadFunc) (string, LineOrigins, error) {
	text, sources, err := ExpandWithSourceMap(content, baseDir, read)
	if err != nil {
		return "", nil, err
	}
	return text, sources.Files(), nil
}

// Origin es de dónde viene una línea del contenido expandido: el archivo
// ("" para el contenido raíz, que no tiene ruta propia; si no, la ruta
// absoluta del archivo incluido que la contiene directamente) y su línea
// (1-based) en ese archivo, contando el frontmatter que el archivo tenía
// aunque la expansión lo descarte.
type Origin struct {
	File string
	Line int
}

// SourceMap es el origen de cada línea del contenido que devuelve
// ExpandWithSourceMap.
//
// Contrato, para cualquier camino de expansión que devuelva un SourceMap
// (también una expansión con opciones, como la que deja sin expandir un
// @include dentro de código):
//
//   - len(Lines) es la cantidad de líneas del texto expandido
//     (strings.Split(texto, "\n")).
//   - Lines[i] es el origen de la línea i+1: con File "" es la línea Line del
//     contenido raíz; si no, la línea Line del archivo File (ruta absoluta),
//     contando el frontmatter que ese archivo tenía.
//   - La línea expandida es exactamente esa línea de su origen. La única
//     excepción es la línea vacía que deja un archivo incluido que es solo
//     frontmatter, sin salto de línea final: no existe en ese archivo, y su
//     origen es la línea del @include que la trajo.
//   - RootLines es la cantidad de líneas del contenido raíz.
//
// TestExpandWithSourceMap_EveryLineMatchesItsOrigin y checkSourceMap (en las
// pruebas del paquete) verifican este contrato y se pueden reusar con otro
// camino de expansión.
type SourceMap struct {
	Lines     []Origin
	RootLines int
}

// At devuelve el origen de la línea line (1-based) del contenido expandido, y
// false si está fuera de rango.
func (m SourceMap) At(line int) (Origin, bool) {
	if line < 1 || line > len(m.Lines) {
		return Origin{}, false
	}
	return m.Lines[line-1], true
}

// Files es m reducido al archivo de cada línea (LineOrigins).
func (m SourceMap) Files() LineOrigins {
	files := make(LineOrigins, len(m.Lines))
	for i, o := range m.Lines {
		files[i] = o.File
	}
	return files
}

// MapDiagnostics devuelve diags con cada posición llevada a su origen. El
// parser y el linter cuentan líneas sobre el contenido expandido; sin esto,
// un diagnóstico después de un @include apunta a una línea que no existe en el
// archivo que el autor tiene abierto.
//
//   - Un diagnóstico en una línea que vino de un @include pasa a decir el
//     archivo incluido (Diagnostic.File) y su línea ahí. File es relativo a
//     docDir, con "/" como separador en cualquier sistema
//     ("partes/intro.doclang", "../comun/pie.doclang"). Si docDir es "" o la
//     ruta no se puede expresar relativa a él (otra unidad en Windows), va la
//     ruta absoluta, también con "/".
//   - Uno del contenido raíz queda con File vacío y la línea que tenía en el
//     contenido raíz.
//   - Uno sin posición (línea 0 o menor) queda como está: es del documento
//     entero.
//   - Uno después de la última línea expandida (un fin de archivo) es del
//     fin del contenido raíz, que es donde termina la expansión: File vacío y
//     RootLines más lo que estaba después del final. Vale también cuando el
//     raíz termina en un @include.
//   - Uno que ya trae File no se toca: ya está en su origen.
//
// La posición final se mapea igual; si cae en otro archivo que la inicial, se
// descarta, porque un rango no puede cruzar archivos. diags no se modifica,
// tampoco la EndPosition a la que apuntan.
//
// Contrato de docDir para quien lo llame: es el directorio del documento
// raíz, el mismo que el del docPath que el reporte pone en document.path y
// en el artifactLocation de SARIF, para que File se lea relativo a él. Los
// dos tienen que estar en la misma forma canónica: si uno pasa por
// filepath.EvalSymlinks (o filepath.Abs), el otro también, y las rutas de
// Origin.File salen de filepath.Abs sobre la ruta confinada a baseDir, así
// que baseDir tiene que estar en esa misma forma; si no, filepath.Rel da un
// "../../…" que da la vuelta por el symlink. Con --base-dir distinto del
// directorio del documento, File sigue siendo relativo al documento (puede
// empezar con "../"), no a baseDir. Si el documento no tiene ruta en disco
// (stdin, un buffer temporal, el MCP con contenido en memoria), docDir es ""
// y File va absoluto.
func (m SourceMap) MapDiagnostics(diags []diagnostics.Diagnostic, docDir string) []diagnostics.Diagnostic {
	if len(diags) == 0 {
		return diags
	}
	out := make([]diagnostics.Diagnostic, len(diags))
	for i, d := range diags {
		if d.File != "" {
			out[i] = d
			continue
		}
		o, ok := m.resolve(d.Position.Line)
		if !ok {
			out[i] = d
			continue
		}
		d.File = displayPath(docDir, o.File)
		d.Position.Line = o.Line
		if d.EndPosition != nil {
			end := *d.EndPosition
			if eo, ok := m.resolve(end.Line); ok && eo.File == o.File {
				end.Line = eo.Line
				d.EndPosition = &end
			} else {
				d.EndPosition = nil
			}
		}
		out[i] = d
	}
	return out
}

// resolve es At extendido más allá del final: una línea después de la última
// es del fin del contenido raíz. Una línea menor que 1, o un mapa sin
// líneas, no tiene origen.
func (m SourceMap) resolve(line int) (Origin, bool) {
	if o, ok := m.At(line); ok {
		return o, true
	}
	if line < 1 || len(m.Lines) == 0 {
		return Origin{}, false
	}
	return Origin{Line: m.RootLines + line - len(m.Lines)}, true
}

// displayPath es cómo se nombra file en un diagnóstico: "" para el contenido
// raíz; si no, relativo a docDir y con "/", o absoluto con "/" si no se puede
// relativo.
func displayPath(docDir, file string) string {
	if file == "" {
		return ""
	}
	if docDir != "" {
		if absDir, err := filepath.Abs(docDir); err == nil {
			if rel, err := filepath.Rel(absDir, file); err == nil {
				return filepath.ToSlash(rel)
			}
		}
	}
	return filepath.ToSlash(file)
}

// ExpandWithSourceMap es Expand más el origen (archivo y línea) de cada línea
// expandida: es la variante para mapear posiciones del parser y del linter al
// archivo que las escribió (SourceMap.MapDiagnostics). El texto expandido y
// los errores son exactamente los de Expand.
func ExpandWithSourceMap(content, baseDir string, read ReadFunc) (string, SourceMap, error) {
	lines, origins, err := expand(content, "", 1, baseDir, read, map[string]bool{}, 0)
	if err != nil {
		return "", SourceMap{}, err
	}
	return strings.Join(lines, "\n"), SourceMap{Lines: origins, RootLines: strings.Count(content, "\n") + 1}, nil
}

// ExpandOptions ajusta la expansión. El valor cero es el comportamiento de
// Expand, ExpandWithOrigins y ExpandWithSourceMap.
type ExpandOptions struct {
	// KeepIncludesInCode deja como está una línea `@include` que cae dentro de
	// un cuerpo de código (CODE strict, cerca de código, :::code-group). El
	// parser guarda esa línea como texto del código, así que con esta opción el
	// build emite lo mismo que el AST y el lint describen (issue #487). doclang
	// la activa; slidelang no, y conserva la expansión en todas partes.
	//
	// Qué es código lo decide, para cada archivo y sobre su propio texto, el
	// AST de ParseDocument, con la gramática de documentos y la normalización
	// de flex prendida, como doclang (build, get_ast, lint): la opción es para
	// documentos (ver ExpandWithOptions). Una fuente .slidelang no se
	// clasifica bien (un SLIDE no es un SECTION).
	//
	// "Código" acá es lo que el AST guarda como bloque de código: el cuerpo de
	// un CODE strict, una cerca de código, una pestaña de code-group. No es lo
	// mismo que el código de RenderContext.LiteralCode, que además cuenta las
	// cercas y los code spans escritos dentro de la prosa: en una cita
	// "> ```" / "> {{x}}" / "> ```", con LiteralCode la {{x}} no se sustituye,
	// pero el AST no tiene un bloque de código ahí, y una línea @include entre
	// esas dos líneas corta la cita y se expande.
	KeepIncludesInCode bool
}

// Expansion es el resultado de ExpandWithOptions.
type Expansion struct {
	// Text es el contenido expandido, el mismo que devolvería Expand con las
	// mismas opciones.
	Text string
	// Origins es el archivo de origen de cada línea de Text (ver LineOrigins).
	Origins LineOrigins
	// Sources es el origen (archivo y línea) de cada línea de Text, como lo
	// devuelve ExpandWithSourceMap; Origins es Sources.Files(). Una línea
	// @include que quedó en código es la línea de su archivo que la tiene.
	Sources SourceMap
	// KeptInCode son las líneas de Text (1-based) con un `@include` que se dejó
	// sin expandir por estar dentro de código (KeepIncludesInCode), para que
	// el llamador lo pueda avisar. Con la opción, toda línea `@include` de Text
	// está en esta lista.
	KeptInCode []int
}

// ExpandWithOptions es ExpandWithOrigins con opciones.
//
// Con KeepIncludesInCode, cada archivo decide sus propios `@include` sobre su
// propio texto, sin expandir nada: como lo lee get_ast de ese archivo. El
// documento raíz, sobre la fuente tal como la leen get_ast y el lint; un
// archivo incluido, sobre su contenido (sin su frontmatter) leído con el
// dialecto del documento raíz (en strict, como cuerpo de una sección si no
// empieza con SECTION). Un `@include` queda como está si el AST de su archivo
// (ParseDocument, con la normalización de flex prendida, como doclang) lo
// tiene dentro de un bloque de código; si no, se expande, y el archivo que
// trae se decide igual. Lo que un archivo hermano o el documento que lo
// incluye abren o cierran no cambia lo que decide un archivo: la expansión
// es textual, así que el AST del resultado puede leer distinto un @include que
// quedó (una cerca que un hermano dejó abierta), pero cada decisión se puede
// comprobar sobre un solo archivo. Los `@include` del frontmatter del raíz se
// expanden antes que nada, y el dialecto sale del frontmatter ya expandido.
// Un `@include` que queda en código no se abre: su ruta no se resuelve ni se
// lee.
//
// Hay un parse por archivo distinto (el de un archivo que se incluye varias
// veces se reutiliza): el costo es lineal en el tamaño de los archivos. Los
// errores salen en el mismo orden que en Expand.
func ExpandWithOptions(content, baseDir string, read ReadFunc, opts ExpandOptions) (Expansion, error) {
	if !opts.KeepIncludesInCode {
		text, sources, err := ExpandWithSourceMap(content, baseDir, read)
		if err != nil {
			return Expansion{}, err
		}
		return Expansion{Text: text, Origins: sources.Files(), Sources: sources}, nil
	}
	root := &includeFrame{visited: map[string]bool{}}
	lines := strings.Split(content, "\n")
	origins := make([]Origin, len(lines))
	frames := make([]*includeFrame, len(lines))
	for i := range lines {
		origins[i] = Origin{Line: i + 1}
		frames[i] = root
	}
	// El frontmatter del raíz primero: no es cuerpo, nunca es código, y de él
	// sale el dialecto.
	for {
		end := frontMatterEnd(lines)
		expand := map[int]bool{}
		for i := 0; i < end; i++ {
			if _, ok := parseIncludeLine(lines[i]); ok {
				expand[i] = true
			}
		}
		if len(expand) == 0 {
			break
		}
		var err error
		if lines, frames, origins, err = expandFrontMatter(lines, frames, origins, expand, baseDir, read); err != nil {
			return Expansion{}, err
		}
	}
	e := &keepExpander{
		baseDir: baseDir,
		read:    read,
		strict:  frontMatterIsStrict(lines),
		cache:   map[string][]bool{},
	}
	e.out = make([]string, 0, len(lines))
	if err := e.emit(lines, frames, origins, includesInCode(nil, lines)); err != nil {
		return Expansion{}, err
	}
	sources := SourceMap{Lines: e.origins, RootLines: strings.Count(content, "\n") + 1}
	return Expansion{Text: strings.Join(e.out, "\n"), Origins: sources.Files(), Sources: sources, KeptInCode: e.kept}, nil
}

// keepExpander arma el resultado de ExpandWithOptions con KeepIncludesInCode.
type keepExpander struct {
	baseDir string
	read    ReadFunc
	strict  bool
	// cache son las líneas @include en código de cada archivo incluido, por
	// su ruta: la decisión depende solo de su texto.
	cache map[string][]bool

	out     []string
	origins []Origin
	kept    []int
}

// emit agrega lines (de los archivos frames, con sus orígenes) al resultado:
// cada @include que su archivo tiene en código (code) queda, y cada uno de los
// otros se reemplaza por su archivo, que se decide sobre su propio texto.
func (e *keepExpander) emit(lines []string, frames []*includeFrame, origins []Origin, code []bool) error {
	for i, line := range lines {
		incPath, ok := parseIncludeLine(line)
		if !ok || code[i] {
			if ok {
				e.kept = append(e.kept, len(e.out)+1)
			}
			e.out = append(e.out, line)
			e.origins = append(e.origins, origins[i])
			continue
		}
		child, err := openInclude(line, incPath, e.baseDir, e.read, frames[i].visited, frames[i].depth)
		if err != nil {
			return err
		}
		childFrames := make([]*includeFrame, len(child.lines))
		childOrigins := make([]Origin, len(child.lines))
		for k := range child.lines {
			childFrames[k] = child
			childOrigins[k] = Origin{File: child.file, Line: child.firstLine + k}
			// La línea vacía que deja un archivo que es solo frontmatter es
			// de la directiva, como en ExpandWithSourceMap.
			if child.frontMatterOnly {
				childOrigins[k] = origins[i]
			}
		}
		childCode, ok := e.cache[child.file]
		if !ok {
			childCode = includesInCode(e.fragmentPrefix(child.lines), child.lines)
			e.cache[child.file] = childCode
		}
		if err := e.emit(child.lines, childFrames, childOrigins, childCode); err != nil {
			return err
		}
	}
	return nil
}

// fragmentPrefix son las líneas que van antes del contenido de un archivo
// incluido para leerlo solo con el dialecto del raíz: en strict, un
// frontmatter strict y, si el archivo no empieza con SECTION, la cabecera de
// una sección que lo contiene.
func (e *keepExpander) fragmentPrefix(lines []string) []string {
	if !e.strict {
		return nil
	}
	prefix := []string{parser.FrontMatterDelimiter, "mode: strict", parser.FrontMatterDelimiter}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed != "SECTION" && !strings.HasPrefix(trimmed, "SECTION ") {
			prefix = append(prefix, `SECTION "include"`)
		}
		break
	}
	return prefix
}

// expandFrontMatter reemplaza, en orden, cada línea @include del frontmatter
// marcada en expand por el contenido de su archivo, con los chequeos y
// errores de openInclude.
func expandFrontMatter(lines []string, frames []*includeFrame, origins []Origin, expand map[int]bool, baseDir string, read ReadFunc) ([]string, []*includeFrame, []Origin, error) {
	outLines := make([]string, 0, len(lines))
	outFrames := make([]*includeFrame, 0, len(lines))
	outOrigins := make([]Origin, 0, len(lines))
	for i, line := range lines {
		if !expand[i] {
			outLines = append(outLines, line)
			outFrames = append(outFrames, frames[i])
			outOrigins = append(outOrigins, origins[i])
			continue
		}
		incPath, _ := parseIncludeLine(line)
		child, err := openInclude(line, incPath, baseDir, read, frames[i].visited, frames[i].depth)
		if err != nil {
			return nil, nil, nil, err
		}
		for k, l := range child.lines {
			outLines = append(outLines, l)
			outFrames = append(outFrames, child)
			if child.frontMatterOnly {
				outOrigins = append(outOrigins, origins[i])
			} else {
				outOrigins = append(outOrigins, Origin{File: child.file, Line: child.firstLine + k})
			}
		}
	}
	return outLines, outFrames, outOrigins, nil
}

// frontMatterEnd es el índice de la línea `---` que cierra el frontmatter de
// lines, con el mismo criterio que ParseDocument (un BOM y líneas en blanco
// antes del `---` de apertura se toleran; sin cierre no hay frontmatter), o -1.
func frontMatterEnd(lines []string) int {
	open := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if trimmed == "" {
			continue
		}
		if trimmed != parser.FrontMatterDelimiter {
			return -1
		}
		open = i
		break
	}
	if open < 0 {
		return -1
	}
	for i := open + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == parser.FrontMatterDelimiter {
			return i
		}
	}
	return -1
}

// frontMatterIsStrict dice si el frontmatter de lines (frontMatterEnd)
// declara `mode: strict`, el criterio con el que ParseDocument elige el
// dialecto strict.
func frontMatterIsStrict(lines []string) bool {
	end := frontMatterEnd(lines)
	if end < 0 {
		return false
	}
	open := 0
	for strings.TrimSpace(strings.TrimPrefix(lines[open], "\ufeff")) == "" {
		open++
	}
	block := append([]string{parser.FrontMatterDelimiter}, lines[open+1:end+1]...)
	fm, _, _ := (&parser.FrontMatterParser{}).Parse(strings.Join(block, "\n"))
	return fm != nil && fm.Mode == "strict"
}

// includesInCode dice cuáles de las líneas @include de lines quedan dentro de
// un bloque de código del AST que da ParseDocument (con la normalización
// prendida, como doclang) para prefix seguido de lines: el contenido de un
// CODE, de una cerca de código o de una pestaña de code-group, que es lo que
// get_ast muestra como código. El elemento i es la línea i de lines. Para
// reconocerlas en el AST, cada una lleva en la copia que se parsea una marca
// única al final; el texto no cambia. Sin ninguna línea @include no se parsea.
func includesInCode(prefix, lines []string) []bool {
	inCode := make([]bool, len(lines))
	var pending []int
	for i, line := range lines {
		if _, ok := parseIncludeLine(line); ok {
			pending = append(pending, i)
		}
	}
	if len(pending) == 0 {
		return inCode
	}
	text := strings.Join(prefix, "\n") + "\n" + strings.Join(lines, "\n")
	tagPrefix := "zzinclude"
	for strings.Contains(text, tagPrefix) {
		tagPrefix += "z"
	}
	tagged := make([]string, 0, len(prefix)+len(lines))
	tagged = append(tagged, prefix...)
	tagged = append(tagged, lines...)
	for _, i := range pending {
		tagged[len(prefix)+i] = lines[i] + " " + tagPrefix + strconv.Itoa(i) + "q"
	}
	doc, _ := parser.New(util.NewNoop()).ParseDocument(strings.Join(tagged, "\n"), "")
	tag := regexp.MustCompile(regexp.QuoteMeta(tagPrefix) + `(\d+)q`)
	mark := func(content string) {
		for _, m := range tag.FindAllStringSubmatch(content, -1) {
			if i, err := strconv.Atoi(m[1]); err == nil && i < len(inCode) {
				inCode[i] = true
			}
		}
	}
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch c := n.(type) {
		case *ast.CodeElement:
			mark(c.Content)
		case *ast.CodeGroupElement:
			for _, b := range c.CodeBlocks {
				mark(b.Content)
			}
		}
		return nil
	})
	return inCode
}

// includeFrame es un archivo incluido: sus líneas sin el frontmatter, su
// ruta, la línea del archivo en la que empiezan (firstLine, más de 1 si se le
// quitó el frontmatter) y la cadena de archivos que lo incluyó (para detectar
// ciclos) con su profundidad. frontMatterOnly marca un archivo que es solo
// frontmatter, sin salto de línea después del `---` de cierre: deja una línea
// vacía que no está en él y cuyo origen es la directiva @include.
type includeFrame struct {
	lines           []string
	file            string
	firstLine       int
	frontMatterOnly bool
	visited         map[string]bool
	depth           int
}

// openInclude resuelve y lee el archivo de una línea `@include`, con los
// mismos chequeos y errores que expand, en el mismo orden: ruta faltante,
// confinamiento, ciclo, lectura y profundidad.
func openInclude(line, incPath, baseDir string, read ReadFunc, visited map[string]bool, depth int) (*includeFrame, error) {
	if incPath == "" {
		return nil, fmt.Errorf("include: %q — falta la ruta", strings.TrimSpace(line))
	}
	resolved, err := util.ResolveConfinedPath(baseDir, incPath)
	if err != nil {
		return nil, fmt.Errorf("include %q: %w", incPath, err)
	}
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return nil, fmt.Errorf("include %q: %w", incPath, err)
	}
	if visited[absResolved] {
		return nil, fmt.Errorf("include %q: ciclo detectado (ya incluido en esta misma cadena)", incPath)
	}
	data, err := read(resolved)
	if err != nil {
		return nil, fmt.Errorf("include %q: %w", incPath, err)
	}
	if depth+1 > MaxDepth {
		return nil, fmt.Errorf("include: profundidad máxima (%d) excedida — revisar si hay un ciclo no detectado", MaxDepth)
	}
	nextVisited := make(map[string]bool, len(visited)+1)
	for k := range visited {
		nextVisited[k] = true
	}
	nextVisited[absResolved] = true
	body, skipped := stripFrontMatterLines(string(data))
	return &includeFrame{
		lines:           strings.Split(body, "\n"),
		file:            absResolved,
		firstLine:       skipped + 1,
		frontMatterOnly: skipped > 0 && skipped == strings.Count(string(data), "\n")+1,
		visited:         nextVisited,
		depth:           depth + 1,
	}, nil
}

// expand devuelve las líneas expandidas de content y, en paralelo, el
// origen de cada una; file es el archivo de content ("" para la raíz) y
// firstLine la línea de ese archivo en la que empieza content (más de 1 si se
// le quitó el frontmatter). Unir las líneas con "\n" da lo mismo que unir
// cada hijo ya expandido como un solo bloque, que es lo que hacía esta
// función antes de devolver orígenes. Es la expansión de Expand,
// ExpandWithOrigins y ExpandWithSourceMap, que no miran regiones de código;
// con KeepIncludesInCode la expansión es la de ExpandWithOptions.
func expand(content, file string, firstLine int, baseDir string, read ReadFunc, visited map[string]bool, depth int) ([]string, []Origin, error) {
	if depth > MaxDepth {
		return nil, nil, fmt.Errorf("include: profundidad máxima (%d) excedida — revisar si hay un ciclo no detectado", MaxDepth)
	}

	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	origins := make([]Origin, 0, len(lines))
	for n, line := range lines {
		incPath, ok := parseIncludeLine(line)
		if !ok {
			out = append(out, line)
			origins = append(origins, Origin{File: file, Line: firstLine + n})
			continue
		}
		child, err := openInclude(line, incPath, baseDir, read, visited, depth)
		if err != nil {
			return nil, nil, err
		}
		childLines, childOrigins, err := expand(strings.Join(child.lines, "\n"), child.file, child.firstLine, baseDir, read, child.visited, child.depth)
		if err != nil {
			return nil, nil, err
		}
		// Un archivo que es solo frontmatter, sin salto de línea después del
		// `---` de cierre, deja una línea vacía que no está en él: esa línea
		// ocupa el lugar del @include, así que se atribuye a la directiva.
		// Cambia también ExpandWithOrigins para esa línea: antes decía el
		// archivo incluido, ahora el que tiene el @include.
		if child.frontMatterOnly {
			childOrigins[0] = Origin{File: file, Line: firstLine + n}
		}
		out = append(out, childLines...)
		origins = append(origins, childOrigins...)
	}
	return out, origins, nil
}

// parseIncludeLine reconoce una línea `@include ruta`, tolerando indentación
// y espacio extra alrededor de la ruta. ok=false si la línea no es una
// directiva include (debe pasar sin tocar). El caso "@include" sin ruta (con
// o sin espacio final — strings.TrimSpace ya se lo come) devuelve ok=true,
// path="": SÍ es la directiva, solo que está mal formada; el caller decide
// qué hacer con eso (error "falta la ruta"), no parseIncludeLine.
func parseIncludeLine(line string) (path string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == directiveKeyword {
		return "", true
	}
	prefix := directiveKeyword + " "
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)), true
}

// stripFrontMatterLines descarta el frontmatter de un archivo incluido, si lo
// tiene, para no splicear un segundo bloque `---` a mitad del documento
// fusionado. Mismo algoritmo EXACTO que parser.FrontMatterParser.Parse
// (delimitador parser.FrontMatterDelimiter en su propia línea, buscado desde
// la línea 1), reimplementado acá deliberadamente en vez de invocar el
// parser completo: Expand corre ANTES de que exista ningún *parser.Parser
// (ver el docstring del paquete) y solo necesita descartar el bloque, no
// interpretar su YAML.
//
// Devuelve también la cantidad de líneas que quitó, para que los orígenes
// sigan contando las líneas del archivo incluido.
func stripFrontMatterLines(content string) (string, int) {
	if !strings.HasPrefix(content, parser.FrontMatterDelimiter) {
		return content, 0
	}
	lines := strings.Split(content, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == parser.FrontMatterDelimiter {
			return strings.Join(lines[i+1:], "\n"), i + 1
		}
	}
	// Delimitador de apertura sin cierre: frontmatter malformado en el
	// archivo incluido. Expand no tiene pipeline de diagnósticos propio (corre
	// antes del parser) — se deja el contenido intacto; el `---` huérfano
	// termina dentro del documento expandido y el parser lo reportará con su
	// propio diagnóstico al toparse con él fuera de posición.
	return content, 0
}
