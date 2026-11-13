// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// webpAnimatedOffset es un WebP animado de 16×12 y dos cuadros, hecho con
// Pillow 12.1.1 y libwebp 1.5.0: el primero es un cuadrado rojo de 6×6 en
// (6,4) sobre transparente, el segundo azul opaco. Es webpAnimOffset16x12 de
// internal/generator, donde está el programa que lo genera.
const webpAnimatedOffset = "UklGRsQAAABXRUJQVlA4WAoAAAACAAAADwAACwAAQU5JTQYAAAAAAAAAAABBTk1GSgAAAAMAAAIAAAUAAAUAAGQAAAJWUDggMgAAADABAJ0BKgYABgABQCYloAADcAD+8ut///mwP/bz/wR6Af//0uD//pcH//S4P/SkAAAAQU5NRkYAAAAAAAAAAAAPAAALAABkAAAAVlA4IC4AAAA0AQCdASoQAAwAAAAmJaAAA3AA/vtV4///S4P/+lwf/9Lg/9Lg//rV5Vesq6AA"

// TestE2E_AnimatedWebPInDOCX: `doclang build --format docx
// --fail-on-unavailable` con una WebP animada termina bien y el DOCX trae su
// primer cuadro en word/media, como PNG o JPEG y no como WebP. Antes el build
// fallaba: x/image/webp no decodifica animaciones.
func TestE2E_AnimatedWebPInDOCX(t *testing.T) {
	dir := t.TempDir()
	data, err := base64.StdEncoding.DecodeString(webpAnimatedOffset)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "animada.webp"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "doc.doclang")
	if err := os.WriteFile(src, []byte("---\ntitle: W\nmode: flex\n---\n\n# W\n\n![Animada](animada.webp)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := runCLI(t, Options{}, []string{"build", src, "--format", "docx", "--output", out, "--fail-on-unavailable"}); err != nil {
		t.Fatalf("build with an animated WebP and --fail-on-unavailable failed: %v", err)
	}
	zr, err := zip.OpenReader(filepath.Join(out, "doc.docx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	images := 0
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "word/media/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case bytes.HasPrefix(body, []byte("RIFF")):
			t.Errorf("%s is a raw WebP", f.Name)
		case bytes.HasPrefix(body, []byte("\x89PNG")), bytes.HasPrefix(body, []byte{0xFF, 0xD8}):
			images++
		}
	}
	if images != 1 {
		t.Errorf("%d PNG or JPEG images in word/media, want 1", images)
	}
}
