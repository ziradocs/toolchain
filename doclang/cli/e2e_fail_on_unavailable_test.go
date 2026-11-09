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
