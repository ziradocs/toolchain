// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestE2E_FailOnUnavailable: `doclang build --fail-on-unavailable` sale con
// error, lista la línea y la fuente de la imagen que falta y no deja salida,
// ni siquiera la de un build anterior con el mismo nombre. Sin la bandera el
// mismo build termina bien.
func TestE2E_FailOnUnavailable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.doclang")
	if err := os.WriteFile(src, []byte("---\ntitle: F\nmode: flex\n---\n\n# F\n\n![falta](no-existe.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := runCLI(t, Options{}, []string{"build", src, "--format", "docx", "--output", out}); err != nil {
		t.Fatalf("default build failed: %v", err)
	}
	docx := filepath.Join(out, "doc.docx")
	if _, err := os.Stat(docx); err != nil {
		t.Fatalf("default build left no output: %v", err)
	}

	err := runCLI(t, Options{}, []string{"build", src, "--format", "docx", "--output", out, "--fail-on-unavailable"})
	if err == nil {
		t.Fatal("build with --fail-on-unavailable succeeded")
	}
	if msg := err.Error(); !strings.Contains(msg, `line 8: image "no-existe.png"`) {
		t.Errorf("error does not name the line and source:\n%s", msg)
	}
	if _, err := os.Stat(docx); !os.IsNotExist(err) {
		t.Errorf("the previous doc.docx survived the failed build (stat err %v)", err)
	}
}

// TestE2E_FailOnUnavailable_KeepsNonRegularOutput: si en la ruta de salida
// hay un enlace simbólico (o algo que no es un archivo regular), el build
// fallido no lo borra.
func TestE2E_FailOnUnavailable_KeepsNonRegularOutput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.doclang")
	if err := os.WriteFile(src, []byte("---\ntitle: F\nmode: flex\n---\n\n# F\n\n![falta](no-existe.png)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "importante.txt")
	if err := os.WriteFile(target, []byte("no borrar"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(out, "doc.docx")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if err := runCLI(t, Options{}, []string{"build", src, "--format", "docx", "--output", out, "--fail-on-unavailable"}); err == nil {
		t.Fatal("build with --fail-on-unavailable succeeded")
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("the symlink at the output path was removed: %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "no borrar" {
		t.Errorf("the symlink target changed: %q, %v", data, err)
	}
}

// TestE2E_FailOnUnavailable_RejectsMarkdown: con --format markdown la
// bandera es un error, no se ignora.
func TestE2E_FailOnUnavailable_RejectsMarkdown(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.doclang")
	if err := os.WriteFile(src, []byte("---\ntitle: F\nmode: flex\n---\n\n# F\n\nTexto.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCLI(t, Options{}, []string{"build", src, "--format", "markdown", "--output", filepath.Join(dir, "out"), "--fail-on-unavailable"})
	if err == nil || !strings.Contains(err.Error(), "markdown") {
		t.Fatalf("err = %v, want a markdown rejection", err)
	}
}
