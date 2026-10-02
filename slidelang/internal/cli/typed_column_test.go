// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #373: la fuente del issue, con la columna tipada, compila con el ID
// ligado al elemento; con la columna cruda sigue fallando por node-id
// huérfano, como antes.
const typedColumnDeck = `---
mode: strict
---
SLIDE content
  title: "Grid"
  <<grid>>
  <<column typed>>
    <!-- node-id: ColTextA -->
    TEXT
      Left side.
  <<column>>
  Right side.
  <<end>>
`

func buildDeck(t *testing.T, source string) (string, error) {
	t.Helper()
	tmpDir := t.TempDir()
	input := filepath.Join(tmpDir, "deck.slidelang")
	if err := os.WriteFile(input, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmpDir, "dist")
	opts := &BuildOptions{InputFile: input, OutputDir: out, Format: "html,json", Mode: "auto", LogLevel: "error", NoColors: true}
	if err := runBuild(opts, nil, nil, nil, nil, nil); err != nil {
		return "", err
	}
	html, err := os.ReadFile(filepath.Join(out, "deck.html"))
	if err != nil {
		t.Fatal(err)
	}
	json, err := os.ReadFile(filepath.Join(out, "deck.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(html) + "\n" + string(json), nil
}

func TestRunBuild_TypedColumnWithNodeID(t *testing.T) {
	out, err := buildDeck(t, typedColumnDeck)
	if err != nil {
		t.Fatalf("runBuild: %v", err)
	}
	for _, want := range []string{"Left side.", "Right side.", `"nodeId": "ColTextA"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q", want)
		}
	}
}

func TestRunBuild_RawColumnStillRejectsNodeID(t *testing.T) {
	raw := strings.Replace(typedColumnDeck, "<<column typed>>\n    <!-- node-id: ColTextA -->\n    TEXT\n      Left side.", "<<column>>\n  <!-- node-id: ColTextA -->\n  TEXT\n    Left side.", 1)
	// runBuild imprime el diagnóstico ("orphan node-id ...") y devuelve un
	// error genérico de parseo: aquí basta con que el build falle.
	if _, err := buildDeck(t, raw); err == nil {
		t.Fatal("a node-id inside a raw column must still fail the build")
	}
}

// fmt escribe la columna tipada con su identidad y una segunda pasada no
// cambia un byte.
func TestRunFmt_TypedColumnRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	input := filepath.Join(tmpDir, "deck.slidelang")
	if err := os.WriteFile(input, []byte(typedColumnDeck), 0644); err != nil {
		t.Fatal(err)
	}
	opts := &FmtOptions{InputFile: input, Strict: true, Write: true}
	if err := runFmt(opts); err != nil {
		t.Fatalf("runFmt: %v", err)
	}
	first, _ := os.ReadFile(input)
	if !strings.Contains(string(first), "<<column typed>>") || !strings.Contains(string(first), "<!-- node-id: ColTextA -->") {
		t.Fatalf("fmt lost the typed column or its identity:\n%s", first)
	}
	if err := runFmt(opts); err != nil {
		t.Fatalf("second runFmt: %v", err)
	}
	second, _ := os.ReadFile(input)
	if string(first) != string(second) {
		t.Fatalf("fmt is not idempotent:\n%s\n---\n%s", first, second)
	}
}
