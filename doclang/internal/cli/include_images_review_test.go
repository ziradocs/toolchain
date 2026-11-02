// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestBuild_IncludedImages_ParagraphCrossesIncludeBorder: un incluido sin
// salto de línea final y el archivo raíz que sigue el mismo párrafo. Las
// dos imágenes dicen (logo.png): solo se reescribe la del incluido.
func TestBuild_IncludedImages_ParagraphCrossesIncludeBorder(t *testing.T) {
	docs := filepath.Join(t.TempDir(), "docs")
	writeFile(t, filepath.Join(docs, "logo.png"), solidPNG(t, color.RGBA{255, 0, 0, 255}))
	writeFile(t, filepath.Join(docs, "chap", "logo.png"), solidPNG(t, color.RGBA{0, 255, 0, 255}))
	writeFile(t, filepath.Join(docs, "chap", "x.md"), []byte("Del capítulo ![i](logo.png)"))
	main := filepath.Join(docs, "main.md")
	writeFile(t, main, []byte("---\ntitle: M\n---\n\n# M\n\n@include chap/x.md\ny del raíz ![r](logo.png) fin.\n"))

	runBuild(t, main, "--format", "html", "--output", docs)
	html := readFile(t, filepath.Join(docs, "main.html"))
	if !strings.Contains(html, `<img src="chap/logo.png" alt="i">`) || !strings.Contains(html, `<img src="logo.png" alt="r">`) {
		t.Errorf("srcs inesperados:\n%s", grepImgs(html))
	}
}

// TestBuild_IncludedImages_RootListContinuesWithInclude: una lista del
// archivo raíz que sigue con un @include: los ítems del incluido se
// reescriben y el del raíz no.
func TestBuild_IncludedImages_RootListContinuesWithInclude(t *testing.T) {
	docs := filepath.Join(t.TempDir(), "docs")
	writeFile(t, filepath.Join(docs, "chap", "items.md"), []byte("- dos ![d](d.png)\n- tres ![t](t.png)\n"))
	main := filepath.Join(docs, "main.md")
	writeFile(t, main, []byte("---\ntitle: M\n---\n\n# M\n\n- uno ![u](u.png)\n@include chap/items.md\n"))

	runBuild(t, main, "--format", "html", "--output", docs)
	html := readFile(t, filepath.Join(docs, "main.html"))
	for _, want := range []string{`src="u.png"`, `src="chap/d.png"`, `src="chap/t.png"`} {
		if !strings.Contains(html, want) {
			t.Errorf("falta %s:\n%s", want, grepImgs(html))
		}
	}
}

// TestBuild_IncludedImages_LongList: la imagen del último ítem de una lista
// de 450 ítems, muy por debajo de la línea donde empieza la lista.
func TestBuild_IncludedImages_LongList(t *testing.T) {
	docs := filepath.Join(t.TempDir(), "docs")
	var list strings.Builder
	for i := 1; i < 450; i++ {
		fmt.Fprintf(&list, "- ítem %d\n", i)
	}
	list.WriteString("- último ![z](z.png)\n")
	writeFile(t, filepath.Join(docs, "chap", "larga.md"), []byte(list.String()))
	writeFile(t, filepath.Join(docs, "chap", "p.md"), []byte(strings.Repeat("Línea de un párrafo largo\n", 450)+"con ![p](p.png) al final.\n"))
	main := filepath.Join(docs, "main.md")
	writeFile(t, main, []byte("---\ntitle: M\n---\n\n# M\n\n@include chap/larga.md\n\n@include chap/p.md\n"))

	runBuild(t, main, "--format", "html", "--output", docs)
	html := readFile(t, filepath.Join(docs, "main.html"))
	for _, want := range []string{`src="chap/z.png"`, `src="chap/p.png"`} {
		if !strings.Contains(html, want) {
			t.Errorf("falta %s:\n%s", want, grepImgs(html))
		}
	}
}

