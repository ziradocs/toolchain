// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Con KeepIncludesInCode el costo es lineal: un parse por archivo distinto, no
// uno del documento entero por @include. 200 @include de archivos de
// 1000 líneas, con prosa, encabezados y bloques de código (y un @include
// dentro de código en cada uno), tienen que expandirse en un tiempo del mismo
// orden que sin la opción. Antes cada @include volvía a parsear el documento
// entero: 55 ms sin la opción contra 17 s con ella.
func TestExpandWithOptions_ScalesWithManyIncludes(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test")
	}
	var part strings.Builder
	for part.Len() == 0 || strings.Count(part.String(), "\n") < 1000 {
		part.WriteString("## Apartado\n\nUn párrafo de prosa con `código` y {{x}}.\nOtra línea.\n\n```go\nx := 1\n@include secreto.txt\n```\n\n- item\n- otro\n\n")
	}
	files := map[string]string{"/base/secreto.txt": "SECRETO"}
	var root strings.Builder
	root.WriteString("---\ntitle: Escala\n---\n\n# Documento\n\n")
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("parte%03d.md", i)
		files["/base/"+name] = part.String()
		fmt.Fprintf(&root, "@include %s\n\n", name)
	}
	start := time.Now()
	res, err := ExpandWithOptions(root.String(), "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Text, "SECRETO") || len(res.KeptInCode) == 0 {
		t.Fatalf("the @include inside each code block must stay")
	}
	if lines := strings.Count(res.Text, "\n"); lines < 200*1000 {
		t.Fatalf("only %d lines", lines)
	}
	// Margen amplio para máquinas de CI lentas: hoy tarda alrededor de un segundo.
	if elapsed > 15*time.Second {
		t.Errorf("ExpandWithOptions took %v for 200 includes of 1000 lines", elapsed)
	}
	t.Logf("200 includes x 1000 lines: %v", elapsed)
}

// Con anidamiento profundo el costo sigue siendo un parse por archivo: 32
// niveles de archivos de 5000 líneas, cada uno con un @include en código y
// el siguiente nivel al final.
func TestExpandWithOptions_ScalesWithDeepNesting(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test")
	}
	var body strings.Builder
	for strings.Count(body.String(), "\n") < 5000 {
		body.WriteString("Prosa con `código` y texto.\n\n```go\n@include secreto.txt\n```\n\n")
	}
	files := map[string]string{"/base/secreto.txt": "SECRETO"}
	for i := 1; i <= MaxDepth; i++ {
		content := body.String()
		if i < MaxDepth {
			content += fmt.Sprintf("@include nivel%02d.md\n", i+1)
		}
		files[fmt.Sprintf("/base/nivel%02d.md", i)] = content
	}
	start := time.Now()
	res, err := ExpandWithOptions("# Documento\n\n@include nivel01.md\n", "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Text, "SECRETO") || strings.Count(res.Text, "\n") < MaxDepth*5000 {
		t.Fatalf("unexpected expansion")
	}
	if elapsed > 15*time.Second {
		t.Errorf("ExpandWithOptions took %v for %d nested levels of 5000 lines", elapsed, MaxDepth)
	}
	t.Logf("%d nested levels x 5000 lines: %v", MaxDepth, elapsed)
}

// La marca de cada @include tiene largo fijo: un texto con 50 000 "z" y 2000
// @include dentro de una cerca no la hace crecer ni la vuelve cuadrática.
func TestExpandWithOptions_TagDoesNotGrowWithTheText(t *testing.T) {
	var b strings.Builder
	b.WriteString("# A\n\n" + strings.Repeat("z", 50000) + "\n\n```\n")
	for i := 0; i < 2000; i++ {
		b.WriteString("@include x.md\n")
	}
	b.WriteString("```\n")
	start := time.Now()
	res, err := ExpandWithOptions(b.String(), "/base", memFS(nil), ExpandOptions{KeepIncludesInCode: true})
	elapsed := time.Since(start)
	if err != nil || len(res.KeptInCode) != 2000 {
		t.Fatalf("err %v, %d kept", err, len(res.KeptInCode))
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %v", elapsed)
	}
}

// Lo que un archivo deja abierto no cuesta un parse por archivo: gate lee cada
// archivo sin parsearlo, y los cierres se comprueban con un parse del texto
// expandido para todos. 2000 archivos con un $$ sin cerrar, 8000 con todo
// cerrado y 2600 con un $$ sin cerrar y sin frontmatter.
func TestExpandWithOptions_ScalesWithManyOpenBlocks(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test")
	}
	for _, c := range []struct {
		n    int
		tail string
		open int
		fm   bool
	}{{2000, "$$\nx^2\n", 2000, true}, {8000, "", 0, true}, {2600, "$$\nx^2\n", 2600, false}} {
		files := map[string]string{}
		var root strings.Builder
		// Sin frontmatter corre la normalización de flex (modo auto) sobre
		// todo el texto: tiene que ser lineal también ahí.
		if c.fm {
			root.WriteString("---\ntitle: Escala\n---\n\n")
		}
		root.WriteString("# Documento\n\n")
		for i := 0; i < c.n; i++ {
			name := fmt.Sprintf("p%04d.md", i)
			files["/base/"+name] = fmt.Sprintf("## Parte %d\n\nPárrafo con `code`.\n\n```js\nconsole.log(1)\n```\n\n", i) + c.tail
			fmt.Fprintf(&root, "@include %s\n\n", name)
		}
		start := time.Now()
		res, err := ExpandWithOptions(root.String(), "/base", memFS(files), ExpandOptions{KeepIncludesInCode: true})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.OpenAtEnd) != c.open {
			t.Fatalf("%d includes: %d open blocks, want %d", c.n, len(res.OpenAtEnd), c.open)
		}
		if elapsed > 15*time.Second {
			t.Errorf("ExpandWithOptions took %v for %d includes", elapsed, c.n)
		}
		t.Logf("%d includes, %d open: %v", c.n, c.open, elapsed)
	}
}
