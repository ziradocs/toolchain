// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/report"
)

// spacedImagesSource tiene una ruta suelta con espacios en una imagen de
// bloque (línea 9, el aviso lo da el parser), en una imagen inline (línea 11,
// lo da el linter) y en una imagen de bloque dentro de un :::note (línea 14:
// el parser la ve como imagen y el linter, desde el texto del bloque, en la
// línea 13 del ":::").
const spacedImagesSource = `---
title: Imagenes
mode: flex
%s---
# T

Texto.

![a](sub dir/a.png)

Inline ![b](sub dir/b.png) aquí.

:::note
![c](sub dir/c.png)
:::
`

func buildSpacedImagesReport(t *testing.T, frontmatter string) report.ReportJSON {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "d.slidelang")
	src := strings.Replace(spacedImagesSource, "%s", frontmatter, 1)
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "report.json")
	if err := runCLI(t, Options{}, []string{"build", in, "--lint-only", "--report", "json", "--report-out", out}); err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var rep report.ReportJSON
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("report: %v", err)
	}
	return rep
}

func link001Lines(rep report.ReportJSON) []int {
	var lines []int
	for _, f := range rep.Findings {
		if f.RuleID == "LINK001" {
			lines = append(lines, f.Position.Line)
		}
	}
	sort.Ints(lines)
	return lines
}

// El LINK001 de una imagen de bloque lo detecta el parser, y antes solo salía
// por stderr: el reporte (--report json/sarif), que es lo que leen los
// editores, no lo traía. Ahora va con los hallazgos del linter, en la línea
// de la imagen, y una sola vez aunque el linter ya avise de la misma imagen
// desde el texto del :::note.
func TestE2E_BlockImageLink001InReport(t *testing.T) {
	rep := buildSpacedImagesReport(t, "")
	got := link001Lines(rep)
	want := []int{9, 11, 14}
	if len(got) != len(want) {
		t.Fatalf("LINK001 lines = %v, want %v (findings: %+v)", got, want, rep.Findings)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("LINK001 lines = %v, want %v", got, want)
		}
	}
}

// El hallazgo del parser pasa por la misma política que los del linter: un
// waiver de LINK001 lo suprime igual que al de una imagen inline.
func TestE2E_BlockImageLink001IsWaivable(t *testing.T) {
	expires := time.Now().AddDate(1, 0, 0).Format(time.RFC3339)
	policy := "lint_policy:\n  rules:\n    LINK001:\n      expires_at: \"" + expires + "\"\n      reason: \"rutas heredadas\"\n      approved_by: \"toolchain-e2e\"\n"
	rep := buildSpacedImagesReport(t, policy)
	if got := link001Lines(rep); len(got) != 3 {
		t.Fatalf("LINK001 lines = %v, want 3 findings", got)
	}
	for _, f := range rep.Findings {
		if f.RuleID == "LINK001" && !f.Waived {
			t.Errorf("LINK001 at line %d is not waived", f.Position.Line)
		}
	}
}

// La deduplicación solo junta el aviso del linter sobre el texto del
// contenedor de la imagen: la misma ruta en una imagen inline de otro párrafo
// es otro hallazgo y se queda.
func TestE2E_BlockImageLink001KeepsSameDestinationElsewhere(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "d.slidelang")
	src := "---\ntitle: T\nmode: flex\n---\n# T\n\nInline ![a](sub dir/a.png) aquí.\n\n![a](sub dir/a.png)\n"
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "report.json")
	if err := runCLI(t, Options{}, []string{"build", in, "--lint-only", "--report", "json", "--report-out", out}); err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var rep report.ReportJSON
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatal(err)
	}
	if got := link001Lines(rep); len(got) != 2 || got[0] != 7 || got[1] != 9 {
		t.Fatalf("LINK001 lines = %v, want [7 9]", got)
	}
}

// Una imagen dentro de un :::note que está dentro de un :::warning: el
// linter avisa desde el texto de cada contenedor (líneas 7 y 8) y el parser
// en la línea de la imagen (9). Queda solo el del parser. Y los hallazgos
// del reporte van en orden de línea, con el del parser en su lugar.
func TestE2E_BlockImageLink001NestedContainersAndOrder(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "d.slidelang")
	src := "---\ntitle: T\nmode: flex\n---\n# T\n\n:::warning\n:::note\n![nn](sub dir/nn.png)\n:::\n:::\n\nVer ![z](sub dir/z.png) fin.\n"
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "report.json")
	if err := runCLI(t, Options{}, []string{"build", in, "--lint-only", "--report", "json", "--report-out", out}); err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var rep report.ReportJSON
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatal(err)
	}
	var lines []int
	for _, f := range rep.Findings {
		if f.RuleID == "LINK001" {
			lines = append(lines, f.Position.Line)
		}
	}
	if len(lines) != 2 || lines[0] != 9 || lines[1] != 13 {
		t.Fatalf("LINK001 lines (en el orden del reporte) = %v, want [9 13]", lines)
	}
}
