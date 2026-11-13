// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// expandKeep expande con KeepIncludesInCode, la opción que activa doclang.
func expandKeep(content, baseDir string, read ReadFunc) (string, error) {
	res, err := ExpandWithOptions(content, baseDir, read, ExpandOptions{KeepIncludesInCode: true})
	return res.Text, err
}

// codeContents parsea src como documento (o presentación) y devuelve el
// contenido de cada bloque de código, en orden.
func codeContents(t *testing.T, src string, document bool) []string {
	t.Helper()
	p := parser.New(util.NewNoop())
	var doc *ast.AST
	if document {
		doc, _ = p.ParseDocument(src, "d.doclang")
	} else {
		doc, _ = p.Parse(src, "d.slidelang")
	}
	var out []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch c := n.(type) {
		case *ast.CodeElement:
			out = append(out, c.Content)
		case *ast.CodeGroupElement:
			for _, b := range c.CodeBlocks {
				out = append(out, b.Content)
			}
		}
		return nil
	})
	return out
}

// Issue #487: un `@include` dentro de un cuerpo de código es código. Expand lo
// deja como está, así que el texto que parsea el build da el mismo bloque de
// código que el texto sin expandir (el que ven get_ast y lint).
func TestExpand_CodeBodiesAreLiteral(t *testing.T) {
	files := map[string]string{
		"/base/other.txt": "    INCLUDED-LINE-1\n    INCLUDED-LINE-2\n",
	}
	strictDoc := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n"
	for _, tc := range []struct {
		name     string
		src      string
		document bool
	}{
		{"strict CODE", strictDoc + "  CODE\n    @include other.txt\n", true},
		{"strict CODE with language", strictDoc + "  CODE yaml\n    a: 1\n\n    @include other.txt\n", true},
		{"strict CODE{verbatim}", strictDoc + "  CODE{verbatim} txt\n    @include other.txt\n", true},
		{"strict CODE, nbsp before the directive", strictDoc + "  CODE\n     @include other.txt\n", true},
		{"strict CODE, ideographic space before the directive", strictDoc + "  CODE\n    　@include other.txt\n", true},
		{"strict code-group", strictDoc + "  :::code-group\n  ```txt [a]\n  @include other.txt\n  ```\n  :::\n", true},
		{"flex fence", "# A\n\n```txt\n@include other.txt\n```\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := expandKeep(tc.src, "/base", memFS(files))
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			if got != tc.src {
				t.Errorf("a code body must not be expanded:\n%s", got)
			}
			before := codeContents(t, tc.src, tc.document)
			if len(before) == 0 || !strings.Contains(strings.Join(before, "\n"), "@include other.txt") {
				t.Fatalf("fixture: the parser should keep the directive as code, got %q", before)
			}
		})
	}
}

