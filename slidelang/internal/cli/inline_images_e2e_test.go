// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// Sin mode: flex, el normalizador de slidelang reescribe la ruta de toda
// imagen, de bloque o inline, a assets/images/<archivo> antes de compilar
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
