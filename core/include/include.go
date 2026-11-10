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
// El texto expandido y los errores son exactamente los de Expand.
func ExpandWithOrigins(content, baseDir string, read ReadFunc) (string, LineOrigins, error) {
	lines, origins, err := expand(content, "", baseDir, read, map[string]bool{}, 0)
	if err != nil {
		return "", nil, err
	}
	return strings.Join(lines, "\n"), origins, nil
}

// ExpandOptions ajusta la expansión. El valor cero es el comportamiento de
// Expand y ExpandWithOrigins.
type ExpandOptions struct {
	// KeepIncludesInCode deja como está una línea `@include` que cae dentro de
	// un cuerpo de código (CODE strict, cerca de código, :::code-group). El
	// parser guarda esa línea como texto del código, así que con esta opción el
	// build emite lo mismo que el AST y el lint describen (issue #487). doclang
	// la activa; slidelang no, y conserva la expansión en todas partes.
	//
	// Qué es código lo decide el AST de ParseDocument, con la gramática de
	// documentos y la normalización de flex prendida, como doclang (build,
	// get_ast, lint): la opción es para documentos. Una fuente .slidelang no se
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
	// KeptInCode son las líneas de Text (1-based) con un `@include` que se dejó
	// sin expandir por estar dentro de código (KeepIncludesInCode), para que
	// el llamador lo pueda avisar. Con la opción, toda línea `@include` de Text
	// está en esta lista.
	KeptInCode []int
}

// ExpandWithOptions es ExpandWithOrigins con opciones.
//
// Con KeepIncludesInCode, un `@include` se decide sobre el texto en el que
// aparece, con los demás `@include` de ese texto todavía sin expandir: los del
// documento raíz, sobre la fuente tal como la leen get_ast y el lint. Queda
// como está si el parser de documentos (ParseDocument, con la normalización
// de flex prendida, como doclang) lo pone dentro de un bloque de código; si
// no, se expande. Las líneas que trae un archivo incluido se deciden en la
// vuelta siguiente, sobre el texto ya expandido, así que un `@include` escrito
// dentro de código en un archivo incluido también queda. Los `@include` del
// frontmatter se expanden antes que nada, y el dialecto sale del frontmatter
// ya expandido. Un `@include` que queda en código no se abre: su ruta no se
// resuelve ni se lee.
//
// Cada vuelta parsea el texto una vez, y hay a lo sumo una vuelta por nivel de
// anidamiento (MaxDepth), más las del frontmatter: el costo es casi lineal en
// el tamaño del resultado.
func ExpandWithOptions(content, baseDir string, read ReadFunc, opts ExpandOptions) (Expansion, error) {
	if !opts.KeepIncludesInCode {
		text, origins, err := ExpandWithOrigins(content, baseDir, read)
		return Expansion{Text: text, Origins: origins}, err
	}
	lines := strings.Split(content, "\n")
	frames := make([]*includeFrame, len(lines))
	root := &includeFrame{visited: map[string]bool{}}
	for i := range frames {
		frames[i] = root
	}
	// kept marca las líneas @include que ya se decidió dejar en código.
	kept := make([]bool, len(lines))
	for {
		var pending []int
		for i, line := range lines {
			if _, ok := parseIncludeLine(line); ok && !kept[i] {
				pending = append(pending, i)
			}
		}
		if len(pending) == 0 {
			break
		}
		// Primero el frontmatter: no es cuerpo del documento, nunca es
		// código, y de él sale el dialecto con el que se lee el resto.
		expand := map[int]bool{}
		if end := frontMatterEnd(lines); end >= 0 {
			for _, i := range pending {
				if i < end {
					expand[i] = true
				}
			}
		}
		if len(expand) == 0 {
			inCode := includesInCode(lines, pending)
			for _, i := range pending {
				if inCode[i] {
					kept[i] = true
				} else {
					expand[i] = true
				}
			}
			if len(expand) == 0 {
				break
			}
		}
		var err error
		if lines, frames, kept, err = expandLines(lines, frames, kept, expand, baseDir, read); err != nil {
			return Expansion{}, err
		}
	}

	origins := make(LineOrigins, len(lines))
	var keptLines []int
	for i := range lines {
		origins[i] = frames[i].file
		if kept[i] {
			keptLines = append(keptLines, i+1)
		}
	}
	return Expansion{Text: strings.Join(lines, "\n"), Origins: origins, KeptInCode: keptLines}, nil
}

