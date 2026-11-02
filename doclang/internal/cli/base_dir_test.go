// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
)

// baseDirFixture es el texto de docs/guide.md copiado a un temporal (el
// buffer sin guardar de un editor): sus imágenes son relativas a docs/, no
// al temporal.
const baseDirFixture = `---
title: Guía
---

# Guía

![Logo](logo.png)

## Diagrama

![Diagrama](./img/a.png)
`

// baseDirTree arma <root>/docs con las dos imágenes de baseDirFixture y
// escribe el texto en un archivo de OTRO directorio temporal. Devuelve la
// raíz, docs/, el temporal y los bytes de cada imagen.
func baseDirTree(t *testing.T, doc string) (root, docs, tmpInput string, images map[string][]byte) {
	t.Helper()
	root = t.TempDir()
	docs = filepath.Join(root, "docs")
	images = map[string][]byte{
		"logo.png":  solidPNG(t, color.RGBA{255, 0, 0, 255}),
		"img/a.png": solidPNG(t, color.RGBA{0, 255, 0, 255}),
	}
	for rel, data := range images {
		writeFile(t, filepath.Join(docs, filepath.FromSlash(rel)), data)
	}
	tmpInput = filepath.Join(t.TempDir(), "guide-unsaved.md")
	writeFile(t, tmpInput, []byte(doc))
	return root, docs, tmpInput, images
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func buildErr(args ...string) error {
	cmd := NewBuildCommand(nil, nil, nil, nil, nil)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.Execute()
}

// TestBuild_BaseDirDOCX: un temporal con --base-dir embebe las imágenes de
// docs/. Sin el flag se pierden, que es la regresión que motivó el flag:
// desde #451 las fuentes se resuelven junto al archivo de entrada.
func TestBuild_BaseDirDOCX(t *testing.T) {
	_, docs, tmpInput, images := baseDirTree(t, baseDirFixture)

	out := t.TempDir()
	runBuild(t, tmpInput, "--format", "docx", "--output", out, "--base-dir", docs)
	media := docxMedia(t, filepath.Join(out, "guide-unsaved.docx"))
	for rel, want := range images {
		if !containsBytes(media, want) {
			t.Errorf("%s no está embebida con --base-dir (%d archivos en word/media/)", rel, len(media))
		}
	}

	out = t.TempDir()
	runBuild(t, tmpInput, "--format", "docx", "--output", out)
	media = docxMedia(t, filepath.Join(out, "guide-unsaved.docx"))
	for rel, want := range images {
		if containsBytes(media, want) {
			t.Errorf("%s se embebió sin --base-dir: la prueba no distingue el flag", rel)
		}
	}
}

// TestBuild_BaseDirHTMLOfflineInline: en HTML las imágenes locales solo se
// leen en offline-inline (en browser el src queda tal cual); ahí tienen que
// salir como data: URI leídas desde --base-dir.
func TestBuild_BaseDirHTMLOfflineInline(t *testing.T) {
	_, docs, tmpInput, images := baseDirTree(t, baseDirFixture)
	out := t.TempDir()
	runBuild(t, tmpInput, "--format", "html", "--render-mode", "offline-inline", "--output", out, "--base-dir", docs)

	data, err := os.ReadFile(filepath.Join(out, "guide-unsaved.html"))
	if err != nil {
		t.Fatal(err)
	}
	for rel, img := range images {
		if !bytes.Contains(data, []byte(base64.StdEncoding.EncodeToString(img))) {
			t.Errorf("%s no quedó inlineada en el HTML", rel)
		}
	}
}

// TestBuild_BaseDirPDF: el PDF pasa por el mismo HTML forzado a
// offline-inline, que lee cada imagen local y avisa con "[IMAGE]" cuando no
// puede. Chromium recomprime las imágenes, así que no hay bytes que buscar
// en el PDF: se mira el log. Sin --base-dir avisa por las dos; con él, por
// ninguna.
func TestBuild_BaseDirPDF(t *testing.T) {
	if _, err := chromium.NewChromiumManager("", false, &renderer.ChromiumLoggerAdapter{Logger: util.NewNoop()}).Detect(); err != nil {
		t.Skip("Chromium not available")
	}
	_, docs, tmpInput, _ := baseDirTree(t, baseDirFixture)

	withFlag := t.TempDir()
	logWith := captureStderr(t, func() {
		runBuild(t, tmpInput, "--format", "pdf", "--output", withFlag, "--base-dir", docs)
	})
	assertPDF(t, filepath.Join(withFlag, "guide-unsaved.pdf"))
	if strings.Contains(logWith, "[IMAGE]") {
		t.Errorf("con --base-dir hubo avisos de imagen:\n%s", logWith)
	}

	without := t.TempDir()
	logWithout := captureStderr(t, func() {
		runBuild(t, tmpInput, "--format", "pdf", "--output", without)
	})
	if n := strings.Count(logWithout, "[IMAGE] failed to read local image"); n != 2 {
		t.Errorf("sin --base-dir se esperaban 2 avisos de imagen y hubo %d: la prueba no distingue el flag", n)
	}
}

func assertPDF(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("%s no es un PDF", path)
	}
}

