// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/base64"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
)

// includeImagesTree arma docs/a/uno.md y docs/b/dos.md, cada uno con una
// imagen relativa a SU directorio, y un manifiesto temporal fuera de docs/
// que los incluye a los dos (el caso de exportar una selección).
func includeImagesTree(t *testing.T) (root, docs, manifest string, images map[string][]byte) {
	t.Helper()
	root = t.TempDir()
	docs = filepath.Join(root, "docs")
	images = map[string][]byte{
		"a/u.png":     solidPNG(t, color.RGBA{255, 0, 0, 255}),
		"b/img/d.png": solidPNG(t, color.RGBA{0, 0, 255, 255}),
	}
	for rel, data := range images {
		writeFile(t, filepath.Join(docs, filepath.FromSlash(rel)), data)
	}
	writeFile(t, filepath.Join(docs, "a", "uno.md"), []byte("---\ntitle: Uno\n---\n\n## Uno\n\n![U](u.png)\n"))
	writeFile(t, filepath.Join(docs, "b", "dos.md"), []byte("## Dos\n\nTexto.\n\n![D](img/d.png)\n"))
	manifest = filepath.Join(t.TempDir(), "seleccion.md")
	writeFile(t, manifest, []byte("---\ntitle: Selección\n---\n\n# Selección\n\n@include a/uno.md\n\n@include b/dos.md\n"))
	return root, docs, manifest, images
}

func TestBuild_IncludedImagesDOCX(t *testing.T) {
	_, docs, manifest, images := includeImagesTree(t)
	out := t.TempDir()
	runBuild(t, manifest, "--format", "docx", "--output", out, "--base-dir", docs)
	media := docxMedia(t, filepath.Join(out, "seleccion.docx"))
	for rel, want := range images {
		if !containsBytes(media, want) {
			t.Errorf("%s, imagen de un archivo incluido, no está embebida", rel)
		}
	}
}

func TestBuild_IncludedImagesHTMLOfflineInline(t *testing.T) {
	_, docs, manifest, images := includeImagesTree(t)
	out := t.TempDir()
	runBuild(t, manifest, "--format", "html", "--render-mode", "offline-inline", "--output", out, "--base-dir", docs)
	data, err := os.ReadFile(filepath.Join(out, "seleccion.html"))
	if err != nil {
		t.Fatal(err)
	}
	for rel, img := range images {
		if !bytes.Contains(data, []byte(base64.StdEncoding.EncodeToString(img))) {
			t.Errorf("%s, imagen de un archivo incluido, no quedó inlineada", rel)
		}
	}
}