// Fuera del código nada cambia: un `@include` a nivel de elemento después de
// un CODE se sigue expandiendo, y uno dentro de un diagrama también (no es
// código; su cuerpo puede venir de un archivo).
func TestExpand_OutsideCodeStillExpands(t *testing.T) {
	files := map[string]string{
		"/base/frag.doclang": "  TEXT\n    Incluido.\n",
		"/base/flow.mmd":     "    A-->B\n",
	}
	strictDoc := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n"
	for _, tc := range []struct {
		name, src, want string
	}{
		{"after a CODE body", strictDoc + "  CODE\n    x = 1\n\n@include frag.doclang\n", strictDoc + "  CODE\n    x = 1\n\n  TEXT\n    Incluido.\n\n"},
		{"at the CODE line's own indentation", strictDoc + "  CODE\n    x = 1\n  @include frag.doclang\n", strictDoc + "  CODE\n    x = 1\n  TEXT\n    Incluido.\n\n"},
		{"inside a diagram", strictDoc + "  <<mermaid>>\n    graph TD\n    @include flow.mmd\n  <<end>>\n", strictDoc + "  <<mermaid>>\n    graph TD\n    A-->B\n\n  <<end>>\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := expandKeep(tc.src, "/base", memFS(files))
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			if got != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

// Qué es código se decide sobre el texto ya expandido: un `@include` escrito
// dentro del cuerpo de un CODE de un archivo incluido queda literal, y uno que
// se expande justo después de un CODE no cambia dónde termina ese cuerpo.
func TestExpand_CodeRegionsFollowTheExpandedText(t *testing.T) {
	files := map[string]string{
		"/base/with-code.doclang": "  CODE\n    @include secret.txt\n",
		"/base/secret.txt":        "    SECRET\n",
		"/base/frag.doclang":      "  TEXT\n    Incluido.\n",
	}
	strictDoc := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n"
	got, err := expandKeep(strictDoc+"@include with-code.doclang\n@include frag.doclang\n", "/base", memFS(files))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := strictDoc + "  CODE\n    @include secret.txt\n\n  TEXT\n    Incluido.\n\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
	if strings.Contains(got, "SECRET") {
		t.Error("an @include inside an included CODE body was expanded")
	}
}

// Los orígenes de línea siguen alineados con el texto expandido cuando parte
// de él es código que no se expandió.
func TestExpandWithOrigins_CodeBodyKeepsLineMap(t *testing.T) {
	files := map[string]string{
		"/base/frag.doclang": "  TEXT\n    Incluido.\n",
	}
	src := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  CODE\n    @include frag.doclang\n\n@include frag.doclang\n"
	res, err := ExpandWithOptions(src, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatalf("ExpandWithOptions: %v", err)
	}
	expanded, origins := res.Text, res.Origins
	if len(res.KeptInCode) != 1 || res.KeptInCode[0] != 8 {
		t.Errorf("KeptInCode %v, want [8]", res.KeptInCode)
	}
	lines := strings.Split(expanded, "\n")
	if len(origins) != len(lines) {
		t.Fatalf("%d origins for %d lines", len(origins), len(lines))
	}
	for i, line := range lines {
		wantFile := ""
		if i >= 9 && i <= 11 {
			wantFile = "/base/frag.doclang"
		}
		if origins[i] != wantFile {
			t.Errorf("line %d %q: origin %q, want %q", i+1, line, origins[i], wantFile)
		}
	}
}

// Una cerca de diagrama (```mermaid) no es código: su @include se expande,
// como el de un <<mermaid>>.
func TestExpand_DiagramFencesAreNotCode(t *testing.T) {
	files := map[string]string{"/base/frag.md": "Incluido."}
	got, err := expandKeep("# A\n\n```mermaid\n@include frag.md\n```\n", "/base", memFS(files))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if want := "# A\n\n```mermaid\nIncluido.\n```\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// El dialecto sale del frontmatter ya expandido: un `mode: strict` que trae un
// @include del frontmatter hace que un CODE strict cuente como código.
func TestExpand_StrictModeFromAnIncludedFrontMatter(t *testing.T) {
	files := map[string]string{
		"/base/mode.yml":  "mode: strict",
		"/base/other.txt": "    INCLUDED",
	}
	src := "---\n@include mode.yml\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  CODE\n    @include other.txt\n"
	got, err := expandKeep(src, "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "INCLUDED") || !strings.Contains(got, "    @include other.txt") {
		t.Errorf("the CODE body must stay literal:\n%s", got)
	}
}

// Sin la opción, la expansión es la de siempre: un @include dentro de código
// también se expande. Es lo que conserva slidelang (y Expand/ExpandWithOrigins
// para cualquier llamador).
func TestExpand_DefaultStillExpandsInsideCode(t *testing.T) {
	files := map[string]string{"/base/other.txt": "    INCLUDED\n"}
	for _, src := range []string{
		"---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n  CODE\n    @include other.txt\n",
		"---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n  CODE\n    @include other.txt\n",
		"# A\n\n```\n@include other.txt\n```\n",
	} {
		got, err := Expand(src, "/base", memFS(files))
		if err != nil || !strings.Contains(got, "INCLUDED") || strings.Contains(got, "@include") {
			t.Errorf("Expand must expand inside code by default: %q, %v", got, err)
		}
		res, err := ExpandWithOptions(src, "/base", memFS(files), ExpandOptions{})
		if err != nil || res.Text != got || len(res.KeptInCode) != 0 {
			t.Errorf("ExpandWithOptions with no options must match Expand: %q %v %v", res.Text, res.KeptInCode, err)
		}
	}
}

// Hallazgo 2 del review de #497: qué es código lo decide el parser, con su
// regla de cercas y su gramática, no un recorrido aparte.
func TestExpand_CodeIsWhatTheParserReadsAsCode(t *testing.T) {
	files := map[string]string{"/base/part2.md": "PART2"}
	strictDoc := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n"
	for _, tc := range []struct {
		name, src string
		kept      bool
	}{
		// La cerca de cuatro sigue abierta en ```` (solo ``` la cierra): el
		// @include es código.
		{"four-backtick fence, ```` inside", "# A\n\n````js\nq\n````\n@include part2.md\n```\n", true},
		// La cerca de cuatro cierra en ```: el @include que sigue no es
		// código y se expande.
		{"four-backtick fence closed by ```", "# A\n\n````js\nq\n```\n@include part2.md\nfin\n", false},
		// Una pestaña de code-group solo cierra en ``` exacto: ```js adentro
		// es contenido.
		{"code-group tab with ```js inside", ":::code-group\n```js [a]\nx\n```js\n@include part2.md\n```\n:::\n", true},
		// ::::code-group (cuatro dos puntos) también es un code-group.
		{"four-colon code-group", "::::code-group\n```js [a]\n@include part2.md\n```\n:::\n", true},
		// Una línea suelta en un code-group no llega al AST: no es código.
		{"stray line in a code-group", ":::code-group\n@include part2.md\n```js [a]\nx\n```\n:::\n", false},
		// En strict, una línea ``` dentro de un TEXT es texto.
		{"``` inside a strict TEXT", strictDoc + "  TEXT\n    ```\n    @include part2.md\n    ```\n", false},
		// En strict, una línea CODE dentro del cuerpo de un TEXT es texto.
		{"CODE inside a strict TEXT body", strictDoc + "  TEXT\n    CODE js\n      @include part2.md\n", false},
		// Un CODE pegado a un TEXT, sin línea en blanco y a su misma
		// sangría, es un elemento propio en el AST: corta el cuerpo del TEXT
		// y su @include es código, como lo muestra get_ast.
		{"CODE right after a TEXT", strictDoc + "  TEXT a\n  CODE js\n    @include part2.md\n", true},
		// El cuerpo flex de un :::bloque tipado lee la cerca como código.
		{"fence inside a typed block", strictDoc + "  :::note{typed}\n  ```js\n  @include part2.md\n  ```\n  :::\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ExpandWithOptions(tc.src, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
			if err != nil {
				t.Fatalf("ExpandWithOptions: %v", err)
			}
			if kept := len(res.KeptInCode) > 0; kept != tc.kept {
				t.Errorf("kept %v, want %v:\n%s", kept, tc.kept, res.Text)
			}
			if kept := strings.Contains(res.Text, "@include part2.md"); kept != tc.kept || kept == strings.Contains(res.Text, "PART2") {
				t.Errorf("text:\n%s", res.Text)
			}
			if got := probeLike(tc.src, "@include part2.md"); got != tc.kept {
				t.Errorf("fixture: the AST of the source has the directive in code=%v", got)
			}
		})
	}
}

// probeLike dice si el AST de src (ParseDocument, con la normalización
// prendida, como doclang) lleva
// directive dentro del contenido de un bloque de código: lo que get_ast
// muestra como código.
func probeLike(src, directive string) bool {
	doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "")
	found := false
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch c := n.(type) {
		case *ast.CodeElement:
			found = found || strings.Contains(c.Content, directive)
		case *ast.CodeGroupElement:
			for _, b := range c.CodeBlocks {
				found = found || strings.Contains(b.Content, directive)
			}
		}
		return nil
	})
	return found
}

