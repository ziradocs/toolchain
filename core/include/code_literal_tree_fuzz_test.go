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
// @include de archivos posteriores (para que no haya ciclos).
var treeTokens = []string{
	"```", "```js", "````", "texto", "", "- item", ":::note", ":::", ":::code-group",
	"```js [a]", "  CODE js", "    x", "SECTION \"S\"", "## H", "@include", "  @include",
}

// oracleInCode es la regla de KeepIncludesInCode escrita aparte: las líneas
// @include de lines que el AST de su archivo (ParseDocument, normalización
// prendida) tiene en un bloque de código. Un archivo incluido se lee con el
// dialecto del raíz y, en strict, como cuerpo de una sección si no empieza con
// SECTION.
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
	marked := append([]string(nil), head...)
	for i, l := range lines {
		if _, ok := parseIncludeLine(l); ok {
			l = fmt.Sprintf("%s ORACLE%dEND", l, i)
		}
		marked = append(marked, l)
	}
	doc, _ := parser.New(util.NewNoop()).ParseDocument(strings.Join(marked, "\n"), "")
	in := map[int]bool{}
	find := func(content string) {
		for i := range lines {
			if strings.Contains(content, fmt.Sprintf("ORACLE%dEND", i)) {
				in[i] = true
			}
		}
	}
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch c := n.(type) {
		case *ast.CodeElement:
			find(c.Content)
		case *ast.CodeGroupElement:
			for _, b := range c.CodeBlocks {
				find(b.Content)
			}
		}
		return nil
	})
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
		out = append(out, oracleExpand(strings.Split(files["/base/"+path], "\n"), files, strict, false)...)
	}
	return out
}

// FuzzKeepIncludesInCodeTree arma un documento y tres archivos con @include
// hermanos y anidados de varias líneas, en strict y en flex, y exige que
// ExpandWithOptions dé exactamente lo que da la regla escrita aparte
// (oracleExpand), con cada línea en su origen.
func FuzzKeepIncludesInCodeTree(f *testing.F) {
	f.Add([]byte{0, 14, 3, 14, 7}, false)
	f.Add([]byte{1, 3, 0, 14, 14, 3, 7, 0, 3}, false)
	f.Add([]byte{12, 10, 11, 15, 3, 15, 14}, true)
	f.Add([]byte{8, 9, 14, 0, 7, 6, 1, 14, 0, 7, 3}, false)
	f.Fuzz(func(t *testing.T, picks []byte, strict bool) {
		if len(picks) > 48 {
			picks = picks[:48]
		}
		// Cuatro archivos (el raíz y f1..f3) repartidos en orden; un @include
		// de un archivo apunta a uno posterior.
		bodies := make([][]string, 4)
		for n, p := range picks {
			file := n * 4 / max(len(picks), 1)
			tok := treeTokens[int(p)%len(treeTokens)]
			if strings.HasSuffix(tok, "@include") {
				if file == 3 {
					tok = "texto"
				} else {
					tok += fmt.Sprintf(" f%d.md", file+1+int(p)%(3-file))
				}
			}
			bodies[file] = append(bodies[file], tok)
		}
		files := map[string]string{}
		for i := 1; i <= 3; i++ {
			files[fmt.Sprintf("/base/f%d.md", i)] = strings.Join(bodies[i], "\n")
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