// captureStderr devuelve lo que fn escribe en os.Stderr, que es donde
// escribe el logger de consola de build.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	defer func() {
		os.Stderr = orig
	}()
	fn()
	os.Stderr = orig
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// TestBuild_BaseDirStillConfines: con --base-dir, una fuente que sale de
// la raíz (por defecto, el propio --base-dir) con ".." o con una ruta
// absoluta sigue rechazada.
func TestBuild_BaseDirStillConfines(t *testing.T) {
	root, docs, _, _ := baseDirTree(t, baseDirFixture)
	secretPath := filepath.Join(root, "secret.png")
	writeFile(t, secretPath, []byte("THIS-IS-SECRET-CONTENT-NOT-A-REAL-PNG"))
	doc := "---\ntitle: T\n---\n\n# S\n\n![x](../secret.png)\n\n![y](" + secretPath + ")\n"
	tmpInput := filepath.Join(t.TempDir(), "escape.md")
	writeFile(t, tmpInput, []byte(doc))

	for _, extra := range [][]string{nil, {"--asset-root", docs}} {
		out := t.TempDir()
		args := append([]string{tmpInput, "--format", "docx", "--output", out, "--base-dir", docs}, extra...)
		runBuild(t, args...)
		for _, data := range docxMedia(t, filepath.Join(out, "escape.docx")) {
			if bytes.Contains(data, []byte("SECRET")) {
				t.Fatalf("un archivo fuera de la raíz terminó en word/media/ (args extra %v)", extra)
			}
		}
	}

	// Con --asset-root en la raíz, ../secret.png sí queda dentro.
	out := t.TempDir()
	runBuild(t, tmpInput, "--format", "docx", "--output", out, "--base-dir", docs, "--asset-root", root)
	found := false
	for _, data := range docxMedia(t, filepath.Join(out, "escape.docx")) {
		if bytes.Contains(data, []byte("SECRET")) {
			found = true
		}
	}
	if !found {
		// No es una imagen válida: el generador la deja como marcador. Lo
		// que importa es que no se rechace por confinamiento, y eso se ve
		// en que el marcador no diga "blocked".
		docXML := docxDocumentXML(t, filepath.Join(out, "escape.docx"))
		if strings.Contains(docXML, "[Image blocked: ../secret.png]") {
			t.Error("../secret.png se rechazó aunque --asset-root la contiene")
		}
	}
}

