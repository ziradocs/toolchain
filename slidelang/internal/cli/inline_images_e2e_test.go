// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// buildOfflineInline compila source con --render-mode offline-inline desde un
// directorio con a.png y devuelve el HTML.
func buildOfflineInline(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	if err := os.WriteFile(filepath.Join(dir, "a.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "deck.slidelang")
	if err := os.WriteFile(input, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "dist")
	opts := &BuildOptions{InputFile: input, OutputDir: out, Format: "html", Mode: "auto", LogLevel: "error", NoColors: true, RenderMode: "offline-inline"}
	if err := runBuild(opts, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("runBuild: %v", err)
	}
	html, err := os.ReadFile(filepath.Join(out, "deck.html"))
	if err != nil {
		t.Fatal(err)
	}
	return string(html)
}

// Por el CLI, una imagen inline de un párrafo, un ítem y una celda se
// incrusta en offline-inline (la ruta del PDF), igual que la de bloque.
func TestRunBuild_InlineImagesOfflineInline(t *testing.T) {
	html := buildOfflineInline(t, `---
title: Imágenes
mode: flex
---

# Uno

Texto ![x](a.png) fin.

- Ítem ![y](a.png "t")

| A | B |
|---|---|
| ![z](a.png) | c |
`)
	uri := "data:image/png;base64," + onePixelPNG
	for _, want := range []string{
		`<img src="` + uri + `" alt="x">`,
		`<img src="` + uri + `" alt="y" title="t">`,
		`<img src="` + uri + `" alt="z">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("falta %s", want[:40]+"…")
		}
	}
	if strings.Contains(html, `src="a.png"`) {
		t.Error("quedó un src relativo")
	}
}

// Sin mode: flex, el normalizador de slidelang reescribe la ruta relativa de
// toda imagen, de bloque o inline, a assets/images/<archivo> antes de compilar
// (enhancement.ImagesRule). La imagen inline sigue a la de bloque: las dos
// buscan assets/images/a.png. Esta prueba fija esa limitación conocida; si
// la regla deja de reescribir, las dos se incrustan.
func TestRunBuild_InlineImagesFollowBlockPathWithoutFlex(t *testing.T) {
	html := buildOfflineInline(t, `---
title: Imágenes
---

# Uno

Texto ![x](a.png) fin.

![b](a.png)
`)
	if !strings.Contains(html, `src="assets/images/a.png" alt="x"`) {
		t.Error("la imagen inline no lleva la ruta que reescribió el normalizador")
	}
	if strings.Contains(html, "data:image/png") {
		t.Error("sin flex ninguna de las dos debería encontrarse en assets/images/")
	}
}

// buildJSONImageSources compila source con --format json y devuelve la fuente
// de cada imagen de bloque, en orden.
func buildJSONImageSources(t *testing.T, source string) []string {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "deck.slidelang")
	if err := os.WriteFile(input, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "dist")
	opts := &BuildOptions{InputFile: input, OutputDir: out, Format: "json", Mode: "auto", LogLevel: "error", NoColors: true}
	if err := runBuild(opts, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("runBuild: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "deck.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		ContentBlocks []struct {
			Elements []struct {
				Type   string `json:"type"`
				Source string `json:"source"`
			} `json:"elements"`
		} `json:"contentBlocks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var sources []string
	for _, b := range doc.ContentBlocks {
		for _, el := range b.Elements {
			if el.Type == "image" {
				sources = append(sources, el.Source)
			}
		}
	}
	return sources
}

// coreKeepsImageURLs dice si el core con el que se compila deja una URL de
// imagen como está cuando el normalizador se enciende. Contra un core
// publicado que todavía la lleva a assets/images/, la prueba de abajo se
// salta.
func coreKeepsImageURLs() bool {
	doc, _ := parser.New(util.NewNoop()).Parse("---\ntitle: P\nmode: flex\n---\n\n# P\n\n![a](https://x.test/a.png)\n\n![b](ruta/imagen.png)\n", "p.slidelang")
	if doc == nil {
		return false
	}
	for _, b := range doc.ContentBlocks {
		for _, el := range b.Elements {
			if img, ok := el.(*ast.ImageElement); ok && img.Source == "https://x.test/a.png" {
				return true
			}
		}
	}
	return false
}

// El normalizador no reescribe una URL, una ruta relativa al protocolo ni una
// ruta desde la raíz a assets/images/. Antes, una foto de
// images.unsplash.com encendía el Detector ("image" en la ruta) y ImagesRule
// llevaba toda URL del deck a assets/images/<archivo>. Con una ruta relativa
// placeholder el normalizador sí se enciende, y solo esa se reescribe.
func TestRunBuild_NormalizerKeepsExternalImageURLs(t *testing.T) {
	if !coreKeepsImageURLs() {
		t.Skip("this core rewrites image URLs to assets/images/")
	}
	const header = "---\ntitle: Imágenes\nmode: flex\n---\n\n# Uno\n\n"
	unsplash := "https://images.unsplash.com/photo-1.jpg"
	wikimedia := "https://upload.wikimedia.org/wikipedia/commons/4/47/PNG_transparency_demonstration_1.png"
	cases := []struct {
		name, body string
		want       []string
	}{
		{
			name: "solo URLs",
			body: "![Uno](" + unsplash + ")\n\n![Dos](" + wikimedia + ")\n",
			want: []string{unsplash, wikimedia},
		},
		{
			name: "con placeholder relativo",
			body: "![Uno](" + unsplash + ")\n\n![Dos](" + wikimedia + ")\n\n![Tres](//cdn.example.com/x.png)\n\n" +
				"![Cuatro](/api/assets/a/b.png)\n\n![Cinco](ruta/imagen.png)\n",
			want: []string{unsplash, wikimedia, "//cdn.example.com/x.png", "/api/assets/a/b.png", "assets/images/imagen.png"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildJSONImageSources(t, header+tc.body)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
