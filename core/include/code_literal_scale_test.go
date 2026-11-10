// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Con KeepIncludesInCode el costo es casi lineal: una vuelta (un parse) por
// nivel de anidamiento, no un parse por @include. 200 @include de archivos de
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
