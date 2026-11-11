// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Un archivo de más de 2 GiB con el tope en math.MaxInt32 (que es math.MaxInt
// con int de 32 bits) se rechaza por tamaño. Antes, el tamaño pasaba a int
// para CheckInputSize: con 32 bits int(2^31) da la vuelta a negativo,
// CheckInputSize devolvía nil y resolveInput regresaba ("", nil, nil), así que
// el export seguía con el contenido vacío. Con 64 bits no hay vuelta, así que
// la prueba solo falla sin el arreglo con GOARCH=386 (CI la corre así). El
// archivo es disperso: no ocupa disco.
func TestResolveInputRejectsFileOver2GiB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "grande.md")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(3 << 30); err != nil {
		_ = f.Close()
		t.Skipf("no se pudo crear un archivo disperso de 3 GiB: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	root, err := newExportRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	abs, content, err := root.resolveInput("grande.md", math.MaxInt32)
	if err == nil {
		t.Fatalf("resolveInput = (%q, %d bytes, nil), want a too-large error", abs, len(content))
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("resolveInput error = %v, want a too-large error", err)
	}
}