// Hallazgo 4 del review de #497: el frontmatter se reconoce como lo reconoce el
// parser, también con un BOM o líneas en blanco antes del `---`, así que un
// CODE strict sigue siendo código.
func TestExpand_StrictFrontMatterAfterBOMOrBlankLines(t *testing.T) {
	files := map[string]string{"/base/other.txt": "    INCLUDED"}
	body := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  CODE\n    @include other.txt\n"
	for _, prefix := range []string{"\ufeff", "\n\n", "\ufeff\n"} {
		got, err := expandKeep(prefix+body, "/base", memFS(files))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, "INCLUDED") {
			t.Errorf("prefix %q: the CODE body must stay literal:\n%s", prefix, got)
		}
	}
}

// Un frontmatter sin cerrar no es frontmatter para ParseDocument: el
// documento se lee como flex desde la primera línea, así que una cerca que
// sigue es código y su @include no se expande. Fija lo que hace el parser.
func TestExpand_UnclosedFrontMatter(t *testing.T) {
	files := map[string]string{"/base/other.txt": "INCLUDED"}
	src := "---\nmode: strict\n\n```\n@include other.txt\n```\n"
	got, err := expandKeep(src, "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if want := probeLike(src, "@include other.txt"); want == strings.Contains(got, "INCLUDED") {
		t.Errorf("the include must follow the AST (in code=%v):\n%s", want, got)
	}
	if !probeLike(src, "@include other.txt") {
		t.Errorf("fixture: ParseDocument reads the fence of an unclosed frontmatter as code")
	}
}

// Un @include que queda en código no se abre: una ruta que no existe o que
// sale de baseDir no es un error.
func TestExpand_KeptIncludeIsNotOpened(t *testing.T) {
	for _, path := range []string{"missing.txt", "../outside.txt", "/etc/passwd"} {
		src := "# A\n\n```\n@include " + path + "\n```\n"
		res, err := ExpandWithOptions(src, "/base", memFS(nil), ExpandOptions{KeepIncludesInCode: true})
		if err != nil || res.Text != src || len(res.KeptInCode) != 1 || res.KeptInCode[0] != 4 {
			t.Errorf("%s: %v %v %q", path, err, res.KeptInCode, res.Text)
		}
	}
}