// TestBuild_IncludedImagesHTMLBrowser: en modo browser el src queda
// relativo al --base-dir, que es donde el HTML de un documento raíz vive
// normalmente; antes quedaba relativo al archivo incluido y no resolvía.
func TestBuild_IncludedImagesHTMLBrowser(t *testing.T) {
	_, docs, manifest, _ := includeImagesTree(t)
	runBuild(t, manifest, "--format", "html", "--output", docs, "--base-dir", docs)
	data, err := os.ReadFile(filepath.Join(docs, "seleccion.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{"a/u.png", "b/img/d.png"} {
		if !bytes.Contains(data, []byte(`src="`+src+`"`)) {
			t.Errorf("el HTML no lleva src=%q", src)
		}
	}
}

func TestBuild_IncludedImagesPDF(t *testing.T) {
	if _, err := chromium.NewChromiumManager("", false, &renderer.ChromiumLoggerAdapter{Logger: util.NewNoop()}).Detect(); err != nil {
		t.Skip("Chromium not available")
	}
	_, docs, manifest, _ := includeImagesTree(t)
	out := t.TempDir()
	log := captureStderr(t, func() {
		runBuild(t, manifest, "--format", "pdf", "--output", out, "--base-dir", docs)
	})
	assertPDF(t, filepath.Join(out, "seleccion.pdf"))
	if strings.Contains(log, "[IMAGE]") {
		t.Errorf("hubo avisos de imagen en el PDF:\n%s", log)
	}
}

// TestBuild_IncludedImagesWithoutBaseDir: el mismo caso sin temporal, un
// archivo raíz real en docs/ con sus @include.
func TestBuild_IncludedImagesWithoutBaseDir(t *testing.T) {
	_, docs, manifest, images := includeImagesTree(t)
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(docs, "main.md")
	writeFile(t, main, data)
	out := t.TempDir()
	runBuild(t, main, "--format", "docx", "--output", out)
	media := docxMedia(t, filepath.Join(out, "main.docx"))
	for rel, want := range images {
		if !containsBytes(media, want) {
			t.Errorf("%s no está embebida", rel)
		}
	}
}

// TestBuild_IncludedImagesStillConfined: un ".." escrito en un archivo
// incluido que sale de la raíz de imágenes sigue rechazado; con una raíz
// que lo contiene, se resuelve desde el archivo incluido.
func TestBuild_IncludedImagesStillConfined(t *testing.T) {
	root, docs, _, _ := includeImagesTree(t)
	writeFile(t, filepath.Join(root, "secret.png"), []byte("THIS-IS-SECRET-CONTENT-NOT-A-REAL-PNG"))
	writeFile(t, filepath.Join(docs, "a", "malo.md"), []byte("## Malo\n\n![x](../../secret.png)\n"))
	manifest := filepath.Join(t.TempDir(), "m.md")
	writeFile(t, manifest, []byte("---\ntitle: M\n---\n\n# M\n\n@include a/malo.md\n"))

	out := t.TempDir()
	log := captureStderr(t, func() {
		runBuild(t, manifest, "--format", "docx", "--output", out, "--base-dir", docs)
	})
	for _, data := range docxMedia(t, filepath.Join(out, "m.docx")) {
		if bytes.Contains(data, []byte("SECRET")) {
			t.Fatal("un archivo fuera de la raíz terminó en word/media/")
		}
	}
	if !strings.Contains(log, "outside asset root") {
		t.Errorf("la imagen fuera de la raíz no se rechazó por confinamiento:\n%s", log)
	}

	out = t.TempDir()
	log = captureStderr(t, func() {
		runBuild(t, manifest, "--format", "docx", "--output", out, "--base-dir", docs, "--asset-root", root)
	})
	if strings.Contains(log, "outside asset root") {
		t.Error("con --asset-root en la raíz, ../../secret.png desde docs/a/ debería quedar dentro")
	}
}

// TestBuild_HTMLTableColumnAlign: la alineación de columnas llega al HTML
// de doclang (el renderer de core, ver core/renderer/table_align_test.go).
func TestBuild_HTMLTableColumnAlign(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "t.md")
	writeFile(t, in, []byte("---\ntitle: T\n---\n\n# T\n\n| A | B | C |\n|:---|:---:|---:|\n| 1 | 2 | 3 |\n"))
	runBuild(t, in, "--format", "html", "--output", dir)
	data, err := os.ReadFile(filepath.Join(dir, "t.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<td style="text-align: left">1</td>`, `<td style="text-align: center">2</td>`, `<td style="text-align: right">3</td>`} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("falta %s", want)
		}
	}
}

// TestBuild_IncludedImagesWithNormalizer: con el normalizador encendido
// (flex-full) la posición de cada imagen sigue cuadrando con su archivo.
// (Una imagen dentro de un grid no llega como ImageElement en ninguna
// salida, ni en el archivo raíz, así que no aplica acá.)
func TestBuild_IncludedImagesWithNormalizer(t *testing.T) {
	_, docs, _, images := includeImagesTree(t)
	manifest := filepath.Join(t.TempDir(), "sel.md")
	writeFile(t, manifest, []byte("---\ntitle: Selección\nmode: flex-full\n---\n\n# Selección\n\nTexto inicial.\n\n- uno\n- dos\n\n@include a/uno.md\n\n@include b/dos.md\n"))
	out := t.TempDir()
	runBuild(t, manifest, "--format", "docx", "--output", out, "--base-dir", docs)
	media := docxMedia(t, filepath.Join(out, "sel.docx"))
	for rel, want := range images {
		if !containsBytes(media, want) {
			t.Errorf("%s no está embebida", rel)
		}
	}
}

