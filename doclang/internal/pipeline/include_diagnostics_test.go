// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

// Active y Waived ordenan como core/report: primero el documento, después
// cada archivo incluido, y dentro de cada uno por línea y columna.
func TestSortByPositionOrdersByFileFirst(t *testing.T) {
	at := func(file string, line, col int) diagnostics.Diagnostic {
		d := diagnostics.NewWarning("m", diagnostics.NewPosition(line, col), "t")
		d.File = file
		return d
	}
	in := []diagnostics.Diagnostic{at("partes/x.doclang", 5, 1), at("", 9, 1), at("partes/a.doclang", 7, 1), at("", 2, 3), at("", 2, 1)}
	want := []diagnostics.Diagnostic{in[4], in[3], in[1], in[2], in[0]}
	got := PolicyResult{LintActive: in}.Active()
	waivedIn := make([]linter.WaivedDiagnostic, len(in))
	for i, d := range in {
		waivedIn[i] = linter.WaivedDiagnostic{Diagnostic: d}
	}
	waived := PolicyResult{LintWaived: waivedIn}.Waived()
	for i := range want {
		if got[i].File != want[i].File || got[i].Position != want[i].Position {
			t.Errorf("Active()[%d] = %s %v, want %s %v", i, got[i].File, got[i].Position, want[i].File, want[i].Position)
		}
		if w := waived[i].Diagnostic; w.File != want[i].File || w.Position != want[i].Position {
			t.Errorf("Waived()[%d] = %s %v, want %s %v", i, w.File, w.Position, want[i].File, want[i].Position)
		}
	}
}

func TestWhere(t *testing.T) {
	d := diagnostics.NewWarning("m", diagnostics.NewPosition(4, 1), "t")
	if got := Where(d); got != "line 4" {
		t.Errorf("Where(document) = %q", got)
	}
	d.File = "partes/x.doclang"
	if got := Where(d); got != "partes/x.doclang:4" {
		t.Errorf("Where(included) = %q", got)
	}
}

// DocumentDir da el directorio del documento en la forma de la raíz de
// includes: con symlinks resueltos si la raíz los tiene resueltos, y tal
// cual (filepath.Abs) si no.
func TestDocumentDir(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(base, "docs")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "enlace")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	input := filepath.Join(link, "main.doclang")
	for _, c := range []struct {
		name, includeRoot, want string
	}{
		{"root through the symlink", link, link},
		{"resolved root", real, real},
		{"no root", "", link},
	} {
		if got := DocumentDir(input, c.includeRoot); got != c.want {
			t.Errorf("%s: DocumentDir = %q, want %q", c.name, got, c.want)
		}
	}
	if got := DocumentDir("", real); got != "" {
		t.Errorf("DocumentDir without a file = %q, want empty", got)
	}
}