// TestBuild_BaseDirValidation: --base-dir tiene que existir, ser un
// directorio y quedar dentro de --asset-root. Se valida antes de parsear,
// así que también falla con --lint-only.
func TestBuild_BaseDirValidation(t *testing.T) {
	root, docs, tmpInput, _ := baseDirTree(t, baseDirFixture)
	other := t.TempDir()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no existe", []string{"--base-dir", filepath.Join(root, "nope")}, "does not exist"},
		{"es un archivo", []string{"--base-dir", filepath.Join(docs, "logo.png")}, "is not a directory"},
		{"fuera de --asset-root", []string{"--base-dir", other, "--asset-root", docs}, "outside the asset root"},
		{"lint-only", []string{"--lint-only", "--base-dir", filepath.Join(root, "nope")}, "does not exist"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{tmpInput, "--format", "docx", "--output", t.TempDir()}, tc.args...)
			err := buildErr(args...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, quería uno con %q", err, tc.want)
			}
		})
	}

	// Dentro de --asset-root (un ancestro) es válido.
	runBuild(t, tmpInput, "--format", "docx", "--output", t.TempDir(), "--base-dir", docs, "--asset-root", root)
}

// TestBuild_BaseDirSymlinkedInsideRoot: un --base-dir que llega a la raíz
// por un symlink se acepta y sus imágenes se leen: el chequeo compara rutas
// con los symlinks resueltos.
func TestBuild_BaseDirSymlinkedInsideRoot(t *testing.T) {
	root, docs, tmpInput, images := baseDirTree(t, baseDirFixture)
	link := filepath.Join(t.TempDir(), "docs-link")
	if err := os.Symlink(docs, link); err != nil {
		t.Skipf("symlinks no disponibles: %v", err)
	}
	out := t.TempDir()
	runBuild(t, tmpInput, "--format", "docx", "--output", out, "--base-dir", link, "--asset-root", root)
	media := docxMedia(t, filepath.Join(out, "guide-unsaved.docx"))
	for rel, want := range images {
		if !containsBytes(media, want) {
			t.Errorf("%s no está embebida con un --base-dir symlink", rel)
		}
	}
}

// TestBuild_BaseDirIncludeFromTempManifest: un manifiesto temporal hace
// @include de archivos reales de docs/. Con --base-dir y sin
// --include-root, los @include se resuelven desde docs/; un
// --include-root explícito sigue mandando.
func TestBuild_BaseDirIncludeFromTempManifest(t *testing.T) {
	root, docs, _, images := baseDirTree(t, baseDirFixture)
	writeFile(t, filepath.Join(docs, "guide.md"), []byte(baseDirFixture))
	writeFile(t, filepath.Join(docs, "intro.md"), []byte("---\ntitle: Intro\n---\n\n# Intro\n\nTexto de la INTRODUCCION.\n"))
	manifest := filepath.Join(t.TempDir(), "seleccion.md")
	writeFile(t, manifest, []byte("---\ntitle: Selección\n---\n\n@include intro.md\n\n@include guide.md\n"))

	out := t.TempDir()
	runBuild(t, manifest, "--format", "docx", "--output", out, "--base-dir", docs)
	if !strings.Contains(docxDocumentXML(t, filepath.Join(out, "seleccion.docx")), "INTRODUCCION") {
		t.Error("el @include no se resolvió desde --base-dir")
	}
	media := docxMedia(t, filepath.Join(out, "seleccion.docx"))
	for rel, want := range images {
		if !containsBytes(media, want) {
			t.Errorf("%s, imagen de un archivo incluido de docs/, no está embebida", rel)
		}
	}

	// Sin --base-dir el @include se busca junto al manifiesto y no existe.
	if err := buildErr(manifest, "--format", "docx", "--output", t.TempDir()); err == nil || !strings.Contains(err.Error(), "include") {
		t.Errorf("sin --base-dir el @include debería fallar, error = %v", err)
	}

	// --include-root explícito: se resuelve desde ahí, no desde --base-dir.
	if err := buildErr(manifest, "--format", "docx", "--output", t.TempDir(), "--base-dir", docs, "--include-root", root); err == nil || !strings.Contains(err.Error(), "include") {
		t.Errorf("con --include-root en la raíz, intro.md no existe ahí y debería fallar, error = %v", err)
	}
}

func docxDocumentXML(t *testing.T, path string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("failed to open docx: %v", err)
	}
	defer func() { _ = r.Close() }()
	for _, f := range r.File {
		if f.Name != "word/document.xml" {
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
		return string(data)
	}
	t.Fatalf("%s no tiene word/document.xml", path)
	return ""
}
