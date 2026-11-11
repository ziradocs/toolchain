// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// treeTokens son las líneas con las que FuzzKeepIncludesInCodeTree arma
// archivos: cercas de código, de diagrama, de chart y de math, $$,
// code-groups (también con :::code-item legado), bloques ::: de prosa, una
// cita, una lista, un CODE strict, prosa y @include (cada uno de un archivo
// distinto, para que no haya ciclos y cada línea @include sea única). "texto"
// y "    x" llevan un número único, para seguir cada una hasta el resultado.
var treeTokens = []string{
	"```", "```js", "````", "texto", "", "- item", ":::note", ":::", ":::code-group",
	"```js [a]", "  CODE js", "    x", "SECTION \"S\"", "## H", "@include", "  @include",
	"$$", "```mermaid", "```chart", "```math", "> cita", ":::code-item{0}", "::: info",
}

const treeFiles = 8

// oracleBody es el contenido de un archivo incluido sin su frontmatter: si
// empieza con "---", lo que sigue a la siguiente línea "---".
func oracleBody(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	lines := strings.Split(content, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return content
}

// oracleInCode es la regla de KeepIncludesInCode escrita aparte y sin marcas:
// cada línea @include es única, así que está en código si el AST de su
// archivo (ParseDocument, normalización prendida) la tiene en el contenido de
// un bloque de código. Un archivo incluido se lee con el dialecto del raíz y,
// en strict, como cuerpo de una sección si no empieza con SECTION.
func oracleInCode(lines []string, strict, root bool) map[int]bool {
	var head []string
	if strict && !root {
		head = []string{"---", "mode: strict", "---"}
		for _, l := range lines {
			if t := strings.TrimSpace(l); t != "" {
				if t != "SECTION" && !strings.HasPrefix(t, "SECTION ") {
					head = append(head, `SECTION "x"`)
				}
				break
			}
		}
	}
	doc, _ := parser.New(util.NewNoop()).ParseDocument(strings.Join(append(head, lines...), "\n"), "")
	var code []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch c := n.(type) {
		case *ast.CodeElement:
			code = append(code, c.Content)
		case *ast.CodeGroupElement:
			for _, b := range c.CodeBlocks {
				code = append(code, b.Content)
			}
		}
		return nil
	})
	in := map[int]bool{}
	for i, l := range lines {
		if _, ok := parseIncludeLine(l); !ok {
			continue
		}
		for _, c := range code {
			for _, cl := range strings.Split(c, "\n") {
				if strings.TrimSpace(cl) == strings.TrimSpace(l) {
					in[i] = true
				}
			}
		}
	}
	return in
}

// oracleExpand expande lines con oracleInCode, recursivamente y sin caché.
// No cierra nada: lo que ExpandWithOptions agrega al final de un archivo
// incluido se compara aparte (withoutClosers y lineClasses).
func oracleExpand(lines []string, files map[string]string, strict, root bool) []string {
	code := oracleInCode(lines, strict, root)
	var out []string
	for i, l := range lines {
		path, ok := parseIncludeLine(l)
		if !ok || code[i] {
			out = append(out, l)
			continue
		}
		out = append(out, oracleExpand(strings.Split(oracleBody(files["/base/"+path]), "\n"), files, strict, false)...)
	}
	return out
}

// balancedBlocks dice si src tiene tantas líneas que abren un bloque :::
// (":::note", ":::code-group") como líneas ":::" que cierran.
func balancedBlocks(src string) bool {
	open := 0
	for _, l := range strings.Split(src, "\n") {
		switch t := strings.TrimSpace(l); {
		case t == ":::":
			open--
		case strings.HasPrefix(t, ":::"):
			open++
		}
	}
	return open == 0
}

