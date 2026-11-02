// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/base64"
	"image/color"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// coreReadsCommonMarkDestinations dice si el core enlazado lee los destinos
// de imagen como CommonMark (título aparte, <...>). Con el core publicado
// anterior (GOWORK=off) los casos que dependen de eso se saltan.
func coreReadsCommonMarkDestinations() bool {
	_, n, ok := renderer.ScanLinkDestination("<a)b>)")
	return ok && n == 6
}

// imageDestinationsTree arma un documento con una imagen distinta (por color)
// para cada forma de destino, en el archivo raíz y en un incluido que vive en
// una carpeta con espacios.
func imageDestinationsTree(t *testing.T) (dir, doc string, images map[string][]byte) {
	t.Helper()
	dir = t.TempDir()
	images = map[string][]byte{}
	n := uint8(0)
	add := func(rel string) {
		n++
		data := solidPNG(t, color.RGBA{n * 20, 255 - n*20, n * 7, 255})
		images[rel] = data
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), data)
	}
	for _, rel := range []string{
		"img/dq.png", "img/sq.png", "img/pq.png", "mi dir/ang.png", "mi dir/pct.png",
		"img/uno.png", "img/dos.png", "img/inl.png",
		"cap uno/img/t.png", "cap uno/mi dir/b.png", "cap uno/img/h.png",
	} {
		add(rel)
	}
	writeFile(t, filepath.Join(dir, "cap uno", "ch.md"), []byte(
		"## Cap ![h](img/h.png \"th\")\n\n![t](img/t.png \"Título\")\n\n![b](<mi dir/b.png>)\n"))
	doc = filepath.Join(dir, "doc.md")
	writeFile(t, doc, []byte(`# Destinos

![dq](img/dq.png "t1")

![sq](img/sq.png 't2')

![pq](img/pq.png (t3))

![ang](<mi dir/ang.png>)

![pct](mi%20dir/pct.png)

![uno](img/uno.png){width=30%} ![dos](img/dos.png)

Inline ![inl](img/inl.png "ti") aquí.

![suelta](img/dq.png otra)

@include cap uno/ch.md
`))
	return dir, doc, images
}

func TestBuild_ImageDestinationsAllOutputs(t *testing.T) {
	if !coreReadsCommonMarkDestinations() {
		t.Skip("el core enlazado no lee los destinos como CommonMark")
	}
	dir, doc, images := imageDestinationsTree(t)

	t.Run("docx", func(t *testing.T) {
		out := t.TempDir()
		runBuild(t, doc, "--format", "docx", "--output", out)
		media := docxMedia(t, filepath.Join(out, "doc.docx"))
		for rel, want := range images {
			if !containsBytes(media, want) {
				t.Errorf("%s no está embebida en el DOCX", rel)
			}
		}
		if xml := docxDocumentXML(t, filepath.Join(out, "doc.docx")); strings.Contains(xml, "{width") {
			t.Error("los atributos de la imagen inline salieron como texto en el DOCX")
		}
	})

	t.Run("html offline-inline", func(t *testing.T) {
		out := t.TempDir()
		runBuild(t, doc, "--format", "html", "--render-mode", "offline-inline", "--output", out)
		html := readFile(t, filepath.Join(out, "doc.html"))
		for rel, img := range images {
			if !strings.Contains(html, base64.StdEncoding.EncodeToString(img)) {
				t.Errorf("%s no quedó inlineada", rel)
			}
		}
	})

	t.Run("html browser", func(t *testing.T) {
		out := filepath.Join(dir, "out")
		runBuild(t, doc, "--format", "html", "--output", out)
		html := readFile(t, filepath.Join(out, "doc.html"))
		if strings.Contains(html, "{width") {
			t.Error("los atributos de la imagen inline salieron como texto en el HTML")
		}
		for _, want := range []string{
			`<img src="img/dq.png" alt="dq" title="t1">`,
			`<img src="img/sq.png" alt="sq" title="t2">`,
			`<img src="img/pq.png" alt="pq" title="t3">`,
			`<img src="mi%20dir/ang.png" alt="ang">`,
			`<img src="mi%20dir/pct.png" alt="pct">`,
			`<img src="img/uno.png" alt="uno"> <img src="img/dos.png" alt="dos">`,
			`<img src="img/inl.png" alt="inl" title="ti">`,
			`<img src="img/dq.png%20otra" alt="suelta">`,
			`<img src="cap%20uno/img/t.png" alt="t" title="Título">`,
			`<img src="cap%20uno/mi%20dir/b.png" alt="b">`,
			`<img src="cap%20uno/img/h.png" alt="h" title="th">`,
		} {
			if !strings.Contains(html, want) {
				t.Errorf("el HTML no lleva %s", want)
			}
		}
	})

	t.Run("markdown", func(t *testing.T) {
		out := t.TempDir()
		runBuild(t, doc, "--format", "markdown", "--output", out)
		md := readFile(t, filepath.Join(out, "doc.md"))
		for _, want := range []string{
			`![dq](img/dq.png "t1")`,
			`![sq](img/sq.png "t2")`,
			`![ang](<mi dir/ang.png>)`,
			`![pct](mi%20dir/pct.png)`,
			`![uno](img/uno.png){width=30%} ![dos](img/dos.png)`,
			`![suelta](<img/dq.png otra>)`,
			`![t](<cap uno/img/t.png> "Título")`,
			`![b](<cap uno/mi dir/b.png>)`,
			`## Cap ![h](cap%20uno/img/h.png "th")`,
		} {
			if !strings.Contains(md, want) {
				t.Errorf("el Markdown no lleva %s:\n%s", want, md)
			}
		}
	})
}

