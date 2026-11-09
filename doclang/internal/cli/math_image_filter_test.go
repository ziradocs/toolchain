// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"

	"go.ziradocs.com/doclang/v2/internal/generator"
	"go.ziradocs.com/doclang/v2/internal/pipeline"
)

// figureFilterSource es un filtro real como el de una plataforma que dibuja
// sus figuras en un navegador propio y luego exporta con `doclang build
// --filter`: cambia cada gráfica por una imagen y cada fórmula sin label por
// otra. Con MODE=prerender, a una fórmula con label le pone su render en
// "image" (math-image-v1) para que conserve número, ancla y \ref; con
// MODE=replace-all la cambia también por una imagen. La declaración raíz solo
// la toca para agregar math-image-v1; lo que quede sin contenido lo recalcula
// el CLI.
const figureFilterSource = `package main

import (
	"encoding/json"
	"io"
	"os"
)

var addedMathImage bool

func walk(v any) {
	switch x := v.(type) {
	case []any:
		for i, child := range x {
			m, ok := child.(map[string]any)
			if !ok {
				continue
			}
			label, _ := m["label"].(string)
			switch {
			case m["type"] == "math" && label != "" && os.Getenv("MODE") == "prerender":
				m["image"] = "eq.png"
				addedMathImage = true
			case m["type"] == "chart" || m["type"] == "math":
				x[i] = map[string]any{"type": "image", "position": m["position"], "endPosition": m["endPosition"], "source": "fig.png", "alt": "figura"}
			default:
				walk(m)
			}
		}
	case map[string]any:
		for _, key := range []string{"contentBlocks", "elements", "columns"} {
			walk(x[key])
		}
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--ziradocs-capabilities" {
		os.Stdout.WriteString(os.Getenv("HANDSHAKE"))
		return
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		panic(err)
	}
	walk(doc)
	if addedMathImage {
		caps, _ := doc["capabilities"].([]any)
		doc["capabilities"] = append(caps, "math-image-v1")
		doc["schemaVersion"] = "2.27.0"
	}
	out, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	os.Stdout.Write(out)
}
`

const figureFilterFixture = `---
title: Figuras
lang: es
ast_capabilities:
  - math-source-v1
---

# Uno

Vea \ref{eq:a} y la gráfica.

<<math>>
E = mc^2
caption: "Energía"
label: "eq:a"
<<end>>

<<math>>
a^2 + b^2 = c^2
<<end>>

<<chart: bar>>
unit: "MXN"
data: [
  ["Q1", 120],
  ["Q2", 250]
]
<<end>>
`

// buildFigureFilter compila figureFilterSource y devuelve un script que lo
// corre en mode, con un handshake que acepta las versiones del documento de
// prueba y todas las capabilities de este core.
func buildFigureFilter(t *testing.T, mode string) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain no disponible en PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(figureFilterSource), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "figure-filter")
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compilando el filtro de prueba: %v\n%s", err, out)
	}
	handshake, err := json.Marshal(map[string][]string{
		"astSchemaVersions": {"2.22.0", "2.23.0", "2.27.0"},
		"features":          ast.KnownCapabilities(),
	})
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "filter.sh")
	body := "#!/bin/sh\nMODE=" + mode + " HANDSHAKE='" + string(handshake) + "' exec " + bin + " \"$@\"\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return script
}

func writeTestPNG(t *testing.T, path string, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, h/2, color.Black)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type figureBuild struct {
	dir    string
	eqPNG  []byte
	filter string
	doc    *ast.AST
}

func newFigureBuild(t *testing.T, mode string) *figureBuild {
	t.Helper()
	b := &figureBuild{dir: t.TempDir(), filter: buildFigureFilter(t, mode)}
	if err := os.WriteFile(filepath.Join(b.dir, "doc.doclang"), []byte(figureFilterFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	b.eqPNG = writeTestPNG(t, filepath.Join(b.dir, "eq.png"), 120, 30)
	writeTestPNG(t, filepath.Join(b.dir, "fig.png"), 200, 100)
	return b
}

// run compila el documento con el filtro y devuelve la salida; el AST que
// llega al lint (después de los filtros) queda en b.doc.
func (b *figureBuild) run(t *testing.T, format string, extra ...string) []byte {
	t.Helper()
	out := filepath.Join(b.dir, "out-"+format+strings.Join(extra, ""))
	postLint := func(doc *ast.AST, _ []diagnostics.Diagnostic, _ []linter.WaivedDiagnostic) error {
		b.doc = doc
		return nil
	}
	args := append([]string{filepath.Join(b.dir, "doc.doclang"), "--filter", b.filter, "--format", format, "--output", out,
		"--chromium-path", filepath.Join(b.dir, "no-chromium")}, extra...)
	cmd := NewBuildCommand(nil, nil, nil, nil, postLint)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("build %s %v: %v", format, extra, err)
	}
	ext := map[string]string{"html": ".html", "docx": ".docx"}[format]
	data, err := os.ReadFile(filepath.Join(out, "doc"+ext))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func docxParts(t *testing.T, data []byte) (document string, media [][]byte) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case f.Name == "word/document.xml":
			document = string(content)
		case strings.HasPrefix(f.Name, "word/media/"):
			media = append(media, content)
		}
	}
	return document, media
}