// TestBuild_IncludedImages_FolderWithSpaces: imágenes de bloque e inline de
// un incluido en "cap uno/". Se resuelven en DOCX, HTML offline-inline y PDF,
// y la salida Markdown las escribe con <...>, válidas en CommonMark.
func TestBuild_IncludedImages_FolderWithSpaces(t *testing.T) {
	docs := filepath.Join(t.TempDir(), "docs")
	images := map[string][]byte{
		"cap uno/spb.png": solidPNG(t, color.RGBA{255, 0, 255, 255}),
		"cap uno/sp.png":  solidPNG(t, color.RGBA{0, 255, 255, 255}),
	}
	for rel, data := range images {
		writeFile(t, filepath.Join(docs, filepath.FromSlash(rel)), data)
	}
	writeFile(t, filepath.Join(docs, "cap uno", "s.md"), []byte("## S\n\n![spb](spb.png)\n\nTexto ![sp](sp.png) fin.\n"))
	manifest := filepath.Join(t.TempDir(), "sel.md")
	writeFile(t, manifest, []byte("---\ntitle: S\n---\n\n# S\n\n@include cap uno/s.md\n"))

	out := t.TempDir()
	runBuild(t, manifest, "--format", "docx", "--output", out, "--base-dir", docs)
	media := docxMedia(t, filepath.Join(out, "sel.docx"))
	for rel, want := range images {
		if !containsBytes(media, want) {
			t.Errorf("docx: %s no está embebida", rel)
		}
	}

	runBuild(t, manifest, "--format", "html", "--render-mode", "offline-inline", "--output", out, "--base-dir", docs)
	html := readFile(t, filepath.Join(out, "sel.html"))
	for rel, img := range images {
		if !strings.Contains(html, base64.StdEncoding.EncodeToString(img)) {
			t.Errorf("html: %s no quedó inlineada", rel)
		}
	}

	runBuild(t, manifest, "--format", "markdown", "--output", out, "--base-dir", docs)
	md := readFile(t, filepath.Join(out, "sel.md"))
	for _, want := range []string{"![spb](<cap uno/spb.png>)", "![sp](<cap uno/sp.png>)"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown: falta %s en\n%s", want, md)
		}
	}

	if _, err := chromium.NewChromiumManager("", false, &renderer.ChromiumLoggerAdapter{Logger: util.NewNoop()}).Detect(); err != nil {
		t.Log("Chromium not available: PDF omitido")
		return
	}
	log := captureStderr(t, func() {
		runBuild(t, manifest, "--format", "pdf", "--output", out, "--base-dir", docs)
	})
	assertPDF(t, filepath.Join(out, "sel.pdf"))
	if strings.Contains(log, "[IMAGE]") {
		t.Errorf("pdf: hubo avisos de imagen:\n%s", log)
	}
}

// TestBuild_IncludedImages_Heading: una imagen dentro de un encabezado de
// un incluido.
func TestBuild_IncludedImages_Heading(t *testing.T) {
	docs := filepath.Join(t.TempDir(), "docs")
	h := solidPNG(t, color.RGBA{10, 20, 30, 255})
	writeFile(t, filepath.Join(docs, "chap", "h.png"), h)
	writeFile(t, filepath.Join(docs, "chap", "x.md"), []byte("## Cap\n\n### Título ![h](h.png)\n\nTexto.\n"))
	main := filepath.Join(docs, "main.md")
	writeFile(t, main, []byte("---\ntitle: M\n---\n\n# M\n\n@include chap/x.md\n"))

	runBuild(t, main, "--format", "html", "--output", docs)
	if html := readFile(t, filepath.Join(docs, "main.html")); !strings.Contains(html, `<img src="chap/h.png"`) {
		t.Errorf("browser: el src del encabezado no se reescribió:\n%s", grepImgs(html))
	}
	out := t.TempDir()
	runBuild(t, main, "--format", "html", "--render-mode", "offline-inline", "--output", out)
	if html := readFile(t, filepath.Join(out, "main.html")); !strings.Contains(html, base64.StdEncoding.EncodeToString(h)) {
		t.Error("offline-inline: la imagen del encabezado no quedó inlineada")
	}
}

// TestBuild_IncludedImages_SymlinkedRoots: con L un symlink,
// `--base-dir L/docs --include-root L/docs` es un solo directorio.
func TestBuild_IncludedImages_SymlinkedRoots(t *testing.T) {
	real := t.TempDir()
	docs := filepath.Join(real, "docs")
	img := solidPNG(t, color.RGBA{1, 2, 3, 255})
	writeFile(t, filepath.Join(docs, "a", "x.png"), img)
	writeFile(t, filepath.Join(docs, "a", "x.md"), []byte("## A\n\n![b](x.png)\n\nInline ![i](x.png).\n"))
	link := filepath.Join(t.TempDir(), "L")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks no disponibles: %v", err)
	}
	manifest := filepath.Join(t.TempDir(), "sel.md")
	writeFile(t, manifest, []byte("---\ntitle: S\n---\n\n# S\n\n@include a/x.md\n"))
	lDocs := filepath.Join(link, "docs")

	out := t.TempDir()
	runBuild(t, manifest, "--format", "html", "--output", out, "--base-dir", lDocs, "--include-root", lDocs)
	html := readFile(t, filepath.Join(out, "sel.html"))
	if strings.Count(html, `src="a/x.png"`) != 2 {
		t.Errorf("srcs inesperados con symlink:\n%s", grepImgs(html))
	}
	log := captureStderr(t, func() {
		runBuild(t, manifest, "--format", "docx", "--output", out, "--base-dir", lDocs, "--include-root", lDocs)
	})
	if !containsBytes(docxMedia(t, filepath.Join(out, "sel.docx")), img) || strings.Contains(log, "not found") {
		t.Errorf("docx con symlink: imagen no embebida\n%s", log)
	}
}

func grepImgs(html string) string {
	var b bytes.Buffer
	for _, part := range strings.Split(html, "<img ")[1:] {
		end := strings.Index(part, ">")
		if end < 0 {
			end = len(part)
		}
		b.WriteString("<img " + part[:end] + ">\n")
	}
	return b.String()
}