// withoutClosers es res.Text sin las líneas de cierre de OpenAtEnd.
func withoutClosers(res Expansion) string {
	drop := map[int]bool{}
	for _, b := range res.OpenAtEnd {
		for k := range b.Closers {
			drop[b.Line+k] = true
		}
	}
	var out []string
	for i, l := range strings.Split(res.Text, "\n") {
		if !drop[i+1] {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// lineClass es cómo lee el AST una línea de contenido: en código (un bloque o
// una pestaña), en otro cuerpo literal, en prosa, o en ninguna parte.
type lineClass int

const (
	classNone lineClass = iota
	classProse
	classCode
	// classBody: dentro de un cuerpo literal que no es código (diagrama,
	// chart, mapa, math).
	classBody
	// classDirective: una línea @include que el AST lee como su directiva.
	classDirective
)

var treeTag = regexp.MustCompile(`(?:texto|x)\d+t|@include f\d+\.md`)

// lineClasses es la clase de cada línea marcada (treeTag) de src en el AST de
// ParseDocument, sin mirar cómo decide ExpandWithOptions.
func lineClasses(src string) map[string]lineClass {
	doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "")
	out := map[string]lineClass{}
	mark := func(s string, c lineClass) {
		for _, m := range treeTag.FindAllString(s, -1) {
			if c > out[m] {
				out[m] = c
			}
		}
	}
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch e := n.(type) {
		case *ast.CodeElement:
			mark(e.Content, classCode)
		case *ast.CodeGroupElement:
			for _, b := range e.CodeBlocks {
				mark(b.Content, classCode)
			}
		case *ast.MermaidElement:
			mark(e.Content, classBody)
		case *ast.PlantUMLElement:
			mark(e.Content, classBody)
		case *ast.MathElement:
			mark(e.Content, classBody)
		case *ast.ChartElement, *ast.MapElement:
			b, _ := json.Marshal(e)
			mark(string(b), classBody)
		case *ast.TextElement:
			mark(e.Content, classProse)
		case *ast.QuoteElement:
			mark(e.Content, classProse)
		case *ast.SpecialBlockElement:
			if len(e.Elements) == 0 {
				mark(e.Content, classProse)
			}
		case *ast.ColumnElement:
			if len(e.Elements) == 0 {
				mark(e.Content, classProse)
			}
		case *ast.GridElement:
			mark(e.Content, classProse)
		case *ast.PointItem:
			mark(e.Content, classProse)
		case *ast.ChecklistItem:
			mark(e.Content, classProse)
		case *ast.DirectiveNode:
			if path, ok := e.Parameters["path"].(string); ok {
				out["@include "+path] = classDirective
			}
		}
		return nil
	})
	return out
}

