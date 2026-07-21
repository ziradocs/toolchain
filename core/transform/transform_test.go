// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

func sampleDoc(t *testing.T) *ast.AST {
	t.Helper()
	pos := diagnostics.Position{Line: 1, Column: 1}
	text := NewText(pos, "hola")
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Título original"
	block.Elements = append(block.Elements, text)
	doc := ast.NewAST(pos)
	doc.FilePath = "/tmp/entrada.slidelang"
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

// NewText es un atajo local para no repetir ast.NewTextElement en cada test.
func NewText(pos diagnostics.Position, content string) ast.Element {
	return ast.NewTextElement(pos, content)
}

func TestRunBuiltins_OrderAndErrorPropagation(t *testing.T) {
	doc := sampleDoc(t)
	var order []string
	first := func(d *ast.AST) (*ast.AST, error) {
		order = append(order, "first")
		d.ContentBlocks[0].Title = "uno"
		return d, nil
	}
	second := func(d *ast.AST) (*ast.AST, error) {
		order = append(order, "second")
		d.ContentBlocks[0].Title += "-dos"
		return d, nil
	}
	out, err := RunBuiltins(doc, []Transform{first, second})
	if err != nil {
		t.Fatalf("RunBuiltins error inesperado: %v", err)
	}
	if got, want := out.ContentBlocks[0].Title, "uno-dos"; got != want {
		t.Errorf("Title = %q, want %q (orden no respetado)", got, want)
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("orden de ejecución = %v, want [first second]", order)
	}

	// Un error en el primer transform debe abortar y no correr el segundo.
	boom := errors.New("boom")
	failing := func(d *ast.AST) (*ast.AST, error) { return nil, boom }
	ran := false
	neverRuns := func(d *ast.AST) (*ast.AST, error) { ran = true; return d, nil }
	_, err = RunBuiltins(sampleDoc(t), []Transform{failing, neverRuns})
	if err == nil {
		t.Fatal("esperaba error de RunBuiltins")
	}
	if ran {
		t.Error("el segundo transform corrió pese al error del primero")
	}
}

func TestRunFilters_Identity(t *testing.T) {
	catPath, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("cat no disponible en PATH; salteando test de identidad")
	}

	doc := sampleDoc(t)
	originalTitle := doc.ContentBlocks[0].Title

	out, err := RunFilters(doc, []string{catPath}, DefaultFilterTimeout)
	if err != nil {
		t.Fatalf("RunFilters con filtro identidad falló: %v", err)
	}
	if got := out.ContentBlocks[0].Title; got != originalTitle {
		t.Errorf("Title tras filtro identidad = %q, want %q", got, originalTitle)
	}
	if out.FilePath != doc.FilePath {
		t.Errorf("FilePath no se preservó: got %q, want %q (FilePath es json:\"-\", el filtro no puede reconstruirlo)", out.FilePath, doc.FilePath)
	}
}

// buildMutatingFilter compila un filtro Go real de prueba: lee el AST del
// stdin, le agrega un sufijo al título del primer bloque Y rellena
// ContentHTML con contenido "hostil" (simulando un filtro buggy o
// malicioso), y lo re-emite por stdout. Sirve para probar tanto la mutación
// real (Title) como la garantía de defensa en profundidad (ContentHTML debe
// llegar VACÍO a quien llama RunFilters, pese a que este filtro lo rellena).
func buildMutatingFilter(t *testing.T) string {
	t.Helper()
	src := `package main

import (
	"encoding/json"
	"io"
	"os"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		panic(err)
	}
	blocks := doc["contentBlocks"].([]interface{})
	block := blocks[0].(map[string]interface{})
	block["title"] = block["title"].(string) + "-mutado"
	block["titleHTML"] = "<img src=x onerror=alert(1)>"
	elems := block["elements"].([]interface{})
	elem := elems[0].(map[string]interface{})
	elem["contentHTML"] = "<img src=x onerror=alert(1)>"
	out, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	os.Stdout.Write(out)
}
`
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatalf("escribiendo fuente del filtro de prueba: %v", err)
	}
	binPath := filepath.Join(dir, "mutating-filter")
	cmd := exec.Command("go", "build", "-o", binPath, srcPath)
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compilando filtro de prueba: %v\n%s", err, out)
	}
	return binPath
}

func TestRunFilters_MutatesAndStripsUntrustedHTML(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain no disponible en PATH; salteando test de filtro compilado")
	}
	binPath := buildMutatingFilter(t)

	doc := sampleDoc(t)
	out, err := RunFilters(doc, []string{binPath}, DefaultFilterTimeout)
	if err != nil {
		t.Fatalf("RunFilters falló: %v", err)
	}

	if got, want := out.ContentBlocks[0].Title, "Título original-mutado"; got != want {
		t.Errorf("Title = %q, want %q — la mutación real del filtro no se aplicó", got, want)
	}

	// La garantía de seguridad: el filtro rellenó titleHTML/contentHTML con
	// HTML "hostil", pero RunFilters debe haberlo blanqueado (ast.ClearRenderedHTML)
	// antes de devolver — nunca se confía en *HTML de un subproceso.
	if got := out.ContentBlocks[0].TitleHTML; got != "" {
		t.Errorf("TitleHTML NO quedó vacío tras RunFilters: %q — el gate de seguridad de #240 está roto", got)
	}
	gotText, ok := out.ContentBlocks[0].Elements[0].(*ast.TextElement)
	if !ok {
		t.Fatalf("elemento decodificado no es *ast.TextElement: %T", out.ContentBlocks[0].Elements[0])
	}
	if gotText.ContentHTML != "" {
		t.Errorf("ContentHTML NO quedó vacío tras RunFilters: %q — el gate de seguridad de #240 está roto", gotText.ContentHTML)
	}
}

func TestRunFilters_NonZeroExitReturnsError(t *testing.T) {
	falsePath, err := exec.LookPath("false")
	if err != nil {
		t.Skip("false no disponible en PATH")
	}
	_, err = RunFilters(sampleDoc(t), []string{falsePath}, DefaultFilterTimeout)
	if err == nil {
		t.Fatal("esperaba error de un filtro que sale con código no-cero")
	}
}

func TestRunFilters_InvalidJSONOutputReturnsError(t *testing.T) {
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh no disponible en PATH")
	}
	// "sh -c ..." no sirve como binaryPath directo (RunFilters execs el path
	// tal cual, sin args) — usamos un script real con shebang.
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "not-json.sh")
	script := "#!" + shPath + "\necho 'esto no es JSON'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("escribiendo script: %v", err)
	}
	_, err = RunFilters(sampleDoc(t), []string{scriptPath}, DefaultFilterTimeout)
	if err == nil {
		t.Fatal("esperaba error al decodificar salida no-JSON del filtro")
	}
}

func TestRunFilters_ChainsMultipleFilters(t *testing.T) {
	catPath, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("cat no disponible en PATH")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain no disponible en PATH")
	}
	mutating := buildMutatingFilter(t)

	// cat (identidad) → filtro mutante: el resultado final debe reflejar la
	// mutación del segundo, confirmando que la salida del primero alimenta
	// al segundo (no que cada uno corre sobre el doc original en paralelo).
	out, err := RunFilters(sampleDoc(t), []string{catPath, mutating}, DefaultFilterTimeout)
	if err != nil {
		t.Fatalf("RunFilters (cadena) falló: %v", err)
	}
	if got, want := out.ContentBlocks[0].Title, "Título original-mutado"; got != want {
		t.Errorf("Title tras la cadena = %q, want %q", got, want)
	}
}
