// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"strings"
	"testing"
)

const probeDirective = "@include probe.txt"

// probeInCode dice si el AST de src (ParseDocument, sin normalización) lleva la
// línea del probe dentro del contenido de un bloque de código: lo que get_ast
// muestra como código.
func probeInCode(src string) bool {
	return probeLike(src, probeDirective)
}

// FuzzKeepIncludesInCode es la propiedad del issue #487 dicha directamente:
// con KeepIncludesInCode, un `@include` queda sin expandir si y solo si el
// AST del documento sin expandir (lo que ven get_ast y el lint: ParseDocument
// con la normalización prendida) lo tiene dentro de un bloque de código. Reemplaza una línea cualquiera del documento,
// conservando su sangría, por un `@include` cuyo archivo existe, en strict y
// en flex. No mira cómo decide ExpandWithOptions qué es código.
//
// Sin la opción, ExpandWithOptions es ExpandWithOrigins.
func FuzzKeepIncludesInCode(f *testing.F) {
	for _, seed := range []struct {
		body   string
		line   uint16
		strict bool
	}{
		{"SECTION \"A\"\n  CODE\n    x\n", 7, true},
		{"SECTION \"A\"\n  TEXT\n    ```\n    x\n    ```\n", 8, true},
		{"SECTION \"A\"\n  TEXT a\n  CODE js\n    x\n", 8, true},
		{"SECTION \"A\"\n  :::code-group\n  ```js [a]\n  x\n  ```\n  :::\n", 8, true},
		{"SECTION \"A\"\n  :::note{typed}\n  ```js\n  x\n  ```\n  :::\n", 8, true},
		{"# A\n\n````js\nq\n```\nx\nfin\n", 5, false},
		{"# A\n\n````js\nq\n````\nx\n```\n", 5, false},
		{"# A\n\n```js\nq\n```\nx\n", 4, false},
		{":::code-group\n```js [a]\nx\n```js\ny\n```\n:::\n", 4, false},
		{"::::code-group\n```js [a]\nx\n```\n:::\n", 2, false},
		{":::note\n```\nx\n```\n:::\n", 2, false},
		{"> cita\n> ```\n> x\n", 2, false},
		{"- item\n  ```\n  x\n  ```\n", 2, false},
		{"```mermaid\ngraph TD\n```\n", 1, false},
		{"CODE js\n  x\n", 1, false},
		// La normalización de flex reescribe un :::code-item legado: la decisión
		// tiene que ser la del AST normalizado, como en doclang.
		{":::code-group\n:::code-item{0}\n```\nx\n", 3, false},
	} {
		f.Add(seed.body, seed.line, seed.strict)
	}
	read := memFS(map[string]string{"/base/probe.txt": "PROBE-MARKER"})
	f.Fuzz(func(t *testing.T, body string, line uint16, strict bool) {
		for _, l := range strings.Split(body, "\n") {
			if _, ok := parseIncludeLine(l); ok {
				return // otro @include cambiaría el documento antes del probe
			}
		}
		src := body
		if strict {
			src = "---\nmode: strict\ntitle: T\n---\n\n" + body
		}
		lines := strings.Split(src, "\n")
		i := int(line) % len(lines)
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
		lines[i] = indent + probeDirective
		src = strings.Join(lines, "\n")

		res, err := ExpandWithOptions(src, "/base", read, ExpandOptions{KeepIncludesInCode: true})
		if err != nil {
			t.Fatalf("ExpandWithOptions(%q): %v", src, err)
		}
		kept := len(res.KeptInCode) > 0
		if want := probeInCode(src); kept != want {
			t.Fatalf("kept=%v, but the AST of the source has the directive in code=%v:\n%s\nexpanded:\n%s", kept, want, src, res.Text)
		}
		if kept != strings.Contains(res.Text, probeDirective) || kept == strings.Contains(res.Text, "PROBE-MARKER") {
			t.Fatalf("KeptInCode=%v disagrees with the text:\n%s", res.KeptInCode, res.Text)
		}
		if len(res.Origins) != strings.Count(res.Text, "\n")+1 {
			t.Fatalf("%d origins for %d lines", len(res.Origins), strings.Count(res.Text, "\n")+1)
		}

		off, err := ExpandWithOptions(src, "/base", read, ExpandOptions{})
		text, origins, err2 := ExpandWithOrigins(src, "/base", read)
		if (err == nil) != (err2 == nil) || off.Text != text || strings.Join(off.Origins, "\x00") != strings.Join(origins, "\x00") {
			t.Fatalf("without the option ExpandWithOptions must be ExpandWithOrigins")
		}
	})
}