// FuzzKeepIncludesInCodeTree arma un documento y hasta ocho archivos, algunos
// con frontmatter, con @include hermanos y anidados de varias líneas, en strict
// y en flex, y exige:
//
//   - que ExpandWithOptions, sin las líneas de cierre que agrega, dé
//     exactamente lo que da la regla de KeepIncludesInCode escrita aparte
//     (oracleExpand), con cada línea en su origen;
//   - en flex, que cada línea marcada del raíz y de cada archivo incluido
//     tenga en el AST del resultado la clase (código, prosa o ninguna) que
//     tiene en el AST de su propio archivo, leído solo. Es lo que el cierre
//     al final de un archivo tiene que lograr, dicho sin repetir cómo se
//     decide. Si ExpandWithOptions avisa un bloque que no pudo cerrar, la
//     propiedad no se exige: el aviso es la salida.
func FuzzKeepIncludesInCodeTree(f *testing.F) {
	f.Add([]byte{0, 14, 3, 14, 7}, false, uint8(0), false)
	f.Add([]byte{1, 3, 0, 14, 14, 3, 7, 0, 3}, false, uint8(5), false)
	f.Add([]byte{12, 10, 11, 15, 3, 15, 14}, true, uint8(2), false)
	f.Add([]byte{8, 9, 14, 0, 7, 6, 1, 14, 0, 7, 3}, false, uint8(255), false)
	// Los casos de la revisión de #519: un archivo que abre una cerca que el
	// raíz cerraba, code-groups dentro de bloques ::: y el :::code-item legado.
	f.Add([]byte{14, 3, 0, 1, 0, 3}, false, uint8(0), true)
	f.Add([]byte{14, 0, 14, 3, 22, 8, 9, 3}, false, uint8(0), true)
	f.Add([]byte{14, 0, 14, 3, 8, 21, 0, 3}, false, uint8(0), true)
	f.Add([]byte{14, 16, 14, 3, 17, 18, 19, 3}, false, uint8(0), true)
	// Dos líneas por archivo: el raíz es "@include f1.md" y prosa, f1 abre
	// una cerca js. Sin el cierre, la prosa del raíz cae en la cerca.
	f.Add([]byte{14, 3, 1, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}, false, uint8(0), true)
	f.Add([]byte{14, 3, 16, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}, false, uint8(0), true)
	f.Add([]byte{14, 3, 8, 9, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}, false, uint8(0), true)
	f.Fuzz(func(t *testing.T, picks []byte, strict bool, withFrontMatter uint8, isolate bool) {
		if len(picks) > 64 {
			picks = picks[:64]
		}
		bodies := make([][]string, treeFiles+1)
		next := 1
		for n, p := range picks {
			file := n * (treeFiles + 1) / max(len(picks), 1)
			tok := treeTokens[int(p)%len(treeTokens)]
			if tok == "texto" || tok == "    x" {
				tok += fmt.Sprintf("%dt", n)
			}
			if strings.HasSuffix(tok, "@include") {
				if next > treeFiles {
					tok = "texto"
				} else {
					tok += fmt.Sprintf(" f%d.md", next)
					next++
				}
			}
			if isolate && strings.HasSuffix(strings.TrimSpace(tok), ".md") {
				// Con isolate, cada @include va entre líneas vacías y cada
				// archivo termina con una: lo de antes y lo de después no se
				// pega al archivo (un párrafo que sigue en la línea de al
				// lado, un ítem de lista).
				bodies[file] = append(bodies[file], "", tok, "")
				continue
			}
			bodies[file] = append(bodies[file], tok)
		}
		files := map[string]string{}
		for i := 1; i <= treeFiles; i++ {
			content := strings.Join(bodies[i], "\n")
			if isolate {
				content += "\n"
			}
			if withFrontMatter&(1<<(i-1)) != 0 {
				content = "---\ntitle: f\n---\n" + content
			}
			files[fmt.Sprintf("/base/f%d.md", i)] = content
		}
		root := strings.Join(bodies[0], "\n")
		if strict {
			root = "---\nmode: strict\ntitle: T\n---\n\n" + root
		}
		res, err := ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
		if err != nil {
			t.Fatalf("ExpandWithOptions: %v", err)
		}
		if want := strings.Join(oracleExpand(strings.Split(root, "\n"), files, strict, true), "\n"); withoutClosers(res) != want {
			t.Fatalf("got:\n%s\nwant:\n%s\nfiles: %q", res.Text, want, files)
		}
		checkExpansion(t, "tree", root, files, res)
		if strict && len(res.OpenAtEnd) > 0 {
			t.Fatalf("strict: nothing is closed, got %+v", res.OpenAtEnd)
		}
		kept := 0
		for _, l := range strings.Split(res.Text, "\n") {
			if _, ok := parseIncludeLine(l); ok {
				kept++
			}
		}
		if kept != len(res.KeptInCode) {
			t.Fatalf("%d @include lines in the text, %d in KeptInCode", kept, len(res.KeptInCode))
		}
		if strict {
			return
		}
		for _, b := range res.OpenAtEnd {
			if b.Line == 0 {
				return
			}
		}
		// Lo que espera cada línea es su clase en su archivo. La propiedad es
		// de los archivos cuyo @include el padre lee como directiva: uno que
		// cae dentro de un cuerpo literal del padre (math, diagrama, chart)
		// puede cambiar dónde termina ese cuerpo (un ```` dentro de un
		// ```mermaid lo cierra), y eso no es de lo que se cierra al final de
		// un archivo. Con uno así, no se exige.
		want := map[string]lineClass{}
		var walk func(src string)
		walk = func(src string) {
			classes := lineClasses(src)
			for _, tag := range treeTag.FindAllString(src, -1) {
				want[tag] = classes[tag]
				var n int
				if _, err := fmt.Sscanf(tag, "@include f%d.md", &n); err == nil && classes[tag] == classDirective {
					walk(oracleBody(files[fmt.Sprintf("/base/f%d.md", n)]))
				}
			}
		}
		walk(root)
		for tag, c := range want {
			if strings.HasPrefix(tag, "@include") && c != classDirective && c != classCode {
				return
			}
		}
		// Una línea puede cambiar de clase sin que ningún archivo deje nada
		// abierto, por lo que la rodea en el texto expandido: un párrafo del
		// padre en la línea de al lado se traga la apertura del archivo (un
		// $$ queda como texto del párrafo), la normalización de flex reescribe
		// lo que sigue a un :::code-item suelto, o un bloque ::: de prosa que
		// un archivo deja abierto (no se cierra) lee distinto las cercas de
		// adentro. Eso no es de lo que se cierra al final de un archivo, así
		// que la propiedad se exige con isolate (cada @include entre líneas
		// vacías), sin :::code-item y con los ::: de cada archivo parejos.
		if !isolate || strings.Contains(res.Text, ":::code-item") || !balancedBlocks(root) {
			return
		}
		for _, content := range files {
			if !balancedBlocks(content) {
				return
			}
		}
		// Código y otro cuerpo literal cuentan como lo mismo: una cerca
		// ```math sin cierre el parser la lee como código, y cerrada, como
		// math; las dos se leen al pie de la letra.
		literal := func(c lineClass) lineClass {
			if c == classBody {
				return classCode
			}
			return c
		}
		got := lineClasses(res.Text)
		for _, tag := range treeTag.FindAllString(res.Text, -1) {
			if literal(got[tag]) != literal(want[tag]) {
				t.Fatalf("%s is %v in the result and %v in its own file:\n%s\nfiles: %q", tag, got[tag], want[tag], res.Text, files)
			}
		}
	})
}
