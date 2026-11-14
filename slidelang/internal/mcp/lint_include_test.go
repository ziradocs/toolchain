// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// El tool lint no expande @include ni lee ningún archivo: el contenido de un
// tool call no es confiable, y una línea de un archivo leído saldría citada
// en un diagnóstico (en strict, todas). Un @include, exista o no el archivo
// en el directorio de trabajo del servidor, da el DIRECTIVE003 de siempre y
// nada del archivo llega al resultado. Mismo resultado que sin el archivo.
func TestLintTool_ReadsNoIncludedFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	const secret = "API_KEY=s3cr3t-valor"
	for name, content := range map[string]string{
		".env":              secret + "\n<<chart>>\n| a | b |\n@" + secret + "\n",
		"parts/x.slidelang": "## Parte\n\n" + secret + "\n<<charts>>\n",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	type result struct {
		valid bool
		rules []string
	}
	lint := func(t *testing.T, source, fileName string) (result, string) {
		t.Helper()
		session, ctx := newTestSession(t)
		args := map[string]any{"source": source}
		if fileName != "" {
			args["fileName"] = fileName
		}
		res := callTool(t, session, ctx, "lint", args)
		text := resultText(t, res)
		var out lintOutput
		if err := json.Unmarshal([]byte(text), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if res.IsError || out.Error != "" {
			t.Fatalf("lint failed: %s", text)
		}
		r := result{valid: out.Valid}
		for _, d := range out.Diagnostics {
			r.rules = append(r.rules, d.RuleID)
			if d.RuleID == "DIRECTIVE003" && d.Message != "La directiva @include no tiene consumidor y fue ignorada" {
				t.Errorf("DIRECTIVE003 keeps its text: %q", d.Message)
			}
		}
		sort.Strings(r.rules)
		return r, text
	}

	for _, tc := range []struct {
		name, source string
	}{
		{"strict", "---\ntitle: T\nmode: strict\n---\n\nSLIDE \"S\"\n  @include .env\n"},
		{"flex", "---\ntitle: T\nmode: flex\n---\n\n# S\n\ntexto\n\n@include .env\n\n@include parts/x.slidelang\n"},
	} {
		for _, fileName := range []string{"", "deck.slidelang"} {
			t.Run(tc.name+"/"+fileName, func(t *testing.T) {
				got, text := lint(t, tc.source, fileName)
				if strings.Contains(text, "s3cr3t") || strings.Contains(text, "API_KEY") || strings.Contains(text, "Parte") {
					t.Fatalf("the result shows the content of a file: %s", text)
				}
				if !strings.Contains(strings.Join(got.rules, ","), "DIRECTIVE003") {
					t.Errorf("want DIRECTIVE003 for the @include, got %v", got.rules)
				}
				// Lo mismo que si el archivo no existiera: no se leyó.
				missing := strings.NewReplacer(".env", "no-such.env", "parts/x.slidelang", "parts/none.slidelang").Replace(tc.source)
				want, _ := lint(t, missing, fileName)
				if got.valid != want.valid || strings.Join(got.rules, ",") != strings.Join(want.rules, ",") {
					t.Errorf("an existing file changed the result: %+v, without it %+v", got, want)
				}
			})
		}
	}
}
