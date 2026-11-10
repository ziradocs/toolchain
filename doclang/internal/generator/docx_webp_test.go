// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Imágenes WebP de prueba, hechas con Pillow: una sin pérdida de 40×20 roja,
// una con pérdida de 30×30 azul con alfa 128 y una animada de dos cuadros de
// 8×8.
const (
	webpRed40x20    = "UklGRh4AAABXRUJQVlA4TBEAAAAvJ8AEAAdQjyLXo/+BiOh/AAA="
	webpAlpha30x30  = "UklGRmwAAABXRUJQVlA4WAoAAAAQAAAAHQAAHQAAQUxQSAoAAAABB1DAiAhERP8DVlA4IDwAAAAQAwCdASoeAB4APm00lkekIyIhKAgAgA2JZQB2AACAoKAA/vkd2//+QH/+QH/+QH/8gP/+IXeyAwAAAAA="
	webpAnimated8x8 = "UklGRsoAAABXRUJQVlA4WAoAAAACAAAABwAABwAAQU5JTQYAAAAAAAAAAABBTk1GSgAAAAAAAAAAAAcAAAcAAGQAAAJWUDggMgAAADABAJ0BKggACAABQCYloAADcAD+8ut///mwP/bz/wR6Af//0uD//pcH//S4P/SkAAAAQU5NRkwAAAAAAAAAAAAHAAAHAABkAAAAVlA4IDQAAAA0AQCdASoIAAgAAAAmJaAAA3AA/ukiH//3nz//ufP/+58/6M///yn7//I4//8jj/5QIAAA"
)

// Un WebP entra en el DOCX como PNG, de bloque y dentro de la prosa, con su
// tamaño natural, su transparencia y su texto alternativo; también si la
// extensión no dice WebP. Antes quedaba como "imagen no disponible". Uno
// animado, que no se puede decodificar, sigue quedando como no disponible.
func TestDOCX_WebPImagesAreInsertedAsPNG(t *testing.T) {
	root := t.TempDir()
	for name, b64 := range map[string]string{
		"roja.webp":      webpRed40x20,
		"alfa.webp":      webpAlpha30x30,
		"disfrazada.png": webpRed40x20,
		"animada.webp":   webpAnimated8x8,
	} {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "WebP"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "S"
	block.Elements = append(block.Elements,
		ast.NewImageElement(diagnostics.NewPosition(3, 1), "roja.webp", "Una imagen roja"),
		ast.NewImageElement(diagnostics.NewPosition(4, 1), "disfrazada.png", "Otra roja"),
		ast.NewTextElement(diagnostics.NewPosition(5, 1), "Texto con ![Azul translúcido](alfa.webp) adentro."),
		ast.NewImageElement(diagnostics.NewPosition(6, 1), "animada.webp", "Animada"),
	)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out := filepath.Join(t.TempDir(), "webp.docx")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: root, SourceDir: root}); err != nil {
		t.Fatal(err)
	}

	if got, want := docxDescrs(t, out, "word/document.xml"), []string{"Una imagen roja", "Otra roja", "Azul translúcido"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("image descriptions = %q, want %q", got, want)
	}
	xml := docxDocumentXML(t, out)
	if !strings.Contains(xml, "[Imagen no disponible: Animada]") {
		t.Error("the animated WebP is not left as unavailable")
	}
	if n := strings.Count(xml, "Imagen no disponible"); n != 1 {
		t.Errorf("%d images left as unavailable, want only the animated one", n)
	}
	// 40 px a 96 DPI son 381000 EMU de ancho.
	if !strings.Contains(xml, `cx="381000" cy="190500"`) {
		t.Error("the 40×20 WebP is not inserted at its natural size")
	}

	media := docxMediaFiles(t, out)
	var sizes []string
	alpha := false
	for name, data := range media {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Errorf("%s is not a PNG: %v", name, err)
			continue
		}
		b := img.Bounds()
		sizes = append(sizes, fmt.Sprintf("%dx%d", b.Dx(), b.Dy()))
		if _, _, _, a := img.At(0, 0).RGBA(); a < 0xffff && a > 0 {
			alpha = true
		}
	}
	sort.Strings(sizes)
	// La roja entra una vez por cada elemento que la usa, o una sola si
	// docxgo comparte el archivo: lo que importa es que estén las dos.
	if !strings.Contains(strings.Join(sizes, " "), "30x30") || !strings.Contains(strings.Join(sizes, " "), "40x20") {
		t.Errorf("media sizes = %v, want the 40x20 and the 30x30 PNGs", sizes)
	}
	if !alpha {
		t.Error("the translucent WebP lost its transparency")
	}
}
