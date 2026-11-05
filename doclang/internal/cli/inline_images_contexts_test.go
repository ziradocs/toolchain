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

// Una imagen inline local se incrusta en el DOCX y en el HTML offline-inline
// (la ruta del PDF) en cualquier texto: un párrafo, un ítem de lista (también
// anidado), una celda de tabla, una cita, un archivo incluido, con título y
// con una ruta con espacios entre <...>. Ya pasaba en v2.49.2; esta prueba lo
// fija.
func TestBuild_InlineImagesInEveryContext(t *testing.T) {
	if !coreReadsCommonMarkDestinations() {
		t.Skip("el core enlazado no lee los destinos como CommonMark")
	}
	dir := t.TempDir()
	images := map[string][]byte{}
	for i, rel := range []string{
		"img/par.png", "img/item.png", "img/sub.png", "img/cell.png", "img/quote.png",
		"img/title.png", "mi dir/angle.png", "mi dir/both.png", "cap/inc.png",
	} {
		n := uint8(i + 1)
		data := solidPNG(t, color.RGBA{n * 25, 255 - n*25, n * 9, 255})
		images[rel] = data
		writeFile(t, filepath.Join(dir, filepath.FromSlash(rel)), data)
	}
	writeFile(t, filepath.Join(dir, "cap", "parte.md"), []byte("Incluido ![inc](inc.png) fin.\n"))
	doc := filepath.Join(dir, "doc.md")
	writeFile(t, doc, []byte(`# Inline

Texto ![par](img/par.png) fin.

- Ítem ![item](img/item.png) fin
  - Sub ![sub](img/sub.png) fin

| A | B |
|---|---|
| ![cell](img/cell.png) | c |

> Cita ![quote](img/quote.png) fin.

Con título ![title](img/title.png "T") fin.

Con espacios ![angle](<mi dir/angle.png>) fin.

Con los dos ![both](<mi dir/both.png> "T") fin.

@include cap/parte.md
`))

	t.Run("docx", func(t *testing.T) {
		out := t.TempDir()
		runBuild(t, doc, "--format", "docx", "--output", out)
		media := docxMedia(t, filepath.Join(out, "doc.docx"))
		for rel, want := range images {
			if !containsBytes(media, want) {
				t.Errorf("%s no está embebida en el DOCX", rel)
			}
		}
	})

	t.Run("html offline-inline", func(t *testing.T) {
		out := t.TempDir()
		runBuild(t, doc, "--format", "html", "--render-mode", "offline-inline", "--output", out)
		html := readFile(t, filepath.Join(out, "doc.html"))
		for rel, img := range images {
			if !strings.Contains(html, base64.StdEncoding.EncodeToString(img)) {
				t.Errorf("%s no quedó incrustada", rel)
			}
		}
		if strings.Contains(html, `class="image-unavailable`) {
			t.Error("alguna imagen salió como no disponible")
		}
	})
}
