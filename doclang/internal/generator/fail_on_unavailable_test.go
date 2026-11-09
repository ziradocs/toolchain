// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

// unavailableDoc tiene, en líneas conocidas, una imagen que sí existe
// (ok.png), una de bloque que falta, una inline que falta dentro de una
// celda, una inline remota, un diagrama PlantUML y un mapa.
func unavailableDoc(t *testing.T) *ast.AST {
	t.Helper()
	src := "---\ntitle: U\nlang: es\nmode: flex\n---\n\n# U\n\n" + // 1-8
		"![bien](ok.png)\n\n" + // 9
		"![falta](falta.png)\n\n" + // 11
		"| A |\n|---|\n| ![celda](celda.png) |\n\n" + // 13-15
		"Texto ![remota](https://example.com/r.png).\n" // 17
	doc := parseInline(t, src)
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	m := ast.NewMapElement(diagnostics.NewPosition(30, 1), "default")
	m.Zoom = 3
	block.Elements = append(block.Elements,
		ast.NewPlantUMLElement(diagnostics.NewPosition(25, 1), "sequence", "@startuml\nA -> B\n@enduml"),
		m,
	)
	return doc
}

func lineOf(t *testing.T, doc *ast.AST, match func(ast.Element) bool) int {
	t.Helper()
	var line int
	WalkElements(doc, func(e ast.Element) bool {
		if line == 0 && match(e) {
			line = e.GetPosition().Line
		}
		return true
	})
	if line == 0 {
		t.Fatal("element not found")
	}
	return line
}

// TestDOCX_FailOnUnavailable: con FailOnUnavailable, un DOCX con contenido
// que quedaría como marcador falla sin escribir el archivo, y el error
// lista cada uno con su línea, tipo y fuente. Sin la opción (el default) el
// DOCX se escribe como siempre.
func TestDOCX_FailOnUnavailable(t *testing.T) {
	dir := t.TempDir()
	writePNGs(t, dir, "ok.png")
	doc := unavailableDoc(t)
	imgLine := lineOf(t, doc, func(e ast.Element) bool {
		img, ok := e.(*ast.ImageElement)
		return ok && img.Source == "falta.png"
	})
	tableLine := lineOf(t, doc, func(e ast.Element) bool { _, ok := e.(*ast.TableElement); return ok })
	textLine := lineOf(t, doc, func(e ast.Element) bool {
		txt, ok := e.(*ast.TextElement)
		return ok && strings.Contains(txt.Content, "remota")
	})

	// NoNetwork deja el PlantUML y el mapa como marcador sin pedir nada.
	base := GeneratorOptions{Format: "docx", AssetRoot: dir, SourceDir: dir, NoNetwork: true}

	out := filepath.Join(t.TempDir(), "default.docx")
	if err := New(newTestLogger()).Generate(doc, out, base); err != nil {
		t.Fatalf("default build failed: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("default build left no output: %v", err)
	}

	strict := base
	strict.FailOnUnavailable = true
	out = filepath.Join(t.TempDir(), "strict.docx")
	err := New(newTestLogger()).Generate(doc, out, strict)
	var ue *UnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("want *UnavailableError, got %v", err)
	}
	want := []UnavailableContent{
		{"image", imgLine, "falta.png"},
		{"image", tableLine, "celda.png"},
		{"image", textLine, "https://example.com/r.png"},
		{"diagram", 25, ""},
		{"map", 30, ""},
	}
	if !reflect.DeepEqual(ue.Items, want) {
		t.Errorf("items = %+v\nwant %+v", ue.Items, want)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("failed build left an output file (stat err %v)", err)
	}
	msg := err.Error()
	for _, s := range []string{"5 items", "line 25: diagram", `image "falta.png"`, "no output was written"} {
		if !strings.Contains(msg, s) {
			t.Errorf("message lacks %q:\n%s", s, msg)
		}
	}
}

