// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/base64"
	"image/color"
	"path/filepath"
	"strings"
	"testing"
)

// assertEmbeds comprueba que el DOCX y el HTML offline-inline de manifest
// llevan cada imagen de want y ninguna de decoys.
func assertEmbeds(t *testing.T, manifest, docs string, want, decoys map[string][]byte) {
	t.Helper()
	out := t.TempDir()
	name := strings.TrimSuffix(filepath.Base(manifest), filepath.Ext(manifest))
	runBuild(t, manifest, "--format", "docx", "--output", out, "--base-dir", docs)
	media := docxMedia(t, filepath.Join(out, name+".docx"))
	runBuild(t, manifest, "--format", "html", "--render-mode", "offline-inline", "--output", out, "--base-dir", docs)
	html := readFile(t, filepath.Join(out, name+".html"))
	for rel, img := range want {
		if !containsBytes(media, img) {
			t.Errorf("docx: %s no está embebida", rel)
		}
		if !strings.Contains(html, base64.StdEncoding.EncodeToString(img)) {
			t.Errorf("html: %s no quedó inlineada", rel)
		}
	}
	for rel, img := range decoys {
		if containsBytes(media, img) {
			t.Errorf("docx: se embebió %s, la del archivo raíz", rel)
		}
		if strings.Contains(html, base64.StdEncoding.EncodeToString(img)) {
			t.Errorf("html: se inlineó %s, la del archivo raíz", rel)
		}
	}
}

// Un nombre con paréntesis después de un espacio, `fotos/a (1).png`, en un
// incluido de "cap uno/". El rebase partía la ruta en el "(" y tomaba
// "(1).png" como título: escribía `<cap uno/fotos/a> (1).png`, que ya no es
// imagen. Core lee la ruta entera, así que es la que se reescribe.
func TestBuild_IncludedImages_ParenthesizedName(t *testing.T) {
	docs := filepath.Join(t.TempDir(), "docs")
	want := map[string][]byte{"cap uno/fotos/a (1).png": solidPNG(t, color.RGBA{0, 200, 0, 255})}
	decoys := map[string][]byte{"fotos/a (1).png": solidPNG(t, color.RGBA{200, 0, 0, 255})}
	for rel, data := range want {
		writeFile(t, filepath.Join(docs, filepath.FromSlash(rel)), data)
	}
	for rel, data := range decoys {
		writeFile(t, filepath.Join(docs, filepath.FromSlash(rel)), data)
	}
	writeFile(t, filepath.Join(docs, "cap uno", "s.md"), []byte("## S\n\nTexto ![x](fotos/a (1).png) fin.\n"))
	manifest := filepath.Join(t.TempDir(), "sel.md")
	writeFile(t, manifest, []byte("---\ntitle: S\n---\n\n# S\n\n@include cap uno/s.md\n"))

	assertEmbeds(t, manifest, docs, want, decoys)

	out := t.TempDir()
	runBuild(t, manifest, "--format", "markdown", "--output", out, "--base-dir", docs)
	if md := readFile(t, filepath.Join(out, "sel.md")); !strings.Contains(md, "![x](<cap uno/fotos/a (1).png>)") {
		t.Errorf("markdown: falta ![x](<cap uno/fotos/a (1).png>) en\n%s", md)
	}
}

// Sin @include, el DOCX buscaba "fotos/a" para `![x](fotos/a (1).png)`
// (partía la ruta en el "(") mientras el HTML mostraba la imagen.
func TestBuild_ParenthesizedNameDOCX(t *testing.T) {
	docs := t.TempDir()
	img := solidPNG(t, color.RGBA{0, 0, 200, 255})
	writeFile(t, filepath.Join(docs, "fotos", "a (1).png"), img)
	doc := filepath.Join(docs, "d.md")
	writeFile(t, doc, []byte("---\ntitle: D\n---\n\n# D\n\nTexto ![x](fotos/a (1).png) fin.\n"))
	assertEmbeds(t, doc, docs, map[string][]byte{"fotos/a (1).png": img}, nil)
}

// Un título con escapes o referencias de caracteres no impide el rebase: el
// rebaser se rendía ante cualquier "\" o "&" del destino, título incluido, y
// con un a.png del mismo nombre junto al archivo raíz se embebía ese. Solo
// la ruta decide; el título queda tal cual.
func TestBuild_IncludedImages_TitleWithEscapes(t *testing.T) {
	docs := filepath.Join(t.TempDir(), "docs")
	want := map[string][]byte{
		"sub/a.png": solidPNG(t, color.RGBA{0, 0, 255, 255}),
		"sub/b.png": solidPNG(t, color.RGBA{0, 255, 0, 255}),
	}
	decoys := map[string][]byte{
		"a.png": solidPNG(t, color.RGBA{255, 0, 0, 255}),
		"b.png": solidPNG(t, color.RGBA{255, 255, 0, 255}),
	}
	for rel, data := range want {
		writeFile(t, filepath.Join(docs, filepath.FromSlash(rel)), data)
	}
	for rel, data := range decoys {
		writeFile(t, filepath.Join(docs, filepath.FromSlash(rel)), data)
	}
	writeFile(t, filepath.Join(docs, "sub", "s.md"), []byte("## S\n\nTexto ![a](a.png \"t\\\"x\") y ![b](b.png \"uno &amp; dos\") fin.\n"))
	manifest := filepath.Join(t.TempDir(), "sel.md")
	writeFile(t, manifest, []byte("---\ntitle: S\n---\n\n# S\n\n@include sub/s.md\n"))

	assertEmbeds(t, manifest, docs, want, decoys)

	out := t.TempDir()
	runBuild(t, manifest, "--format", "html", "--output", out, "--base-dir", docs)
	html := readFile(t, filepath.Join(out, "sel.html"))
	for _, w := range []string{`src="sub/a.png"`, `title="t&quot;x"`, `src="sub/b.png"`, `title="uno &amp; dos"`} {
		if !strings.Contains(html, w) {
			t.Errorf("html: falta %s en\n%s", w, grepImgs(html))
		}
	}
}
