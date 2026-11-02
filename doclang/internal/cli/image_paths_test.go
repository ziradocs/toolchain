// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// imagePathsFixture es un .md de proyecto con tres imágenes relativas al
// archivo: al lado, en un subdirectorio y en un hermano del directorio. Lleva
// frontmatter para que el resultado no dependa del normalizador (con
// frontmatter y sin otras señales el Detector no lo enciende).
const imagePathsFixture = `---
title: Proyecto
---

# Introducción

![Logo](logo.png)

## Diagrama

![Diagrama](./img/a.png)

## Compartida

![Compartida](../shared/b.png)
`

// imageTree arma <tmp>/proj/README.md con sus tres imágenes y devuelve la
// raíz (<tmp>), el .md y los bytes de cada imagen por ruta relativa a la raíz.
func imageTree(t *testing.T, doc string) (root, docPath string, images map[string][]byte) {
	t.Helper()
	root = t.TempDir()
	images = map[string][]byte{
		"proj/logo.png":  solidPNG(t, color.RGBA{255, 0, 0, 255}),
		"proj/img/a.png": solidPNG(t, color.RGBA{0, 255, 0, 255}),
		"shared/b.png":   solidPNG(t, color.RGBA{0, 0, 255, 255}),
	}
	for rel, data := range images {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	docPath = filepath.Join(root, "proj", "README.md")
	if err := os.WriteFile(docPath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, docPath, images
}

func solidPNG(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func runBuild(t *testing.T, args ...string) {
	t.Helper()
	cmd := NewBuildCommand(nil, nil, nil, nil, nil)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
}

// docxMedia devuelve el contenido de cada archivo bajo word/media/.
func docxMedia(t *testing.T, path string) [][]byte {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("failed to open docx: %v", err)
	}
	defer func() { _ = r.Close() }()
	var media [][]byte
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, "word/media/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		media = append(media, data)
	}
	return media
}

func containsBytes(all [][]byte, want []byte) bool {
	for _, b := range all {
		if bytes.Equal(b, want) {
			return true
		}
	}
	return false
}

// TestBuild_DOCXResolvesImagesFromSourceWithAssetRoot: con --asset-root en el
// directorio padre, las tres imágenes se resuelven contra el .md y se
// embeben. Antes se resolvían contra --asset-root: logo.png e img/a.png se
// buscaban en el padre y ../shared/b.png quedaba fuera de la raíz.
func TestBuild_DOCXResolvesImagesFromSourceWithAssetRoot(t *testing.T) {
	root, docPath, images := imageTree(t, imagePathsFixture)
	out := t.TempDir()
	runBuild(t, docPath, "--format", "docx", "--output", out, "--asset-root", root)

	media := docxMedia(t, filepath.Join(out, "README.docx"))
	for rel, want := range images {
		if !containsBytes(media, want) {
			t.Errorf("%s no está embebida en word/media/ (%d archivos)", rel, len(media))
		}
	}
}

// TestBuild_DOCXDefaultAssetRootStillConfines: sin --asset-root la raíz es el
// directorio del .md, así que ../shared/b.png queda fuera y no se embebe; las
// otras dos sí.
func TestBuild_DOCXDefaultAssetRootStillConfines(t *testing.T) {
	_, docPath, images := imageTree(t, imagePathsFixture)
	out := t.TempDir()
	runBuild(t, docPath, "--format", "docx", "--output", out)

	media := docxMedia(t, filepath.Join(out, "README.docx"))
	for _, rel := range []string{"proj/logo.png", "proj/img/a.png"} {
		if !containsBytes(media, images[rel]) {
			t.Errorf("%s no está embebida en word/media/", rel)
		}
	}
	if containsBytes(media, images["shared/b.png"]) {
		t.Error("../shared/b.png se embebió aunque está fuera de la raíz por defecto (el directorio del .md)")
	}
}

// TestBuild_DOCXAssetRootRejectsEscape: una ruta que sale de --asset-root
// sigue rechazada aunque la raíz sea un ancestro del .md.
func TestBuild_DOCXAssetRootRejectsEscape(t *testing.T) {
	parent := t.TempDir()
	secret := []byte("THIS-IS-SECRET-CONTENT-NOT-A-REAL-PNG")
	if err := os.WriteFile(filepath.Join(parent, "secret.png"), secret, 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "root")
	docDir := filepath.Join(root, "proj")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatal(err)
	}
	docPath := filepath.Join(docDir, "README.md")
	doc := "---\ntitle: T\n---\n\n# S\n\n![x](../../secret.png)\n\n![y](" + filepath.Join(parent, "secret.png") + ")\n"
	if err := os.WriteFile(docPath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	runBuild(t, docPath, "--format", "docx", "--output", out, "--asset-root", root)

	for _, data := range docxMedia(t, filepath.Join(out, "README.docx")) {
		if bytes.Contains(data, []byte("SECRET")) {
			t.Fatal("un archivo fuera de --asset-root terminó en word/media/")
		}
	}
}

// TestBuild_HTMLKeepsAuthoredImagePaths: en HTML (modo browser) cada src
// queda tal como lo escribió el autor y, con la salida junto al .md, resuelve
// al archivo real.
func TestBuild_HTMLKeepsAuthoredImagePaths(t *testing.T) {
	root, docPath, _ := imageTree(t, imagePathsFixture)
	out := filepath.Dir(docPath)
	runBuild(t, docPath, "--format", "html", "--output", out, "--asset-root", root)

	data, err := os.ReadFile(filepath.Join(out, "README.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, src := range []string{"logo.png", "./img/a.png", "../shared/b.png"} {
		if !strings.Contains(html, `src="`+src+`"`) {
			t.Errorf("el HTML no conserva src=%q", src)
			continue
		}
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(src))); err != nil {
			t.Errorf("src=%q no resuelve desde el HTML: %v", src, err)
		}
	}
}