// Cada @include se decide sobre el texto en el que aparece, con los demás
// @include de ese texto sin expandir: los del documento raíz sobre la fuente,
// como la leen get_ast y el lint. Un archivo incluido que abre una cerca sin
// cerrarla no convierte en código un @include que en la fuente no lo era: los
// dos se expanden, y la cerca cierra donde termina su archivo. Un @include que
// trae ese archivo dentro de su cerca sí se decide sobre el texto ya
// expandido, y queda.
func TestExpand_IncludesAreDecidedOnTheTextTheyAppearIn(t *testing.T) {
	files := map[string]string{
		"/base/open.md":   "```",
		"/base/other.md":  "INCLUDED",
		"/base/nested.md": "```\n@include other.md\n```",
	}
	got, err := expandKeep("# A\n\n@include open.md\n@include other.md\n", "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if want := "# A\n\n```\n```\nINCLUDED\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	got, err = expandKeep("# A\n\n@include nested.md\n", "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if want := "# A\n\n```\n@include other.md\n```\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// "Código" para KeepIncludesInCode es lo que el AST guarda como bloque de
// código. Una cerca escrita dentro de una cita es código para LiteralCode (una
// {{variable}} en "> {{x}}" entre "> ```" no se sustituye), pero el AST no
// tiene ahí un bloque de código, y una línea @include corta la cita: entre dos
// líneas "> ```" se expande.
func TestExpand_FenceInsideAQuoteIsNotCode(t *testing.T) {
	files := map[string]string{"/base/x.md": "INCLUIDO"}
	got, err := expandKeep("# A\n\n> ```\n@include x.md\n> ```\n", "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "INCLUIDO") {
		t.Errorf("the @include between quoted fence lines is not code in the AST and must expand:\n%s", got)
	}
}

// Cada archivo decide sus @include sobre su propio texto: lo que un hermano
// deja abierto no vuelve código un @include que en su archivo es prosa (y la
// cerca del hermano cierra donde termina él).
func TestExpand_EachFileDecidesOnItsOwnText(t *testing.T) {
	files := map[string]string{
		"/base/a.md": "```",
		"/base/b.md": "texto\n@include c.md",
		"/base/c.md": "C",
	}
	got, err := expandKeep("# A\n\n@include a.md\n@include b.md\n", "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if want := "# A\n\n```\n```\ntexto\nC\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Los @include del frontmatter se expanden primero, también con un BOM y
// líneas en blanco antes del ---, y el dialecto sale del frontmatter
// expandido.
func TestExpand_FrontMatterIncludesWithBOM(t *testing.T) {
	files := map[string]string{"/base/mode.yml": "mode: strict", "/base/x.txt": "SECRETO"}
	src := "\ufeff\n\n---\n@include mode.yml\ntitle: T\n---\n\nSECTION \"A\"\n  CODE\n    @include x.txt\n"
	got, err := expandKeep(src, "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "SECRETO") || !strings.Contains(got, "mode: strict") {
		t.Errorf("got %q", got)
	}
}

// El límite de anidamiento es el de Expand: MaxDepth niveles se expanden, uno
// más es un error, con el mismo mensaje.
func TestExpand_MaxDepthLikeExpand(t *testing.T) {
	for _, depth := range []int{MaxDepth, MaxDepth + 1} {
		files := map[string]string{}
		for i := 1; i <= depth; i++ {
			body := fmt.Sprintf("nivel %d", i)
			if i < depth {
				body += fmt.Sprintf("\n@include f%d.md", i+1)
			}
			files[fmt.Sprintf("/base/f%d.md", i)] = body
		}
		want, wantErr := Expand("# A\n\n@include f1.md\n", "/base", memFS(files))
		got, err := expandKeep("# A\n\n@include f1.md\n", "/base", memFS(files))
		if fmt.Sprint(err) != fmt.Sprint(wantErr) || got != want {
			t.Errorf("depth %d: got %v, Expand %v", depth, err, wantErr)
		}
		if (depth > MaxDepth) != (err != nil) {
			t.Errorf("depth %d: error %v", depth, err)
		}
	}
}

// Con varios errores, el que sale es el mismo que en Expand: el primero en el
// orden del documento, también dentro de un archivo incluido.
func TestExpand_ErrorOrderLikeExpand(t *testing.T) {
	files := map[string]string{"/base/a.md": "@include missing-a.md"}
	src := "# A\n\n@include a.md\n@include missing-b.md\n@include ../outside.md\n"
	_, wantErr := Expand(src, "/base", memFS(files))
	_, err := expandKeep(src, "/base", memFS(files))
	if wantErr == nil || fmt.Sprint(err) != fmt.Sprint(wantErr) {
		t.Errorf("got %v, Expand %v", err, wantErr)
	}
}

// Una ReadFunc que devuelve otra cosa en otra lectura no rompe la expansión:
// cada ruta se lee una sola vez y la decisión se toma sobre esas líneas.
func TestExpand_ReadFuncThatChanges(t *testing.T) {
	reads := 0
	read := func(path string) ([]byte, error) {
		reads++
		if reads == 1 {
			return []byte("```\n@include x.md\n```"), nil
		}
		return []byte("otra"), nil
	}
	got, err := ExpandWithOptions("# A\n\n@include a.md\n@include a.md\n", "/base", read, ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := "# A\n\n```\n@include x.md\n```\n```\n@include x.md\n```\n"; got.Text != want || reads != 1 {
		t.Errorf("got %q after %d reads", got.Text, reads)
	}
}

// Los errores del frontmatter salen en el orden de Expand (en profundidad): un
// @include anidado que falla antes que uno posterior del mismo frontmatter.
func TestExpand_FrontMatterErrorOrderLikeExpand(t *testing.T) {
	files := map[string]string{"/base/a.yaml": "@include missing-nested.yaml"}
	src := "---\n@include a.yaml\n@include missing-second.yaml\n---\n\n# A\n"
	_, wantErr := Expand(src, "/base", memFS(files))
	_, err := expandKeep(src, "/base", memFS(files))
	if wantErr == nil || fmt.Sprint(err) != fmt.Sprint(wantErr) || !strings.Contains(fmt.Sprint(err), "missing-nested") {
		t.Errorf("got %v, Expand %v", err, wantErr)
	}
}

// openInclude rechaza pasar de MaxDepth: el nivel MaxDepth se abre, uno más
// es un error.
func TestOpenInclude_DepthLimit(t *testing.T) {
	read := memFS(map[string]string{"/base/a.md": "x"})
	if _, err := openInclude("@include a.md", "a.md", "/base", read, map[string]bool{}, MaxDepth-1); err != nil {
		t.Errorf("depth %d: %v", MaxDepth, err)
	}
	if _, err := openInclude("@include a.md", "a.md", "/base", read, map[string]bool{}, MaxDepth); err == nil {
		t.Errorf("depth %d must be an error", MaxDepth+1)
	}
}

// Un archivo incluido es un contenedor: la cerca, el code-group o el bloque $$
// que deja abierto cierra donde termina él, con las líneas de cierre que pide
// lo que está abierto, cuyo origen es la directiva. Antes la cerca de open.md
// se tragaba la apertura de la cerca del raíz, el @include del raíz (que quedó
// sin expandir por estar en código en su archivo) caía fuera de código en el
// resultado y el build lo dibujaba como
// "<!-- Unsupported element type: *ast.DirectiveNode -->".
func TestExpand_OpenBlockClosesAtTheEndOfItsFile(t *testing.T) {
	root := "# A\n\n@include open.md\n\n```\n@include after.md\n```\n\nFin.\n"
	for _, c := range []struct {
		open    string
		closers []string
		opening string
	}{
		// Un ```mermaid vacío no: en un documento sin frontmatter la
		// normalización lo lee mal también sin @include ("```mermaid", "",
		// "```", "", "```", "X", "```" deja X como prosa), así que el
		// @include del raíz sigue cayendo fuera de código.
		{"```text\n", []string{"```"}, "```text"},
		{"```mermaid\ngraph TD\n", []string{"```"}, "```mermaid"},
		{"```chart\n", []string{"```"}, "```chart"},
		{"```math\n", []string{"```"}, "```math"},
		// Una cerca de math sin cierre más adelante es de código para el
		// parser (parseFenceForm), y cierra con ``` exacto.
		{"````math\nx\n", []string{"```"}, "````math"},
		{"````js\n", []string{"```"}, "````js"},
		{"$$\nx\n", []string{"$$"}, "$$"},
		// PlantUML nativo: hasta la línea que empieza con @enduml.
		{"@startuml\nA -> B\n", []string{"@enduml"}, "@startuml"},
		{":::code-group\n", []string{":::"}, ":::code-group"},
		{":::code-group\n```js [a]\nx\n", []string{"```", ":::"}, ":::code-group"},
		// La sintaxis legada: la pestaña, el code-item y el grupo.
		{":::code-group\n:::code-item{0}\n```\nx\n", []string{"```", ":::", ":::"}, ":::code-group"},
		// Dentro de un bloque ::: de prosa se cierra el cuerpo literal y el
		// bloque de prosa queda abierto.
		{"::: info\n```\n", []string{"```"}, "```"},
		{"::: info\n:::code-group\n```js [a]\nx\n", []string{"```", ":::"}, ":::code-group"},
		{":::note\n:::code-group\n", []string{":::"}, ":::code-group"},
		// Un $$ después de un párrafo es texto del párrafo; lo que queda
		// abierto es la cerca que sigue.
		{"texto\n$$\n````\n$$\n", []string{"```"}, "````"},
	} {
		files := map[string]string{"/base/open.md": c.open, "/base/after.md": "AFTER"}
		res, err := ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
		if err != nil {
			t.Fatalf("%q: %v", c.open, err)
		}
		if len(res.KeptInCode) != 1 || strings.Contains(res.Text, "AFTER") {
			t.Fatalf("%q: the root's @include is code in the root and must stay: %v\n%s", c.open, res.KeptInCode, res.Text)
		}
		if len(res.OpenAtEnd) != 1 {
			t.Fatalf("%q: OpenAtEnd = %+v, want one block", c.open, res.OpenAtEnd)
		}
		b := res.OpenAtEnd[0]
		want := strings.Join(append([]string{"# A", "", strings.TrimSuffix(c.open, "\n"), ""}, c.closers...), "\n") + "\n\n```\n@include after.md\n```\n\nFin.\n"
		if res.Text != want || !slices.Equal(b.Closers, c.closers) || b.Opening != c.opening || b.Path != "open.md" || b.Directive != (Origin{Line: 3}) {
			t.Errorf("%q: got %q %+v, want %q closed with %q", c.open, res.Text, b, want, c.closers)
		}
		lines := strings.Split(res.Text, "\n")
		if code := includesInCode(nil, lines); !code[res.KeptInCode[0]-1] {
			t.Errorf("%q: the kept @include is not code in the AST of the result:\n%s", c.open, res.Text)
		}
		doc, _ := parser.New(util.NewNoop()).ParseDocument(res.Text, "")
		_ = ast.Walk(doc, func(n ast.Node) error {
			if d, ok := n.(*ast.DirectiveNode); ok {
				t.Errorf("%q: directive %q in the AST of the result:\n%s", c.open, d.Name, res.Text)
			}
			if p, ok := n.(*ast.TextElement); ok && strings.Contains(p.Content, "@include") {
				t.Errorf("%q: the kept @include is prose in the AST of the result:\n%s", c.open, res.Text)
			}
			return nil
		})
		checkExpansion(t, c.open, root, files, res)
	}
}

// El cambio de comportamiento: un archivo incluido ya no puede abrir un bloque
// para que lo cierre el documento que lo incluye. header.md abre una cerca js
// y el raíz la cerraba; ahora la cerca cierra al final de header.md, la línea
// de código del raíz queda como prosa y la ``` del raíz abre otra cerca. Queda
// en OpenAtEnd, para que doclang lo avise en la línea del @include.
func TestExpand_ParentNoLongerClosesTheChildsBlock(t *testing.T) {
	files := map[string]string{"/base/header.md": "---\ntitle: h\n---\n```js"}
	root := "# A\n\n@include header.md\nconsole.log(1)\n```\n\nFin.\n"
	res, err := ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := "# A\n\n```js\n```\nconsole.log(1)\n```\n\nFin.\n"; res.Text != want {
		t.Errorf("got %q, want %q", res.Text, want)
	}
	want := []OpenBlock{{Line: 4, Closers: []string{"```"}, Directive: Origin{Line: 3}, Path: "header.md",
		Opener: Origin{File: "/base/header.md", Line: 4}, Opening: "```js", OpenerLine: 3}}
	if !reflect.DeepEqual(res.OpenAtEnd, want) {
		t.Errorf("OpenAtEnd = %+v, want %+v", res.OpenAtEnd, want)
	}
	diags := res.OpenAtEndDiagnostics("/base")
	if len(diags) != 1 || diags[0].Position.Line != 4 || !diags[0].IsWarning() ||
		!strings.Contains(diags[0].Message, "header.md leaves \"```js\" open (header.md:4)") {
		t.Errorf("diagnostics = %+v", diags)
	}
	if mapped := res.Sources.MapDiagnostics(diags, ""); mapped[0].File != "" || mapped[0].Position.Line != 3 {
		t.Errorf("the warning maps to %s:%d, want the @include line (3) of the document", mapped[0].File, mapped[0].Position.Line)
	}
	if res, _ := ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{}); res.OpenAtEnd != nil {
		t.Error("without KeepIncludesInCode nothing is closed")
	}
}

// Lo que rodea al archivo cuenta: m.md abre un $$ que, leído solo, queda
// abierto, pero en el documento sigue a un párrafo, que se traga el $$ como
// texto. Ahí m.md no deja nada abierto, y no se cierra ni se avisa: un $$ de
// cierre abriría un bloque de math.
func TestExpand_ContextDecidesWhatIsOpen(t *testing.T) {
	files := map[string]string{"/base/m.md": "$$\nx\n"}
	root := "# A\n\ntexto previo\n@include m.md\n\nFin.\n"
	res, err := ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := Expand(root, "/base", memFS(files))
	if res.Text != plain || len(res.OpenAtEnd) != 0 {
		t.Errorf("got %q %+v, want the plain expansion %q and nothing open", res.Text, res.OpenAtEnd, plain)
	}
	// Con una línea vacía antes, el $$ abre math y se cierra.
	root = "# A\n\ntexto previo\n\n@include m.md\n\nFin.\n"
	res, err = ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.OpenAtEnd) != 1 || !slices.Equal(res.OpenAtEnd[0].Closers, []string{"$$"}) {
		t.Errorf("OpenAtEnd = %+v, want the $$ closed", res.OpenAtEnd)
	}
}

// Muchos archivos pegados a un párrafo, que en el documento no dejan nada
// abierto (el párrafo se traga su $$), no cuestan una vuelta cada uno: se
// leen después de la línea de párrafo a la que van pegados. Los que se
// corrigen en contexto (un $$ dentro de un ::: info es texto; una ````math sin
// cierre dentro de un ::: info es código y cierra con ```) se corrigen sin
// mover lo demás: el cierre del archivo de antes, el @include que quedó en
// código y las líneas de cada bloque.
func TestExpand_ContextCorrectionsKeepTheRest(t *testing.T) {
	files := map[string]string{"/base/m.md": "$$\nx\n", "/base/js.md": "```js", "/base/k.md": "K", "/base/q.md": "::: info\n````math\nx"}
	var root strings.Builder
	root.WriteString("# A\n\n")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&root, "párrafo %d\n@include m.md\n\n", i)
	}
	root.WriteString("@include js.md\n\n```\n@include k.md\n```\n\n")
	root.WriteString("::: info\n@include m.md\n:::\n\n::: info\n@include m.md\n:::\n\n@include q.md\n\n@include js.md\n\nFin.\n")
	parses := 0
	onContextParse = func() { parses++ }
	t.Cleanup(func() { onContextParse = nil })
	res, err := ExpandWithOptions(root.String(), "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.OpenAtEnd) != 3 || res.OpenAtEnd[0].Path != "js.md" || res.OpenAtEnd[1].Path != "q.md" || res.OpenAtEnd[2].Path != "js.md" {
		t.Fatalf("OpenAtEnd = %+v, want js.md, q.md and js.md closed", res.OpenAtEnd)
	}
	for _, b := range res.OpenAtEnd {
		if !slices.Equal(b.Closers, []string{"```"}) {
			t.Errorf("%s closed with %q, want ```", b.Path, b.Closers)
		}
	}
	lines := strings.Split(res.Text, "\n")
	for _, b := range res.OpenAtEnd {
		if lines[b.Line-1] != "```" || strings.TrimSpace(lines[b.OpenerLine-1]) != b.Opening {
			t.Errorf("%s: line %d is %q and opener line %d is %q", b.Path, b.Line, lines[b.Line-1], b.OpenerLine, lines[b.OpenerLine-1])
		}
	}
	if len(res.KeptInCode) != 1 || lines[res.KeptInCode[0]-1] != "@include k.md" || !includesInCode(nil, lines)[res.KeptInCode[0]-1] {
		t.Errorf("KeptInCode = %v", res.KeptInCode)
	}
	checkExpansion(t, "context", root.String(), files, res)
	// Una vuelta quita los cierres de los dos ::: info (eran texto) y
	// corrige q.md, y la siguiente comprueba q.md y el js.md de después: las
	// diez líneas de párrafo no agregan ninguna.
	if parses > 2 {
		t.Errorf("%d parses of the expanded text, want at most 2", parses)
	}
}

// Un @include dentro de un bloque ::: de prosa del padre: el archivo es
// contenido del bloque, y la cerca que deja abierta cierra antes del ::: del
// padre, que así cierra el bloque.
func TestExpand_IncludeInsideACalloutIsClosed(t *testing.T) {
	files := map[string]string{"/base/js.md": "```js\ncode"}
	root := "# A\n\n::: info\n@include js.md\n:::\n\nFin.\n"
	res, err := ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := "# A\n\n::: info\n```js\ncode\n```\n:::\n\nFin.\n"; res.Text != want || len(res.OpenAtEnd) != 1 {
		t.Errorf("got %q %+v, want %q", res.Text, res.OpenAtEnd, want)
	}
}

// Un cierre no cambia cómo se leen las líneas del propio archivo. ````math
// sin cierre más adelante es, para el parser, una cerca de código que cierra
// la ``` siguiente: el archivo no deja nada abierto, y el párrafo sigue siendo
// párrafo. Antes gate la veía como math abierta, agregaba ```` y el párrafo
// pasaba a ser math, con un INCLUDE002 falso.
func TestExpand_ClosingDoesNotChangeTheFilesOwnLines(t *testing.T) {
	files := map[string]string{"/base/m.md": "````math\n```\n\nUn párrafo del incluido.\n"}
	root := "# A\n\n@include m.md\n\nFin.\n"
	res, err := ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := Expand(root, "/base", memFS(files))
	if res.Text != plain || len(res.OpenAtEnd) != 0 {
		t.Errorf("got %q %+v, want the plain expansion %q and nothing open", res.Text, res.OpenAtEnd, plain)
	}
	if got := lineClasses(res.Text + "\ntexto1t"); got["texto1t"] != classProse {
		t.Errorf("the line after the file is %v, want prose", got["texto1t"])
	}
}

// Doce copias del mismo archivo, que se cierra igual en todas: no cuestan una
// vuelta cada una ni se agotan las vueltas (antes, de la octava en adelante
// salía "could not be closed" y el bloque se tragaba el resto).
func TestExpand_SameFileManyTimesClosesInEveryCopy(t *testing.T) {
	files := map[string]string{"/base/q.md": "::: info\n````math\nx"}
	var root strings.Builder
	root.WriteString("# A\n\n")
	for range 12 {
		root.WriteString("@include q.md\n:::\n\n")
	}
	root.WriteString("Fin.\n")
	parses := 0
	onContextParse = func() { parses++ }
	t.Cleanup(func() { onContextParse = nil })
	res, err := ExpandWithOptions(root.String(), "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.OpenAtEnd) != 12 {
		t.Fatalf("%d open blocks, want 12", len(res.OpenAtEnd))
	}
	for _, b := range res.OpenAtEnd {
		if b.Line == 0 || !slices.Equal(b.Closers, []string{"```"}) {
			t.Errorf("block %+v, want closed with ```", b)
		}
	}
	if got := lineClasses(res.Text); got["Fin."] != classNone && strings.Count(res.Text, "Fin.") != 1 {
		t.Errorf("unexpected text %q", res.Text)
	}
	if parses > 2 {
		t.Errorf("%d parses of the expanded text, want at most 2", parses)
	}
}

// Dentro de un bloque ::: de prosa el parser lee una cerca ```math distinto
// que en el nivel superior: gate no está seguro y lo decide el parser.
func TestExpand_MathFenceInsideAProseBlock(t *testing.T) {
	files := map[string]string{"/base/m.md": "::: info\n```math\nx\n````\n"}
	root := "# A\n\n@include m.md\n:::\n\n```\n@include k.md\n```\n\nFin.\n"
	res, err := ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(res.Text, "\n")
	if len(res.KeptInCode) != 1 || !includesInCode(nil, lines)[res.KeptInCode[0]-1] {
		t.Errorf("the root's kept @include is not code in the result: %q %+v", res.Text, res.OpenAtEnd)
	}
}

// Si se acaban las vueltas, los cierres sin comprobar se quitan: el bloque
// queda como uno que no se pudo cerrar, con su aviso, y el texto es el de sin
// cerrar. Con cero vueltas, ninguno se comprueba.
func TestExpand_ContextRoundsRunOut(t *testing.T) {
	old := maxContextRounds
	maxContextRounds = 0
	t.Cleanup(func() { maxContextRounds = old })
	files := map[string]string{"/base/q.md": "```js\nx"}
	root := "# A\n\n@include q.md\n\nFin.\n"
	res, err := ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := Expand(root, "/base", memFS(files))
	if res.Text != plain || len(res.OpenAtEnd) != 1 || res.OpenAtEnd[0].Line != 0 {
		t.Errorf("got %q %+v, want the plain expansion and q.md not closed", res.Text, res.OpenAtEnd)
	}
}

// Un bloque que ningún cierre cierra queda como estaba, en OpenAtEnd sin
// líneas de cierre (Line 0), y el aviso va en la línea que lo abre. El caso:
// una cerca con un :::code-item legado adentro, que la normalización de flex
// reescribe, así que ni ``` ni ningún cierre de code-group devuelve a prosa
// la línea que sigue.
func TestExpand_UnclosableBlockIsReported(t *testing.T) {
	files := map[string]string{"/base/open.md": "```js\n:::code-group\n:::code-item{0}"}
	root := "# A\n\n@include open.md\n\nFin.\n"
	res, err := ExpandWithOptions(root, "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := Expand(root, "/base", memFS(files))
	if res.Text != plain {
		t.Errorf("an unclosable block must not add lines: got %q, want %q", res.Text, plain)
	}
	if len(res.OpenAtEnd) != 1 || res.OpenAtEnd[0].Line != 0 || len(res.OpenAtEnd[0].Closers) != 0 || res.OpenAtEnd[0].OpenerLine != 3 {
		t.Fatalf("OpenAtEnd = %+v, want the block at line 3, not closed", res.OpenAtEnd)
	}
	d := res.OpenAtEndDiagnostics("/base")
	if len(d) != 1 || d[0].Position.Line != 3 || !strings.Contains(d[0].Message, "could not be closed") || !strings.Contains(d[0].Message, "at line 3)") {
		t.Errorf("diagnostics = %+v", d)
	}
	if mapped := res.Sources.MapDiagnostics(d, "/base"); mapped[0].File != "open.md" || mapped[0].Position.Line != 1 {
		t.Errorf("the warning maps to %s:%d, want open.md:1", mapped[0].File, mapped[0].Position.Line)
	}
}

// Lo que no se cierra: un bloque ::: de prosa (su contenido no es código, así
// que no cambia qué @include es código), un archivo que cierra todo lo que
// abre, una cerca dentro de una cita (no es un bloque de código en el AST), un
// CODE strict (termina por sangría), una cerca ~~~math sin cierre (el parser
// flex no tiene cercas de código con ~~~, y una ~~~math sin cierre es texto:
// no se traga nada) y un $$ que sigue a un párrafo (es texto del párrafo).
// Ninguno agrega líneas. Sin KeepIncludesInCode (slidelang) tampoco se cierra
// nada.
func TestExpand_ClosedOrProseBlocksAreNotClosed(t *testing.T) {
	for _, c := range []struct{ root, open string }{
		{"# A\n\n@include open.md\nfin\n", "::: info\ntexto\n"},
		{"# A\n\n@include open.md\nfin\n", "```\nx\n```\n"},
		{"# A\n\n@include open.md\nfin\n", "> ```\n> x\n"},
		{"# A\n\n@include open.md\n\n```\nx\n```\n", "~~~math\nx\n"},
		{"# A\n\n@include open.md\n\nfin\n", "texto\n$$\nx\n"},
		{"---\nmode: strict\n---\n\nSECTION \"A\"\n@include open.md\n  TEXT\n    fin\n", "  CODE js\n    x\n"},
	} {
		files := map[string]string{"/base/open.md": c.open}
		got, err := expandKeep(c.root, "/base", memFS(files))
		if err != nil {
			t.Fatal(err)
		}
		want, err := Expand(c.root, "/base", memFS(files))
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%q: got %q, want the plain expansion %q", c.open, got, want)
		}
	}
	files := map[string]string{"/base/open.md": "```text\n"}
	if got, _ := Expand("# A\n\n@include open.md\nfin\n", "/base", memFS(files)); got != "# A\n\n```text\n\nfin\n" {
		t.Errorf("without the option an open fence is not closed: %q", got)
	}
}