// expandLines reemplaza, en orden, cada línea @include marcada en expand por
// el contenido de su archivo, con los chequeos y errores de openInclude.
func expandLines(lines []string, frames []*includeFrame, kept []bool, expand map[int]bool, baseDir string, read ReadFunc) ([]string, []*includeFrame, []bool, error) {
	outLines := make([]string, 0, len(lines))
	outFrames := make([]*includeFrame, 0, len(lines))
	outKept := make([]bool, 0, len(lines))
	for i, line := range lines {
		if !expand[i] {
			outLines = append(outLines, line)
			outFrames = append(outFrames, frames[i])
			outKept = append(outKept, kept[i])
			continue
		}
		incPath, _ := parseIncludeLine(line)
		child, err := openInclude(line, incPath, baseDir, read, frames[i].visited, frames[i].depth)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, l := range child.lines {
			outLines = append(outLines, l)
			outFrames = append(outFrames, child)
			outKept = append(outKept, false)
		}
	}
	return outLines, outFrames, outKept, nil
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

// includesInCode dice cuáles de las líneas @include pending de lines quedan
// dentro de un bloque de código del AST que da ParseDocument (con la
// normalización prendida, como doclang): el contenido de un CODE, de una
// cerca de código o de una pestaña de code-group, que es lo que get_ast
// muestra como código. Para reconocerlas en el AST, cada una lleva en la copia
// que se parsea una marca única al final; el texto del documento no cambia.
func includesInCode(lines []string, pending []int) map[int]bool {
	text := strings.Join(lines, "\n")
	prefix := "zzinclude"
	for strings.Contains(text, prefix) {
		prefix += "z"
	}
	tagged := make([]string, len(lines))
	copy(tagged, lines)
	for _, i := range pending {
		tagged[i] = lines[i] + " " + prefix + strconv.Itoa(i) + "q"
	}
	doc, _ := parser.New(util.NewNoop()).ParseDocument(strings.Join(tagged, "\n"), "")
	tag := regexp.MustCompile(regexp.QuoteMeta(prefix) + `(\d+)q`)
	inCode := map[int]bool{}
	mark := func(content string) {
		for _, m := range tag.FindAllStringSubmatch(content, -1) {
			if i, err := strconv.Atoi(m[1]); err == nil {
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

// includeFrame es un archivo incluido: sus líneas, su ruta y la cadena de
// archivos que lo incluyó (para detectar ciclos) con su profundidad.
type includeFrame struct {
	lines   []string
	file    string
	visited map[string]bool
	depth   int
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
	return &includeFrame{
		lines:   strings.Split(stripFrontMatter(string(data)), "\n"),
		file:    absResolved,
		visited: nextVisited,
		depth:   depth + 1,
	}, nil
}

// expand devuelve las líneas expandidas de content y, en paralelo, el
// archivo de origen de cada una; file es el de content ("" para la raíz).
// Unir las líneas con "\n" da lo mismo que unir cada hijo ya expandido como
// un solo bloque, que es lo que hacía esta función antes de devolver
// orígenes. Es la expansión de Expand y ExpandWithOrigins, que no miran
// regiones de código; con KeepIncludesInCode la expansión es la de
// ExpandWithOptions.
func expand(content, file, baseDir string, read ReadFunc, visited map[string]bool, depth int) ([]string, LineOrigins, error) {
	if depth > MaxDepth {
		return nil, nil, fmt.Errorf("include: profundidad máxima (%d) excedida — revisar si hay un ciclo no detectado", MaxDepth)
	}

	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	origins := make(LineOrigins, 0, len(lines))
	for _, line := range lines {
		incPath, ok := parseIncludeLine(line)
		if !ok {
			out = append(out, line)
			origins = append(origins, file)
			continue
		}
		child, err := openInclude(line, incPath, baseDir, read, visited, depth)
		if err != nil {
			return nil, nil, err
		}
		childLines, childOrigins, err := expand(strings.Join(child.lines, "\n"), child.file, baseDir, read, child.visited, child.depth)
		if err != nil {
			return nil, nil, err
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

// stripFrontMatter descarta el frontmatter de un archivo incluido, si lo
// tiene, para no splicear un segundo bloque `---` a mitad del documento
// fusionado. Mismo algoritmo EXACTO que parser.FrontMatterParser.Parse
// (delimitador parser.FrontMatterDelimiter en su propia línea, buscado desde
// la línea 1), reimplementado acá deliberadamente en vez de invocar el
// parser completo: Expand corre ANTES de que exista ningún *parser.Parser
// (ver el docstring del paquete) y solo necesita descartar el bloque, no
// interpretar su YAML.
func stripFrontMatter(content string) string {
	if !strings.HasPrefix(content, parser.FrontMatterDelimiter) {
		return content
	}
	lines := strings.Split(content, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == parser.FrontMatterDelimiter {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	// Delimitador de apertura sin cierre: frontmatter malformado en el
	// archivo incluido. Expand no tiene pipeline de diagnósticos propio (corre
	// antes del parser) — se deja el contenido intacto; el `---` huérfano
	// termina dentro del documento expandido y el parser lo reportará con su
	// propio diagnóstico al toparse con él fuera de posición.
	return content
}
