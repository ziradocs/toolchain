// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package findings

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

// Las mismas pruebas que doclang/internal/pipeline/include_diagnostics_test.go
// para el mismo código.

// Active y Waived ordenan como core/report: primero el documento, después
// cada archivo incluido, y dentro de cada uno por línea y columna.
func TestSortByPositionOrdersByFileFirst(t *testing.T) {
	at := func(file string, line, col int) diagnostics.Diagnostic {
		d := diagnostics.NewWarning("m", diagnostics.NewPosition(line, col), "t")
		d.File = file
		return d
	}
	in := []diagnostics.Diagnostic{at("parts/x.slidelang", 5, 1), at("", 9, 1), at("parts/a.slidelang", 7, 1), at("", 2, 3), at("", 2, 1)}
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

// DocumentDir da el directorio del documento sin resolver symlinks, en la
// forma de la ruta de entrada.
func TestDocumentDir(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(base, "decks")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "enlace")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	if got := DocumentDir(filepath.Join(link, "main.slidelang")); got != link {
		t.Errorf("DocumentDir = %q, want %q", got, link)
	}
	if got := DocumentDir(""); got != "" {
		t.Errorf("DocumentDir without a file = %q, want empty", got)
	}
}

func TestDedupeIncluded(t *testing.T) {
	at := func(file string, line int, msg string) diagnostics.Diagnostic {
		d := diagnostics.NewWarning(msg, diagnostics.NewPosition(line, 1), "linter")
		d.File = file
		return d
	}
	// raw: la línea en el contenido expandido; mapped: la del archivo.
	// b.slidelang está anidado en a.slidelang, que se incluyó dos veces
	// (expansiones en las líneas 5 y 12 de b.slidelang:2).
	raw := []diagnostics.Diagnostic{at("", 5, "x"), at("", 5, "x"), at("", 3, "doc"), at("", 3, "doc"), at("", 12, "x"), at("", 12, "x"), at("", 12, "x"), at("", 9, "y")}
	mapped := []diagnostics.Diagnostic{at("b.slidelang", 2, "x"), at("b.slidelang", 2, "x"), at("", 3, "doc"), at("", 3, "doc"), at("b.slidelang", 2, "x"), at("b.slidelang", 2, "x"), at("b.slidelang", 2, "x"), at("a.slidelang", 4, "y")}
	got := DedupeIncluded(raw, mapped)
	var summary []string
	for _, d := range got {
		summary = append(summary, d.File+":"+strconv.Itoa(d.Position.Line)+" "+d.Message)
	}
	want := "b.slidelang:2 x|b.slidelang:2 x|:3 doc|:3 doc|b.slidelang:2 x|a.slidelang:4 y"
	if strings.Join(summary, "|") != want {
		t.Errorf("DedupeIncluded = %s, want %s", strings.Join(summary, "|"), want)
	}
}

// Si raw y mapped no están alineados índice por índice, DedupeIncluded no
// puede saber de qué expansión es cada hallazgo y devuelve mapped tal cual.
func TestDedupeIncludedMisalignedReturnsMapped(t *testing.T) {
	d := diagnostics.NewWarning("x", diagnostics.NewPosition(2, 1), "linter")
	d.File = "b.slidelang"
	mapped := []diagnostics.Diagnostic{d, d}
	for _, raw := range [][]diagnostics.Diagnostic{nil, mapped[:1], {d, d, d}} {
		if got := DedupeIncluded(raw, mapped); len(got) != len(mapped) {
			t.Errorf("raw of %d: got %d diagnostics, want mapped as is (%d)", len(raw), len(got), len(mapped))
		}
	}
}
