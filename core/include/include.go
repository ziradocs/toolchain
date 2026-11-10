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

// expand devuelve las líneas expandidas de content y, en paralelo, el
// origen de cada una; file es el archivo de content ("" para la raíz) y
// firstLine la línea de ese archivo en la que empieza content (más de 1 si se
// le quitó el frontmatter). Unir las líneas con "\n" da lo mismo que unir
// cada hijo ya expandido como un solo bloque, que es lo que hacía esta
// función antes de devolver orígenes.
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
		if incPath == "" {
			return nil, nil, fmt.Errorf("include: %q — falta la ruta", strings.TrimSpace(line))
		}

		resolved, err := util.ResolveConfinedPath(baseDir, incPath)
		if err != nil {
			return nil, nil, fmt.Errorf("include %q: %w", incPath, err)
		}
		absResolved, err := filepath.Abs(resolved)
		if err != nil {
			return nil, nil, fmt.Errorf("include %q: %w", incPath, err)
		}
		if visited[absResolved] {
			return nil, nil, fmt.Errorf("include %q: ciclo detectado (ya incluido en esta misma cadena)", incPath)
		}

		data, err := read(resolved)
		if err != nil {
			return nil, nil, fmt.Errorf("include %q: %w", incPath, err)
		}

		nextVisited := make(map[string]bool, len(visited)+1)
		for k := range visited {
			nextVisited[k] = true
		}
		nextVisited[absResolved] = true

		body, skipped := stripFrontMatterLines(string(data))
		childLines, childOrigins, err := expand(body, absResolved, skipped+1, baseDir, read, nextVisited, depth+1)
		if err != nil {
			return nil, nil, err
		}
		// Un archivo que es solo frontmatter, sin salto de línea después del
		// `---` de cierre, deja una línea vacía que no está en él: esa línea
		// ocupa el lugar del @include, así que se atribuye a la directiva.
		// Cambia también ExpandWithOrigins para esa línea: antes decía el
		// archivo incluido, ahora el que tiene el @include.
		if skipped > 0 && skipped == strings.Count(string(data), "\n")+1 {
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