func elementsOf(doc *ast.AST) []ast.Element {
	var out []ast.Element
	_ = ast.Walk(doc, func(n ast.Node) error {
		if e, ok := n.(ast.Element); ok {
			out = append(out, e)
		}
		return nil
	})
	return out
}

func requireMathImageCore(t *testing.T) {
	t.Helper()
	if !slices.Contains(ast.KnownCapabilities(), "math-image-v1") {
		t.Skip("el core enlazado no tiene math-image-v1")
	}
}

// Punta a punta: un filtro real reemplaza las gráficas y las fórmulas; la de
// label conserva número, ancla y \ref con su render previo, en HTML y DOCX,
// sin Chromium, y el AST que sigue el pipeline declara lo que trae.
func TestBuild_FilterPrerendersEquationsAndReplacesFigures(t *testing.T) {
	requireMathImageCore(t)
	b := newFigureBuild(t, "prerender")

	htmlOut := string(b.run(t, "html"))

	// El contrato se recalculó: chart-content-v1 se fue con la gráfica,
	// math-source-v1 sigue (queda una fórmula) y math-image-v1 entró.
	if b.doc.SchemaVersion != "2.27.0" || !reflect.DeepEqual(b.doc.Capabilities, []string{"math-source-v1", "math-image-v1"}) {
		t.Fatalf("contrato tras el filtro = %s %v", b.doc.SchemaVersion, b.doc.Capabilities)
	}
	var maths, images int
	for _, e := range elementsOf(b.doc) {
		switch v := e.(type) {
		case *ast.MathElement:
			maths++
			if v.Label != "eq:a" || v.Number != 1 || generator.MathImage(v) != "eq.png" || v.Content != "E = mc^2" {
				t.Errorf("fórmula con label = %+v", v)
			}
		case *ast.ImageElement:
			images++
		case *ast.ChartElement:
			t.Error("quedó una gráfica")
		}
	}
	if maths != 1 || images != 2 {
		t.Fatalf("fórmulas=%d imágenes=%d", maths, images)
	}
	for _, target := range []struct{ format, mode string }{{"docx", "browser"}, {"html", "offline-inline"}, {"html", "offline-assets"}} {
		if reasons := pipeline.ChromiumRequirements(b.doc, target.format, target.mode, "chromium"); len(reasons) != 0 {
			t.Errorf("%s %s pide Chromium: %v", target.format, target.mode, reasons)
		}
	}

	// HTML: número, ancla, leyenda, render y el \ref que apunta al ancla.
	for _, want := range []string{
		`<div class="math-block" id="eq-a"><img src="eq.png" alt="E = mc^2" class="math-diagram math-image"><span class="math-number">(1)</span><div class="math-caption">Energía</div></div>`,
		`href="#eq-a"`,
		`Ecuación 1`,
	} {
		if !strings.Contains(htmlOut, want) {
			t.Errorf("HTML sin %q", want)
		}
	}

	// HTML offline-inline (la ruta del PDF): el render entra como data: URI y
	// no hace falta Chromium (--chromium-path apunta a un archivo que no
	// existe; si se pidiera, el build fallaría).
	inline := string(b.run(t, "html", "--render-mode", "offline-inline"))
	if !strings.Contains(inline, `id="eq-a"><img src="data:image/png;base64,`) || !strings.Contains(inline, `(1)</span>`) {
		t.Error("HTML offline-inline sin el render inlineado o sin número")
	}

	// DOCX: el PNG del filtro, el "(1)", la leyenda y el texto del \ref, sin
	// el marcador de ecuación no disponible.
	document, media := docxParts(t, b.run(t, "docx"))
	for _, want := range []string{">(1)<", ">Energía<", ">Ecuación 1<"} {
		if !strings.Contains(document, want) {
			t.Errorf("DOCX sin %q", want)
		}
	}
	if strings.Contains(document, "no disponible") {
		t.Error("DOCX dejó un marcador de no disponible")
	}
	found := false
	for _, m := range media {
		found = found || bytes.Equal(m, b.eqPNG)
	}
	if !found {
		t.Errorf("DOCX sin el PNG de la ecuación (%d archivos en word/media)", len(media))
	}
}

// Un filtro que reemplaza todas las fórmulas, con y sin label, ya no choca
// con math-source-v1, y el AST que sigue el pipeline queda en el contrato
// legado porque no le queda ninguna extensión.
func TestBuild_FilterReplacesAllEquations(t *testing.T) {
	requireMathImageCore(t) // el recálculo llegó al core en la misma versión
	b := newFigureBuild(t, "replace-all")
	b.run(t, "html")
	if b.doc.SchemaVersion != ast.LegacySchemaVersion || len(b.doc.Capabilities) != 0 {
		t.Fatalf("contrato tras el filtro = %s %v", b.doc.SchemaVersion, b.doc.Capabilities)
	}
	for _, e := range elementsOf(b.doc) {
		switch e.(type) {
		case *ast.MathElement, *ast.ChartElement:
			t.Fatalf("quedó un %T", e)
		}
	}
	document, _ := docxParts(t, b.run(t, "docx"))
	if strings.Contains(document, "no disponible") {
		t.Error("DOCX dejó un marcador de no disponible")
	}
}