// threeFolderTree: un manifiesto temporal que incluye un archivo de cada
// una de tres carpetas. Cada archivo tiene una imagen de bloque y una inline
// (en un párrafo, en un ítem de lista y en una celda), todas relativas a su
// propia carpeta.
func threeFolderTree(t *testing.T) (docs, manifest string, images map[string][]byte) {
	t.Helper()
	docs = filepath.Join(t.TempDir(), "docs")
	images = map[string][]byte{
		"a/block.png":     solidPNG(t, color.RGBA{255, 0, 0, 255}),
		"a/inline.png":    solidPNG(t, color.RGBA{128, 0, 0, 255}),
		"b/img/block.png": solidPNG(t, color.RGBA{0, 255, 0, 255}),
		"b/inline.png":    solidPNG(t, color.RGBA{0, 128, 0, 255}),
		"c/block.png":     solidPNG(t, color.RGBA{0, 0, 255, 255}),
		"c/i/inline.png":  solidPNG(t, color.RGBA{0, 0, 128, 255}),
	}
	for rel, data := range images {
		writeFile(t, filepath.Join(docs, filepath.FromSlash(rel)), data)
	}
	writeFile(t, filepath.Join(docs, "a", "a.md"), []byte("---\ntitle: A\n---\n\n## A\n\n![Bloque A](block.png)\n\nTexto con ![inline A](inline.png) en medio.\n"))
	writeFile(t, filepath.Join(docs, "b", "b.md"), []byte("## B\n\n![Bloque B](img/block.png)\n\n- uno\n- ítem con ![inline B](./inline.png)\n"))
	writeFile(t, filepath.Join(docs, "c", "c.md"), []byte("## C\n\n![Bloque C](block.png)\n\n| Col | Img |\n|---|---|\n| x | ![inline C](i/inline.png) |\n"))
	manifest = filepath.Join(t.TempDir(), "seleccion.md")
	writeFile(t, manifest, []byte("---\ntitle: Selección\n---\n\n# Selección\n\n@include a/a.md\n\n@include b/b.md\n\n@include c/c.md\n"))
	return docs, manifest, images
}

func TestBuild_IncludedBlockAndInlineImages(t *testing.T) {
	docs, manifest, images := threeFolderTree(t)

	t.Run("docx", func(t *testing.T) {
		out := t.TempDir()
		runBuild(t, manifest, "--format", "docx", "--output", out, "--base-dir", docs)
		media := docxMedia(t, filepath.Join(out, "seleccion.docx"))
		for rel, want := range images {
			if !containsBytes(media, want) {
				t.Errorf("%s no está embebida", rel)
			}
		}
	})

	t.Run("html offline-inline", func(t *testing.T) {
		out := t.TempDir()
		runBuild(t, manifest, "--format", "html", "--render-mode", "offline-inline", "--output", out, "--base-dir", docs)
		data, err := os.ReadFile(filepath.Join(out, "seleccion.html"))
		if err != nil {
			t.Fatal(err)
		}
		for rel, img := range images {
			if !bytes.Contains(data, []byte(base64.StdEncoding.EncodeToString(img))) {
				t.Errorf("%s no quedó inlineada", rel)
			}
		}
	})

	// browser y offline-assets dejan el src relativo: queda relativo a
	// --base-dir, y con el HTML escrito ahí cada src resuelve.
	for _, mode := range []string{"browser", "offline-assets"} {
		t.Run("html "+mode, func(t *testing.T) {
			runBuild(t, manifest, "--format", "html", "--render-mode", mode, "--output", docs, "--base-dir", docs)
			data, err := os.ReadFile(filepath.Join(docs, "seleccion.html"))
			if err != nil {
				t.Fatal(err)
			}
			for rel := range images {
				if !bytes.Contains(data, []byte(`src="`+rel+`"`)) {
					t.Errorf("el HTML no lleva src=%q", rel)
				}
			}
		})
	}

	t.Run("pdf", func(t *testing.T) {
		if _, err := chromium.NewChromiumManager("", false, &renderer.ChromiumLoggerAdapter{Logger: util.NewNoop()}).Detect(); err != nil {
			t.Skip("Chromium not available")
		}
		out := t.TempDir()
		log := captureStderr(t, func() {
			runBuild(t, manifest, "--format", "pdf", "--output", out, "--base-dir", docs)
		})
		assertPDF(t, filepath.Join(out, "seleccion.pdf"))
		if strings.Contains(log, "[IMAGE]") {
			t.Errorf("hubo avisos de imagen en el PDF:\n%s", log)
		}
	})
}
