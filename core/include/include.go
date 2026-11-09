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
	// el llamador lo pueda avisar.
	KeptInCode []int
}

// ExpandWithOptions es ExpandWithOrigins con opciones.
func ExpandWithOptions(content, baseDir string, read ReadFunc, opts ExpandOptions) (Expansion, error) {
	if !opts.KeepIncludesInCode {
		text, origins, err := ExpandWithOrigins(content, baseDir, read)
		return Expansion{Text: text, Origins: origins}, err
	}
	lines := strings.Split(content, "\n")

	// El frontmatter del contenido raíz se expande como siempre, línea por
	// línea y sin mirar regiones de código: no es cuerpo del documento.
	bodyStart := frontMatterLines(lines)
	out, origins, err := expand(strings.Join(lines[:bodyStart], "\n"), "", baseDir, read, map[string]bool{}, 0)
	if err != nil {
		return Expansion{}, err
	}
	if bodyStart == 0 {
		out, origins = nil, nil
	}

	body, bodyOrigins, kept, err := expandBody(lines[bodyStart:], parser.SourceIsStrict(content), baseDir, read)
	if err != nil {
		return Expansion{}, err
	}
	for i := range kept {
		kept[i] += len(out)
	}
	out = append(out, body...)
	origins = append(origins, bodyOrigins...)
	return Expansion{Text: strings.Join(out, "\n"), Origins: origins, KeptInCode: kept}, nil
}

// frontMatterLines cuenta las líneas del frontmatter de lines (delimitadores
// incluidos), con el mismo criterio que el pre-pase de node-id del parser: la
// primera línea es `---` y el bloque termina en el siguiente `---`, o en el
// fin del archivo si no lo hay. 0 si no hay frontmatter.
func frontMatterLines(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != parser.FrontMatterDelimiter {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == parser.FrontMatterDelimiter {
			return i + 1
		}
	}
	return len(lines)
}

// includeFrame es un archivo en curso de expansión: sus líneas, la próxima a
// leer y la cadena de archivos que lo incluyó (para detectar ciclos).
type includeFrame struct {
	lines   []string
	next    int
	file    string
	visited map[string]bool
	depth   int
}

// expandBody expande el cuerpo del documento (lo que sigue al frontmatter
// raíz) como un solo flujo de líneas: en el orden en que las va a ver el
// parser, con cada archivo incluido en el lugar de su `@include`.
//
// Una línea `@include` que cae dentro de un cuerpo de código (el de un CODE
// strict, una cerca ```, un :::code-group) es código, no una directiva: el
// parser la guarda tal cual en el AST, y get_ast y el lint la muestran así.
// Sin KeepIncludesInCode se expande igual, y el build emite dentro del bloque
// de código el contenido de un archivo que el AST no menciona (issue #487).
// Devuelve también las líneas, 1-based sobre lo que devuelve, de cada
// `@include` que dejó en el código. Qué es código lo
// decide parser.CodeLiteralScanner, recorriendo el texto ya expandido: la
// línea `@include` no está en ese texto (en su lugar van las líneas del
// archivo incluido), así que solo se la mira con Peek, y el escáner sigue con
// lo que de verdad la reemplaza.
func expandBody(lines []string, strict bool, baseDir string, read ReadFunc) ([]string, LineOrigins, []int, error) {
	scanner := parser.NewCodeLiteralScanner(strict)
	out := make([]string, 0, len(lines))
	origins := make(LineOrigins, 0, len(lines))
	var kept []int
	stack := []*includeFrame{{lines: lines, visited: map[string]bool{}}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		if frame.next >= len(frame.lines) {
			stack = stack[:len(stack)-1]
			continue
		}
		line := frame.lines[frame.next]
		frame.next++

		if incPath, ok := parseIncludeLine(line); ok {
			if !scanner.Peek(line) {
				child, err := openInclude(line, incPath, baseDir, read, frame.visited, frame.depth)
				if err != nil {
					return nil, nil, nil, err
				}
				stack = append(stack, child)
				continue
			}
			kept = append(kept, len(out)+1)
		}
		scanner.Next(line)
		out = append(out, line)
		origins = append(origins, frame.file)
	}
	return out, origins, kept, nil
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
// regiones de código; con KeepIncludesInCode solo se usa para el frontmatter
// del contenido raíz y el cuerpo va por expandBody.
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
