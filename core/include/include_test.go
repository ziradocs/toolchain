// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package include

import (
	"fmt"
	"strings"
	"testing"
)

// memFS es un ReadFunc respaldado por un mapa en memoria — no depende del
// filesystem real, así que baseDir puede ser una ruta puramente virtual
// (util.ResolveConfinedPath tolera una base inexistente: EvalSymlinks
// devuelve ENOENT y cae al path sin resolver, ver su propio docstring).
func memFS(files map[string]string) ReadFunc {
	return func(path string) ([]byte, error) {
		content, ok := files[path]
		if !ok {
			return nil, fmt.Errorf("archivo no encontrado: %s", path)
		}
		return []byte(content), nil
	}
}

func TestExpand_NoIncludes(t *testing.T) {
	content := "línea uno\nlínea dos\n"
	got, err := Expand(content, "/base", memFS(nil))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got != content {
		t.Errorf("contenido sin @include no debe cambiar: got %q, want %q", got, content)
	}
}

func TestExpand_SingleInclude(t *testing.T) {
	files := map[string]string{
		"/base/partial.doclang": "contenido incluido",
	}
	content := "antes\n@include partial.doclang\ndespués"
	got, err := Expand(content, "/base", memFS(files))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	want := "antes\ncontenido incluido\ndespués"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpand_ToleratesIndentationAndExtraSpace(t *testing.T) {
	files := map[string]string{"/base/x.doclang": "X"}
	content := "  @include   x.doclang  "
	got, err := Expand(content, "/base", memFS(files))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got != "X" {
		t.Errorf("got %q, want %q", got, "X")
	}
}

func TestExpand_NestedIncludes(t *testing.T) {
	files := map[string]string{
		"/base/a.doclang": "inicio-a\n@include b.doclang\nfin-a",
		"/base/b.doclang": "inicio-b\n@include c.doclang\nfin-b",
		"/base/c.doclang": "contenido-c",
	}
	got, err := Expand("@include a.doclang", "/base", memFS(files))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	want := "inicio-a\ninicio-b\ncontenido-c\nfin-b\nfin-a"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpand_StripsFrontMatterOfIncludedFile(t *testing.T) {
	files := map[string]string{
		"/base/partial.doclang": "---\nmode: flex\ntitle: Partial\n---\ncuerpo del partial",
	}
	got, err := Expand("@include partial.doclang", "/base", memFS(files))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got != "cuerpo del partial" {
		t.Errorf("got %q, want %q (el frontmatter del incluido debe descartarse)", got, "cuerpo del partial")
	}
	if strings.Contains(got, "---") {
		t.Errorf("el resultado no debe contener el delimitador de frontmatter: %q", got)
	}
}

func TestExpand_RejectsAbsolutePath(t *testing.T) {
	_, err := Expand("@include /etc/passwd", "/base", memFS(nil))
	if err == nil {
		t.Fatal("esperaba error para una ruta absoluta")
	}
}

func TestExpand_RejectsPathEscapingBaseDir(t *testing.T) {
	_, err := Expand("@include ../../etc/passwd", "/base/docs", memFS(nil))
	if err == nil {
		t.Fatal("esperaba error para una ruta que escapa baseDir vía '..'")
	}
}

func TestExpand_DetectsDirectCycle(t *testing.T) {
	files := map[string]string{
		"/base/a.doclang": "@include a.doclang",
	}
	_, err := Expand("@include a.doclang", "/base", memFS(files))
	if err == nil {
		t.Fatal("esperaba error de ciclo (a se incluye a sí mismo)")
	}
}

func TestExpand_DetectsIndirectCycle(t *testing.T) {
	files := map[string]string{
		"/base/a.doclang": "@include b.doclang",
		"/base/b.doclang": "@include a.doclang",
	}
	_, err := Expand("@include a.doclang", "/base", memFS(files))
	if err == nil {
		t.Fatal("esperaba error de ciclo indirecto (a incluye b incluye a)")
	}
}

func TestExpand_DoesNotFalsePositiveOnSiblingIncludesOfSameFile(t *testing.T) {
	// b y c ambos incluyen shared.doclang, sin formar un ciclo real — el
	// visited-set es por CADENA (ancestros), no un "ya visto en todo el
	// documento", así que esto debe expandir sin error.
	files := map[string]string{
		"/base/b.doclang":      "@include shared.doclang",
		"/base/c.doclang":      "@include shared.doclang",
		"/base/shared.doclang": "compartido",
	}
	content := "@include b.doclang\n@include c.doclang"
	got, err := Expand(content, "/base", memFS(files))
	if err != nil {
		t.Fatalf("no debería ser ciclo (b y c comparten un include, no se incluyen entre sí): %v", err)
	}
	want := "compartido\ncompartido"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpand_MaxDepthExceeded(t *testing.T) {
	files := map[string]string{}
	// Cadena lineal de MaxDepth+2 archivos SIN ciclo real — cada uno incluye
	// el siguiente con nombre distinto, así que solo el tope de profundidad
	// (no la detección de ciclos) puede abortarla.
	for i := 0; i < MaxDepth+5; i++ {
		files[fmt.Sprintf("/base/f%d.doclang", i)] = fmt.Sprintf("@include f%d.doclang", i+1)
	}
	files[fmt.Sprintf("/base/f%d.doclang", MaxDepth+5)] = "fin"

	_, err := Expand("@include f0.doclang", "/base", memFS(files))
	if err == nil {
		t.Fatal("esperaba error de profundidad máxima excedida")
	}
}

func TestExpand_MissingPathErrors(t *testing.T) {
	_, err := Expand("@include ", "/base", memFS(nil))
	if err == nil {
		t.Fatal("esperaba error para @include sin ruta")
	}
}

func TestExpand_ReadFailurePropagates(t *testing.T) {
	_, err := Expand("@include no-existe.doclang", "/base", memFS(nil))
	if err == nil {
		t.Fatal("esperaba error al no encontrar el archivo incluido")
	}
}

func TestExpand_LineNotStartingWithIncludeIsUntouched(t *testing.T) {
	// Una línea que MENCIONA "@include" pero no está al inicio (tras trim) no
	// debe interpretarse como directiva.
	content := "esto no es @include algo.doclang, es texto normal"
	got, err := Expand(content, "/base", memFS(nil))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q (no debía tratarse como directiva)", got, content)
	}
}
