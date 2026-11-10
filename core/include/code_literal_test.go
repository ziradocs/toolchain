// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"fmt"
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
		// La cerca de cuatro cierra en su ```` (CommonMark): el @include que
		// sigue no es código y se expande.
		{"four-backtick fence, ```` inside", "# A\n\n````js\nq\n````\n@include part2.md\n```\n", false},
		// La cerca de cuatro no tiene un cierre de CommonMark y cierra en
		// ```, como antes (CODE003): el @include que sigue se expande.
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
		// Un CODE pegado a un TEXT, sin línea en blanco, no es un elemento
		// propio en el AST: el include se expande, como lo muestra get_ast.
		{"CODE right after a TEXT", strictDoc + "  TEXT a\n  CODE js\n    @include part2.md\n", false},
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
// dos se expanden. Un @include que trae ese archivo dentro de su cerca sí se
// decide sobre el texto ya expandido, y queda.
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
	if want := "# A\n\n```\nINCLUDED\n"; got != want {
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

// Una pestaña de :::code-group se acota a su grupo, que termina en el primer
// ":::" en el que todo lo abierto cierra (fence.ContainerEnd): una pestaña de
// cuatro con un ``` y después un ":::" cierra en el ``` (como en las versiones
// anteriores), el grupo en ese ":::", y el @include que sigue no es código. El
// segundo @include cae en la cerca de cuatro que abre después y queda.
func TestExpand_CodeGroupTabWithALongerFence(t *testing.T) {
	files := map[string]string{"/base/frag.md": "Incluido."}
	src := "# A\n\n:::code-group\n````md [doc]\n```\n:::\n@include frag.md\n````\n:::\n@include frag.md\n"
	got, err := expandKeep(src, "/base", memFS(files))
	if err != nil {
		t.Fatal(err)
	}
	want := "# A\n\n:::code-group\n````md [doc]\n```\n:::\nIncluido.\n````\n:::\n@include frag.md\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// Un @include que se expandió por estar fuera de código, pero cuyo contenido
// terminó dentro de un bloque de código que empieza antes que él, se reporta en
// CapturedInCode (doclang lo avisa como CODE005).
func TestExpand_CapturedInCode(t *testing.T) {
	strictDoc := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n"
	for _, tc := range []struct {
		name, src string
		files     map[string]string
		want      []CapturedInclude
	}{
		// La cerca de cuatro cerraba en ``` (regla anterior); el ```` que trae
		// el archivo la cierra con CommonMark después, y el contenido incluido
		// queda dentro.
		{"included closer changes where the fence closes", "# A\n\n````js\nq\n```\n@include a.md\n",
			map[string]string{"/base/a.md": "INCLUIDO\n````"}, []CapturedInclude{{Line: 6, Directive: "@include a.md", Origin: Origin{Line: 6}}}},
		// El contenido incluido, más sangrado que la cabecera, alarga el cuerpo
		// de un CODE strict.
		{"included lines extend a strict CODE body", strictDoc + "  CODE\n    x\n@include more.txt\n",
			map[string]string{"/base/more.txt": "    y"}, []CapturedInclude{{Line: 9, Directive: "@include more.txt", Origin: Origin{Line: 9}}}},
		// La directiva anidada se ancla en su archivo (Origin, de #501).
		{"nested directive is anchored in its file", "# A\n\n@include outer.md\n",
			map[string]string{"/base/outer.md": "````js\nq\n```\n@include a.md", "/base/a.md": "IN\n````"},
			[]CapturedInclude{{Line: 6, Directive: "@include a.md", Origin: Origin{File: "/base/outer.md", Line: 4}}}},
		// Un archivo que trae su propio bloque de código no cuenta.
		{"included file with its own code block", "# A\n\n@include c.md\n",
			map[string]string{"/base/c.md": "```\ncode\n```"}, nil},
		// Un archivo que agrega una pestaña a un code-group tampoco.
		{"included file adds a tab", "# A\n\n:::code-group\n```js [a]\nx\n```\n@include tab.md\n:::\n",
			map[string]string{"/base/tab.md": "```py [b]\ny\n```"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ExpandWithOptions(tc.src, "/base", memFS(tc.files), ExpandOptions{KeepIncludesInCode: true})
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(res.CapturedInCode) != fmt.Sprint(tc.want) {
				t.Errorf("CapturedInCode %v, want %v\n%s", res.CapturedInCode, tc.want, res.Text)
			}
		})
	}
}
