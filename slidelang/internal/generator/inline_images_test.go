// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/base64"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// onePixelPNG es un PNG de 1x1.
const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// Una imagen inline (en un párrafo, un ítem o una celda) se incrusta en
// offline-inline como la de bloque. Antes quedaba el src relativo y el PDF,
// que se imprime desde about:blank, mostraba el ícono de imagen rota.
func TestRenderHTMLPreview_InlineImagesAreInlinedOffline(t *testing.T) {
	dir := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if err := os.WriteFile(filepath.Join(dir, "a.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	pos := diagnostics.NewPosition(1, 1)
	text := ast.NewTextElement(pos, "Texto ![x](a.png) y ![falta](no.png) fin.")
	points := ast.NewPointsElement(pos)
	points.Items = []ast.PointItem{{Content: `Ítem ![y](a.png "t")`}}
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Imágenes"
	block.Elements = []ast.Element{text, points}
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

	render := func(mode string) string {
		ctx := renderer.NewDefaultRenderContext()
		ctx.ImageMode, ctx.AssetRoot = mode, dir
		html, err := New(util.NewNoop()).RenderHTMLPreview(doc, GeneratorOptions{}, ctx)
		if err != nil {
			t.Fatalf("RenderHTMLPreview: %v", err)
		}
		return html
	}

	html := render("offline-inline")
	uri := "data:image/png;base64," + onePixelPNG
	if !strings.Contains(html, `<img src="`+uri+`" alt="x">`) {
		t.Errorf("la imagen inline del párrafo no se incrustó:\n%s", html)
	}
	if !strings.Contains(html, `<img src="`+uri+`" alt="y" title="t">`) {
		t.Errorf("la imagen inline del ítem no se incrustó con su título:\n%s", html)
	}
	if !strings.Contains(html, `<img src="no.png" alt="falta">`) {
		t.Errorf("una imagen que falta debe quedar como estaba:\n%s", html)
	}

	if html := render("browser"); !strings.Contains(html, `<img src="a.png" alt="x">`) {
		t.Errorf("fuera de offline-inline el src relativo no cambia:\n%s", html)
	}
}

// Una ruta que sale de AssetRoot no se lee.
func TestInlineLocalInlineImages_Confined(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "afuera.png")
	png, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if err := os.WriteFile(outside, png, 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(outside) }()
	ctx := renderer.NewDefaultRenderContext()
	ctx.ImageMode, ctx.AssetRoot = "offline-inline", root
	tag := `<img src="../afuera.png" alt="a">`
	if got := inlineLocalInlineImages(tag, ctx); got != tag {
		t.Errorf("una ruta fuera de la raíz se incrustó: %s", got)
	}
}

// slideInlineImageTagOracle es el regex con el que se resolvía antes.
var slideInlineImageTagOracle = regexp.MustCompile(`<img src="([^"]*)" alt="([^"]*)"((?: title="[^"]*")?)>`)

// El recorrido sin regex reconoce los mismos tags que el regex anterior.
func TestInlineLocalInlineImages_MatchesRegex(t *testing.T) {
	dir := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if err := os.WriteFile(filepath.Join(dir, "a.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := renderer.NewDefaultRenderContext()
	ctx.ImageMode, ctx.AssetRoot = "offline-inline", dir
	oracle := func(html string) string {
		return slideInlineImageTagOracle.ReplaceAllStringFunc(html, func(tag string) string {
			m := slideInlineImageTagOracle.FindStringSubmatch(tag)
			uri, ok := renderer.TryInlineLocalImage(renderer.UnescapeHTML(m[1]), ctx)
			if !ok {
				return tag
			}
			return `<img src="` + uri + `" alt="` + m[2] + `"` + m[3] + `>`
		})
	}
	parts := []string{`<img src="`, `a.png`, `no.png`, `data:image/png;base64,AA`, `"`, ` alt="`, `x`, ` title="`, `>`, ` `, `<img `, `<p>`}
	r := rand.New(rand.NewSource(10))
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for n := 1 + r.Intn(16); n > 0; n-- {
			b.WriteString(parts[r.Intn(len(parts))])
		}
		s := b.String()
		if got, want := inlineLocalInlineImages(s, ctx), oracle(s); got != want {
			t.Fatalf("%q:\n %q\n regex %q", s, got, want)
		}
	}
}

// Las imágenes de bloque ya incrustadas (data: URI) no se resuelven, y una
// misma imagen inline repetida se lee del disco una sola vez. Se cuentan las
// lecturas en vez de medir tiempo: con data: URI de megas, el tiempo depende
// sobre todo de la caché de memoria de la máquina.
func TestInlineLocalInlineImages_ReadsEachSourceOnce(t *testing.T) {
	dir := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	for _, name := range []string{"a.png", "b.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), png, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := renderer.NewDefaultRenderContext()
	ctx.ImageMode, ctx.AssetRoot = "offline-inline", dir
	reads := map[string]int{}
	orig := tryInlineLocalImage
	tryInlineLocalImage = func(src string, ctx *renderer.RenderContext) (string, bool) {
		reads[src]++
		return orig(src, ctx)
	}
	defer func() { tryInlineLocalImage = orig }()

	big := `<img src="data:image/png;base64,` + strings.Repeat("A", 1<<16) + `" alt="b">`
	html := strings.Repeat(big+`<p>Texto <img src="a.png" alt="x"> y <img src="b.png" alt="y" title="t"> fin</p>`, 200)
	got := inlineLocalInlineImages(html, ctx)
	if want := map[string]int{"a.png": 1, "b.png": 1}; !reflect.DeepEqual(reads, want) {
		t.Errorf("lecturas = %v, quiero %v (los data: no se resuelven)", reads, want)
	}
	if strings.Contains(got, `src="a.png"`) || strings.Contains(got, `src="b.png"`) {
		t.Error("quedó una imagen inline sin incrustar")
	}
	if n := strings.Count(got, big); n != 200 {
		t.Errorf("las imágenes de bloque cambiaron: %d de 200 intactas", n)
	}
}
