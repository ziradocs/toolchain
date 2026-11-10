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

// treeTokens son las líneas con las que FuzzKeepIncludesInCodeTree arma
// archivos: cercas, code-groups, bloques :::, un CODE strict, prosa y
// @include (cada uno de un archivo distinto, para que no haya ciclos y cada
// línea @include sea única).
var treeTokens = []string{
	"```", "```js", "````", "texto", "", "- item", ":::note", ":::", ":::code-group",
	"```js [a]", "  CODE js", "    x", "SECTION \"S\"", "## H", "@include", "  @include",
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

// FuzzKeepIncludesInCodeTree arma un documento y hasta ocho archivos, algunos
// con frontmatter, con @include hermanos y anidados de varias líneas, en strict
// y en flex, y exige que ExpandWithOptions dé exactamente lo que da la regla
// escrita aparte (oracleExpand), con cada línea en su origen.
func FuzzKeepIncludesInCodeTree(f *testing.F) {
	f.Add([]byte{0, 14, 3, 14, 7}, false, uint8(0))
	f.Add([]byte{1, 3, 0, 14, 14, 3, 7, 0, 3}, false, uint8(5))
	f.Add([]byte{12, 10, 11, 15, 3, 15, 14}, true, uint8(2))
	f.Add([]byte{8, 9, 14, 0, 7, 6, 1, 14, 0, 7, 3}, false, uint8(255))
	f.Fuzz(func(t *testing.T, picks []byte, strict bool, withFrontMatter uint8) {
		if len(picks) > 64 {
			picks = picks[:64]
		}
		bodies := make([][]string, treeFiles+1)
		next := 1
		for n, p := range picks {
			file := n * (treeFiles + 1) / max(len(picks), 1)
			tok := treeTokens[int(p)%len(treeTokens)]
			if strings.HasSuffix(tok, "@include") {
				if next > treeFiles {
					tok = "texto"
				} else {
					tok += fmt.Sprintf(" f%d.md", next)
					next++
				}
			}
			bodies[file] = append(bodies[file], tok)
		}
		files := map[string]string{}
		for i := 1; i <= treeFiles; i++ {
			content := strings.Join(bodies[i], "\n")
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
		if want := strings.Join(oracleExpand(strings.Split(root, "\n"), files, strict, true), "\n"); res.Text != want {
			t.Fatalf("got:\n%s\nwant:\n%s\nfiles: %q", res.Text, want, files)
		}
		checkSourceMap(t, "tree", root, files, res.Text, res.Sources)
		kept := 0
		for _, l := range strings.Split(res.Text, "\n") {
			if _, ok := parseIncludeLine(l); ok {
				kept++
			}
		}
		if kept != len(res.KeptInCode) {
			t.Fatalf("%d @include lines in the text, %d in KeptInCode", kept, len(res.KeptInCode))
		}
	})
}
