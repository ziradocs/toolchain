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
	"strings"

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
// Con KeepIncludesInCode, qué es código lo decide el parser
// (parser.CodeLiteralLines) sobre el documento tal como va quedando: se
// expande un `@include` a la vez, el primero en orden que no cae en código,
// y se vuelve a preguntar. Así el contenido de un archivo incluido cuenta
// para lo que sigue (un archivo que abre una cerca deja como código un
// `@include` posterior dentro de ella), y el dialecto sale del frontmatter
// ya expandido: un `mode: strict` que trae un @include del frontmatter cuenta
// igual que uno escrito ahí. Un `@include` que queda en código no se abre, así
// que su ruta no se resuelve ni se lee.
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
	for {
		next := -1
		var code []bool
		for i, line := range lines {
			if _, ok := parseIncludeLine(line); !ok {
				continue
			}
			if code == nil {
				code = parser.CodeLiteralLines(strings.Join(lines, "\n"))
			}
			if !code[i] {
				next = i
				break
			}
		}
		if next < 0 {
			break
		}
		incPath, _ := parseIncludeLine(lines[next])
		frame := frames[next]
		child, err := openInclude(lines[next], incPath, baseDir, read, frame.visited, frame.depth)
		if err != nil {
			return Expansion{}, err
		}
		childFrames := make([]*includeFrame, len(child.lines))
		for i := range childFrames {
			childFrames[i] = child
		}
		lines = append(lines[:next], append(child.lines, lines[next+1:]...)...)
		frames = append(frames[:next], append(childFrames, frames[next+1:]...)...)
	}

	origins := make(LineOrigins, len(lines))
	var kept []int
	for i, line := range lines {
		origins[i] = frames[i].file
		if _, ok := parseIncludeLine(line); ok {
			kept = append(kept, i+1)
		}
	}
	return Expansion{Text: strings.Join(lines, "\n"), Origins: origins, KeptInCode: kept}, nil
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
