// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"slices"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/internal/fence"
	"go.ziradocs.com/core/v2/util"
)

// fenceTokens son las líneas con las que FuzzFenceReadingIsReported arma
// documentos: cercas de tres, cuatro y cinco backticks, con y sin info
// string, con un backtick en el info string, prosa, encabezados, listas y
// bloques.
var fenceTokens = []string{
	"```", "````", "```js", "````md", "```js`", "```ts title=`a.ts`", "`````",
	"texto", "## H", "", ":::note", ":::", "- item", "```mermaid", ":::code-group", "```js [a]",
	"::::code-group", "````js [b]", ":::tip",
}

// referenceCode lee lines con una regla escrita aparte de internal/fence y
// devuelve el contenido de cada cerca, en orden, y si alguna quedó sin
// cerrar. commonMark: abre una línea de tres o más backticks con un info
// string sin backticks y cierra una de solo backticks, al menos tantos; si
// no, la regla de las versiones anteriores: abre toda línea que empieza con
// ``` y cierra la primera ``` exacta.
func referenceCode(lines []string, commonMark bool) ([]string, bool) {
	var out []string
	run := func(s string) int { return len(s) - len(strings.TrimLeft(s, "`")) }
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		n := run(t)
		if n < 3 || commonMark && strings.Contains(t[n:], "`") {
			continue
		}
		end := -1
		for j := i + 1; j < len(lines); j++ {
			u := strings.TrimSpace(lines[j])
			if commonMark && run(u) == len(u) && len(u) >= n || !commonMark && u == "```" {
				end = j
				break
			}
		}
		if end < 0 {
			return append(out, strings.Join(lines[i+1:], "\n")), true
		}
		out = append(out, strings.Join(lines[i+1:end], "\n"))
		i = end
	}
	return out, false
}

// parsedCode es el contenido de cada bloque de código y de cada pestaña de
// code-group del AST, en orden.
func parsedCode(doc *ast.AST) []string {
	var out []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch e := n.(type) {
		case *ast.CodeElement:
			out = append(out, e.Content)
		case *ast.CodeGroupElement:
			for _, b := range e.CodeBlocks {
				out = append(out, b.Content)
			}
		}
		return nil
	})
	return out
}

// walkedCode es el contenido de cada cerca que ve fence.Walk, el recorrido
// sin parser.
func walkedCode(lines []string) []string {
	var out []string
	for _, b := range fence.Walk(lines, 0) {
		if b != nil {
			out = append(out, strings.Join(b.Body(lines), "\n"))
		}
	}
	return out
}

// FuzzFenceReadingIsReported arma documentos con fenceTokens y exige:
//
//   - con CODE005 (el documento se leyó con la regla anterior), el código es
//     el de esa regla;
//   - sin CODE003 ni CODE005, el código es el de CommonMark (con una
//     implementación de referencia escrita en la prueba), también con cercas
//     dentro de :::, de code-groups y después de listas;
//   - el recorrido sin parser (fence.Walk) ve el mismo código que el parser;
//   - si la regla anterior leía el documento sin cercas abiertas y el código
//     no es el de esa regla, hay un aviso CODE002, CODE003, CODE004 o
//     CODE005: ningún documento bien formado cambia de significado en
//     silencio;
//   - si la regla anterior leía el documento sin cercas abiertas y CommonMark
//     dejaría una abierta, hay un CODE003 o un CODE005.
//
// Las cercas de mermaid no son código: los documentos con ellas no se
// comparan.
func FuzzFenceReadingIsReported(f *testing.F) {
	f.Add([]byte{3, 7, 0, 8})
	f.Add([]byte{6, 7, 1, 7, 0})
	f.Add([]byte{5, 7, 0, 8, 2, 7, 1})
	f.Add([]byte{3, 7, 0, 8, 3, 7, 3})
	f.Add([]byte{14, 15, 7, 0, 11})
	f.Add([]byte{10, 3, 0, 1, 11, 7})
	f.Add([]byte{14, 4, 7, 6, 11})
	f.Fuzz(func(t *testing.T, picks []byte) {
		if len(picks) > 40 {
			picks = picks[:40]
		}
		body := make([]string, len(picks))
		for i, p := range picks {
			body[i] = fenceTokens[int(p)%len(fenceTokens)]
		}
		src := "# A\n\n" + strings.Join(body, "\n")
		lines := strings.Split(src, "\n")
		p := New(util.NewNoop())
		p.SetNormalization(false)
		doc, diags := p.ParseDocument(src, "")
		reported, fallback, earlier := false, false, false
		for _, d := range diags {
			switch d.RuleID {
			case "CODE002", "CODE004":
				reported = true
			case "CODE003":
				reported, fallback = true, true
			case "CODE005":
				reported, earlier = true, true
			}
		}
		if strings.Contains(src, "```mermaid") {
			return
		}
		got := parsedCode(doc)
		want, mainUnclosed := referenceCode(lines, false)
		if earlier {
			if !slices.Equal(got, want) {
				t.Fatalf("CODE005, so the code must be the earlier rule's:\n%s\n got %q\nwant %q", src, got, want)
			}
			return
		}
		if walked := walkedCode(lines); !slices.Equal(got, walked) {
			t.Fatalf("fence.Walk and the parser disagree:\n%s\nparser %q\nwalker %q", src, got, walked)
		}
		cm, cmUnclosed := referenceCode(lines, true)
		if !fallback && !slices.Equal(got, cm) {
			t.Fatalf("no CODE003, so the code must be CommonMark's:\n%s\n got %q\nwant %q", src, got, cm)
		}
		if !mainUnclosed && !slices.Equal(got, want) && !reported {
			t.Fatalf("code changed from the earlier rule without a CODE00x warning:\n%s\n got %q\nwant %q", src, got, want)
		}
		if !mainUnclosed && cmUnclosed && !fallback {
			t.Fatalf("CommonMark leaves a fence open but there is no CODE003:\n%s\n%v", src, diags)
		}
	})
}