// Una carpeta incluida con "%" en el nombre: la ruta reescrita es una URL, así
// que el "%" va codificado y al leer el archivo se decodifica.
func TestBuild_ImageDestinationsPercentFolder(t *testing.T) {
	if !coreReadsCommonMarkDestinations() {
		t.Skip("el core enlazado no lee los destinos como CommonMark")
	}
	dir := t.TempDir()
	img := solidPNG(t, color.RGBA{1, 2, 3, 255})
	writeFile(t, filepath.Join(dir, "100%", "c.png"), img)
	writeFile(t, filepath.Join(dir, "100%", "p.md"), []byte("![c](c.png)\n\nTexto ![ci](c.png).\n"))
	doc := filepath.Join(dir, "doc.md")
	writeFile(t, doc, []byte("# P\n\n@include 100%/p.md\n"))
	out := t.TempDir()
	runBuild(t, doc, "--format", "docx", "--output", out)
	if !containsBytes(docxMedia(t, filepath.Join(out, "doc.docx")), img) {
		t.Error("la imagen de 100%/ no está en el DOCX")
	}
	runBuild(t, doc, "--format", "html", "--render-mode", "offline-inline", "--output", out)
	if html := readFile(t, filepath.Join(out, "doc.html")); strings.Count(html, base64.StdEncoding.EncodeToString(img)) != 2 {
		t.Error("las imágenes de 100%/ no quedaron inlineadas")
	}
}

// %20 en una imagen de bloque se decodifica al leer el archivo del DOCX con
// cualquier core: la decodificación vive en doclang.
func TestBuild_ImagePercentDestinationDOCX(t *testing.T) {
	dir := t.TempDir()
	img := solidPNG(t, color.RGBA{9, 8, 7, 255})
	writeFile(t, filepath.Join(dir, "mi dir", "a.png"), img)
	doc := filepath.Join(dir, "doc.md")
	writeFile(t, doc, []byte("# P\n\n![a](mi%20dir/a.png)\n"))
	out := t.TempDir()
	runBuild(t, doc, "--format", "docx", "--output", out)
	if !containsBytes(docxMedia(t, filepath.Join(out, "doc.docx")), img) {
		t.Error("mi%20dir/a.png no se encontró")
	}
}

// Una imagen en un encabezado (también dentro de un enlace) se inserta en el
// DOCX; antes salía la etiqueta <img> como texto.
func TestBuild_HeadingImageDOCX(t *testing.T) {
	dir := t.TempDir()
	plain := solidPNG(t, color.RGBA{10, 20, 30, 255})
	linked := solidPNG(t, color.RGBA{30, 20, 10, 255})
	writeFile(t, filepath.Join(dir, "h.png"), plain)
	writeFile(t, filepath.Join(dir, "l.png"), linked)
	doc := filepath.Join(dir, "doc.md")
	writeFile(t, doc, []byte("# P\n\n## Título ![h](h.png) y [![l](l.png)](https://example.com)\n\nTexto.\n"))
	out := t.TempDir()
	runBuild(t, doc, "--format", "docx", "--output", out)
	path := filepath.Join(out, "doc.docx")
	media := docxMedia(t, path)
	if !containsBytes(media, plain) || !containsBytes(media, linked) {
		t.Error("las imágenes del encabezado no están en el DOCX")
	}
	if xml := docxDocumentXML(t, path); strings.Contains(xml, "&lt;img") {
		t.Error("el encabezado lleva el <img> como texto")
	}
}
