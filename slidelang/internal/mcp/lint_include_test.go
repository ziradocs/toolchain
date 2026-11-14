// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
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

// directive003 devuelve los DIRECTIVE003 de out.
func directive003(out lintOutput) []diagnostics.Diagnostic {
	var got []diagnostics.Diagnostic
	for _, d := range out.Diagnostics {
		if d.RuleID == "DIRECTIVE003" {
			got = append(got, d)
		}
	}
	return got
}

// Un @include que no se puede expandir (el archivo no existe, o queda fuera
// del directorio de trabajo del servidor) no corta el lint: el documento se
// lintea como está, con el DIRECTIVE003 de esa línea, el mismo Valid y la
// misma severidad que antes de que el tool expandiera. Nada del archivo de
// afuera se lee. El DIRECTIVE003 dice por qué no se expandió.
func TestLintTool_UnexpandableIncludeKeepsDIRECTIVE003(t *testing.T) {
	base := t.TempDir()
	work := filepath.Join(base, "work")
	writeFile(t, filepath.Join(base, "secret.slidelang"), "## Secreto\n\n<<charts>>\n")
	writeFile(t, filepath.Join(work, "parts/ok.slidelang"), "## Bien\n\ntexto\n")
	t.Chdir(work)
	const head = "---\ntitle: T\nmode: flex\n---\n\n# S\n\ntexto\n\n"

	for _, tc := range []struct {
		name, fileName, include, why string
	}{
		{"not found", "deck.slidelang", "parts/missing.slidelang", `no se expandió "parts/missing.slidelang": el archivo no existe`},
		{"escapes the document's directory", "deck.slidelang", "../secret.slidelang", `no se expandió "../secret.slidelang": queda fuera del directorio de trabajo del servidor`},
		{"fileName outside the working directory", "../deck.slidelang", "secret.slidelang", `no se expandió "secret.slidelang": queda fuera del directorio de trabajo del servidor`},
		{"absolute fileName", filepath.Join(base, "deck.slidelang"), "secret.slidelang", `no se expandió "secret.slidelang": queda fuera del directorio de trabajo del servidor`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, isErr := lintIn(t, map[string]any{"source": head + "@include " + tc.include + "\n", "fileName": tc.fileName})
			// Lo mismo que en la base: resultado válido, un DIRECTIVE003
			// de aviso en la línea del @include.
			got := directive003(out)
			if isErr || out.Error != "" || !out.Valid {
				t.Fatalf("want a valid lint result, got isError=%v %+v", isErr, out)
			}
			if len(got) != 1 || !got[0].IsWarning() || got[0].Position.Line != 10 || got[0].File != "" {
				t.Fatalf("want one DIRECTIVE003 warning on line 10, got %+v", out.Diagnostics)
			}
			for _, d := range out.Diagnostics {
				if strings.Contains(d.Message, "Secreto") || d.RuleID == "FLEX001" {
					t.Errorf("nothing of the file outside should be read: %+v", d)
				}
			}
			// Lo nuevo: el aviso dice por qué no se expandió.
			if !strings.HasSuffix(got[0].Message, tc.why) {
				t.Errorf("DIRECTIVE003 should say %q: %q", tc.why, got[0].Message)
			}
		})
	}

	// Un @include que se expande y otro que no, en el mismo documento: el
	// primero se expande y el segundo queda con su DIRECTIVE003.
	out, isErr := lintIn(t, map[string]any{"source": head + "@include parts/ok.slidelang\n\n@include parts/missing.slidelang\n", "fileName": "deck.slidelang"})
	got := directive003(out)
	if isErr || !out.Valid || len(got) != 1 || got[0].Position.Line != 12 {
		t.Errorf("want the missing include alone reported on line 12, got isError=%v %+v", isErr, out)
	}
}
