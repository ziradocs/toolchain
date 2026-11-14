// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lintIn llama al tool lint con args y devuelve su salida.
func lintIn(t *testing.T, args map[string]any) (lintOutput, bool) {
	t.Helper()
	session, ctx := newTestSession(t)
	res := callTool(t, session, ctx, "lint", args)
	var out lintOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out, res.IsError
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const includeDeck = "---\ntitle: T\nmode: flex\n---\n\n# S\n\n@include parts/x.slidelang\n\n# Fin\n\n<<charts>>\n"

// part tiene una línea que el parser no reconoce (FLEX001) en su línea 3.
const includePart = "## Parte\n\n<<charts>>\n"

// El tool lint expande los @include como `slidelang build`, desde el
// directorio del documento (fileName, relativo al directorio de trabajo del
// servidor), y el diagnóstico de un archivo incluido nombra ese archivo y su
// línea. Antes el @include no se expandía: el contenido del archivo incluido
// no se linteaba.
func TestLintTool_ExpandsIncludes(t *testing.T) {
	for _, tc := range []struct {
		name, fileName, partPath string
	}{
		{"document in the working directory", "deck.slidelang", "parts/x.slidelang"},
		{"document in a subdirectory", "decks/main.slidelang", "decks/parts/x.slidelang"},
		{"no fileName", "", "parts/x.slidelang"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			writeFile(t, filepath.Join(dir, tc.partPath), includePart)
			args := map[string]any{"source": includeDeck}
			if tc.fileName != "" {
				args["fileName"] = tc.fileName
			}
			out, isErr := lintIn(t, args)
			if isErr || out.Error != "" {
				t.Fatalf("lint failed: %+v", out)
			}
			got := map[string]int{}
			for _, d := range out.Diagnostics {
				if d.RuleID == "FLEX001" {
					got[d.File] = d.Position.Line
				}
			}
			want := map[string]int{"parts/x.slidelang": 3, "": 12}
			if len(got) != len(want) || got["parts/x.slidelang"] != 3 || got[""] != 12 {
				t.Errorf("want FLEX001 at %v, got %v (diagnostics: %+v)", want, got, out.Diagnostics)
			}
		})
	}
}

// Los @include no salen del directorio de trabajo del servidor: ni un
// fileName afuera de él ni un @include que escape del directorio del
// documento leen nada. Un documento sin @include se lintea igual con
// cualquier fileName.
func TestLintTool_IncludesStayInTheWorkingDirectory(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "work")
	writeFile(t, filepath.Join(base, "secret.slidelang"), "## Secreto\n\n<<charts>>\n")
	writeFile(t, filepath.Join(work, "keep"), "")
	t.Chdir(work)

	for _, tc := range []struct {
		name, fileName, source, wantErr string
	}{
		{"fileName outside", "../deck.slidelang", "---\ntitle: T\nmode: flex\n---\n\n# S\n\n@include secret.slidelang\n", "inside the server's working directory"},
		{"absolute fileName", filepath.Join(base, "deck.slidelang"), "---\ntitle: T\nmode: flex\n---\n\n# S\n\n@include secret.slidelang\n", "inside the server's working directory"},
		{"include escaping", "deck.slidelang", "---\ntitle: T\nmode: flex\n---\n\n# S\n\n@include ../secret.slidelang\n", "failed to expand includes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, isErr := lintIn(t, map[string]any{"source": tc.source, "fileName": tc.fileName})
			if !isErr || out.Valid || !strings.Contains(out.Error, tc.wantErr) {
				t.Fatalf("want an error result with %q, got isError=%v %+v", tc.wantErr, isErr, out)
			}
			if strings.Contains(out.Error, "Secreto") || len(out.Diagnostics) != 0 {
				t.Errorf("nothing of the file outside should show: %+v", out)
			}
		})
	}

	out, isErr := lintIn(t, map[string]any{"source": validSource, "fileName": filepath.Join(base, "deck.slidelang")})
	if isErr || !out.Valid {
		t.Errorf("a document without @include lints with any fileName: isError=%v %+v", isErr, out)
	}
}