// TestDOCX_FailOnUnavailable_CleanDocumentPasses: sin nada faltante la
// opción no cambia nada.
func TestDOCX_FailOnUnavailable_CleanDocumentPasses(t *testing.T) {
	dir := t.TempDir()
	writePNGs(t, dir, "ok.png")
	doc := parseInline(t, "---\ntitle: C\nmode: flex\n---\n\n# C\n\n![bien](ok.png)\n")
	out := filepath.Join(t.TempDir(), "out.docx")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: dir, SourceDir: dir, FailOnUnavailable: true}); err != nil {
		t.Fatal(err)
	}
}

func coreReportsUnavailable() bool {
	_, ok := any(renderer.NewDefaultRenderContext()).(unavailableReporterSetter)
	return ok
}

// TestHTML_FailOnUnavailable: el HTML offline-inline (la misma ruta que el
// PDF, sin Chromium) falla igual, con la línea que reporta core, y no
// escribe el archivo. Necesita un core con SetUnavailableReporter; contra
// uno que no lo tiene, la opción es un error explícito en vez de pasar en
// silencio.
func TestHTML_FailOnUnavailable(t *testing.T) {
	dir := t.TempDir()
	writePNGs(t, dir, "ok.png")
	doc := parseInline(t, "---\ntitle: H\nmode: flex\n---\n\n# H\n\n![bien](ok.png)\n\n![falta](falta.png)\n\nTexto ![p](otra.png).\n")
	imgLine := lineOf(t, doc, func(e ast.Element) bool {
		img, ok := e.(*ast.ImageElement)
		return ok && img.Source == "falta.png"
	})
	textLine := lineOf(t, doc, func(e ast.Element) bool { _, ok := e.(*ast.TextElement); return ok })
	out := filepath.Join(t.TempDir(), "out.html")
	opts := GeneratorOptions{Format: "html", ImageMode: "offline-inline", AssetRoot: dir, SourceDir: dir, FailOnUnavailable: true}
	err := New(newTestLogger()).Generate(doc, out, opts)
	if !coreReportsUnavailable() {
		if !errors.Is(err, errUnavailableNeedsCore) {
			t.Fatalf("old core: want errUnavailableNeedsCore, got %v", err)
		}
		t.Skip("this core has no RenderContext.SetUnavailableReporter")
	}
	var ue *UnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("want *UnavailableError, got %v", err)
	}
	want := []UnavailableContent{{"image", imgLine, "falta.png"}, {"image", textLine, "otra.png"}}
	if !reflect.DeepEqual(ue.Items, want) {
		t.Errorf("items = %+v, want %+v", ue.Items, want)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("failed build left an output file (stat err %v)", err)
	}

	// Sin la opción el HTML se escribe con sus marcadores.
	opts.FailOnUnavailable = false
	if err := New(newTestLogger()).Generate(doc, out, opts); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), "image-unavailable") {
		t.Fatalf("default HTML lacks its marker (err %v)", err)
	}
}

// TestPDF_FailOnUnavailable (Chromium real): el PDF falla antes de imprimir
// y no deja el archivo.
func TestPDF_FailOnUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Chromium")
	}
	if !coreReportsUnavailable() {
		t.Skip("this core has no RenderContext.SetUnavailableReporter")
	}
	dir := t.TempDir()
	doc := parseInline(t, "---\ntitle: P\nmode: flex\n---\n\n# P\n\n![falta](falta.png)\n")
	out := filepath.Join(t.TempDir(), "out.pdf")
	err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "pdf", AssetRoot: dir, SourceDir: dir, FailOnUnavailable: true})
	if err != nil && strings.Contains(err.Error(), "failed to initialize chromium") {
		t.Skipf("no Chromium: %v", err)
	}
	var ue *UnavailableError
	if !errors.As(err, &ue) || len(ue.Items) != 1 || ue.Items[0].Detail != "falta.png" {
		t.Fatalf("want one missing image, got %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("failed build left an output file (stat err %v)", err)
	}
}
