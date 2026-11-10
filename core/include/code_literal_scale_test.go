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
